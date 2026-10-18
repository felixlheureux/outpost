package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/felixlheureux/outpost/internal/codexadapter"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/supervisor"
	"github.com/felixlheureux/outpost/internal/validation"
	"github.com/labstack/echo/v5"
)

func registerStartRoute(e *echo.Echo, db *store.Store, stateDir string, private bool, logger *slog.Logger) {
	e.POST("/api/v1/projects/:id/runs", func(c *echo.Context) error {
		if !private || stateDir == "" {
			return fail(c, 503, "execution_unavailable", "Start work from the installed private service.")
		}
		var input struct {
			Mode         string `json:"mode"`
			Selected     int    `json:"selected"`
			Model        string `json:"model"`
			Reasoning    string `json:"reasoning"`
			Delivery     string `json:"delivery"`
			Review       string `json:"review"`
			CloseIssue   bool   `json:"close_issue"`
			TaskLimit    int    `json:"task_limit"`
			DeadlineAt   string `json:"deadline_at"`
			Instructions string `json:"instructions"`
			RequestKey   string `json:"request_key"`
		}
		decoder := json.NewDecoder(io.LimitReader(c.Request().Body, 40<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return fail(c, 400, "invalid_start", "Check the start settings.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_start", "Send one start request.")
		}
		if input.Mode != "follow" && input.Mode != "fixed" || input.TaskLimit < 1 || input.TaskLimit > 100 || len(input.Instructions) > 32<<10 || len(input.RequestKey) < 8 || len(input.RequestKey) > 128 {
			return fail(c, 400, "invalid_start", "Check the scope, budget and request identity.")
		}
		if input.DeadlineAt != "" {
			deadline, parseErr := time.Parse(time.RFC3339Nano, input.DeadlineAt)
			if parseErr != nil || deadline.After(time.Now().Add(7*24*time.Hour)) {
				return fail(c, 400, "invalid_deadline", "Choose a runtime budget of at most seven days.")
			}
		}
		if input.Mode == "fixed" && input.Selected <= 0 || input.Mode == "follow" && input.Selected != 0 {
			return fail(c, 400, "invalid_scope", "Choose one fixed issue or Follow queue.")
		}
		if input.Delivery != "local_changes" && input.Delivery != "commit" && input.Delivery != "push" || input.Review != "none" && input.Review != "required" {
			return fail(c, 400, "invalid_policy", "Choose a supported delivery and review policy.")
		}
		ctx := c.Request().Context()
		if active, e := db.ActiveRun(ctx); e == nil && active.State == "completed" {
			if reconcileErr := supervisor.ReconcileTerminal(ctx, db, stateDir, active); reconcileErr != nil {
				return fail(c, 409, "slot_reconciliation", "Previous worker needs ownership reconciliation before another run.")
			}
		}
		project, err := db.GetProjectSetup(ctx, c.Param("id"))
		if err != nil {
			return fail(c, 404, "project_not_found", "Repository was not found.")
		}
		if project.State != "ready" {
			return fail(c, 409, "project_not_ready", "Finish repository setup before starting work.")
		}
		path, err := codexadapter.FindExecutable()
		if err != nil {
			return fail(c, 503, "provider_missing", "Install Codex for the service user.")
		}
		capabilities, err := codexadapter.Probe(ctx, path, project.WorkspacePath)
		if err != nil {
			return fail(c, 503, "provider_unavailable", "Codex capability discovery failed.")
		}
		valid := false
		for _, item := range capabilities.Models {
			if item.ID == input.Model {
				for _, effort := range item.ReasoningEfforts {
					if effort.Effort == input.Reasoning {
						valid = true
					}
				}
			}
		}
		if !valid {
			return fail(c, 400, "invalid_provider_settings", "Select a discovered model and reasoning effort.")
		}
		raw, err := db.DefaultCheckProfile(ctx, project.ID)
		if err != nil {
			return fail(c, 409, "checks_unconfigured", "Configure a default project check profile before starting.")
		}
		profile, err := validation.Parse([]byte(raw))
		if err != nil {
			return fail(c, 409, "checks_invalid", "Configured check profile is invalid.")
		}
		scope := []int{}
		if input.Mode == "fixed" {
			scope = []int{input.Selected}
		}
		scopeJSON, _ := json.Marshal(scope)
		policyJSON, _ := json.Marshal(map[string]any{"agent": map[string]string{"model": input.Model, "reasoning": input.Reasoning, "sandbox": "workspace-write", "approval_policy": "on-request"}, "check_profile": profile, "delivery": input.Delivery, "review": input.Review, "close_issue": input.CloseIssue})
		run, replay, err := db.ReserveScheduledRun(ctx, project.ID, project.WorkspacePath, input.RequestKey, store.Schedule{Kind: "queue", Mode: input.Mode, ScopeJSON: string(scopeJSON), PolicyJSON: string(policyJSON), Instructions: input.Instructions, TaskLimit: input.TaskLimit, DeadlineAt: input.DeadlineAt})
		if errors.Is(err, store.ErrSlotBusy) {
			return fail(c, 409, "slot_busy", "Another run owns the execution slot.")
		}
		if err != nil {
			return fail(c, 409, "start_conflict", "Start settings changed or the run could not be reserved.")
		}
		if replay && (run.State == "completed" || run.State == "stopped") {
			return c.JSON(202, map[string]any{"run_id": run.ID, "state": "completed"})
		}
		executable, e := os.Executable()
		if e != nil {
			return e
		}
		if _, e = supervisor.Launch(ctx, db, run, executable, stateDir, false); e != nil {
			logger.Warn("worker launch needs reconciliation", "run_id", run.ID, "error", e)
			return c.JSON(202, map[string]any{"run_id": run.ID, "state": "launch_unknown"})
		}
		return c.JSON(202, map[string]any{"run_id": run.ID, "state": "reserved"})
	})
}
