package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type QueueSource struct{ SHA, IssuesJSON, State, FetchedAt, LastError string }
type QueueOperation struct{ ID, ProjectID, RequestKey, Kind, ExpectedSHA, IntendedJSON, ResultJSON, ObservedJSON, State, Reason, CreatedAt, UpdatedAt string }

func (s *Store) QueueSource(ctx context.Context, projectID string) (QueueSource, error) {
	var source QueueSource
	err := s.DB.QueryRowContext(ctx, `SELECT blob_sha, issues_json, state, fetched_at, last_error FROM queue_sources WHERE project_id=?`, projectID).Scan(&source.SHA, &source.IssuesJSON, &source.State, &source.FetchedAt, &source.LastError)
	return source, err
}

func (s *Store) SaveQueueSource(ctx context.Context, projectID, sha, issuesJSON string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO queue_sources(project_id,blob_sha,issues_json,state,fetched_at,last_error) VALUES(?,?,?,'ready',?,'') ON CONFLICT(project_id) DO UPDATE SET blob_sha=excluded.blob_sha,issues_json=excluded.issues_json,state='ready',fetched_at=excluded.fetched_at,last_error=''`, projectID, sha, issuesJSON, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) FailQueueSource(ctx context.Context, projectID, reason string) error {
	if len(reason) > 300 {
		reason = reason[:300]
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO queue_sources(project_id,state,last_error) VALUES(?,'needs_attention',?) ON CONFLICT(project_id) DO UPDATE SET state='needs_attention',last_error=excluded.last_error`, projectID, reason)
	return err
}

func (s *Store) ReserveQueueOperation(ctx context.Context, proposed QueueOperation) (QueueOperation, bool, error) {
	if proposed.ProjectID == "" || proposed.RequestKey == "" || len(proposed.RequestKey) > 100 || proposed.ExpectedSHA == "" || len(proposed.IntendedJSON) > 256<<10 || (proposed.Kind != "replace" && proposed.Kind != "append" && proposed.Kind != "remove" && proposed.Kind != "move") {
		return QueueOperation{}, false, fmt.Errorf("invalid queue operation")
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return QueueOperation{}, false, err
	}
	proposed.ID = hex.EncodeToString(id)
	proposed.State = "pending"
	proposed.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	proposed.UpdatedAt = proposed.CreatedAt
	result, err := s.DB.ExecContext(ctx, `INSERT INTO queue_operations(id,project_id,request_key,kind,expected_sha,intended_json,state,created_at,updated_at) VALUES(?,?,?,?,?,?,'pending',?,?) ON CONFLICT(project_id,request_key) DO NOTHING`, proposed.ID, proposed.ProjectID, proposed.RequestKey, proposed.Kind, proposed.ExpectedSHA, proposed.IntendedJSON, proposed.CreatedAt, proposed.UpdatedAt)
	if err != nil {
		return QueueOperation{}, false, err
	}
	affected, _ := result.RowsAffected()
	stored, err := s.QueueOperation(ctx, proposed.ProjectID, proposed.RequestKey)
	if err != nil {
		return QueueOperation{}, false, err
	}
	if stored.Kind != proposed.Kind || stored.ExpectedSHA != proposed.ExpectedSHA || stored.IntendedJSON != proposed.IntendedJSON {
		return QueueOperation{}, false, fmt.Errorf("request key belongs to another queue operation")
	}
	return stored, affected == 1, nil
}

func (s *Store) QueueOperation(ctx context.Context, projectID, key string) (QueueOperation, error) {
	var op QueueOperation
	err := s.DB.QueryRowContext(ctx, `SELECT id,project_id,request_key,kind,expected_sha,intended_json,result_json,observed_json,state,reason,created_at,updated_at FROM queue_operations WHERE project_id=? AND request_key=?`, projectID, key).Scan(&op.ID, &op.ProjectID, &op.RequestKey, &op.Kind, &op.ExpectedSHA, &op.IntendedJSON, &op.ResultJSON, &op.ObservedJSON, &op.State, &op.Reason, &op.CreatedAt, &op.UpdatedAt)
	return op, err
}

func (s *Store) ListQueueOperations(ctx context.Context, projectID string) ([]QueueOperation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,project_id,request_key,kind,expected_sha,intended_json,result_json,observed_json,state,reason,created_at,updated_at FROM queue_operations WHERE project_id=? ORDER BY created_at DESC LIMIT 50`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]QueueOperation, 0)
	for rows.Next() {
		var op QueueOperation
		if err := rows.Scan(&op.ID, &op.ProjectID, &op.RequestKey, &op.Kind, &op.ExpectedSHA, &op.IntendedJSON, &op.ResultJSON, &op.ObservedJSON, &op.State, &op.Reason, &op.CreatedAt, &op.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, op)
	}
	return result, rows.Err()
}

func (s *Store) SetQueueOperation(ctx context.Context, id, state, reason, resultJSON string) error {
	if state != "confirmed" && state != "rejected" && state != "conflict" && state != "unknown" {
		return fmt.Errorf("invalid queue operation state")
	}
	if len(reason) > 300 {
		reason = reason[:300]
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE queue_operations SET state=?,reason=?,observed_json=?,updated_at=? WHERE id=?`, state, reason, resultJSON, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SetQueueIntentResult(ctx context.Context, id, resultJSON string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE queue_operations SET result_json=?, updated_at=? WHERE id=? AND state='pending'`, resultJSON, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}
