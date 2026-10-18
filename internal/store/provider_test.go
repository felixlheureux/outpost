package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestProviderReceiptKeepsPinnedIntentAndIdentity(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','k','/tmp/work','token','unit','running','acknowledged','now','now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',8,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',8,1,'running','{}','now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'implementing','running','now')`,
	}
	for _, statement := range statements {
		if _, err := db.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	intent := ProviderAttempt{AssignmentID: "a", Version: "codex-cli 0.163.0", Model: "model", Reasoning: "low", Sandbox: "workspace-write", ApprovalPolicy: "on-request", Prompt: "pinned prompt"}
	if err := db.BeginProvider(ctx, intent); err != nil {
		t.Fatal(err)
	}
	if err := db.BeginProvider(ctx, intent); err == nil {
		t.Fatal("duplicate provider launch intent accepted")
	}
	if err := db.ProviderThread(ctx, "a", "thread"); err != nil {
		t.Fatal(err)
	}
	if err := db.ProviderTurn(ctx, "a", "turn"); err != nil {
		t.Fatal(err)
	}
	if err := db.FinishProvider(ctx, "a", "completed", `{"summary":"proposal"}`, ""); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetProviderAttempt(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if got.AttemptID != "attempt" || got.Prompt != "pinned prompt" || got.ThreadID != "thread" || got.TurnID != "turn" || got.State != "completed" {
		t.Fatalf("wrong receipt: %+v", got)
	}
	if err := db.AdvanceAttemptPhase(ctx, "a", "implementing", "validating"); err != nil {
		t.Fatal(err)
	}
	var assignmentState string
	if err := db.DB.QueryRowContext(ctx, `SELECT state FROM assignments WHERE id='a'`).Scan(&assignmentState); err != nil {
		t.Fatal(err)
	}
	if assignmentState != "running" {
		t.Fatalf("provider claim settled assignment: %s", assignmentState)
	}
}
