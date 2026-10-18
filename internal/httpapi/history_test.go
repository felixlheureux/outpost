package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHistoryKeepsStartupAndAttemptReceipts(t *testing.T) {
	db, state := activityFixture(t)
	defer db.Close()
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO run_schedules(run_id,kind,mode,scope_json,policy_json,instructions,task_limit,created_at) VALUES('r','task','single','[9]','{}','',2,'now')`,
		`UPDATE assignments SET snapshot_json='{"title":"Pinned issue"}' WHERE id='a'`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',8,1,'succeeded','older')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at,settled_at) VALUES('older','r','p',8,1,'succeeded','{"title":"Earlier task","issue_revision":3,"queue_sha":"queue-revision"}','earlier','earlier')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at,settled_at) VALUES('older-attempt','older',1,'settled','succeeded','earlier','earlier')`,
		`INSERT INTO provider_attempts(assignment_id,attempt_id,state,version,model,reasoning,sandbox,approval_policy,prompt,thread_id,turn_id,created_at,updated_at) VALUES('older','older-attempt','completed','fixture','old-model','low','workspace-write','on-request','prompt','thread-old','turn-old','earlier','earlier')`,
		`INSERT INTO check_receipts(id,assignment_id,attempt_id,generation,check_id,name,required,command_json,contract_digest,input_digest,candidate_digest,status,reason,started_at,output_path,terminal_path) VALUES('check-old','older','older-attempt',1,'test','Tests',1,'["test"]','contract','input','candidate','passed','','earlier','/tmp/check-old.log','/tmp/check-old.json')`,
		`INSERT INTO delivery_receipts(id,assignment_id,policy,candidate_digest,baseline_head,branch,paths_json,state,created_at,updated_at) VALUES('delivery-old','older','local_changes','candidate','head','main','[]','settled','earlier','earlier')`,
		`INSERT INTO github_reports(id,assignment_id,marker,state,reason,updated_at) VALUES('report-old','older','marker-old','comment_unknown','GitHub unavailable','earlier')`,
		`INSERT INTO work_messages(id,run_id,assignment_id,attempt_id,thread_id,expected_turn_id,target,body,digest,state,created_at,updated_at) VALUES('message-old','r','older','older-attempt','thread-old','turn-old','live','Keep the fix','digest-old','accepted','earlier','earlier')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,reservation_state,created_at,updated_at) VALUES('startup','p','startup-key','/tmp/work','startup-token','startup-unit','stopped','unknown','released','earlier','earlier')`,
	} {
		if _, err := db.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewWithRepositorySetup(db, nil, "test", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, state, "")
	if _, err := db.ListHistory(ctx, "p", 30, 0); err != nil {
		t.Fatalf("list query: %v", err)
	}
	get := func(path string) (int, string) {
		t.Helper()
		req := httptest.NewRequest("GET", "http://localhost"+path, nil)
		req.Host = "localhost"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	if code, body := get("/api/v1/history?project=p"); code != 200 || !strings.Contains(body, `"id":"startup"`) || !strings.Contains(body, `"assignment_count":0`) {
		t.Fatalf("list: %d %s", code, body)
	}
	if code, body := get("/api/v1/history/r"); code != 200 || !strings.Contains(body, `"title":"Pinned issue"`) || !strings.Contains(body, `"model":"model"`) || !strings.Contains(body, `"outcome":"running"`) {
		t.Fatalf("detail: %d %s", code, body)
	}
	if code, body := get("/api/v1/history/r"); code != 200 || !strings.Contains(body, `"title":"Earlier task"`) || !strings.Contains(body, `"name":"Tests"`) || !strings.Contains(body, `"state":"comment_unknown"`) || !strings.Contains(body, `"body":"Keep the fix"`) || !strings.Contains(body, `"turn_id":"turn-old"`) {
		t.Fatalf("earlier assignment receipts: %d %s", code, body)
	}
	if code, body := get("/api/v1/history/r/messages/message-old"); code != 200 || !strings.Contains(body, `"body":"Keep the fix"`) {
		t.Fatalf("full message: %d %s", code, body)
	}
	if code, _ := get("/api/v1/history/startup/messages/message-old"); code != 404 {
		t.Fatalf("cross-run message: %d", code)
	}
	if code, body := get("/api/v1/history/startup"); code != 200 || !strings.Contains(body, `"assignments":[]`) || !strings.Contains(body, `"launch_observation":"unknown"`) {
		t.Fatalf("startup detail: %d %s", code, body)
	}
	if code, _ := get("/api/v1/history?offset=-1"); code != 400 {
		t.Fatalf("bad offset: %d", code)
	}
	req := httptest.NewRequest("POST", "http://localhost/api/v1/runs/r/reports/retry", nil)
	req.Host = "localhost"
	req.Header.Set("Origin", "http://localhost")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "run_active") {
		t.Fatalf("active report retry: %d %s", rec.Code, rec.Body.String())
	}
}
