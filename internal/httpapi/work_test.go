package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/worker"
)

func TestWorkCheckLivenessUsesRecordedProcessIdentity(t *testing.T) {
	db, state := activityFixture(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO run_schedules(run_id,kind,mode,scope_json,policy_json,instructions,task_limit,created_at) VALUES('r','task','single','[9]','{}','',1,'now')`); err != nil {
		t.Fatal(err)
	}
	ticks, err := worker.StartTicks(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO check_receipts(id,assignment_id,attempt_id,generation,check_id,name,required,command_json,contract_digest,input_digest,status,pid,start_ticks,started_at,output_path,terminal_path) VALUES('check-live','a','attempt',1,'test','Tests',1,'["test"]','contract','input','running',?,?,'now','/tmp/check-live.log','/tmp/check-live.json')`, os.Getpid(), ticks); err != nil {
		t.Fatal(err)
	}
	handler := NewWithRepositorySetup(db, nil, "test", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, state, "")
	get := func() string {
		req := httptest.NewRequest("GET", "http://localhost/api/v1/runs/r/work", nil)
		req.Host = "localhost"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("work: %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	if body := get(); !strings.Contains(body, `"liveness":"active"`) {
		t.Fatalf("owned check not observed: %s", body)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE check_receipts SET start_ticks='different' WHERE id='check-live'`); err != nil {
		t.Fatal(err)
	}
	if body := get(); !strings.Contains(body, `"liveness":"stopped"`) {
		t.Fatalf("reused PID accepted: %s", body)
	}
}

func TestWorkRoutesKeepTargetAndIdempotentReceipt(t *testing.T) {
	db, state := activityFixture(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO run_schedules(run_id,kind,mode,scope_json,policy_json,instructions,task_limit,created_at) VALUES('r','task','single','[9]','{}','',2,'now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE provider_attempts SET state='turn_started',thread_id='thread',turn_id='turn',turn_open=1 WHERE attempt_id='attempt'`); err != nil {
		t.Fatal(err)
	}
	handler := NewWithRepositorySetup(db, nil, "test", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, state, "")
	post := func(body string) (int, string) {
		t.Helper()
		req := httptest.NewRequest("POST", "http://localhost/api/v1/runs/r/messages", strings.NewReader(body))
		req.Host = "localhost"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	input := `{"id":"message-12345678","attempt_id":"attempt","thread_id":"thread","expected_turn_id":"turn","target":"live","body":"Check the UI"}`
	status, body := post(input)
	if status != 200 {
		t.Fatalf("first message: %d %s", status, body)
	}
	var receipt store.WorkMessage
	if err := json.Unmarshal([]byte(body), &receipt); err != nil || receipt.State != "queued" || receipt.ExpectedTurnID != "turn" {
		t.Fatalf("receipt: %+v %v", receipt, err)
	}
	status, body = post(input)
	if status != 200 || !strings.Contains(body, `"state":"queued"`) {
		t.Fatalf("replay: %d %s", status, body)
	}
	status, body = post(strings.Replace(input, "Check the UI", "Change the UI", 1))
	if status != 409 || !strings.Contains(body, "message_conflict") {
		t.Fatalf("changed payload: %d %s", status, body)
	}
	if err := db.CloseProviderTurn(ctx, "attempt", "turn"); err != nil {
		t.Fatal(err)
	}
	status, body = post(strings.Replace(input, "message-12345678", "message-87654321", 1))
	if status != 409 || !strings.Contains(body, "stale_target") {
		t.Fatalf("stale target: %d %s", status, body)
	}
}

func TestChangesRouteUsesExactCheckoutPath(t *testing.T) {
	db, state := activityFixture(t)
	defer db.Close()
	ctx := context.Background()
	workspace := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", workspace}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(workspace, "tracked.txt"), []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("commit", "-q", "-m", "base")
	if err := os.WriteFile(filepath.Join(workspace, "tracked.txt"), []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "new.txt"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE runs SET workspace_path=? WHERE id='r'`, workspace); err != nil {
		t.Fatal(err)
	}
	handler := NewWithRepositorySetup(db, nil, "test", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, state, "")
	get := func(path string) (int, string) {
		t.Helper()
		req := httptest.NewRequest("GET", "http://localhost"+path, nil)
		req.Host = "localhost"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	status, body := get("/api/v1/runs/r/changes")
	if status != 200 || !strings.Contains(body, "tracked.txt") || !strings.Contains(body, "new.txt") {
		t.Fatalf("paths: %d %s", status, body)
	}
	status, body = get("/api/v1/runs/r/changes?path=tracked.txt")
	if status != 200 || !strings.Contains(body, "-before") || !strings.Contains(body, "+after") {
		t.Fatalf("tracked diff: %d %s", status, body)
	}
	status, body = get("/api/v1/runs/r/changes?path=new.txt")
	if status != 200 || !strings.Contains(body, `"diff":"new\n"`) {
		t.Fatalf("untracked preview: %d %s", status, body)
	}
	status, body = get("/api/v1/runs/r/changes?path=../secret")
	if status != 409 || !strings.Contains(body, "stale_path") {
		t.Fatalf("unlisted path: %d %s", status, body)
	}
}
