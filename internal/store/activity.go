package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

const ActivityPageMax = 100

type ActivityEvent struct {
	Cursor     int64           `json:"cursor"`
	ProjectID  string          `json:"project_id"`
	RunID      string          `json:"run_id"`
	AttemptID  string          `json:"attempt_id"`
	Kind       string          `json:"kind"`
	Payload    json.RawMessage `json:"payload"`
	ArtifactID string          `json:"artifact_id,omitempty"`
	CreatedAt  string          `json:"created_at"`
}

type ActivitySnapshot struct {
	Cursor          int64  `json:"cursor"`
	OldestCursor    int64  `json:"oldest_cursor"`
	PrunedThrough   int64  `json:"pruned_through"`
	RunID           string `json:"run_id,omitempty"`
	ProjectID       string `json:"project_id,omitempty"`
	RunState        string `json:"run_state,omitempty"`
	AssignmentID    string `json:"assignment_id,omitempty"`
	AttemptPhase    string `json:"attempt_phase,omitempty"`
	ProviderState   string `json:"provider_state,omitempty"`
	ProviderVersion string `json:"provider_version,omitempty"`
	Model           string `json:"model,omitempty"`
	Reasoning       string `json:"reasoning,omitempty"`
	Sandbox         string `json:"sandbox,omitempty"`
	ApprovalPolicy  string `json:"approval_policy,omitempty"`
}

func (s *Store) ReadActivitySnapshot(ctx context.Context, runID string) (ActivitySnapshot, error) {
	var v ActivitySnapshot
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT id,project_id,state FROM runs WHERE id=?`, runID).Scan(&v.RunID, &v.ProjectID, &v.RunState); err != nil {
		return v, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(cursor),0),COALESCE(MIN(cursor),0) FROM activity_events WHERE run_id=?`, runID).Scan(&v.Cursor, &v.OldestCursor); err != nil {
		return v, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT pruned_through FROM activity_cursor_floor WHERE run_id=?),0)`, runID).Scan(&v.PrunedThrough); err != nil {
		return v, err
	}
	var phase, provider, version, model, reasoning, sandbox, approval sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT a.id,t.phase,p.state,p.version,p.model,p.reasoning,p.sandbox,p.approval_policy FROM assignments a LEFT JOIN assignment_attempts t ON t.assignment_id=a.id AND t.outcome='running' LEFT JOIN provider_attempts p ON p.attempt_id=t.id WHERE a.run_id=? ORDER BY a.created_at DESC LIMIT 1`, runID).Scan(&v.AssignmentID, &phase, &provider, &version, &model, &reasoning, &sandbox, &approval)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return v, err
	}
	v.AttemptPhase, v.ProviderState, v.ProviderVersion, v.Model, v.Reasoning, v.Sandbox, v.ApprovalPolicy = phase.String, provider.String, version.String, model.String, reasoning.String, sandbox.String, approval.String
	return v, tx.Commit()
}

func scanActivity(rows *sql.Rows) (ActivityEvent, error) {
	var v ActivityEvent
	var project, run, attempt sql.NullString
	var payload string
	err := rows.Scan(&v.Cursor, &project, &run, &attempt, &v.Kind, &payload, &v.ArtifactID, &v.CreatedAt)
	v.ProjectID, v.RunID, v.AttemptID = project.String, run.String, attempt.String
	v.Payload = json.RawMessage(payload)
	return v, err
}

// ListActivity returns ascending cursors. before=0 selects the newest page;
// after>0 replays newer events. Both limits are fixed by the caller.
func (s *Store) ListActivity(ctx context.Context, runID string, after, before int64, limit int) ([]ActivityEvent, error) {
	if runID == "" || after < 0 || before < 0 || (after > 0 && before > 0) || limit < 1 || limit > ActivityPageMax {
		return nil, fmt.Errorf("invalid activity page")
	}
	query := `SELECT cursor,project_id,run_id,attempt_id,kind,payload_json,artifact_id,created_at FROM activity_events WHERE run_id=?`
	args := []any{runID}
	if after > 0 {
		query += ` AND cursor>?`
		args = append(args, after)
	}
	if before > 0 {
		query += ` AND cursor<?`
		args = append(args, before)
	}
	if after == 0 {
		query += ` ORDER BY cursor DESC`
	} else {
		query += ` ORDER BY cursor ASC`
	}
	query += ` LIMIT ?`
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ActivityEvent, 0, limit)
	for rows.Next() {
		v, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if after == 0 {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}
	return result, nil
}

