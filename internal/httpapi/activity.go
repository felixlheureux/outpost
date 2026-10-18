package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/labstack/echo/v5"
)

func cursorParam(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return v, nil
}

func registerActivityRoutes(e *echo.Echo, db *store.Store, stateDir string, slots chan struct{}) {
	e.GET("/api/v1/activity/storage", func(c *echo.Context) error {
		count, bytes, truncated, err := db.ArtifactStorage(c.Request().Context())
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"artifact_count": count, "recorded_bytes": bytes, "truncated_count": truncated, "capture_limit_bytes": 8 << 20})
	})
	e.GET("/api/v1/runs/:id/activity", func(c *echo.Context) error {
		runID := c.Param("id")
		snapshot, err := db.ReadActivitySnapshot(c.Request().Context(), runID)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "run_not_found", "Run was not found.")
		}
		if err != nil {
			return err
		}
		before, err := cursorParam(c.QueryParam("before"))
		if err != nil {
			return fail(c, 400, "invalid_cursor", "Use a nonnegative activity cursor.")
		}
		limit := store.ActivityPageMax
		if raw := c.QueryParam("limit"); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil || v < 1 || v > store.ActivityPageMax {
				return fail(c, 400, "invalid_limit", "Choose 1 to 100 activity entries.")
			}
			limit = v
		}
		items, err := db.ListActivity(c.Request().Context(), runID, 0, before, limit)
		if err != nil {
			return err
		}
		var next int64
		if len(items) > 0 && items[0].Cursor > snapshot.OldestCursor {
			next = items[0].Cursor
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(200, map[string]any{"snapshot": snapshot, "events": items, "next_before": next, "history_truncated": snapshot.PrunedThrough > 0})
	})
	e.GET("/api/v1/activity/stream", func(c *echo.Context) error {
		runID := c.QueryParam("run")
		if runID == "" {
			return fail(c, 400, "run_required", "Choose a run to observe.")
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			return fail(c, 503, "too_many_streams", "Too many live viewers; retry shortly.")
		}
		snapshot, err := db.ReadActivitySnapshot(c.Request().Context(), runID)
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "run_not_found", "Run was not found.")
		}
		if err != nil {
			return err
		}
		requested := c.Request().Header.Get("Last-Event-ID")
		if requested == "" {
			requested = c.QueryParam("after")
		}
		cursor, err := cursorParam(requested)
		if err != nil {
			return fail(c, 400, "invalid_cursor", "Use a nonnegative activity cursor.")
		}
		w := c.Response()
		flusher, ok := w.(http.Flusher)
		if !ok {
			return fail(c, 501, "stream_unavailable", "Streaming is unavailable on this server.")
		}
		h := c.Response().Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Accel-Buffering", "no")
		if err := sendSnapshot(w, flusher, snapshot); err != nil {
			return nil
		}
		if requested == "" {
			cursor = snapshot.Cursor
		}
		if requested != "" && ((snapshot.PrunedThrough > 0 && cursor <= snapshot.PrunedThrough) || cursor > snapshot.Cursor) {
			if err := sendSSE(w, flusher, "reset", snapshot.Cursor, map[string]any{"reason": "cursor_expired", "snapshot": snapshot}); err != nil {
				return nil
			}
			cursor = snapshot.Cursor
		}
		poll := time.NewTicker(750 * time.Millisecond)
		defer poll.Stop()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		for {
			fresh, err := db.ReadActivitySnapshot(c.Request().Context(), runID)
			if err != nil {
				return nil
			}
			if fresh.PrunedThrough > 0 && cursor <= fresh.PrunedThrough {
				if err := sendSSE(w, flusher, "reset", fresh.Cursor, map[string]any{"reason": "cursor_expired", "snapshot": fresh}); err != nil {
					return nil
				}
				cursor = fresh.Cursor
			}
			events, err := db.ReplayActivity(c.Request().Context(), runID, cursor)
			if err != nil {
				return nil
			}
			for _, event := range events {
				if err := sendSSE(w, flusher, event.Kind, event.Cursor, event); err != nil {
					return nil
				}
				cursor = event.Cursor
			}
			if len(events) == store.ActivityPageMax {
				continue
			}
			select {
			case <-c.Request().Context().Done():
				return nil
			case <-poll.C:
			case <-heartbeat.C:
				if err := writeSSE(w, flusher, ": heartbeat\n\n"); err != nil {
					return nil
				}
			}
		}
	})
	e.GET("/api/v1/artifacts/:id", func(c *echo.Context) error {
		a, err := db.GetActivityArtifact(c.Request().Context(), c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "artifact_not_found", "Artifact was not found.")
		}
		if err != nil {
			return err
		}
		if a.RetentionState == "removed" {
			return fail(c, 410, "artifact_removed", "Retained output was removed.")
		}
		if a.RetentionState != "retained" {
			return fail(c, 409, "artifact_uncertain", "Artifact retention is being reconciled.")
		}
		if stateDir == "" {
			return fail(c, 503, "artifact_unavailable", "Artifact storage is not configured.")
		}
		f, err := openArtifact(stateDir, a)
		if errors.Is(err, os.ErrNotExist) {
			return fail(c, 410, "artifact_removed", "Retained output is no longer available.")
		}
		if err != nil {
			return fail(c, 409, "artifact_unsafe", "Artifact could not be opened safely.")
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		if a.Truncated {
			c.Response().Header().Set("X-Outpost-Truncated", "true")
		}
		if c.QueryParam("download") == "1" {
			c.Response().Header().Set("Content-Disposition", `attachment; filename="provider-events.jsonl"`)
			c.Response().Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
			http.ServeContent(c.Response(), c.Request(), "provider-events.jsonl", info.ModTime(), f)
			return nil
		}
		offset, err := cursorParam(c.QueryParam("offset"))
		if err != nil {
			return fail(c, 400, "invalid_offset", "Use a nonnegative byte offset.")
		}
		limit := 12 << 10
		if raw := c.QueryParam("limit"); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil || v < 4 || v > 64<<10 {
				return fail(c, 400, "invalid_limit", "Choose 4 to 65536 preview bytes.")
			}
			limit = v
		}
		if offset > info.Size() {
			return fail(c, 416, "offset_out_of_range", "Offset exceeds retained output.")
		}
		if offset < info.Size() {
			var first [1]byte
			if _, err := f.ReadAt(first[:], offset); err != nil {
				return err
			}
			if first[0]&0xc0 == 0x80 {
				return fail(c, 400, "invalid_offset", "Offset must start at a UTF-8 character boundary.")
			}
		}
		b := make([]byte, limit)
		n, err := f.ReadAt(b, offset)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		b = b[:n]
		for len(b) > 0 && !utf8.Valid(b) {
			b = b[:len(b)-1]
		}
		return c.JSON(200, map[string]any{"artifact_id": a.ID, "text": string(b), "offset": offset, "next_offset": offset + int64(len(b)), "retained_bytes": info.Size(), "capture_truncated": a.Truncated, "preview_truncated": offset+int64(len(b)) < info.Size(), "download_url": "/api/v1/artifacts/" + a.ID + "?download=1"})
	})
	e.DELETE("/api/v1/artifacts/:id", func(c *echo.Context) error {
		if stateDir == "" {
			return fail(c, 503, "artifact_unavailable", "Artifact storage is not configured.")
		}
		a, err := db.GetActivityArtifact(c.Request().Context(), c.Param("id"))
		if errors.Is(err, sql.ErrNoRows) {
			return fail(c, 404, "artifact_not_found", "Artifact was not found.")
		}
		if err != nil {
			return err
		}
		if a.RetentionState == "removed" {
			return c.JSON(200, map[string]string{"state": "removed"})
		}
		if err := db.BeginArtifactRemoval(c.Request().Context(), a.ID); err != nil {
			return fail(c, 409, "artifact_active", "Only settled, released run output can be removed.")
		}
		f, err := openArtifact(stateDir, a)
		if err == nil {
			_ = f.Close()
			err = os.Remove(filepath.Join(stateDir, a.RelativePath))
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = db.FinishArtifactRemoval(c.Request().Context(), a.ID, "unknown")
			return fail(c, 409, "artifact_uncertain", "Artifact removal needs inspection before retry.")
		}
		if err := db.FinishArtifactRemoval(c.Request().Context(), a.ID, "removed"); err != nil {
			return err
		}
		return c.JSON(200, map[string]string{"state": "removed"})
	})
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, kind string, cursor int64, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeSSE(w, flusher, "id: "+strconv.FormatInt(cursor, 10)+"\nevent: "+kind+"\ndata: "+string(b)+"\n\n")
}

