package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type IssueObservation struct {
	NodeID, URL, Title, Body, State, SourceUpdatedAt, ContentDigest, RequirementsJSON, ParseError string
	GithubID                                                                                      int64
	Number                                                                                        int
}

type IssueSource struct {
	IssueObservation
	FetchedAt     string
	LocalRevision int64
}

type IssueSourceStatus struct {
	State, LastSuccessAt, LastError string
}

func (s *Store) ApplyIssueObservations(ctx context.Context, projectID string, observations []IssueObservation) error {
	if projectID == "" || len(observations) > 500 {
		return fmt.Errorf("invalid issue source batch")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, issue := range observations {
		if issue.NodeID == "" || issue.Number <= 0 || issue.GithubID <= 0 || issue.ContentDigest == "" || issue.RequirementsJSON == "" {
			return fmt.Errorf("incomplete issue observation")
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO issue_sources
			(project_id, node_id, github_id, number, url, title, body, state, source_updated_at, content_digest, fetched_at, local_revision, requirements_json, parse_error)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
			ON CONFLICT(project_id, node_id) DO UPDATE SET
			github_id=excluded.github_id, number=excluded.number, url=excluded.url,
			title=excluded.title, body=excluded.body, state=excluded.state,
			source_updated_at=excluded.source_updated_at, content_digest=excluded.content_digest,
			fetched_at=excluded.fetched_at, requirements_json=excluded.requirements_json,
			parse_error=excluded.parse_error,
			local_revision=CASE WHEN issue_sources.content_digest=excluded.content_digest AND issue_sources.state=excluded.state
				THEN issue_sources.local_revision ELSE issue_sources.local_revision+1 END`,
			projectID, issue.NodeID, issue.GithubID, issue.Number, issue.URL, issue.Title, issue.Body, issue.State,
			issue.SourceUpdatedAt, issue.ContentDigest, now, issue.RequirementsJSON, issue.ParseError)
		if err != nil {
			return fmt.Errorf("cache issue #%d: %w", issue.Number, err)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO issue_source_status(project_id, state, last_success_at, last_error)
		VALUES (?, 'ready', ?, NULL)
		ON CONFLICT(project_id) DO UPDATE SET state='ready', last_success_at=excluded.last_success_at, last_error=NULL`, projectID, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RecordIssueSourceFailure(ctx context.Context, projectID, message string) error {
	if projectID == "" || len(message) > 300 {
		return fmt.Errorf("invalid issue source failure")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO issue_source_status(project_id, state, last_error)
		VALUES (?, 'needs_attention', ?)
		ON CONFLICT(project_id) DO UPDATE SET state='needs_attention', last_error=excluded.last_error`, projectID, message)
	return err
}

func (s *Store) ListIssueSources(ctx context.Context, projectID string) ([]IssueSource, IssueSourceStatus, error) {
	var status IssueSourceStatus
	var lastSuccess, lastError sql.NullString
	err := s.DB.QueryRowContext(ctx, "SELECT state, last_success_at, last_error FROM issue_source_status WHERE project_id = ?", projectID).Scan(&status.State, &lastSuccess, &lastError)
	if err != nil && err != sql.ErrNoRows {
		return nil, status, err
	}
	status.LastSuccessAt, status.LastError = lastSuccess.String, lastError.String
	rows, err := s.DB.QueryContext(ctx, `SELECT node_id, github_id, number, url, title, body, state, source_updated_at,
		content_digest, fetched_at, local_revision, requirements_json, parse_error
		FROM issue_sources WHERE project_id = ? ORDER BY number DESC`, projectID)
	if err != nil {
		return nil, status, err
	}
	defer rows.Close()
	issues := make([]IssueSource, 0)
	for rows.Next() {
		var issue IssueSource
		var parseError sql.NullString
		if err := rows.Scan(&issue.NodeID, &issue.GithubID, &issue.Number, &issue.URL, &issue.Title,
			&issue.Body, &issue.State, &issue.SourceUpdatedAt, &issue.ContentDigest, &issue.FetchedAt,
			&issue.LocalRevision, &issue.RequirementsJSON, &parseError); err != nil {
			return nil, status, err
		}
		issue.ParseError = parseError.String
		issues = append(issues, issue)
	}
	return issues, status, rows.Err()
}
