package codexadapter

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Opt in only: this performs a real authenticated model turn in a disposable repo.
func TestRealAuthenticatedTurn(t *testing.T) {
	if os.Getenv("OUTPOST_REAL_CODEX_TEST") != "1" {
		t.Skip("set OUTPOST_REAL_CODEX_TEST=1 for the authenticated provider smoke")
	}
	executable, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if b, err := exec.Command("git", "-C", workspace, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cap, err := Probe(ctx, executable, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(cap.Models) == 0 {
		t.Fatal("no discovered models")
	}
	model := cap.Models[0]
	effort := model.DefaultReasoningEffort
	if effort == "" && len(model.ReasoningEfforts) > 0 {
		effort = model.ReasoningEfforts[0].Effort
	}
	if effort == "" {
		t.Fatal("no discovered reasoning effort")
	}
	c, err := Start(ctx, executable, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	thread, err := c.Thread(ctx, Settings{Workspace: workspace, Model: model.ID, Reasoning: effort, Sandbox: "workspace-write", ApprovalPolicy: "on-request"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := c.StartTurn(ctx, thread, "Create smoke.txt containing exactly outpost provider smoke, followed by a newline. Then return a structured handoff with its changed path. Do not run other commands.", effort, HandoffSchema)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.WaitTurn(ctx, thread, turn, nil)
	if err != nil {
		t.Fatalf("thread %s turn %s: %v", thread, turn, err)
	}
	b, err := os.ReadFile(filepath.Join(workspace, "smoke.txt"))
	if err != nil || string(b) != "outpost provider smoke\n" {
		t.Fatalf("thread %s turn %s: file %q, %v", thread, turn, b, err)
	}
	t.Logf("real Codex %s, model %s, effort %s, thread %s, turn %s, status %s, handoff paths %v", cap.Version, model.ID, effort, thread, turn, result.Status, result.Handoff.ChangedPaths)
}

// Separate opt in: exercises acknowledgements from a signed-in Codex account.
// The workspace is disposable and cannot affect a managed Outpost checkout.
func TestRealSteerAndInterrupt(t *testing.T) {
	if os.Getenv("OUTPOST_REAL_CODEX_CONTROL_TEST") != "1" {
		t.Skip("set OUTPOST_REAL_CODEX_CONTROL_TEST=1 for live control acceptance")
	}
	executable, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if b, err := exec.Command("git", "-C", workspace, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cap, err := Probe(ctx, executable, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(cap.Models) == 0 {
		t.Fatal("no model")
	}
	model := cap.Models[0]
	effort := model.DefaultReasoningEffort
	if effort == "" && len(model.ReasoningEfforts) > 0 {
		effort = model.ReasoningEfforts[0].Effort
	}
	if effort == "" {
		t.Fatal("no reasoning effort")
	}
	c, err := Start(ctx, executable, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	thread, err := c.Thread(ctx, Settings{Workspace: workspace, Model: model.ID, Reasoning: effort, Sandbox: "workspace-write", ApprovalPolicy: "on-request"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := c.StartTurn(ctx, thread, "Create base.txt containing the word base. Return a structured handoff.", effort, HandoffSchema)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Steer(ctx, thread, turn, "Also create steered.txt containing the word steered. Include both paths in the handoff."); err != nil {
		t.Fatalf("real steer acknowledgement: %v", err)
	}
	if _, err := c.WaitTurn(ctx, thread, turn, nil); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "steered.txt")); err != nil || strings.TrimSpace(string(b)) != "steered" {
		t.Fatalf("steered work: %q %v", b, err)
	}
	turn, err = c.StartTurn(ctx, thread, "First run the shell command `sleep 30` so the operator can exercise interruption during an owned command. After it completes, create long-note.txt and return a structured handoff.", effort, HandoffSchema)
	if err != nil {
		t.Fatal(err)
	}
	started := false
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for !started {
		select {
		case event := <-c.Events():
			if event.Method != "item/started" {
				continue
			}
			var v struct {
				Item struct {
					Type string `json:"type"`
				} `json:"item"`
			}
			if json.Unmarshal(event.Params, &v) == nil && v.Item.Type == "commandExecution" {
				started = true
			}
		case <-deadline.C:
			t.Fatal("real provider did not start a command to interrupt")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if err := c.Interrupt(ctx, thread, turn); err != nil {
		t.Fatalf("real interrupt acknowledgement: %v", err)
	}
	result, err := c.WaitTurn(ctx, thread, turn, nil)
	if err == nil || result.Status == "completed" {
		t.Fatalf("interrupted turn reported completed: %+v %v", result, err)
	}
	pgid := c.cmd.Process.Pid
	_ = c.Close()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if err := syscall.Kill(-pgid, 0); err == syscall.ESRCH {
			break
		}
	}
	if err := syscall.Kill(-pgid, 0); err != syscall.ESRCH {
		t.Fatalf("owned process group survived interruption: %v", err)
	}
	continued, err := Start(ctx, executable, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer continued.Close()
	if err := continued.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	resumed, err := continued.Thread(ctx, Settings{Workspace: workspace, Model: model.ID, Reasoning: effort, Sandbox: "workspace-write", ApprovalPolicy: "on-request", ThreadID: thread})
	if err != nil {
		t.Fatalf("saved-session resume: %v", err)
	}
	if resumed != thread {
		t.Fatalf("resumed session identity changed: %q", resumed)
	}
	resumeTurn, err := continued.StartTurn(ctx, resumed, "Create resumed.txt containing resumed. Return a structured handoff with that path.", effort, HandoffSchema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := continued.WaitTurn(ctx, resumed, resumeTurn, nil); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "resumed.txt")); err != nil || strings.TrimSpace(string(b)) != "resumed" {
		t.Fatalf("resumed work: %q %v", b, err)
	}
	t.Logf("real Codex control receipts: %s model %s effort %s thread %s steer turn %s interruption status %s", cap.Version, model.ID, effort, thread, turn, result.Status)
}
