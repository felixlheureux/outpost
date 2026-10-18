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
