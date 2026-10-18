package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrSlotBusy = errors.New("execution slot is reserved")
var ErrRunConflict = errors.New("run request conflicts with its original scope")

type Run struct {
	ID, ProjectID, RequestKey, WorkspacePath, LaunchToken, UnitName string
	State, LaunchObservation                                        string
	WorkerPID                                                       int64
	WorkerStartTicks                                                string
}

func newRandomID(prefix string, bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

func canonicalWorkspace(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("workspace path must be absolute")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("workspace must be an existing directory")
	}
	return canonical, nil
}

func scanRun(row *sql.Row) (Run, error) {
	var run Run
	var pid sql.NullInt64
	var ticks sql.NullString
	err := row.Scan(&run.ID, &run.ProjectID, &run.RequestKey, &run.WorkspacePath, &run.LaunchToken, &run.UnitName, &run.State, &run.LaunchObservation, &pid, &ticks)
	if pid.Valid {
		run.WorkerPID = pid.Int64
	}
	if ticks.Valid {
		run.WorkerStartTicks = ticks.String
	}
	return run, err
}

const runColumns = "id, project_id, request_key, workspace_path, launch_token, unit_name, state, launch_observation, worker_pid, worker_start_ticks"

func (s *Store) GetRun(ctx context.Context, id string) (Run, error) {
	return scanRun(s.DB.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE id = ?", id))
}

func (s *Store) ActiveRun(ctx context.Context) (Run, error) {
	return scanRun(s.DB.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE state IN ('reserved','running','interrupted') LIMIT 1"))
}

// ReserveRun commits the global slot and immutable unit/token before any
// systemd request. The caller must then observe that exact unit on uncertainty.
func (s *Store) ReserveRun(ctx context.Context, projectID, workspacePath, requestKey string) (Run, bool, error) {
	var zero Run
	if projectID == "" || len(requestKey) < 8 || len(requestKey) > 128 || strings.TrimSpace(requestKey) != requestKey {
		return zero, false, fmt.Errorf("invalid run project or request key")
	}
	workspace, err := canonicalWorkspace(workspacePath)
	if err != nil {
		return zero, false, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return zero, false, err
	}
	defer tx.Rollback()
	if existing, err := scanRun(tx.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE request_key = ?", requestKey)); err == nil {
		if existing.ProjectID != projectID || existing.WorkspacePath != workspace {
			return zero, false, ErrRunConflict
		}
		return existing, true, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return zero, false, err
	}
	id, err := newRandomID("run_", 16)
	if err != nil {
		return zero, false, err
	}
	token, err := newRandomID("", 32)
	if err != nil {
		return zero, false, err
	}
	unit := "outpost-" + id + ".service"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO runs (id, project_id, request_key, workspace_path, launch_token, unit_name, state, launch_observation, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'reserved', 'pending', ?, ?)`, id, projectID, requestKey, workspace, token, unit, now, now)
	if err != nil {
		var occupied int
		if checkErr := tx.QueryRowContext(ctx, "SELECT count(*) FROM runs WHERE state IN ('reserved','running','interrupted')").Scan(&occupied); checkErr == nil && occupied > 0 {
			return zero, false, ErrSlotBusy
		}
		return zero, false, err
	}
	if err := tx.Commit(); err != nil {
		return zero, false, err
	}
	return Run{ID: id, ProjectID: projectID, RequestKey: requestKey, WorkspacePath: workspace, LaunchToken: token, UnitName: unit, State: "reserved", LaunchObservation: "pending"}, false, nil
}

// ObserveLaunch records only the systemd launch observation, never task success.
func (s *Store) ObserveLaunch(ctx context.Context, id, observation string) error {
	if observation != "acknowledged" && observation != "unknown" {
		return fmt.Errorf("invalid launch observation")
	}
	result, err := s.DB.ExecContext(ctx, "UPDATE runs SET launch_observation = ?, updated_at = ? WHERE id = ? AND state IN ('reserved','running')", observation, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return fmt.Errorf("run is no longer reserved")
	}
	return nil
}

func (s *Store) InterruptWorker(ctx context.Context, id string, pid int, startTicks string) error {
	result, err := s.DB.ExecContext(ctx, "UPDATE runs SET state = 'interrupted', updated_at = ? WHERE id = ? AND state = 'running' AND worker_pid = ? AND worker_start_ticks = ?", time.Now().UTC().Format(time.RFC3339Nano), id, pid, startTicks)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return fmt.Errorf("worker interruption did not match its recorded identity")
	}
	return nil
}

// ClaimWorker is guarded by the immutable launch token. A second process
// cannot claim the same run, even if it inherited an old start request.
func (s *Store) ClaimWorker(ctx context.Context, id, token string, pid int, startTicks string) error {
	if pid <= 0 || startTicks == "" {
		return fmt.Errorf("worker process identity is incomplete")
	}
	result, err := s.DB.ExecContext(ctx, "UPDATE runs SET state = 'running', worker_pid = ?, worker_start_ticks = ?, updated_at = ? WHERE id = ? AND launch_token = ? AND state = 'reserved' AND worker_pid IS NULL", pid, startTicks, time.Now().UTC().Format(time.RFC3339Nano), id, token)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return fmt.Errorf("worker claim was rejected")
	}
	return nil
}
