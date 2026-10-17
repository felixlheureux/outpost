// Package httpapi owns HTTP contracts and SPA delivery, not SQL records or host processes.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/labstack/echo/v5"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}
type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}
type projectDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RepositoryURL string `json:"repository_url"`
	SetupState    string `json:"setup_state"`
	CreatedAt     string `json:"created_at"`
}

func fail(c *echo.Context, status int, code, message string) error {
	return c.JSON(status, errorBody{Error: errorDetail{Code: code, Message: message, RequestID: c.Response().Header().Get("X-Request-ID")}})
}

// New currently serves development mode only. Production identity is implemented by OP-002.
func New(db *store.Store, assets fs.FS, version string, logger *slog.Logger) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		logger.Error("request failed", "request_id", c.Response().Header().Get("X-Request-ID"), "error", err)
		_ = fail(c, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			id := make([]byte, 12)
			if _, err := rand.Read(id); err != nil {
				return err
			}
			c.Response().Header().Set("X-Request-ID", hex.EncodeToString(id))
			ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
			defer cancel()
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/healthz", func(c *echo.Context) error { return c.JSON(200, map[string]string{"status": "ok"}) })
	e.GET("/readyz", func(c *echo.Context) error {
		if err := db.DB.PingContext(c.Request().Context()); err != nil {
			return fail(c, 503, "not_ready", "Local state is unavailable.")
		}
		return c.JSON(200, map[string]string{"status": "ready"})
	})
	e.GET("/api/v1/bootstrap", func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{
			"version": version, "mode": "development", "current_run": nil,
			"capabilities": map[string]bool{"repository_setup": false, "execution": false},
		})
	})
	e.GET("/api/v1/projects", func(c *echo.Context) error {
		rows, err := db.Queries.ListProjects(c.Request().Context())
		if err != nil {
			return err
		}
		projects := make([]projectDTO, 0, len(rows))
		for _, row := range rows {
			projects = append(projects, projectDTO{ID: row.ID, Name: row.GithubOwner + "/" + row.GithubName, RepositoryURL: row.RepositoryUrl, SetupState: row.SetupState, CreatedAt: row.CreatedAt})
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"projects": projects})
	})
	e.POST("/api/v1/projects", func(c *echo.Context) error {
		return fail(c, 501, "feature_unavailable", "Repository setup is the next implementation step. Your URL has been kept.")
	})
	missingAPI := func(c *echo.Context) error { return fail(c, 404, "not_found", "API endpoint not found.") }
	e.Any("/api", missingAPI)
	e.Any("/api/*", missingAPI)
	fallback := func(c *echo.Context) error {
		if c.Request().Method != http.MethodGet && c.Request().Method != http.MethodHead {
			return fail(c, 405, "method_not_allowed", "Use GET or HEAD.")
		}
		if assets == nil {
			return fail(c, 404, "development_ui", "Use the Vite development URL for the UI.")
		}
		filename := strings.TrimPrefix(path.Clean(c.Request().URL.Path), "/")
		if filename != "" && filename != "." {
			if info, err := fs.Stat(assets, filename); err == nil && !info.IsDir() {
				if strings.HasPrefix(filename, "assets/") {
					c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.FileServer(http.FS(assets)).ServeHTTP(c.Response(), c.Request())
				return nil
			}
		}
		if strings.HasPrefix(filename, "assets/") || path.Ext(filename) != "" || !strings.Contains(c.Request().Header.Get("Accept"), "text/html") {
			return fail(c, 404, "not_found", "Resource not found.")
		}
		data, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-cache")
		if c.Request().Method == http.MethodHead {
			c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
			return c.NoContent(200)
		}
		return c.HTML(200, string(data))
	}
	e.Any("/", fallback)
	e.Any("/*", fallback)

	csrf := http.NewCrossOriginProtection()
	// The fixed dev UI proxy is the only additional origin; this is not production authentication.
	_ = csrf.AddTrustedOrigin("http://127.0.0.1:5173")
	_ = csrf.AddTrustedOrigin("http://localhost:5173")
	protected := csrf.Handler(e)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "Loopback host required", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		protected.ServeHTTP(w, r)
	})
}
