package providerwork

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/felixlheureux/outpost/internal/codexadapter"
	"github.com/felixlheureux/outpost/internal/store"
)

// Opt-in acceptance: the real provider changes only a disposable repository.
func TestRealOwnedProviderHandoff(t *testing.T) {
	if os.Getenv("OUTPOST_REAL_CODEX_TEST") != "1" {
		t.Skip("set OUTPOST_REAL_CODEX_TEST=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	executable, err := codexadapter.FindExecutable()
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "repository")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("git", "-C", workspace, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	cap, err := codexadapter.Probe(ctx, executable, workspace)
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
		t.Fatal("no reasoning effort")
	}
	state := filepath.Join(t.TempDir(), "state")
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(map[string]any{"issue_number": 8, "workspace_path": workspace, "title": "Create a disposable smoke file", "body": "Create smoke.txt containing exactly outpost owned provider smoke followed by a newline. Do not run other commands."})
	statements := []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','key-12345678',?, 'token','unit','running','acknowledged','now','now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',8,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',8,1,'running',?,'now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'implementing','running','now')`,
	}
	for i, statement := range statements {
		var args []any
		if i == 1 {
			args = []any{workspace}
		}
		if i == 3 {
			args = []any{string(snapshot)}
		}
		if _, err := db.DB.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	policy, _ := json.Marshal(map[string]any{"agent": map[string]string{"model": model.ID, "reasoning": effort, "sandbox": "workspace-write", "approval_policy": "on-request"}})
	result, err := Execute(ctx, db, state, executable, store.Assignment{ID: "a", SnapshotJSON: string(snapshot)}, store.Schedule{PolicyJSON: string(policy)}, workspace)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(workspace, "smoke.txt"))
	if err != nil || string(b) != "outpost owned provider smoke\n" {
		t.Fatalf("provider file %q: %v", b, err)
	}
	receipt, err := db.GetProviderAttempt(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != "completed" || receipt.ThreadID != result.ThreadID || receipt.TurnID != result.TurnID || receipt.Prompt == "" {
		t.Fatalf("provider receipt incomplete: %+v", receipt)
	}
	artifact, err := db.GetActivityArtifact(ctx, "artifact_a")
	if err != nil || artifact.BytesWritten == 0 || artifact.Truncated {
		t.Fatalf("real provider artifact: %+v %v", artifact, err)
	}
	if info, err := os.Stat(filepath.Join(state, artifact.RelativePath)); err != nil || info.Size() != artifact.BytesWritten {
		t.Fatalf("retained provider file: %v %v", info, err)
	}
	activity, err := db.ReplayActivity(ctx, "r", 0)
	if err != nil {
		t.Fatal(err)
	}
	visible := false
	for _, event := range activity {
		if event.Kind == "activity.appended" && event.ArtifactID == artifact.ID {
			visible = true
		}
	}
	if !visible {
		t.Fatal("real provider output did not produce a retained activity reference")
	}
	var phase, assignmentState string
	if err := db.DB.QueryRowContext(ctx, `SELECT phase FROM assignment_attempts WHERE assignment_id='a'`).Scan(&phase); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT state FROM assignments WHERE id='a'`).Scan(&assignmentState); err != nil {
		t.Fatal(err)
	}
	if phase != "validating" || assignmentState != "running" {
		t.Fatalf("provider claim settled assignment: phase=%s state=%s", phase, assignmentState)
	}
	t.Logf("real owned provider %s, model %s, effort %s, thread %s, turn %s, paths %v", receipt.Version, receipt.Model, receipt.Reasoning, receipt.ThreadID, receipt.TurnID, result.Handoff.ChangedPaths)
}
