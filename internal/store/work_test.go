package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/felixlheureux/outpost/migrations"
	"github.com/pressly/goose/v3"
)

func workFixture(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','work-key','/tmp/work','token','unit','running','acknowledged','now','now')`,
		`INSERT INTO run_schedules(run_id,kind,mode,scope_json,policy_json,instructions,task_limit,created_at) VALUES('r','task','single','[8]','{}','',3,'now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',8,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',8,1,'running','{"issue_number":8,"title":"Task","body":"Do it"}','now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'implementing','running','now')`,
		`INSERT INTO provider_attempts(assignment_id,attempt_id,state,version,model,reasoning,sandbox,approval_policy,prompt,thread_id,turn_id,turn_open,created_at,updated_at) VALUES('a','attempt','turn_started','codex-cli 0.163.0','model','low','workspace-write','on-request','prompt','thread','turn',1,'now','now')`,
	} {
		if _, err = db.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestUpgradeFromActivitySchemaPreservesProviderAndArtifact(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := fstest.MapFS{}
	for i := 1; i <= 8; i++ {
		matches, err := fs.Glob(migrations.Files, fmt.Sprintf("%05d_*.sql", i))
		if err != nil || len(matches) != 1 {
			t.Fatalf("migration %d: %v %v", i, matches, err)
		}
		b, err := fs.ReadFile(migrations.Files, matches[0])
		if err != nil {
			t.Fatal(err)
		}
		old[matches[0]] = &fstest.MapFile{Data: b}
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db.DB, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','key-12345678','/tmp/work','token','unit','running','acknowledged','now','now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',8,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',8,1,'running','{}','now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'implementing','running','now')`,
		`INSERT INTO provider_attempts(assignment_id,attempt_id,state,version,model,reasoning,sandbox,approval_policy,prompt,thread_id,turn_id,created_at,updated_at) VALUES('a','attempt','turn_started','v','m','low','workspace-write','on-request','prompt','thread','turn','now','now')`,
		`INSERT INTO activity_artifacts(id,run_id,attempt_id,assignment_id,kind,relative_path,created_at,updated_at) VALUES('artifact_a','r','attempt','a','provider-events','artifacts/r/attempt/provider-events.jsonl','now','now')`,
	} {
		if _, err := db.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}
	var id, path string
	if err := db.DB.QueryRowContext(ctx, `SELECT id,relative_path FROM activity_artifacts WHERE attempt_id='attempt'`).Scan(&id, &path); err != nil || id != "artifact_a" || path != "artifacts/r/attempt/provider-events.jsonl" {
		t.Fatalf("artifact changed: %q %q %v", id, path, err)
	}
	p, err := db.GetProviderAttempt(ctx, "a")
	if err != nil || p.ThreadID != "thread" || p.TurnID != "turn" {
		t.Fatalf("provider changed: %+v %v", p, err)
	}
}

func TestWorkMessageIdentityAndTerminalRace(t *testing.T) {
	ctx := context.Background()
	db := workFixture(t)
	input := WorkMessage{ID: "message-12345678", RunID: "r", AttemptID: "attempt", ThreadID: "thread", ExpectedTurnID: "turn", Target: "live", Body: "Change the test"}
	first, err := db.SubmitWorkMessage(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := db.SubmitWorkMessage(ctx, input)
	if err != nil || replay.ID != first.ID || replay.State != "queued" {
		t.Fatalf("idempotent replay: %+v %v", replay, err)
	}
	changed := input
	changed.Body = "different"
	if _, err = db.SubmitWorkMessage(ctx, changed); !errors.Is(err, ErrWorkConflict) {
		t.Fatalf("changed payload: %v", err)
	}
	claimed, err := db.ClaimWorkMessage(ctx, "attempt", "thread", "turn")
	if err != nil || claimed.State != "sending" {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if err = db.CloseProviderTurn(ctx, "attempt", "turn"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.SubmitWorkMessage(ctx, WorkMessage{ID: "message-87654321", RunID: "r", AttemptID: "attempt", ThreadID: "thread", ExpectedTurnID: "turn", Target: "live", Body: "late"}); !errors.Is(err, ErrStaleWorkTarget) {
		t.Fatalf("late turn accepted: %v", err)
	}
	if err = db.ReconcileWorkMessages(ctx, "attempt"); err != nil {
		t.Fatal(err)
	}
	receipt, err := db.GetWorkMessage(ctx, input.ID)
	if err != nil || receipt.State != "unknown" {
		t.Fatalf("lost ack: %+v %v", receipt, err)
	}
	if _, err = db.ClaimWorkMessage(ctx, "attempt", "thread", "turn"); err == nil {
		t.Fatal("unknown message replayed")
	}
}

func TestWorkControlsStopAndLinkedContinuation(t *testing.T) {
	ctx := context.Background()
	db := workFixture(t)
	finish := WorkControl{ID: "finish-12345678", RunID: "r", AttemptID: "attempt", Action: "finish"}
	receipt, err := db.SubmitWorkControl(ctx, finish)
	if err != nil || receipt.State != "accepted" {
		t.Fatalf("finish: %+v %v", receipt, err)
	}
	schedule, err := db.GetSchedule(ctx, "r")
	if err != nil || !schedule.StopRequested {
		t.Fatalf("stop guard missing: %+v %v", schedule, err)
	}
	if _, err = db.SubmitWorkControl(ctx, finish); err != nil {
		t.Fatalf("finish replay: %v", err)
	}
	undo := WorkControl{ID: "undo-12345678", RunID: "r", AttemptID: "attempt", Action: "undo"}
	if _, err = db.SubmitWorkControl(ctx, undo); err != nil {
		t.Fatal(err)
	}
	schedule, err = db.GetSchedule(ctx, "r")
	if err != nil || schedule.StopRequested {
		t.Fatalf("undo guard: %+v %v", schedule, err)
	}
	if _, err = db.SubmitWorkControl(ctx, WorkControl{ID: "interrupt-12345", RunID: "r", AttemptID: "attempt", Action: "interrupt"}); err != nil {
		t.Fatal(err)
	}
	schedule, err = db.GetSchedule(ctx, "r")
	if err != nil || !schedule.StopRequested {
		t.Fatalf("interrupt did not guard admission: %+v %v", schedule, err)
	}
	if _, err = db.DB.ExecContext(ctx, `UPDATE provider_attempts SET state='interrupted',turn_open=0 WHERE attempt_id='attempt'`); err != nil {
		t.Fatal(err)
	}
	continuation := WorkControl{ID: "continue-12345", RunID: "r", AttemptID: "attempt", Action: "continue", Mode: "resume", Instructions: "Finish with a test"}
	if _, err = db.SubmitWorkControl(ctx, continuation); err != nil {
		t.Fatal(err)
	}
	claimed, err := db.ClaimWorkControl(ctx, "r", "continue")
	if err != nil {
		t.Fatal(err)
	}
	next, thread, err := db.BeginContinuation(ctx, claimed)
	if err != nil || thread != "thread" || next == "attempt" {
		t.Fatalf("linked attempt: %q %q %v", next, thread, err)
	}
	if _, _, err = db.BeginContinuation(ctx, claimed); !errors.Is(err, ErrStaleWorkTarget) {
		t.Fatalf("duplicate continuation: %v", err)
	}
	var predecessor string
	var ordinal int
	if err = db.DB.QueryRowContext(ctx, `SELECT predecessor_id,ordinal FROM assignment_attempts WHERE id=?`, next).Scan(&predecessor, &ordinal); err != nil || predecessor != "attempt" || ordinal != 2 {
		t.Fatalf("attempt history: %s %d %v", predecessor, ordinal, err)
	}
	schedule, err = db.GetSchedule(ctx, "r")
	if err != nil || !schedule.StopRequested || schedule.SuccessfulCount != 0 || schedule.TaskLimit != 3 {
		t.Fatalf("budget/stop changed: %+v %v", schedule, err)
	}
}
