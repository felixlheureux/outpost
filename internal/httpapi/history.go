package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"sync"

	"github.com/felixlheureux/outpost/internal/delivery"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/labstack/echo/v5"
)

func registerHistoryRoutes(e *echo.Echo, db *store.Store) {
	var reportMu sync.Mutex
	e.GET("/api/v1/history", func(c *echo.Context) error {
		offset := 0
		if raw := c.QueryParam("offset"); raw != "" {
			var err error
			offset, err = strconv.Atoi(raw)
			if err != nil || offset < 0 || offset > 10000 {
				return fail(c, 400, "invalid_offset", "Choose a valid history offset.")
			}
		}
		projectID := c.QueryParam("project")
		if projectID != "" {
			var exists int
			if err := db.DB.QueryRowContext(c.Request().Context(), "SELECT 1 FROM projects WHERE id=?", projectID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
				return fail(c, 404, "project_not_found", "Repository was not found.")
			} else if err != nil {
				return err
			}
		}
		items, err := db.ListHistory(c.Request().Context(), projectID, 30, offset)
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"runs": items, "next_offset": offset + len(items)})
	})
	e.GET("/api/v1/history/:id", func(c *echo.Context) error {
		item, err := db.HistoryDetail(c.Request().Context(), c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "run_not_found", "Run was not found.")
		}
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, item)
	})
	e.GET("/api/v1/history/:id/artifacts", func(c *echo.Context) error {
		if _, err := db.GetRun(c.Request().Context(), c.Param("id")); errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "run_not_found", "Run was not found.")
		} else if err != nil {
			return err
		}
		items, err := db.ListHistoryArtifacts(c.Request().Context(), c.Param("id"))
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"artifacts": items})
	})
	e.POST("/api/v1/runs/:id/reports/retry", func(c *echo.Context) error {
		reportMu.Lock()
		defer reportMu.Unlock()
		run, err := db.GetRun(c.Request().Context(), c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "run_not_found", "Run was not found.")
		}
		if err != nil {
			return err
		}
		if run.ReservationState != "released" {
			return fail(c, 409, "run_active", "Wait until the recorded worker and checkout are released before retrying GitHub reporting.")
		}
		schedule, err := db.GetSchedule(c.Request().Context(), run.ID)
		if err != nil {
			return err
		}
		var policy struct {
			CloseIssue bool `json:"close_issue"`
		}
		if err := json.Unmarshal([]byte(schedule.PolicyJSON), &policy); err != nil {
			return err
		}
		pending, err := db.UnreportedAssignments(c.Request().Context(), run.ID)
		if err != nil {
			return err
		}
		settled := 0
		for _, a := range pending {
			if _, err := delivery.Report(c.Request().Context(), db, a, policy.CloseIssue, nil); err != nil {
				return fail(c, 409, "report_pending", "GitHub reporting remains pending. Inspect the retained report reason and source access before retrying.")
			}
			settled++
		}
		return c.JSON(200, map[string]int{"settled": settled})
	})
}
