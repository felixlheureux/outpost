package httpapi

import (
	"database/sql"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/labstack/echo/v5"
)

type changedPath struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

func changesForRun(c *echo.Context, db *store.Store) error {
	run, err := db.GetRun(c.Request().Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		return fail(c, 404, "run_not_found", "Run was not found.")
	}
	if err != nil {
		return err
	}
	raw, err := hostcmd.Run(c.Request().Context(), run.WorkspacePath, "git", nil, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return fail(c, 409, "changes_unavailable", "The owned checkout could not be inspected.")
	}
	parts := strings.Split(string(raw), "\x00")
	paths := make([]changedPath, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		entry := parts[i]
		if len(entry) < 4 {
			continue
		}
		status := entry[:2]
		path := entry[3:]
		if strings.ContainsAny(status, "RC") && i+1 < len(parts) {
			i++
		}
		paths = append(paths, changedPath{Path: path, Status: status})
		if len(paths) >= 200 {
			break
		}
	}
	requested := c.QueryParam("path")
	c.Response().Header().Set("Cache-Control", "no-store")
	if requested == "" {
		return c.JSON(200, map[string]any{"paths": paths, "truncated": len(paths) >= 200})
	}
	found := false
	untracked := false
	for _, path := range paths {
		if path.Path == requested {
			found = true
			untracked = path.Status == "??"
			break
		}
	}
	if !found {
		return fail(c, 409, "stale_path", "This path is no longer among the checkout changes.")
	}
	if untracked {
		root, err := os.OpenRoot(run.WorkspacePath)
		if err != nil {
			return fail(c, 409, "diff_unavailable", "Checkout content could not be opened.")
		}
		defer root.Close()
		file, err := root.Open(requested)
		if err != nil {
			return fail(c, 409, "diff_unavailable", "Changed file could not be opened safely.")
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return fail(c, 409, "diff_unavailable", "Changed path is not a regular file.")
		}
		content, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
		if err != nil {
			return fail(c, 409, "diff_unavailable", "Changed file could not be read.")
		}
		truncated := len(content) > 64<<10
		if truncated {
			content = content[:64<<10]
		}
		for len(content) > 0 && !utf8.Valid(content) {
			content = content[:len(content)-1]
		}
		if strings.ContainsRune(string(content), '\x00') {
			return c.JSON(200, map[string]any{"path": requested, "diff": "Binary untracked file; no text preview.", "truncated": truncated})
		}
		return c.JSON(200, map[string]any{"path": requested, "diff": string(content), "truncated": truncated})
	}
	diff, err := hostcmd.Run(c.Request().Context(), run.WorkspacePath, "git", nil, "diff", "--no-ext-diff", "--no-color", "HEAD", "--", requested)
	if err != nil {
		return fail(c, 409, "diff_unavailable", "Git could not read this changed path.")
	}
	truncated := len(diff) > 64<<10
	if truncated {
		diff = diff[:64<<10]
	}
	for len(diff) > 0 && !utf8.Valid(diff) {
		diff = diff[:len(diff)-1]
	}
	return c.JSON(200, map[string]any{"path": requested, "diff": string(diff), "truncated": truncated})
}
