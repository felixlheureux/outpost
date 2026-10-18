package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/felixlheureux/outpost/internal/privateaccess"
	"github.com/felixlheureux/outpost/internal/store"
)

func TestRoutingAndRequestBoundary(t *testing.T) {
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := New(db, fstest.MapFS{"index.html": {Data: []byte("<!doctype html><div>Outpost</div>")}, "assets/app.js": {Data: []byte("console.log('ok')")}}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	cases := []struct {
		name, method, path, host, origin, content string
		status                                    int
	}{
		{"navigation", "GET", "/work", "localhost", "", "text/html", 200},
		{"head", "HEAD", "/history", "localhost", "", "text/html", 200},
		{"api", "GET", "/api/v1/projects", "localhost", "", "application/json", 200},
		{"api typo", "GET", "/api/v1/missing", "localhost", "", "text/html", 404},
		{"bare api", "GET", "/api", "localhost", "", "text/html", 404},
		{"missing asset", "GET", "/assets/no.js", "localhost", "", "text/html", 404},
		{"static", "GET", "/assets/app.js", "localhost", "", "*/*", 200},
		{"non navigation", "GET", "/missing", "localhost", "", "*/*", 404},
		{"unsupported setup", "POST", "/api/v1/projects", "localhost", "http://localhost", "application/json", 501},
		{"external origin", "POST", "/api/v1/projects", "localhost", "https://evil.example", "application/json", 403},
		{"rebinding host", "GET", "/api/v1/bootstrap", "evil.example", "", "application/json", 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = tc.host
			req.Header.Set("Accept", tc.content)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			if tc.method == "HEAD" && response.Body.Len() != 0 {
				t.Fatal("HEAD included body")
			}
			if tc.path == "/api/v1/projects" && tc.status == 200 && strings.TrimSpace(response.Body.String()) != "{\"projects\":[]}" {
				t.Fatalf("unexpected project projection %s", response.Body.String())
			}
			if tc.path == "/api/v1/missing" && !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatal("API typo returned HTML")
			}
		})
	}
}

func TestPrivateAccessCoversAllRoutes(t *testing.T) {
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	policy := &privateaccess.Config{Origin: "https://host.tailnet.ts.net", AllowedLogin: "owner@example.com"}
	handler := NewWithAccess(db, fstest.MapFS{"index.html": {Data: []byte("<html></html>")}, "assets/app.js": {Data: []byte("ok")}}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	cases := []struct {
		name, method, path, host, remote, login, origin string
		want                                            int
	}{
		{"allowed html", "GET", "/work", "host.tailnet.ts.net", "127.0.0.1:41000", "owner@example.com", "", 200},
		{"allowed api", "GET", "/api/v1/projects", "host.tailnet.ts.net", "127.0.0.1:41000", "owner@example.com", "", 200},
		{"allowed asset", "GET", "/assets/app.js", "host.tailnet.ts.net", "127.0.0.1:41000", "owner@example.com", "", 200},
		{"allowed mutation", "POST", "/api/v1/projects", "host.tailnet.ts.net", "127.0.0.1:41000", "owner@example.com", "https://host.tailnet.ts.net", 501},
		{"missing identity", "GET", "/", "host.tailnet.ts.net", "127.0.0.1:41000", "", "", 403},
		{"wrong identity", "GET", "/api/v1/projects", "host.tailnet.ts.net", "127.0.0.1:41000", "other@example.com", "", 403},
		{"asset denied", "GET", "/assets/app.js", "host.tailnet.ts.net", "127.0.0.1:41000", "", "", 403},
		{"health denied", "GET", "/healthz", "host.tailnet.ts.net", "127.0.0.1:41000", "", "", 403},
		{"nonlocal peer", "GET", "/", "host.tailnet.ts.net", "192.0.2.1:41000", "owner@example.com", "", 403},
		{"host spoof", "GET", "/", "evil.example", "127.0.0.1:41000", "owner@example.com", "", 403},
		{"origin spoof", "POST", "/api/v1/projects", "host.tailnet.ts.net", "127.0.0.1:41000", "owner@example.com", "https://evil.example", 403},
		{"origin missing", "POST", "/api/v1/projects", "host.tailnet.ts.net", "127.0.0.1:41000", "owner@example.com", "", 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = tc.host
			req.RemoteAddr = tc.remote
			req.Header.Set("Accept", "text/html")
			if tc.login != "" {
				req.Header.Set("Tailscale-User-Login", tc.login)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Fatalf("status %d, wanted %d: %s", response.Code, tc.want, response.Body.String())
			}
		})
	}
}