func sendSnapshot(w http.ResponseWriter, flusher http.Flusher, value store.ActivitySnapshot) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// A snapshot is informational. Advancing EventSource's replay ID before the
	// pending events have been delivered could skip them on another disconnect.
	return writeSSE(w, flusher, "event: snapshot\ndata: "+string(b)+"\n\n")
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, value string) error {
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	_, err := io.WriteString(w, value)
	if err == nil {
		flusher.Flush()
	}
	return err
}

func openArtifact(stateDir string, a store.ActivityArtifact) (*os.File, error) {
	safeID := regexp.MustCompile(`^[A-Za-z0-9_]{1,100}$`)
	if a.Kind != "provider-events" || !safeID.MatchString(a.AssignmentID) || !safeID.MatchString(a.RunID) || !safeID.MatchString(a.AttemptID) || (a.ID != "artifact_"+a.AssignmentID && a.ID != "artifact_"+a.AttemptID) || a.RelativePath != "artifacts/"+a.RunID+"/"+a.AttemptID+"/provider-events.jsonl" {
		return nil, fmt.Errorf("invalid artifact manifest")
	}
	root, err := os.OpenRoot(stateDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for _, dir := range []string{"artifacts", "artifacts/" + a.RunID, "artifacts/" + a.RunID + "/" + a.AttemptID} {
		parent, err := root.Lstat(dir)
		if err != nil {
			return nil, err
		}
		if !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unsafe artifact directory")
		}
	}
	info, err := root.Lstat(a.RelativePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe artifact type")
	}
	f, err := root.Open(filepath.ToSlash(a.RelativePath))
	if err != nil {
		return nil, err
	}
	return f, nil
}
