package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestActivityCursorAndBusinessStateTriggers(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','key-12345678','/tmp/work','token','unit','running','acknowledged','now','now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',9,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',9,1,'running','{}','now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'implementing','running','now')`,
	} {
		if _, err := db.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := db.ReadActivitySnapshot(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RunState != "running" || snapshot.AssignmentID != "a" || snapshot.AttemptPhase != "implementing" || snapshot.Cursor < 3 {
		t.Fatalf("wrong snapshot: %+v", snapshot)
	}
	events, err := db.ReplayActivity(ctx, "r", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 3 || events[0].Cursor >= events[len(events)-1].Cursor {
		t.Fatalf("wrong replay: %+v", events)
	}
	if !json.Valid(events[0].Payload) {
		t.Fatal("invalid event payload")
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE assignment_attempts SET phase='validating' WHERE id='attempt'`); err != nil {
		t.Fatal(err)
	}
	newEvents, err := db.ReplayActivity(ctx, "r", snapshot.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(newEvents) != 1 || newEvents[0].Kind != "phase.changed" {
		t.Fatalf("phase event not emitted with state update: %+v", newEvents)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO activity_events(project_id,run_id,kind,payload_json,created_at) VALUES('p','r','activity.appended',?, 'now')`, string(make([]byte, 65<<10))); err == nil {
		t.Fatal("oversized event accepted")
	}
}