func (s *Store) ReplayActivity(ctx context.Context, runID string, after int64) ([]ActivityEvent, error) {
	if runID == "" || after < 0 {
		return nil, fmt.Errorf("invalid replay cursor")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT cursor,project_id,run_id,attempt_id,kind,payload_json,artifact_id,created_at FROM activity_events WHERE run_id=? AND cursor>? ORDER BY cursor ASC LIMIT ?`, runID, after, ActivityPageMax)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ActivityEvent, 0, ActivityPageMax)
	for rows.Next() {
		v, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

type ActivityArtifact struct {
	ID, RunID, AttemptID, AssignmentID, Kind, RelativePath, RetentionState, RemovedAt, CreatedAt, UpdatedAt string
	BytesWritten                                                                                            int64 `json:"bytes_written"`
	Truncated                                                                                               bool  `json:"truncated"`
}

func (s *Store) BeginProviderArtifact(ctx context.Context, assignmentID string) (ActivityArtifact, error) {
	var a ActivityArtifact
	safeID := regexp.MustCompile(`^[A-Za-z0-9_]{1,100}$`)
	if !safeID.MatchString(assignmentID) {
		return a, fmt.Errorf("invalid assignment artifact identity")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT a.run_id,p.attempt_id FROM assignments a JOIN provider_attempts p ON p.assignment_id=a.id WHERE a.id=? AND p.state='pending' ORDER BY p.created_at DESC LIMIT 1`, assignmentID).Scan(&a.RunID, &a.AttemptID); err != nil {
		return a, err
	}
	if !safeID.MatchString(a.RunID) || !safeID.MatchString(a.AttemptID) {
		return a, fmt.Errorf("invalid run or attempt artifact identity")
	}
	a.ID = "artifact_" + a.AttemptID
	a.AssignmentID = assignmentID
	a.Kind = "provider-events"
	a.RelativePath = "artifacts/" + a.RunID + "/" + a.AttemptID + "/provider-events.jsonl"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_artifacts(id,run_id,attempt_id,assignment_id,kind,relative_path,created_at,updated_at) VALUES(?,?,?,?,'provider-events',?,?,?)`, a.ID, a.RunID, a.AttemptID, a.AssignmentID, a.RelativePath, now, now); err != nil {
		return a, err
	}
	return a, tx.Commit()
}

func (s *Store) UpdateArtifact(ctx context.Context, id string, bytes int64, truncated bool) error {
	if bytes < 0 {
		return fmt.Errorf("invalid artifact byte count")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE activity_artifacts SET bytes_written=?,truncated=?,updated_at=? WHERE id=? AND bytes_written<=?`, bytes, truncated, time.Now().UTC().Format(time.RFC3339Nano), id, bytes)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) GetActivityArtifact(ctx context.Context, id string) (ActivityArtifact, error) {
	var a ActivityArtifact
	err := s.DB.QueryRowContext(ctx, `SELECT id,run_id,attempt_id,assignment_id,kind,relative_path,bytes_written,truncated,retention_state,removed_at,created_at,updated_at FROM activity_artifacts WHERE id=?`, id).Scan(&a.ID, &a.RunID, &a.AttemptID, &a.AssignmentID, &a.Kind, &a.RelativePath, &a.BytesWritten, &a.Truncated, &a.RetentionState, &a.RemovedAt, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func (s *Store) ArtifactStorage(ctx context.Context) (count, bytes, truncated int64, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT count(*),COALESCE(sum(bytes_written),0),COALESCE(sum(truncated),0) FROM activity_artifacts WHERE retention_state='retained'`).Scan(&count, &bytes, &truncated)
	return
}

func (s *Store) BeginArtifactRemoval(ctx context.Context, id string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE activity_artifacts SET retention_state='pending',updated_at=? WHERE id=? AND retention_state IN ('retained','pending','unknown') AND EXISTS(SELECT 1 FROM runs r JOIN assignments a ON a.run_id=r.id WHERE r.id=activity_artifacts.run_id AND a.id=activity_artifacts.assignment_id AND r.reservation_state='released' AND a.state IN ('succeeded','failed','blocked','interrupted'))`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) FinishArtifactRemoval(ctx context.Context, id, state string) error {
	if state != "removed" && state != "unknown" {
		return fmt.Errorf("invalid artifact removal state")
	}
	when := ""
	if state == "removed" {
		when = time.Now().UTC().Format(time.RFC3339Nano)
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE activity_artifacts SET retention_state=?,removed_at=?,updated_at=? WHERE id=? AND retention_state='pending'`, state, when, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) AppendProviderActivity(ctx context.Context, assignmentID, category, summary, artifactID string) error {
	if len(category) > 60 || len(summary) > 12<<10 || category == "" || artifactID == "" {
		return fmt.Errorf("invalid provider activity")
	}
	payload, err := json.Marshal(map[string]string{"category": category, "summary": summary})
	if err != nil {
		return err
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO activity_events(project_id,run_id,attempt_id,kind,payload_json,artifact_id,created_at)
	 SELECT a.project_id,a.run_id,p.attempt_id,'activity.appended',?,?,? FROM assignments a JOIN provider_attempts p ON p.assignment_id=a.id WHERE a.id=? AND p.state IN ('thread_started','turn_started') ORDER BY p.created_at DESC LIMIT 1`, string(payload), artifactID, time.Now().UTC().Format(time.RFC3339Nano), assignmentID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}
