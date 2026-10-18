package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type IssueOperation struct {
	ID, ProjectID, RequestKey, Kind, ExpectedDigest, Title, Body, Marker, State, Reason, CreatedAt, UpdatedAt string
	IssueNumber, ResultNumber                                                                                 int
	ExpectedRevision                                                                                          int64
}

func (s *Store) ReserveIssueOperation(ctx context.Context, proposed IssueOperation) (IssueOperation, bool, error) {
	if proposed.ProjectID == "" || proposed.RequestKey == "" || len(proposed.RequestKey) > 100 || proposed.Title == "" || len(proposed.Title) > 500 || len(proposed.Body) > 256<<10 || (proposed.Kind != "create" && proposed.Kind != "edit") {
		return IssueOperation{}, false, fmt.Errorf("invalid issue operation")
	}
	if proposed.Kind == "edit" && (proposed.IssueNumber <= 0 || proposed.ExpectedDigest == "" || proposed.ExpectedRevision <= 0) {
		return IssueOperation{}, false, fmt.Errorf("edit needs a source revision")
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return IssueOperation{}, false, err
	}
	proposed.ID = hex.EncodeToString(id)
	proposed.Marker = "<!-- outpost-issue-operation:" + proposed.ID + " -->"
	proposed.State = "pending"
	proposed.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	proposed.UpdatedAt = proposed.CreatedAt
	result, err := s.DB.ExecContext(ctx, `INSERT INTO issue_operations
 (id, project_id, request_key, kind, issue_number, expected_digest, expected_revision, title, body, marker, state, created_at, updated_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)
 ON CONFLICT(project_id, request_key) DO NOTHING`, proposed.ID, proposed.ProjectID, proposed.RequestKey, proposed.Kind,
		nullableInt(proposed.IssueNumber), proposed.ExpectedDigest, nullableInt64(proposed.ExpectedRevision), proposed.Title, proposed.Body,
		proposed.Marker, proposed.CreatedAt, proposed.UpdatedAt)
	if err != nil {
		return IssueOperation{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return IssueOperation{}, false, err
	}
	stored, err := s.GetIssueOperation(ctx, proposed.ProjectID, proposed.RequestKey)
	if err != nil {
		return IssueOperation{}, false, err
	}
	if stored.Kind != proposed.Kind || stored.IssueNumber != proposed.IssueNumber || stored.ExpectedDigest != proposed.ExpectedDigest || stored.ExpectedRevision != proposed.ExpectedRevision || stored.Title != proposed.Title || stored.Body != proposed.Body {
		return IssueOperation{}, false, fmt.Errorf("request key belongs to another issue operation")
	}
	return stored, affected == 1, nil
}

func nullableInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
func nullableInt64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func (s *Store) GetIssueOperation(ctx context.Context, projectID, requestKey string) (IssueOperation, error) {
	var op IssueOperation
	var issueNumber, resultNumber sql.NullInt64
	var revision sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT id, project_id, request_key, kind, issue_number, expected_digest, expected_revision,
 title, body, marker, state, result_number, reason, created_at, updated_at FROM issue_operations WHERE project_id=? AND request_key=?`, projectID, requestKey).
		Scan(&op.ID, &op.ProjectID, &op.RequestKey, &op.Kind, &issueNumber, &op.ExpectedDigest, &revision,
			&op.Title, &op.Body, &op.Marker, &op.State, &resultNumber, &op.Reason, &op.CreatedAt, &op.UpdatedAt)
	op.IssueNumber, op.ResultNumber, op.ExpectedRevision = int(issueNumber.Int64), int(resultNumber.Int64), revision.Int64
	return op, err
}

func (s *Store) ListIssueOperations(ctx context.Context, projectID string) ([]IssueOperation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT request_key FROM issue_operations WHERE project_id=? ORDER BY created_at DESC LIMIT 50`, projectID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]IssueOperation, 0, len(keys))
	for _, key := range keys {
		op, err := s.GetIssueOperation(ctx, projectID, key)
		if err != nil {
			return nil, err
		}
		result = append(result, op)
	}
	return result, nil
}

func (s *Store) SetIssueOperation(ctx context.Context, id, state string, resultNumber int, reason string) error {
	if state != "confirmed" && state != "rejected" && state != "conflict" && state != "unknown" || len(reason) > 500 {
		return fmt.Errorf("invalid issue operation result")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE issue_operations SET state=?, result_number=?, reason=?, updated_at=? WHERE id=?`, state,
		nullableInt(resultNumber), reason, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("issue operation missing")
	}
	return nil
}

func (s *Store) GetIssueSource(ctx context.Context, projectID string, number int) (IssueSource, error) {
	var issue IssueSource
	var parseError sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT node_id, github_id, number, url, title, body, state, source_updated_at,
 content_digest, fetched_at, local_revision, requirements_json, parse_error FROM issue_sources WHERE project_id=? AND number=?`, projectID, number).
		Scan(&issue.NodeID, &issue.GithubID, &issue.Number, &issue.URL, &issue.Title, &issue.Body, &issue.State,
			&issue.SourceUpdatedAt, &issue.ContentDigest, &issue.FetchedAt, &issue.LocalRevision, &issue.RequirementsJSON, &parseError)
	issue.ParseError = parseError.String
	return issue, err
}
