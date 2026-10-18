package codexadapter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
