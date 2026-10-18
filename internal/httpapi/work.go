package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/labstack/echo/v5"
)

func registerWorkRoutes(e *echo.Echo, db *store.Store) {
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
		return c.JSON(200, map[string]any{"run_id": overview.RunID, "project_id": overview.ProjectID, "run_state": overview.RunState, "assignment_id": overview.AssignmentID, "assignment_started_at": overview.AssignmentStartedAt, "attempt_id": overview.AttemptID, "attempt_phase": overview.AttemptPhase, "attempt_outcome": overview.AttemptOutcome, "provider_state": overview.ProviderState, "provider_failure_code": overview.ProviderFailureCode, "thread_id": overview.ThreadID, "turn_id": overview.TurnID, "turn_open": overview.TurnOpen, "issue_number": overview.IssueNumber, "snapshot": snapshot, "model": overview.Model, "reasoning": overview.Reasoning, "sandbox": overview.Sandbox, "approval_policy": overview.ApprovalPolicy, "successful_count": overview.SuccessfulCount, "task_limit": overview.TaskLimit, "stop_requested": overview.StopRequested, "schedule_outcome": overview.ScheduleOutcome, "schedule_reason": overview.ScheduleReason, "messages": overview.Messages, "controls": overview.Controls, "attempts": overview.Attempts})
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
