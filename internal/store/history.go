package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// HistoryRun is a bounded projection of durable receipts, never a new task source.
type HistoryRun struct {
	ID                string              `json:"id"`
	ProjectID         string              `json:"project_id"`
	State             string              `json:"state"`
	ReservationState  string              `json:"reservation_state"`
	LaunchObservation string              `json:"launch_observation"`
	ScheduleOutcome   string              `json:"schedule_outcome"`
	ScheduleReason    string              `json:"schedule_reason"`
	CreatedAt         string              `json:"created_at"`
	UpdatedAt         string              `json:"updated_at"`
	SuccessfulCount   int                 `json:"successful_count"`
	TaskLimit         int                 `json:"task_limit"`
	AssignmentCount   int                 `json:"assignment_count"`
	Assignments       []HistoryAssignment `json:"assignments"`
}

type HistoryAssignment struct {
	ID            string           `json:"id"`
	IssueNumber   int              `json:"issue_number"`
	Title         string           `json:"title"`
	IssueDigest   string           `json:"issue_digest"`
	IssueRevision int64            `json:"issue_revision"`
	QueueSHA      string           `json:"queue_sha"`
	BaselineHead  string           `json:"baseline_head"`
	Branch        string           `json:"branch"`
	State         string           `json:"state"`
	CreatedAt     string           `json:"created_at"`
	SettledAt     string           `json:"settled_at"`
	Attempts      []HistoryAttempt `json:"attempts"`
}

type HistoryAttempt struct {
	ID             string `json:"id"`
	Ordinal        int    `json:"ordinal"`
	Phase          string `json:"phase"`
	Outcome        string `json:"outcome"`
	ProviderState  string `json:"provider_state"`
	FailureCode    string `json:"failure_code"`
	Model          string `json:"model"`
	Reasoning      string `json:"reasoning"`
	Sandbox        string `json:"sandbox"`
	ApprovalPolicy string `json:"approval_policy"`
	CreatedAt      string `json:"created_at"`
	SettledAt      string `json:"settled_at"`
}

type HistoryArtifact struct {
	ID             string `json:"id"`
	AttemptID      string `json:"attempt_id"`
	BytesWritten   int64  `json:"bytes_written"`
	Truncated      bool   `json:"truncated"`
	RetentionState string `json:"retention_state"`
	RemovedAt      string `json:"removed_at"`
}

func (s *Store) ListHistoryArtifacts(ctx context.Context, runID string) ([]HistoryArtifact, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,attempt_id,bytes_written,truncated,retention_state,removed_at FROM activity_artifacts WHERE run_id=? ORDER BY created_at DESC LIMIT 100`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []HistoryArtifact{}
	for rows.Next() {
		var item HistoryArtifact
		if err := rows.Scan(&item.ID, &item.AttemptID, &item.BytesWritten, &item.Truncated, &item.RetentionState, &item.RemovedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListHistory(ctx context.Context, projectID string, limit, offset int) ([]HistoryRun, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT r.id,r.project_id,r.state,r.reservation_state,r.launch_observation,
	 COALESCE(s.outcome,''),COALESCE(s.reason,''),COALESCE(s.successful_count,0),COALESCE(s.task_limit,0),
	 r.created_at,r.updated_at,(SELECT count(*) FROM assignments a WHERE a.run_id=r.id)
	 FROM runs r LEFT JOIN run_schedules s ON s.run_id=r.id
	 WHERE (?='' OR r.project_id=?) ORDER BY r.created_at DESC,r.id DESC LIMIT ? OFFSET ?`, projectID, projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]HistoryRun, 0, limit)
	for rows.Next() {
		var r HistoryRun
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.State, &r.ReservationState, &r.LaunchObservation, &r.ScheduleOutcome, &r.ScheduleReason, &r.SuccessfulCount, &r.TaskLimit, &r.CreatedAt, &r.UpdatedAt, &r.AssignmentCount); err != nil {
			return nil, err
		}
		r.Assignments = []HistoryAssignment{}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) HistoryDetail(ctx context.Context, runID string) (HistoryRun, error) {
	var r HistoryRun
	err := s.DB.QueryRowContext(ctx, `SELECT r.id,r.project_id,r.state,r.reservation_state,r.launch_observation,
	 COALESCE(s.outcome,''),COALESCE(s.reason,''),COALESCE(s.successful_count,0),COALESCE(s.task_limit,0),
	 r.created_at,r.updated_at,(SELECT count(*) FROM assignments a WHERE a.run_id=r.id)
	 FROM runs r LEFT JOIN run_schedules s ON s.run_id=r.id WHERE r.id=?`, runID).Scan(&r.ID, &r.ProjectID, &r.State, &r.ReservationState, &r.LaunchObservation, &r.ScheduleOutcome, &r.ScheduleReason, &r.SuccessfulCount, &r.TaskLimit, &r.CreatedAt, &r.UpdatedAt, &r.AssignmentCount)
	if err != nil {
		return r, err
	}
	r.Assignments = []HistoryAssignment{}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,issue_number,snapshot_json,state,created_at,settled_at FROM assignments WHERE run_id=? ORDER BY created_at DESC,id DESC LIMIT 100`, runID)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var a HistoryAssignment
		var snapshot string
		var settled sql.NullString
		if err = rows.Scan(&a.ID, &a.IssueNumber, &snapshot, &a.State, &a.CreatedAt, &settled); err != nil {
			break
		}
		var brief struct {
			Title         string `json:"title"`
			IssueDigest   string `json:"issue_digest"`
			IssueRevision int64  `json:"issue_revision"`
			QueueSHA      string `json:"queue_sha"`
			Head          string `json:"head"`
			Branch        string `json:"branch"`
		}
		_ = json.Unmarshal([]byte(snapshot), &brief)
		a.Title, a.IssueDigest, a.IssueRevision, a.QueueSHA, a.BaselineHead, a.Branch, a.SettledAt = brief.Title, brief.IssueDigest, brief.IssueRevision, brief.QueueSHA, brief.Head, brief.Branch, settled.String
		a.Attempts = []HistoryAttempt{}
		r.Assignments = append(r.Assignments, a)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return r, err
	}
	for i := range r.Assignments {
		attempts, queryErr := s.DB.QueryContext(ctx, `SELECT t.id,t.ordinal,t.phase,t.outcome,COALESCE(p.state,''),COALESCE(p.failure_code,''),COALESCE(p.model,''),COALESCE(p.reasoning,''),COALESCE(p.sandbox,''),COALESCE(p.approval_policy,''),t.created_at,t.settled_at FROM assignment_attempts t LEFT JOIN provider_attempts p ON p.attempt_id=t.id WHERE t.assignment_id=? ORDER BY t.ordinal DESC LIMIT 100`, r.Assignments[i].ID)
		if queryErr != nil {
			return r, queryErr
		}
		for attempts.Next() {
			var a HistoryAttempt
			var settled sql.NullString
			if err = attempts.Scan(&a.ID, &a.Ordinal, &a.Phase, &a.Outcome, &a.ProviderState, &a.FailureCode, &a.Model, &a.Reasoning, &a.Sandbox, &a.ApprovalPolicy, &a.CreatedAt, &settled); err != nil {
				break
			}
			a.SettledAt = settled.String
			r.Assignments[i].Attempts = append(r.Assignments[i].Attempts, a)
		}
		if err == nil {
			err = attempts.Err()
		}
		attempts.Close()
		if err != nil {
			return r, err
		}
	}
	return r, nil
}
