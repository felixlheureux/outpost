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
	ReservationState                                                string
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
	err := row.Scan(&run.ID, &run.ProjectID, &run.RequestKey, &run.WorkspacePath, &run.LaunchToken, &run.UnitName, &run.State, &run.LaunchObservation, &pid, &ticks, &run.ReservationState)
	if pid.Valid {
		run.WorkerPID = pid.Int64
	}
	if ticks.Valid {
		run.WorkerStartTicks = ticks.String
	}
	return run, err
}

const runColumns = "id, project_id, request_key, workspace_path, launch_token, unit_name, state, launch_observation, worker_pid, worker_start_ticks, reservation_state"

func (s *Store) GetRun(ctx context.Context, id string) (Run, error) {
	return scanRun(s.DB.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE id = ?", id))
}

func (s *Store) ActiveRun(ctx context.Context) (Run, error) {
	return scanRun(s.DB.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE reservation_state='reserved' LIMIT 1"))
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
		if checkErr := tx.QueryRowContext(ctx, "SELECT count(*) FROM runs WHERE reservation_state='reserved'").Scan(&occupied); checkErr == nil && occupied > 0 {
			return zero, false, ErrSlotBusy
		}
		return zero, false, err
	}
	if err := tx.Commit(); err != nil {
		return zero, false, err
	}
	return Run{ID: id, ProjectID: projectID, RequestKey: requestKey, WorkspacePath: workspace, LaunchToken: token, UnitName: unit, State: "reserved", LaunchObservation: "pending", ReservationState: "reserved"}, false, nil
}

// ReleaseReservation is called only after the supervisor has verified the
// exact worker unit is stopped and the workspace advisory lock is obtainable.
func (s *Store) ReleaseReservation(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM runs WHERE id=? AND reservation_state='reserved'`, id).Scan(&state); err != nil {
		return err
	}
	if state != "running" && state != "interrupted" && state != "completed" && state != "stopped" {
		return ErrAdmissionChanged
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `UPDATE execution_generations SET state='interrupted' WHERE assignment_id IN (SELECT id FROM assignments WHERE run_id=? AND state IN ('pending','running'))`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE assignments SET state='interrupted',settled_at=? WHERE run_id=? AND state IN ('pending','running')`, now, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE assignment_attempts SET phase='settled',outcome='interrupted',settled_at=? WHERE assignment_id IN (SELECT id FROM assignments WHERE run_id=?) AND outcome IN ('pending','running')`, now, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE run_schedules SET outcome='stopped',reason='worker stopped after owned processes settled',revision=revision+1 WHERE run_id=? AND outcome='active'`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE runs SET reservation_state='released',state=CASE WHEN state='completed' OR (SELECT outcome FROM run_schedules WHERE run_id=runs.id)='complete' THEN 'completed' ELSE 'stopped' END,updated_at=? WHERE id=? AND reservation_state='reserved'`, now, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return tx.Commit()
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
