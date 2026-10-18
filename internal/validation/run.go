package validation

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/worker"
)

var ErrActiveChild = errors.New("validation child still owns the workspace; inspect or stop the exact process")
var ErrUnknownChild = errors.New("validation child outcome is unknown; inspect retained output before explicit recovery")

type Result struct {
	Passed    bool
	Repeated  bool
	Failure   string
	ReceiptID string
	Candidate string
}
type Runner struct {
	DB                              *store.Store
	StateDir, Workspace, Executable string
	Lock                            *os.File
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "check_" + hex.EncodeToString(b), nil
}
func (r Runner) Run(ctx context.Context, a store.Assignment, schedule store.Schedule) (Result, error) {
	var policy struct {
		CheckProfile Profile `json:"check_profile"`
	}
	if err := json.Unmarshal([]byte(schedule.PolicyJSON), &policy); err != nil {
		return Result{}, err
	}
	raw, _ := json.Marshal(policy.CheckProfile)
	profile, err := Parse(raw)
	if err != nil {
		return Result{}, fmt.Errorf("operator check profile unavailable: %w", err)
	}
	var pinned struct {
		Requirements struct {
			Outcome  string   `json:"outcome"`
			Criteria []string `json:"criteria"`
			Notes    string   `json:"notes"`
			Settings struct {
				CheckProfile string `json:"check_profile"`
			} `json:"settings"`
		} `json:"requirements"`
	}
	if err := json.Unmarshal([]byte(a.SnapshotJSON), &pinned); err != nil {
		return Result{}, err
	}
	if selected := pinned.Requirements.Settings.CheckProfile; selected != "" && selected != "inherit" && selected != profile.ID {
		return Result{}, fmt.Errorf("issue selected unconfigured check profile %q", selected)
	}
	var attemptID string
	var phase string
	if err := r.DB.DB.QueryRowContext(ctx, `SELECT id,phase FROM assignment_attempts WHERE assignment_id=? AND outcome='running' ORDER BY ordinal DESC LIMIT 1`, a.ID).Scan(&attemptID, &phase); err != nil {
		return Result{}, err
	}
	if phase != "validating" {
		return Result{}, fmt.Errorf("assignment is in %s, not validation", phase)
	}
	// Reconcile every owned child before preparation can write the checkout.
	prior, err := r.DB.ListChecks(ctx, a.ID)
	if err != nil {
		return Result{}, err
	}
	for _, receipt := range prior {
		if receipt.Status == "unknown" {
			return Result{}, fmt.Errorf("%w: receipt %s", ErrUnknownChild, receipt.ID)
		}
		if receipt.Status == "pending" || receipt.Status == "running" {
			if _, err := r.reconcile(ctx, receipt); err != nil {
				return Result{}, err
			}
		}
	}
	criteria, _ := json.Marshal(pinned.Requirements.Criteria)
	var directions []string
	rows, err := r.DB.DB.QueryContext(ctx, `SELECT body FROM work_messages WHERE assignment_id=? AND state='accepted' AND contract_changed=1 ORDER BY created_at,id`, a.ID)
	if err != nil {
		return Result{}, err
	}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			break
		}
		directions = append(directions, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return Result{}, err
	}
	directionJSON, _ := json.Marshal(directions)
	contract := Digest(pinned.Requirements.Outcome, string(criteria), pinned.Requirements.Notes, schedule.Instructions, string(directionJSON), string(raw))
	// Preparation may write generated source. Its output is intentionally not
	// cached; a no-op prepare leaves the content identity unchanged.
	for _, c := range profile.Prepare {
		receipt, err := r.execute(ctx, a, attemptID, 0, c, contract, "prepare", "", "Preparation step", nil)
		if err != nil {
			return Result{}, err
		}
		if receipt.Status != "passed" {
			return Result{Failure: failureText(receipt)}, nil
		}
	}
	candidate, err := CandidateIdentity(ctx, r.Workspace, nil)
	if err != nil {
		return Result{}, err
	}
	var final string
	for _, c := range profile.Checks {
		input, err := CandidateIdentity(ctx, r.Workspace, c.SupplementalInputs)
		if err != nil {
			return Result{}, err
		}
		command, _ := json.Marshal(c)
		env := selectedEnv(c.EnvKeys)
		envJSON, _ := json.Marshal(env)
		probeOutput, probeErr := probes(ctx, r.Workspace, c.ToolchainProbes)
		if probeErr != nil {
			return Result{}, probeErr
		}
		key := Digest(input, contract, string(command), string(envJSON), probeOutput)
		previous, err := r.DB.LatestCheck(ctx, a.ID, c.ID)
		generation := 1
		reason := "First runner execution"
		if err == nil {
			generation = previous.Generation + 1
			if previous.ContractDigest != contract {
				reason = "Validation contract changed"
			} else if previous.InputDigest != key {
				reason = "Relevant inputs, environment or toolchain changed"
			} else {
				reason = "Earlier result was not reusable"
			}
			if previous.Status == "unknown" {
				return Result{}, fmt.Errorf("%w: receipt %s", ErrUnknownChild, previous.ID)
			}
			if previous.Status == "pending" || previous.Status == "running" {
				previous, err = r.reconcile(ctx, previous)
				if err != nil {
					return Result{}, err
				}
			}
			if previous.InputDigest == key && previous.ContractDigest == contract && (previous.Status == "passed" || previous.Status == "reused") {
				id, e := newID()
				if e != nil {
					return Result{}, e
				}
				if e = r.DB.ReuseCheck(ctx, previous, id, attemptID); e != nil {
					return Result{}, e
				}
				if c.Required {
					final = id
				}
				continue
			}
			if previous.InputDigest == key && previous.ContractDigest == contract && previous.Status == "failed" && c.Required {
				return Result{Failure: failureText(previous), ReceiptID: previous.ID, Candidate: candidate, Repeated: true}, nil
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Result{}, err
		}
		receipt, err := r.execute(ctx, a, attemptID, generation, c, contract, key, candidate, reason, env)
		if err != nil {
			return Result{}, err
		}
		if c.Required && receipt.Status == "passed" {
			final = receipt.ID
		}
		if receipt.Status != "passed" && c.Required {
			return Result{Failure: failureText(receipt), ReceiptID: receipt.ID, Candidate: candidate}, nil
		}
	}
	after, err := CandidateIdentity(ctx, r.Workspace, nil)
	if err != nil {
		return Result{}, err
	}
	if after != candidate {
		return Result{}, fmt.Errorf("checks changed candidate files; repeat final validation after writes settle")
	}
	return Result{Passed: true, ReceiptID: final, Candidate: candidate}, nil
}
func selectedEnv(keys []string) map[string]string {
	out := map[string]string{}
	for _, k := range keys {
		out[k] = os.Getenv(k)
	}
	return out
}

