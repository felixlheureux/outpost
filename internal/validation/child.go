package validation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

type ChildSpec struct {
	ID             string            `json:"id"`
	ContractDigest string            `json:"contract_digest"`
	InputDigest    string            `json:"input_digest"`
	Workspace      string            `json:"workspace"`
	Check          Check             `json:"check"`
	OutputPath     string            `json:"output_path"`
	TerminalPath   string            `json:"terminal_path"`
	Env            map[string]string `json:"env"`
}
type Terminal struct {
	ID             string `json:"id"`
	ContractDigest string `json:"contract_digest"`
	InputDigest    string `json:"input_digest"`
	ExitCode       *int   `json:"exit_code"`
	Signal         string `json:"signal"`
	CaseCount      *int   `json:"case_count"`
	DurationMS     int64  `json:"duration_ms"`
	FinishedAt     string `json:"finished_at"`
	OutputBytes    int64  `json:"output_bytes"`
	Truncated      bool   `json:"truncated"`
	Reason         string `json:"reason"`
}
type boundedWriter struct {
	f         *os.File
	n         int64
	truncated bool
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	n := len(p)
	remain := int64(1<<20) - w.n
	if remain > 0 {
		if int64(n) < remain {
			remain = int64(n)
		}
		written, err := w.f.Write(p[:int(remain)])
		w.n += int64(written)
		if err != nil {
			return n, err
		}
	}
	if int64(n) > remain {
		w.truncated = true
	}
	return n, nil
}

// Child runs separately from the worker and inherits the workspace lock. It
// writes an atomic terminal file before exiting, so supervisor loss after a
// check result does not force a duplicate run.
func Child(specPath string) error {
	b, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	if len(b) > 32<<10 {
		return fmt.Errorf("check child spec too large")
	}
	var spec ChildSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return err
	}
	if spec.ID == "" || !filepath.IsAbs(spec.Workspace) || !filepath.IsAbs(spec.OutputPath) || !filepath.IsAbs(spec.TerminalPath) || len(spec.Check.Argv) == 0 {
		return fmt.Errorf("invalid check child spec")
	}
	output, err := os.OpenFile(spec.OutputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	w := &boundedWriter{f: output}
	started := time.Now()
	cwd := spec.Check.Cwd
	if cwd == "" {
		cwd = "."
	}
	dir := filepath.Join(spec.Workspace, cwd)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(spec.Workspace, resolved)
	if err != nil || !safeRelative(rel) {
		return fmt.Errorf("check cwd escapes workspace")
	}
	cmd := exec.Command(spec.Check.Argv[0], spec.Check.Argv[1:]...)
	cmd.Dir = resolved
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "LANG=C", "LC_ALL=C", "CI=1", "GIT_TERMINAL_PROMPT=0"}
	for k, v := range spec.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	// FD 3 is the inherited workspace flock. The command also inherits it, so
	// it retains exclusion if this receipt-writing child is killed first.
	if lock := os.NewFile(3, "workspace-lock"); lock != nil {
		cmd.ExtraFiles = []*os.File{lock}
	}
	terminal := Terminal{ID: spec.ID, ContractDigest: spec.ContractDigest, InputDigest: spec.InputDigest}
	if err := cmd.Start(); err != nil {
		terminal.Reason = err.Error()
	} else {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		limit := time.Duration(spec.Check.TimeoutSeconds) * time.Second
		if limit == 0 {
			limit = 30 * time.Minute
		}
		timer := time.NewTimer(limit)
		select {
		case err = <-done:
		case <-timer.C:
			terminal.Reason = "check deadline exceeded"
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			err = <-done
		}
		timer.Stop()
		if cmd.ProcessState != nil {
			if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
				if status.Exited() {
					code := status.ExitStatus()
					terminal.ExitCode = &code
				}
				if status.Signaled() {
					terminal.Signal = status.Signal().String()
				}
			}
		}
		if err != nil && terminal.Reason == "" && terminal.ExitCode == nil && terminal.Signal == "" {
			terminal.Reason = err.Error()
		}
	}
	if err := output.Sync(); err != nil {
		return err
	}
	terminal.DurationMS = time.Since(started).Milliseconds()
	terminal.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	terminal.OutputBytes = w.n
	terminal.Truncated = w.truncated
	if spec.Check.RequireCases {
		if _, err := output.Seek(0, io.SeekStart); err == nil { /* file opened write-only; read below */
		}
		data, err := os.ReadFile(spec.OutputPath)
		if err != nil {
			return err
		}
		r := regexp.MustCompile(spec.Check.CaseCountPattern)
		m := r.FindAllStringSubmatch(string(data), -1)
		if len(m) > 0 {
			n, e := strconv.Atoi(m[len(m)-1][1])
			if e == nil {
				terminal.CaseCount = &n
			}
		}
	}
	return WriteTerminal(spec.TerminalPath, terminal)
}
func WriteTerminal(path string, t Terminal) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func ReadTerminal(path, id, contract, input string) (Terminal, error) {
	var t Terminal
	b, err := os.ReadFile(path)
	if err != nil {
		return t, err
	}
	if err = json.Unmarshal(b, &t); err != nil {
		return t, err
	}
	if t.ID != id || t.ContractDigest != contract || t.InputDigest != input || t.FinishedAt == "" {
		return t, fmt.Errorf("terminal receipt identity mismatch")
	}
	return t, nil
}
