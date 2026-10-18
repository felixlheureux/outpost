package httpapi

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/privateaccess"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/supervisor"
	"github.com/labstack/echo/v5"
)

func registerDiagnosticsRoute(e *echo.Echo, db *store.Store, access *privateaccess.Config, stateDir, version string) {
	e.GET("/api/v1/diagnostics", func(c *echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 9*time.Second)
		defer cancel()
		result := map[string]any{"version": version, "observed_at": time.Now().UTC().Format(time.RFC3339), "mode": "development", "private_url": "", "allowed_identity": "not configured", "tailscale": "not configured", "serve_route": "not configured", "web_service": "unknown", "github": "unknown", "provider": "unknown"}
		if access != nil {
			result["mode"] = "private"
			result["private_url"] = access.Origin
			result["allowed_identity"] = access.AllowedLogin
			if err := privateaccess.CheckNode(ctx, *access); err == nil {
				result["tailscale"] = "node matches configuration"
			} else {
				result["tailscale"] = "unavailable or node differs"
			}
			if present, err := privateaccess.ServeRoute(ctx, *access, "http://127.0.0.1:8080"); err == nil && present {
				result["serve_route"] = "Outpost route present"
			} else {
				result["serve_route"] = "unavailable or different route"
			}
		}
		if unit, err := supervisor.Observe(ctx, "outpost.service"); err == nil {
			result["web_service"] = unit.ActiveState
		} else {
			result["web_service"] = "unavailable"
		}
		if err := githubcli.CheckAuth(ctx); err == nil {
			result["github"] = "authenticated"
		} else {
			result["github"] = "unavailable or signed out"
		}
		if b, err := hostcmd.Run(ctx, "", "codex", nil, "--version"); err == nil && regexp.MustCompile(`^codex-cli [0-9]+\.[0-9]+\.[0-9]+$`).MatchString(strings.TrimSpace(string(b))) {
			result["provider_version"] = strings.TrimSpace(string(b))
			if _, err := hostcmd.Run(ctx, "", "codex", nil, "login", "status"); err == nil {
				result["provider"] = "signed in"
			} else {
				result["provider"] = "sign-in unavailable"
			}
		} else {
			result["provider"] = "unavailable"
		}
		if stateDir != "" {
			if count, bytes, truncated, err := db.ArtifactStorage(ctx); err == nil {
				result["artifacts"] = map[string]any{"count": count, "recorded_bytes": bytes, "truncated_count": truncated}
			}
			if info, err := os.Stat(stateDir); err == nil {
				result["state_access"] = info.IsDir() && info.Mode().Perm()&0077 == 0
			} else {
				result["state_access"] = false
			}
			var stat syscall.Statfs_t
			if err := syscall.Statfs(stateDir, &stat); err == nil {
				result["disk_available_bytes"] = stat.Bavail * uint64(stat.Bsize)
			}
		}
		if projectID := c.QueryParam("project"); projectID != "" {
			var setup string
			if err := db.DB.QueryRowContext(ctx, `SELECT setup_state FROM projects WHERE id=?`, projectID).Scan(&setup); err == nil {
				result["project_setup"] = setup
				var source, queue, observed sql.NullString
				_ = db.DB.QueryRowContext(ctx, `SELECT state,last_success_at FROM issue_source_status WHERE project_id=?`, projectID).Scan(&source, &observed)
				_ = db.DB.QueryRowContext(ctx, `SELECT state FROM queue_sources WHERE project_id=?`, projectID).Scan(&queue)
				result["issue_source"] = source.String
				result["queue_source"] = queue.String
				result["source_observed_at"] = observed.String
			}
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, result)
	})
}