type probeBuffer struct{ bytes.Buffer }

func (b *probeBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 64<<10 {
		return 0, fmt.Errorf("toolchain probe output exceeds 64 KiB")
	}
	return b.Buffer.Write(p)
}
func probes(ctx context.Context, workspace string, vectors [][]string) (string, error) {
	var results []string
	for _, argv := range vectors {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cmd := exec.CommandContext(probeCtx, argv[0], argv[1:]...)
		cmd.Dir = workspace
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "LANG=C", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"}
		output := &probeBuffer{}
		cmd.Stdout = output
		cmd.Stderr = output
		err := cmd.Run()
		cancel()
		if err != nil {
			return "", fmt.Errorf("toolchain probe %q failed: %w", argv[0], err)
		}
		results = append(results, output.String())
	}
	b, _ := json.Marshal(results)
	return string(b), nil
}
func failureText(c store.CheckReceipt) string {
	return fmt.Sprintf("check %s %s: %s (receipt %s; inspect retained output)", c.CheckID, c.Status, c.Reason, c.ID)
}
func (r Runner) execute(ctx context.Context, a store.Assignment, attemptID string, generation int, c Check, contract, input, candidate, reason string, env map[string]string) (store.CheckReceipt, error) {
	id, err := newID()
	if err != nil {
		return store.CheckReceipt{}, err
	}
	dir := filepath.Join(r.StateDir, "checks", a.RunID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return store.CheckReceipt{}, err
	}
	output := filepath.Join(dir, id+".log")
	terminal := filepath.Join(dir, id+".terminal.json")
	specPath := filepath.Join(dir, id+".spec.json")
	command, _ := json.Marshal(c)
	envJSON, _ := json.Marshal(env)
	receipt := store.CheckReceipt{ID: id, AssignmentID: a.ID, AttemptID: attemptID, Generation: generation, CheckID: c.ID, Name: c.Name, Required: c.Required, CommandJSON: string(command), EnvJSON: string(envJSON), ContractDigest: contract, InputDigest: input, CandidateDigest: candidate, Reason: reason, RequireCases: c.RequireCases, OutputPath: output, TerminalPath: terminal}
	if err := r.DB.BeginCheck(ctx, receipt); err != nil {
		return receipt, err
	}
	spec := ChildSpec{ID: id, ContractDigest: contract, InputDigest: input, Workspace: r.Workspace, Check: c, OutputPath: output, TerminalPath: terminal, Env: env}
	b, _ := json.Marshal(spec)
	if err := os.WriteFile(specPath, b, 0600); err != nil {
		return receipt, err
	}
	cmd := exec.Command(r.Executable, "check-child", "--spec", specPath)
	cmd.ExtraFiles = []*os.File{r.Lock}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return receipt, err
	}
	ticks, err := worker.StartTicks(cmd.Process.Pid)
	if err == nil {
		if err := r.DB.StartCheck(ctx, id, cmd.Process.Pid, ticks); err != nil {
			return receipt, err
		}
	}
	// Wait can be interrupted with the worker; the child retains the inherited
	// lock and writes its terminal receipt independently.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-ctx.Done():
		return receipt, ErrActiveChild
	}
	return r.reconcile(ctx, receipt)
}
func (r Runner) reconcile(ctx context.Context, c store.CheckReceipt) (store.CheckReceipt, error) {
	terminal, err := ReadTerminal(c.TerminalPath, c.ID, c.ContractDigest, c.InputDigest)
	if err != nil {
		if c.PID > 0 {
			ticks, e := worker.StartTicks(c.PID)
			if e == nil && ticks == c.StartTicks {
				return c, fmt.Errorf("%w: PID %d", ErrActiveChild, c.PID)
			}
		}
		_ = r.DB.UnknownCheck(ctx, c.ID, "terminal receipt missing after child exit")
		return c, fmt.Errorf("%w: receipt %s, PID %d", ErrUnknownChild, c.ID, c.PID)
	}
	c.FinishedAt = terminal.FinishedAt
	c.DurationMS = terminal.DurationMS
	c.Signal = terminal.Signal
	c.OutputBytes = terminal.OutputBytes
	c.Truncated = terminal.Truncated
	initialReason := c.Reason
	c.Reason = terminal.Reason
	if terminal.ExitCode != nil {
		c.ExitCode = sql.NullInt64{Int64: int64(*terminal.ExitCode), Valid: true}
	}
	if terminal.CaseCount != nil {
		c.CaseCount = sql.NullInt64{Int64: int64(*terminal.CaseCount), Valid: true}
	}
	c.Status = "passed"
	if terminal.ExitCode == nil || *terminal.ExitCode != 0 || terminal.Signal != "" || terminal.Reason != "" {
		c.Status = "failed"
		if c.Reason == "" {
			c.Reason = "exit " + strconv.Itoa(value(terminal.ExitCode)) + " signal " + terminal.Signal
		}
	}
	if c.RequireCases && (terminal.CaseCount == nil || *terminal.CaseCount == 0) {
		c.Status = "failed"
		c.Reason = "zero or unknown selected cases"
	}
	if c.Status == "passed" {
		c.Reason = initialReason
	} else if initialReason != "" {
		c.Reason = initialReason + "; " + c.Reason
	}
	if err := r.DB.FinishCheck(ctx, c); err != nil {
		return c, err
	}
	return c, nil
}
func value(n *int) int {
	if n == nil {
		return -1
	}
	return *n
}
func (r Runner) FailureEvidence(ctx context.Context, id string) (string, error) {
	rows, err := r.DB.ListChecks(ctx, id)
	if err != nil {
		return "", err
	}
	for i := len(rows) - 1; i >= 0; i-- {
		c := rows[i]
		if c.Status != "failed" {
			continue
		}
		b, err := os.ReadFile(c.OutputPath)
		if err != nil {
			return failureText(c), nil
		}
		if len(b) > 8<<10 {
			b = b[len(b)-(8<<10):]
		}
		return strings.TrimSpace(failureText(c) + "\n" + string(b)), nil
	}
	return "", nil
}
