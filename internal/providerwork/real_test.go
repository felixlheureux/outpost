package providerwork

import (
	"context"
	"database/sql"
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
	artifact, err := db.GetActivityArtifact(ctx, "artifact_attempt")
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

// Separate opt in for the SQLite-to-worker-to-provider control path. It uses
// a disposable checkout and never touches an Outpost-managed repository.
func TestRealOwnedWorkControls(t *testing.T) {
	if os.Getenv("OUTPOST_REAL_CODEX_WORK_CONTROL_TEST") != "1" {
		t.Skip("set OUTPOST_REAL_CODEX_WORK_CONTROL_TEST=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
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
		t.Fatal("no model")
	}
	model := cap.Models[0]
	effort := model.DefaultReasoningEffort
	if effort == "" && len(model.ReasoningEfforts) > 0 {
		effort = model.ReasoningEfforts[0].Effort
	}
	if effort == "" {
		t.Fatal("no effort")
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
	snapshot, _ := json.Marshal(map[string]any{"issue_number": 8, "workspace_path": workspace, "title": "Control receipt smoke", "body": "First run the shell command `sleep 30` so the operator can steer and interrupt during an owned command. Then create done.txt containing done and return the structured handoff."})
	policy, _ := json.Marshal(map[string]any{"agent": map[string]string{"model": model.ID, "reasoning": effort, "sandbox": "workspace-write", "approval_policy": "on-request"}})
	statements := []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','key-12345678',?,'token','unit','running','acknowledged','now','now')`,
		`INSERT INTO run_schedules(run_id,kind,mode,scope_json,policy_json,instructions,task_limit,created_at) VALUES('r','task','single','[8]',?,'',2,'now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',8,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',8,1,'running',?,'now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'implementing','running','now')`,
	}
	for i, statement := range statements {
		var args []any
		switch i {
		case 1:
			args = []any{workspace}
		case 2:
			args = []any{string(policy)}
		case 4:
			args = []any{string(snapshot)}
		}
		if _, err := db.DB.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	assignment := store.Assignment{ID: "a", RunID: "r", SnapshotJSON: string(snapshot)}
	schedule := store.Schedule{PolicyJSON: string(policy)}
	done := make(chan error, 1)
	go func() { _, err := Execute(ctx, db, state, executable, assignment, schedule, workspace); done <- err }()
	var receipt store.ProviderAttempt
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		receipt, err = db.GetProviderAttempt(ctx, "a")
		if err == nil && receipt.State == "turn_started" {
			var commands int
			_ = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM activity_events WHERE run_id='r' AND kind='activity.appended' AND json_extract(payload_json,'$.summary')='Command started'`).Scan(&commands)
			if commands > 0 {
				break
			}
		}
		if err != nil && err != sql.ErrNoRows {
			t.Fatal(err)
		}
	}
	if receipt.State != "turn_started" {
		t.Fatalf("provider did not reach active turn: %+v %v", receipt, err)
	}
	message, err := db.SubmitWorkMessage(ctx, store.WorkMessage{ID: "message-real-1234", RunID: "r", AttemptID: "attempt", ThreadID: receipt.ThreadID, ExpectedTurnID: receipt.TurnID, Target: "live", Body: "After the sleep, also create steered.txt containing steered."})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		message, err = db.GetWorkMessage(ctx, message.ID)
		if err != nil {
			t.Fatal(err)
		}
		if message.State != "queued" && message.State != "sending" {
			break
		}
	}
	if message.State != "accepted" {
		t.Fatalf("real message receipt: %+v", message)
	}
	control, err := db.SubmitWorkControl(ctx, store.WorkControl{ID: "interrupt-real-1234", RunID: "r", AttemptID: "attempt", Action: "interrupt"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case executeErr := <-done:
		if executeErr == nil {
			t.Fatal("interrupted provider returned success")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("provider did not settle interruption")
	}
	control, err = db.GetWorkControl(ctx, control.ID)
	if err != nil || control.State != "accepted" {
		t.Fatalf("real interrupt receipt: %+v %v", control, err)
	}
	receipt, err = db.GetProviderAttempt(ctx, "a")
	if err != nil || receipt.State != "interrupted" {
		t.Fatalf("provider state: %+v %v", receipt, err)
	}
	stop, err := db.GetSchedule(ctx, "r")
	if err != nil || !stop.StopRequested {
		t.Fatalf("stop guard: %+v %v", stop, err)
	}
	continuation, err := db.SubmitWorkControl(ctx, store.WorkControl{ID: "continue-real-1234", RunID: "r", AttemptID: "attempt", Action: "continue", Mode: "resume", Instructions: "Create resumed.txt containing resumed, then return a structured handoff."})
	if err != nil {
		t.Fatal(err)
	}
	continuation, err = db.ClaimWorkControl(ctx, "r", "continue")
	if err != nil {
		t.Fatal(err)
	}
	newAttempt, thread, err := db.BeginContinuation(ctx, continuation)
	if err != nil || thread != receipt.ThreadID {
		t.Fatalf("continuation link: %s %s %v", newAttempt, thread, err)
	}
	continuation, err = db.GetWorkControl(ctx, continuation.ID)
	if err != nil || continuation.State != "accepted" {
		t.Fatalf("continuation receipt: %+v %v", continuation, err)
	}
	if _, err := ExecuteContinuation(ctx, db, state, executable, assignment, schedule, workspace, thread, continuation.Instructions); err != nil {
		t.Fatalf("real saved-session continuation: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "resumed.txt")); err != nil || string(b) == "" {
		t.Fatalf("continued work: %q %v", b, err)
	}
	for _, artifactID := range []string{"artifact_attempt", "artifact_" + newAttempt} {
		artifact, err := db.GetActivityArtifact(ctx, artifactID)
		if err != nil || artifact.BytesWritten == 0 {
			t.Fatalf("linked artifact %s: %+v %v", artifactID, artifact, err)
		}
	}
	var oldPrompt, oldState string
	if err := db.DB.QueryRowContext(ctx, `SELECT prompt,state FROM provider_attempts WHERE attempt_id='attempt'`).Scan(&oldPrompt, &oldState); err != nil || oldPrompt == "" || oldState != "interrupted" {
		t.Fatalf("original prompt/receipt lost: %q %s %v", oldPrompt, oldState, err)
	}
	stop, err = db.GetSchedule(ctx, "r")
	if err != nil || !stop.StopRequested || stop.SuccessfulCount != 0 || stop.TaskLimit != 2 {
		t.Fatalf("continuation changed run budget or stop: %+v %v", stop, err)
	}
	t.Logf("real Outpost message=%s interrupt=%s continuation=%s linked_attempt=%s", message.State, control.State, continuation.State, newAttempt)
}
