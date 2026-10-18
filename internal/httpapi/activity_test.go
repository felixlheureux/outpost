package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/felixlheureux/outpost/internal/privateaccess"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/labstack/echo/v5"
)

func activityFixture(t *testing.T) (*store.Store, string) {
	t.Helper()
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO activity_events(kind,payload_json,created_at) VALUES('source.changed','{}','now')`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','key-12345678','/tmp/work','token','unit','running','acknowledged','now','now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',9,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',9,1,'running','{}','now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'implementing','running','now')`,
		`INSERT INTO provider_attempts(assignment_id,attempt_id,state,version,model,reasoning,sandbox,approval_policy,prompt,created_at,updated_at) VALUES('a','attempt','pending','codex-cli 0.163.0','model','low','workspace-write','on-request','prompt','now','now')`,
	} {
		if _, err := db.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	return db, state
}

type streamRecorder struct {
	headers http.Header
	chunks  chan string
}

func (w *streamRecorder) Header() http.Header              { return w.headers }
func (w *streamRecorder) WriteHeader(int)                  {}
func (w *streamRecorder) Write(p []byte) (int, error)      { w.chunks <- string(p); return len(p), nil }
func (w *streamRecorder) Flush()                           {}
func (w *streamRecorder) SetWriteDeadline(time.Time) error { return nil }

func TestActivitySSEReplayResetAndRestart(t *testing.T) {
	db, state := activityFixture(t)
	defer db.Close()
	ctx := context.Background()
	snapshot, err := db.ReadActivitySnapshot(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE assignment_attempts SET phase='validating' WHERE id='attempt'`); err != nil {
		t.Fatal(err)
	}
	handler := NewWithRepositorySetup(db, nil, "test", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, state, "")
	read := func(after string) string {
		t.Helper()
		requestCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest("GET", "/api/v1/activity/stream?run=r&after="+after, nil).WithContext(requestCtx)
		req.Host = "localhost"
		writer := &streamRecorder{headers: make(http.Header), chunks: make(chan string, 12)}
		done := make(chan struct{})
		go func() { handler.ServeHTTP(writer, req); close(done) }()
		var output strings.Builder
		deadline := time.After(3 * time.Second)
		for {
			select {
			case chunk := <-writer.chunks:
				output.WriteString(chunk)
				if strings.Contains(output.String(), "event: phase.changed") || strings.Contains(output.String(), "event: reset") {
					cancel()
					<-done
					return output.String()
				}
			case <-deadline:
				cancel()
				<-done
				t.Fatalf("SSE did not replay expected event: %s", output.String())
				return ""
			}
		}
	}
	if got := read(strconvFormat(snapshot.Cursor)); !strings.HasPrefix(got, "event: snapshot") || !strings.Contains(got, "event: phase.changed") {
		t.Fatalf("replay after restart: %s", got)
	}
	if got := read("0"); strings.Contains(got, "event: reset") || !strings.Contains(got, "event: phase.changed") {
		t.Fatalf("unrelated earlier cursor caused a false reset: %s", got)
	}
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM activity_events WHERE run_id='r' AND cursor<=?`, snapshot.Cursor); err != nil {
		t.Fatal(err)
	}
	if got := read("0"); !strings.Contains(got, "event: reset") {
		t.Fatalf("missing expiry reset: %s", got)
	}
}

func strconvFormat(v int64) string { return fmt.Sprintf("%d", v) }

func TestArtifactManifestAndPrivateAccess(t *testing.T) {
	db, state := activityFixture(t)
	defer db.Close()
	ctx := context.Background()
	a, err := db.BeginProviderArtifact(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("{\"method\":\"item/completed\"}\n")
	path := filepath.Join(state, a.RelativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateArtifact(ctx, a.ID, int64(len(content)), false); err != nil {
		t.Fatal(err)
	}
	policy := &privateaccess.Config{Origin: "https://host.tailnet.ts.net", AllowedLogin: "owner@example.com"}
	handler := NewWithRepositorySetup(db, nil, "test", slog.New(slog.NewTextHandler(io.Discard, nil)), policy, state, "")
	request := func(method, id, login string) int {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/artifacts/"+id, nil)
		req.Host = "host.tailnet.ts.net"
		req.RemoteAddr = "127.0.0.1:41234"
		if method == "DELETE" {
			req.Header.Set("Origin", "https://host.tailnet.ts.net")
		}
		if login != "" {
			req.Header.Set("Tailscale-User-Login", login)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response.Code
	}
	if got := request("GET", a.ID, ""); got != 403 {
		t.Fatalf("denied identity got %d", got)
	}
	if got := request("GET", a.ID, "owner@example.com"); got != 200 {
		t.Fatalf("allowed artifact got %d", got)
	}
	if got := request("GET", "../../etc/passwd", "owner@example.com"); got == 200 {
		t.Fatal("path-like artifact ID accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", path); err != nil {
		t.Fatal(err)
	}
	if got := request("GET", a.ID, "owner@example.com"); got == 200 {
		t.Fatal("symlink artifact was served")
	}
	if got := request("DELETE", a.ID, "owner@example.com"); got != 409 {
		t.Fatalf("active artifact removal got %d", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE assignments SET state='failed' WHERE id='a'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE runs SET reservation_state='released' WHERE id='r'`); err != nil {
		t.Fatal(err)
	}
	if got := request("DELETE", a.ID, "owner@example.com"); got != 200 {
		t.Fatalf("settled artifact removal got %d", got)
	}
	if got := request("GET", a.ID, "owner@example.com"); got != 410 {
		t.Fatalf("removed artifact read got %d", got)
	}
}

func TestActivityStreamSlotBound(t *testing.T) {
	db, state := activityFixture(t)
	defer db.Close()
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	e := echo.New()
	registerActivityRoutes(e, db, state, slots)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/activity/stream?run=r", nil))
	if response.Code != 503 || !strings.Contains(response.Body.String(), "too_many_streams") {
		t.Fatalf("unbounded stream admission: %d %s", response.Code, response.Body.String())
	}
}
