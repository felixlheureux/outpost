package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/validation"
	"github.com/labstack/echo/v5"
)

func registerWorkRoutes(e *echo.Echo, db *store.Store, stateDir string) {
	e.GET("/api/v1/runs/:id/changes", func(c *echo.Context) error { return changesForRun(c, db) })
	e.GET("/api/v1/runs/:id/work", func(c *echo.Context) error {
		overview, err := db.WorkOverview(c.Request().Context(), c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "run_not_found", "Run was not found.")
		}
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		var snapshot any
		if overview.SnapshotJSON != "" {
			snapshot = json.RawMessage(overview.SnapshotJSON)
		}
		checks := []map[string]any{}
		seenChecks := map[string]bool{}
		if overview.AssignmentID != "" {
			items, err := db.ListChecks(c.Request().Context(), overview.AssignmentID)
			if err != nil {
				return err
			}
			for _, item := range items {
				seenChecks[item.CheckID] = true
				var exitCode, caseCount any
				if item.ExitCode.Valid {
					exitCode = item.ExitCode.Int64
				}
				if item.CaseCount.Valid {
					caseCount = item.CaseCount.Int64
				}
				checks = append(checks, map[string]any{"id": item.ID, "check_id": item.CheckID, "name": item.Name, "required": item.Required, "status": item.Status, "reason": item.Reason, "input_digest": item.InputDigest, "candidate_digest": item.CandidateDigest, "contract_digest": item.ContractDigest, "duration_ms": item.DurationMS, "exit_code": exitCode, "signal": item.Signal, "case_count": caseCount, "require_cases": item.RequireCases, "started_at": item.StartedAt, "finished_at": item.FinishedAt, "pid": item.PID, "output_bytes": item.OutputBytes, "truncated": item.Truncated})
			}
		}
		if overview.AssignmentID != "" {
			schedule, err := db.GetSchedule(c.Request().Context(), overview.RunID)
			if err != nil {
				return err
			}
			var policy struct {
				CheckProfile validation.Profile `json:"check_profile"`
			}
			if json.Unmarshal([]byte(schedule.PolicyJSON), &policy) == nil {
				for _, item := range policy.CheckProfile.Checks {
					if seenChecks[item.ID] {
						continue
					}
					checks = append(checks, map[string]any{"id": "not-run-" + item.ID, "check_id": item.ID, "name": item.Name, "required": item.Required, "status": "not-run", "reason": "Awaiting runner validation", "input_digest": "", "candidate_digest": "", "contract_digest": "", "duration_ms": 0, "exit_code": nil, "signal": "", "case_count": nil, "require_cases": item.RequireCases, "started_at": "", "finished_at": "", "pid": 0, "output_bytes": 0, "truncated": false})
				}
			}
		}
		return c.JSON(200, map[string]any{"run_id": overview.RunID, "project_id": overview.ProjectID, "run_state": overview.RunState, "assignment_id": overview.AssignmentID, "assignment_started_at": overview.AssignmentStartedAt, "attempt_id": overview.AttemptID, "attempt_phase": overview.AttemptPhase, "attempt_outcome": overview.AttemptOutcome, "provider_state": overview.ProviderState, "provider_failure_code": overview.ProviderFailureCode, "thread_id": overview.ThreadID, "turn_id": overview.TurnID, "turn_open": overview.TurnOpen, "issue_number": overview.IssueNumber, "snapshot": snapshot, "model": overview.Model, "reasoning": overview.Reasoning, "sandbox": overview.Sandbox, "approval_policy": overview.ApprovalPolicy, "successful_count": overview.SuccessfulCount, "task_limit": overview.TaskLimit, "stop_requested": overview.StopRequested, "schedule_outcome": overview.ScheduleOutcome, "schedule_reason": overview.ScheduleReason, "messages": overview.Messages, "controls": overview.Controls, "attempts": overview.Attempts, "checks": checks})
	})
	e.GET("/api/v1/runs/:id/checks/:receipt/output", func(c *echo.Context) error {
		if stateDir == "" {
			return fail(c, 503, "output_unavailable", "Private check output is not configured.")
		}
		item, err := db.GetRunCheck(c.Request().Context(), c.Param("id"), c.Param("receipt"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "check_not_found", "Check receipt was not found.")
		}
		if err != nil {
			return err
		}
		if !regexp.MustCompile(`^check_[0-9a-f]{32}$`).MatchString(item.ID) {
			return fail(c, 409, "invalid_receipt", "Check receipt identity is invalid.")
		}
		root, err := os.OpenRoot(stateDir)
		if err != nil {
			return err
		}
		defer root.Close()
		rel, err := filepath.Rel(stateDir, item.OutputPath)
		if err != nil || filepath.Dir(rel) != filepath.Join("checks", c.Param("id")) || !regexp.MustCompile(`^check_[0-9a-f]{32}\.log$`).MatchString(filepath.Base(rel)) {
			return fail(c, 409, "output_unsafe", "Check output path is invalid.")
		}
		f, err := root.Open(filepath.ToSlash(rel))
		if errors.Is(err, os.ErrNotExist) {
			return fail(c, 410, "output_missing", "Retained check output is unavailable.")
		}
		if err != nil {
			return fail(c, 409, "output_unsafe", "Check output could not be opened safely.")
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
		if err != nil {
			return err
		}
		if len(b) > 1<<20 {
			return fail(c, 409, "output_oversize", "Check output exceeds retention limit.")
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return c.JSON(200, map[string]any{"id": item.ID, "output": string(b), "truncated": item.Truncated})
	})
	e.POST("/api/v1/runs/:id/messages", func(c *echo.Context) error {
		var input struct {
			ID             string `json:"id"`
			AttemptID      string `json:"attempt_id"`
			ThreadID       string `json:"thread_id"`
			ExpectedTurnID string `json:"expected_turn_id"`
			Target         string `json:"target"`
			Body           string `json:"body"`
		}
		decoder := json.NewDecoder(c.Request().Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return fail(c, 400, "invalid_message", "Provide a message ID, current attempt and turn target, and instructions.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_message", "Send one message.")
		}
		result, err := db.SubmitWorkMessage(c.Request().Context(), store.WorkMessage{ID: input.ID, RunID: c.Param("id"), AttemptID: input.AttemptID, ThreadID: input.ThreadID, ExpectedTurnID: input.ExpectedTurnID, Target: input.Target, Body: input.Body})
		if errors.Is(err, store.ErrWorkConflict) {
			return fail(c, 409, "message_conflict", "This message ID already has different content.")
		}
		if errors.Is(err, store.ErrStaleWorkTarget) {
			return fail(c, 409, "stale_target", "The targeted attempt or turn has changed. Your draft is preserved.")
		}
		if err != nil {
			return fail(c, 400, "invalid_message", err.Error())
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, result)
	})
	e.POST("/api/v1/runs/:id/controls", func(c *echo.Context) error {
		var input struct {
			ID           string `json:"id"`
			AttemptID    string `json:"attempt_id"`
			Action       string `json:"action"`
			Mode         string `json:"mode"`
			Instructions string `json:"instructions"`
		}
		decoder := json.NewDecoder(c.Request().Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return fail(c, 400, "invalid_control", "Provide a control ID, attempt, and action.")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return fail(c, 400, "invalid_control", "Send one control.")
		}
		result, err := db.SubmitWorkControl(c.Request().Context(), store.WorkControl{ID: input.ID, RunID: c.Param("id"), AttemptID: input.AttemptID, Action: input.Action, Mode: input.Mode, Instructions: input.Instructions})
		if errors.Is(err, store.ErrWorkConflict) {
			return fail(c, 409, "control_conflict", "This control ID already has different content.")
		}
		if errors.Is(err, store.ErrStaleWorkTarget) {
			return fail(c, 409, "stale_target", "The targeted run or attempt has changed.")
		}
		if err != nil {
			return fail(c, 400, "invalid_control", err.Error())
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, result)
	})
}
