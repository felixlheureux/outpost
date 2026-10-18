package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	Checks        []HistoryCheck   `json:"checks"`
	Messages      []HistoryMessage `json:"messages"`
	Delivery      *HistoryDelivery `json:"delivery"`
	Report        *HistoryReport   `json:"report"`
}

type HistoryCheck struct {
	ID              string `json:"id"`
	AttemptID       string `json:"attempt_id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	Required        bool   `json:"required"`
	CandidateDigest string `json:"candidate_digest"`
	FinishedAt      string `json:"finished_at"`
}

type HistoryMessage struct {
	ID            string `json:"id"`
	AttemptID     string `json:"attempt_id"`
	TurnID        string `json:"turn_id"`
	Body          string `json:"body"`
	BodyTruncated bool   `json:"body_truncated"`
	State         string `json:"state"`
	Reason        string `json:"reason"`
	CreatedAt     string `json:"created_at"`
}

type HistoryDelivery struct {
	Policy    string `json:"policy"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
	CommitSHA string `json:"commit_sha"`
}

type HistoryReport struct {
	State     string `json:"state"`
	Reason    string `json:"reason"`
	CommentID int64  `json:"comment_id"`
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
	PredecessorID  string `json:"predecessor_id"`
	ThreadID       string `json:"thread_id"`
	TurnID         string `json:"turn_id"`
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
		a.Checks = []HistoryCheck{}
		a.Messages = []HistoryMessage{}
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
		attempts, queryErr := s.DB.QueryContext(ctx, `SELECT t.id,t.ordinal,t.phase,t.outcome,COALESCE(p.state,''),COALESCE(p.failure_code,''),COALESCE(p.model,''),COALESCE(p.reasoning,''),COALESCE(p.sandbox,''),COALESCE(p.approval_policy,''),COALESCE(t.predecessor_id,''),COALESCE(p.thread_id,''),COALESCE(p.turn_id,''),t.created_at,t.settled_at FROM assignment_attempts t LEFT JOIN provider_attempts p ON p.attempt_id=t.id WHERE t.assignment_id=? ORDER BY t.ordinal DESC LIMIT 30`, r.Assignments[i].ID)
		if queryErr != nil {
			return r, queryErr
		}
		for attempts.Next() {
			var a HistoryAttempt
			var settled sql.NullString
			if err = attempts.Scan(&a.ID, &a.Ordinal, &a.Phase, &a.Outcome, &a.ProviderState, &a.FailureCode, &a.Model, &a.Reasoning, &a.Sandbox, &a.ApprovalPolicy, &a.PredecessorID, &a.ThreadID, &a.TurnID, &a.CreatedAt, &settled); err != nil {
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
		checks, queryErr := s.DB.QueryContext(ctx, `SELECT id,attempt_id,name,status,reason,required,candidate_digest,finished_at FROM check_receipts WHERE assignment_id=? ORDER BY started_at DESC,id DESC LIMIT 30`, r.Assignments[i].ID)
		if queryErr != nil {
			return r, queryErr
		}
		for checks.Next() {
			var item HistoryCheck
			if err = checks.Scan(&item.ID, &item.AttemptID, &item.Name, &item.Status, &item.Reason, &item.Required, &item.CandidateDigest, &item.FinishedAt); err != nil {
				break
			}
			r.Assignments[i].Checks = append(r.Assignments[i].Checks, item)
		}
		if err == nil {
			err = checks.Err()
		}
		checks.Close()
		if err != nil {
			return r, err
		}
		messages, queryErr := s.DB.QueryContext(ctx, `SELECT id,attempt_id,expected_turn_id,substr(body,1,512),length(body)>512,state,reason,created_at FROM work_messages WHERE assignment_id=? ORDER BY created_at DESC,id DESC LIMIT 30`, r.Assignments[i].ID)
		if queryErr != nil {
			return r, queryErr
		}
		for messages.Next() {
			var item HistoryMessage
			if err = messages.Scan(&item.ID, &item.AttemptID, &item.TurnID, &item.Body, &item.BodyTruncated, &item.State, &item.Reason, &item.CreatedAt); err != nil {
				break
			}
			r.Assignments[i].Messages = append(r.Assignments[i].Messages, item)
		}
		if err == nil {
			err = messages.Err()
		}
		messages.Close()
		if err != nil {
			return r, err
		}
		var delivery HistoryDelivery
		if err = s.DB.QueryRowContext(ctx, `SELECT policy,state,reason,commit_sha FROM delivery_receipts WHERE assignment_id=?`, r.Assignments[i].ID).Scan(&delivery.Policy, &delivery.State, &delivery.Reason, &delivery.CommitSHA); err == nil {
			r.Assignments[i].Delivery = &delivery
		} else if !errors.Is(err, sql.ErrNoRows) {
			return r, err
		}
		var report HistoryReport
		if err = s.DB.QueryRowContext(ctx, `SELECT state,reason,comment_id FROM github_reports WHERE assignment_id=?`, r.Assignments[i].ID).Scan(&report.State, &report.Reason, &report.CommentID); err == nil {
			r.Assignments[i].Report = &report
		} else if !errors.Is(err, sql.ErrNoRows) {
			return r, err
		}
		err = nil
	}
	return r, nil
}
