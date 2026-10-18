// Package httpapi owns HTTP contracts and SPA delivery, not SQL records or host processes.
package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/felixlheureux/outpost/internal/codexadapter"
	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/issues"
	"github.com/felixlheureux/outpost/internal/privateaccess"
	"github.com/felixlheureux/outpost/internal/projectsetup"
	"github.com/felixlheureux/outpost/internal/queue"
	"github.com/felixlheureux/outpost/internal/scheduler"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/supervisor"
	"github.com/felixlheureux/outpost/internal/worker"
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
	SetupStage    string `json:"setup_stage"`
	SetupError    string `json:"setup_error,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

func fail(c *echo.Context, status int, code, message string) error {
	return c.JSON(status, errorBody{Error: errorDetail{Code: code, Message: message, RequestID: c.Response().Header().Get("X-Request-ID")}})
}

func New(db *store.Store, assets fs.FS, version string, logger *slog.Logger) http.Handler {
	return newServer(db, assets, version, logger, nil, "", "")
}

// NewWithAccess applies one explicit Tailscale login and origin to every route.
// A nil policy is restricted to local development.
func NewWithAccess(db *store.Store, assets fs.FS, version string, logger *slog.Logger, access *privateaccess.Config) http.Handler {
	return newServer(db, assets, version, logger, access, "", "")
}

func NewWithRepositorySetup(db *store.Store, assets fs.FS, version string, logger *slog.Logger, access *privateaccess.Config, stateDir, workspacesDir string) http.Handler {
	return newServer(db, assets, version, logger, access, stateDir, workspacesDir)
}

func newServer(db *store.Store, assets fs.FS, version string, logger *slog.Logger, access *privateaccess.Config, stateDir, workspacesDir string) http.Handler {
	e := echo.New()
	activitySlots := make(chan struct{}, 8)
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
			timeout := 15 * time.Second
			if c.Request().Method == http.MethodGet && c.Request().URL.Path == "/api/v1/activity/stream" {
				return next(c)
			}
			if c.Request().Method == http.MethodPost &&
				(c.Request().URL.Path == "/api/v1/projects" ||
					(strings.HasPrefix(c.Request().URL.Path, "/api/v1/projects/") &&
						(strings.HasSuffix(c.Request().URL.Path, "/retry") || strings.HasSuffix(c.Request().URL.Path, "/issues/sync") || strings.HasSuffix(c.Request().URL.Path, "/queue/sync") || strings.HasSuffix(c.Request().URL.Path, "/queue/operations") || strings.HasSuffix(c.Request().URL.Path, "/admission-preview") || strings.HasSuffix(c.Request().URL.Path, "/inspect")))) {
				timeout = 10 * time.Minute
			}
			if c.Request().Method == http.MethodGet && c.Request().URL.Path == "/api/v1/agent/capabilities" {
				timeout = 20 * time.Second
			}
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
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
		mode := "development"
		if access != nil {
			mode = "private"
		}
		var currentRun any
		run, err := db.ActiveRun(c.Request().Context())
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			liveness := "unknown"
			if observed, observeErr := supervisor.Observe(c.Request().Context(), run.UnitName); observeErr == nil {
				if observed.ActiveState != "active" && observed.MainPID == 0 {
					liveness = "stopped"
				} else if observed.MainPID > 0 && run.WorkerPID > 0 {
					if ticks, tickErr := worker.StartTicks(observed.MainPID); tickErr == nil && int64(observed.MainPID) == run.WorkerPID && ticks == run.WorkerStartTicks {
						liveness = "active"
					}
				}
			}
			currentRun = map[string]string{"id": run.ID, "project_id": run.ProjectID, "state": run.State, "liveness": liveness}
		}
		return c.JSON(200, map[string]any{
			"version": version, "mode": mode, "current_run": currentRun,
			"capabilities": map[string]bool{"repository_setup": workspacesDir != "", "execution": false},
		})
	})
	e.GET("/api/v1/agent/capabilities", func(c *echo.Context) error {
		projectID := c.QueryParam("project")
		if projectID == "" {
			return fail(c, 400, "project_required", "Choose a repository before checking Codex settings.")
		}
		project, err := db.GetProjectSetup(c.Request().Context(), projectID)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if err != nil {
			return err
		}
		if project.State != "ready" {
			return fail(c, 409, "project_not_ready", "Finish repository setup before starting work.")
		}
		executable, err := codexadapter.FindExecutable()
		if err != nil {
			return fail(c, 503, "provider_missing", "Install Codex for the Outpost service user, then sign in with codex login.")
		}
		capabilities, err := codexadapter.Probe(c.Request().Context(), executable, project.WorkspacePath)
		if err != nil {
			logger.Warn("Codex capability probe failed", "error", err)
			return fail(c, 503, "provider_unavailable", "Codex 0.163.0 and a working codex login are required for this release. Run outpost doctor for details.")
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, capabilities)
	})
	registerActivityRoutes(e, db, stateDir, activitySlots)
	e.GET("/api/v1/projects", func(c *echo.Context) error {
		rows, err := db.Queries.ListProjects(c.Request().Context())
		if err != nil {
			return err
		}
		projects := make([]projectDTO, 0, len(rows))
		for _, row := range rows {
			projects = append(projects, projectDTO{ID: row.ID, Name: row.GithubOwner + "/" + row.GithubName, RepositoryURL: row.RepositoryUrl, SetupState: row.SetupState, SetupStage: row.SetupStage, SetupError: row.SetupError.String, CreatedAt: row.CreatedAt})
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"projects": projects})
	})
	e.POST("/api/v1/projects", func(c *echo.Context) error {
		if workspacesDir == "" {
			return fail(c, 501, "feature_unavailable", "Repository setup is not configured on this server.")
		}
		var input struct {
			RepositoryURL string `json:"repository_url"`
		}
		decoder := json.NewDecoder(c.Request().Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || len(input.RepositoryURL) > 500 {
			return fail(c, 400, "invalid_repository", "Enter one GitHub repository URL.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_repository", "Enter one GitHub repository URL.")
		}
		if _, _, err := githubcli.ParseURL(input.RepositoryURL); err != nil {
			return fail(c, 400, "invalid_repository", err.Error())
		}
		project, err := projectsetup.Setup(c.Request().Context(), db, stateDir, workspacesDir, input.RepositoryURL)
		if err != nil {
			logger.Warn("repository setup needs attention", "error", err)
			return fail(c, 409, "setup_needs_attention", "Repository setup needs attention. Check host GitHub access or the retained staging checkout, then retry.")
		}
		return c.JSON(200, projectDTO{ID: project.ID, Name: project.Owner + "/" + project.Name, RepositoryURL: project.RepositoryURL, SetupState: project.State, SetupStage: project.Stage, SetupError: project.Error})
	})
	e.POST("/api/v1/projects/existing", func(c *echo.Context) error {
		var input struct {
			CheckoutPath string `json:"checkout_path"`
		}
		decoder := json.NewDecoder(c.Request().Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || len(input.CheckoutPath) > 4096 {
			return fail(c, 400, "invalid_checkout", "Enter one absolute checkout path.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_checkout", "Enter one absolute checkout path.")
		}
		project, err := projectsetup.RegisterExisting(c.Request().Context(), db, input.CheckoutPath)
		if err != nil {
			logger.Warn("existing checkout registration needs attention", "error", err)
			return fail(c, 409, "setup_needs_attention", "Existing checkout could not be registered. Check its GitHub origin, host access, and local Git state.")
		}
		return c.JSON(200, projectDTO{ID: project.ID, Name: project.Owner + "/" + project.Name, RepositoryURL: project.RepositoryURL, SetupState: project.State, SetupStage: project.Stage, SetupError: project.Error})
	})
	e.POST("/api/v1/projects/:id/retry", func(c *echo.Context) error {
		if workspacesDir == "" {
			return fail(c, 501, "feature_unavailable", "Repository setup is not configured on this server.")
		}
		project, err := projectsetup.Retry(c.Request().Context(), db, stateDir, workspacesDir, c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository setup was not found.")
		}
		if err != nil {
			logger.Warn("repository setup retry needs attention", "error", err)
			return fail(c, 409, "setup_needs_attention", "Repository setup still needs attention. Check host access or the retained checkout, then retry.")
		}
		return c.JSON(200, projectDTO{ID: project.ID, Name: project.Owner + "/" + project.Name, RepositoryURL: project.RepositoryURL, SetupState: project.State, SetupStage: project.Stage, SetupError: project.Error})
	})
	e.GET("/api/v1/projects/:id/issues", func(c *echo.Context) error {
		projectID := c.Param("id")
		if _, err := db.GetProjectSetup(c.Request().Context(), projectID); errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		} else if err != nil {
			return err
		}
		rows, status, err := db.ListIssueSources(c.Request().Context(), projectID)
		if err != nil {
			return err
		}
		items := make([]map[string]any, 0, len(rows))
		for _, issue := range rows {
			items = append(items, map[string]any{
				"number": issue.Number, "url": issue.URL, "title": issue.Title, "body": issue.Body,
				"state": issue.State, "source_updated_at": issue.SourceUpdatedAt,
				"content_digest": issue.ContentDigest, "fetched_at": issue.FetchedAt,
				"local_revision": issue.LocalRevision, "requirements": json.RawMessage(issue.RequirementsJSON),
				"parse_error": issue.ParseError, "stale": status.LastSuccessAt != "" && issue.FetchedAt != status.LastSuccessAt,
			})
		}
		if status.State == "" {
			status.State = "not_synced"
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"issues": items, "source": map[string]string{"state": status.State, "last_success_at": status.LastSuccessAt, "last_error": status.LastError}})
	})
	e.POST("/api/v1/projects/:id/issues/sync", func(c *echo.Context) error {
		count, err := issues.Sync(c.Request().Context(), db, c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if err != nil {
			logger.Warn("GitHub issue sync needs attention", "error", err)
			return fail(c, 503, "source_unavailable", "GitHub issues could not be refreshed. Existing observations were preserved.")
		}
		return c.JSON(200, map[string]int{"observed": count})
	})
	issueOperationDTO := func(op store.IssueOperation) map[string]any {
		return map[string]any{"id": op.ID, "request_key": op.RequestKey, "kind": op.Kind,
			"state": op.State, "issue_number": op.IssueNumber, "result_number": op.ResultNumber,
			"reason": op.Reason, "created_at": op.CreatedAt}
	}
	mutateIssue := func(c *echo.Context, kind string) error {
		var input issues.Mutation
		decoder := json.NewDecoder(c.Request().Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return fail(c, 400, "invalid_issue", "Enter an issue title, body, and stable request key.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_issue", "Send one issue operation.")
		}
		if kind == "edit" {
			number, err := strconv.Atoi(c.Param("number"))
			if err != nil || number <= 0 || (input.IssueNumber != 0 && input.IssueNumber != number) {
				return fail(c, 400, "invalid_issue", "Issue number does not match the edit target.")
			}
			input.IssueNumber = number
		}
		op, err := issues.Mutate(c.Request().Context(), db, c.Param("id"), kind, input)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if err != nil {
			logger.Warn("issue operation failed locally", "error", err)
			return fail(c, 400, "invalid_issue", "Issue operation could not be accepted. Check the title, body, request key, and source revision.")
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, issueOperationDTO(op))
	}
	e.POST("/api/v1/projects/:id/issues", func(c *echo.Context) error { return mutateIssue(c, "create") })
	e.PATCH("/api/v1/projects/:id/issues/:number", func(c *echo.Context) error { return mutateIssue(c, "edit") })
	e.GET("/api/v1/projects/:id/issue-operations", func(c *echo.Context) error {
		if _, err := db.GetProjectSetup(c.Request().Context(), c.Param("id")); errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		} else if err != nil {
			return err
		}
		ops, err := db.ListIssueOperations(c.Request().Context(), c.Param("id"))
		if err != nil {
			return err
		}
		items := make([]map[string]any, 0, len(ops))
		for _, op := range ops {
			items = append(items, issueOperationDTO(op))
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"operations": items})
	})
	e.GET("/api/v1/projects/:id/issue-operations/:key", func(c *echo.Context) error {
		op, err := db.GetIssueOperation(c.Request().Context(), c.Param("id"), c.Param("key"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "operation_not_found", "Issue operation was not found.")
		}
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, issueOperationDTO(op))
	})
	e.POST("/api/v1/projects/:id/issue-operations/:key/inspect", func(c *echo.Context) error {
		op, err := issues.Inspect(c.Request().Context(), db, c.Param("id"), c.Param("key"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "operation_not_found", "Issue operation was not found.")
		}
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, issueOperationDTO(op))
	})
	queueOperationDTO := func(op store.QueueOperation) map[string]any {
		return map[string]any{"id": op.ID, "request_key": op.RequestKey, "kind": op.Kind, "state": op.State, "expected_sha": op.ExpectedSHA, "intended": json.RawMessage(op.IntendedJSON), "current_issues": json.RawMessage(op.ObservedJSON), "reason": op.Reason, "created_at": op.CreatedAt}
	}
	e.GET("/api/v1/projects/:id/queue", func(c *echo.Context) error {
		source, err := queue.Cached(c.Request().Context(), db, c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, source)
	})
	e.POST("/api/v1/projects/:id/admission-preview", func(c *echo.Context) error {
		var input struct {
			Mode     string `json:"mode"`
			Selected int    `json:"selected"`
		}
		decoder := json.NewDecoder(c.Request().Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return fail(c, 400, "invalid_preview", "Choose Follow queue or a selected fixed task.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_preview", "Send one preview request.")
		}
		projection, err := (scheduler.Engine{DB: db}).PreviewProject(c.Request().Context(), c.Param("id"), input.Mode, input.Selected)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if err != nil {
			logger.Warn("admission preview failed", "error", err)
			return fail(c, 400, "invalid_preview", "Admission preview could not be completed.")
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, projection)
	})
	e.POST("/api/v1/projects/:id/queue/sync", func(c *echo.Context) error {
		source, err := queue.Sync(c.Request().Context(), db, c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, source)
	})
	e.POST("/api/v1/projects/:id/queue/operations", func(c *echo.Context) error {
		var edit queue.Edit
		decoder := json.NewDecoder(c.Request().Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&edit); err != nil {
			return fail(c, 400, "invalid_queue_edit", "Enter a queue edit with a stable request key and expected SHA.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_queue_edit", "Send one queue edit.")
		}
		op, err := queue.Mutate(c.Request().Context(), db, c.Param("id"), edit)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if err != nil {
			return fail(c, 400, "invalid_queue_edit", err.Error())
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, queueOperationDTO(op))
	})
	e.GET("/api/v1/projects/:id/queue/operations", func(c *echo.Context) error {
		if _, err := db.GetProjectSetup(c.Request().Context(), c.Param("id")); errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		} else if err != nil {
			return err
		}
		ops, err := db.ListQueueOperations(c.Request().Context(), c.Param("id"))
		if err != nil {
			return err
		}
		items := make([]map[string]any, 0, len(ops))
		for _, op := range ops {
			items = append(items, queueOperationDTO(op))
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"operations": items})
	})
	e.GET("/api/v1/projects/:id/queue/operations/:key", func(c *echo.Context) error {
		op, err := db.QueueOperation(c.Request().Context(), c.Param("id"), c.Param("key"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "operation_not_found", "Queue operation was not found.")
		}
		if err != nil {
			return err
		}
		return c.JSON(200, queueOperationDTO(op))
	})
	e.POST("/api/v1/projects/:id/queue/operations/:key/inspect", func(c *echo.Context) error {
		op, err := queue.Inspect(c.Request().Context(), db, c.Param("id"), c.Param("key"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "operation_not_found", "Queue operation was not found.")
		}
		if err != nil {
			return err
		}
		return c.JSON(200, queueOperationDTO(op))
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
	if access == nil {
		// The fixed dev UI proxy is the only additional origin.
		_ = csrf.AddTrustedOrigin("http://127.0.0.1:5173")
		_ = csrf.AddTrustedOrigin("http://localhost:5173")
	}
	protected := csrf.Handler(e)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if access != nil {
			remote, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || net.ParseIP(remote) == nil || !net.ParseIP(remote).IsLoopback() || r.Host != access.Hostname() || len(r.Header.Values("Tailscale-User-Login")) != 1 || r.Header.Get("Tailscale-User-Login") != access.AllowedLogin {
				http.Error(w, "Private operator access required", http.StatusForbidden)
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && r.Header.Get("Origin") != access.Origin {
				http.Error(w, "Private origin required", http.StatusForbidden)
				return
			}
		} else {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			ip := net.ParseIP(host)
			if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
				http.Error(w, "Loopback host required", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		r.Body = http.MaxBytesReader(w, r.Body, 300<<10)
		protected.ServeHTTP(w, r)
	})
}
