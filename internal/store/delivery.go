package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DeliveryReceipt struct {
	ID, AssignmentID, Policy, Candidate, BaselineHead, Branch, Upstream, RemoteDigest string
	PathsJSON, CommitSHA, State, Reason                                               string
}

const deliveryColumns = `id,assignment_id,policy,candidate_digest,baseline_head,branch,upstream,remote_digest,paths_json,commit_sha,state,reason`

func scanDelivery(row interface{ Scan(...any) error }) (DeliveryReceipt, error) {
	var r DeliveryReceipt
	err := row.Scan(&r.ID, &r.AssignmentID, &r.Policy, &r.Candidate, &r.BaselineHead, &r.Branch, &r.Upstream, &r.RemoteDigest, &r.PathsJSON, &r.CommitSHA, &r.State, &r.Reason)
	return r, err
}

func (s *Store) GetDelivery(ctx context.Context, assignmentID string) (DeliveryReceipt, error) {
	return scanDelivery(s.DB.QueryRowContext(ctx, `SELECT `+deliveryColumns+` FROM delivery_receipts WHERE assignment_id=?`, assignmentID))
}

// BeginDelivery commits the intended Git effect before any index, commit or remote mutation.
func (s *Store) BeginDelivery(ctx context.Context, r DeliveryReceipt) (DeliveryReceipt, error) {
	if r.AssignmentID == "" || r.Candidate == "" || r.BaselineHead == "" || r.Branch == "" || r.PathsJSON == "" {
		return DeliveryReceipt{}, fmt.Errorf("incomplete delivery intent")
	}
	if old, err := s.GetDelivery(ctx, r.AssignmentID); err == nil {
		if old.Policy != r.Policy || old.Candidate != r.Candidate || old.BaselineHead != r.BaselineHead || old.Branch != r.Branch || old.Upstream != r.Upstream || old.RemoteDigest != r.RemoteDigest || old.PathsJSON != r.PathsJSON {
			return DeliveryReceipt{}, ErrAdmissionChanged
		}
		return old, nil
	} else if err != sql.ErrNoRows {
		return DeliveryReceipt{}, err
	}
	r.ID = "delivery_" + r.AssignmentID
	r.State = "pending"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO delivery_receipts(id,assignment_id,policy,candidate_digest,baseline_head,branch,upstream,remote_digest,paths_json,state,created_at,updated_at)
		SELECT ?,?,?,?,?,?,?,?,?,'pending',?,? WHERE EXISTS(SELECT 1 FROM assignment_attempts WHERE assignment_id=? AND phase='delivering' AND outcome='running')`, r.ID, r.AssignmentID, r.Policy, r.Candidate, r.BaselineHead, r.Branch, r.Upstream, r.RemoteDigest, r.PathsJSON, now, now, r.AssignmentID)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	return s.GetDelivery(ctx, r.AssignmentID)
}

func (s *Store) UpdateDelivery(ctx context.Context, id, state, commit, reason string) error {
	if state != "pending" && state != "commit_unknown" && state != "push_unknown" && state != "settled" && state != "blocked" {
		return fmt.Errorf("invalid delivery state")
	}
	if len(reason) > 300 {
		reason = reason[:300]
	}
	r, err := s.DB.ExecContext(ctx, `UPDATE delivery_receipts SET state=?,commit_sha=CASE WHEN ?='' THEN commit_sha ELSE ? END,reason=?,updated_at=? WHERE id=? AND state!='settled'`, state, commit, commit, reason, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

type GitHubReport struct {
	ID, AssignmentID, Marker, State, Reason string
	CommentID                               int64
	CloseIssue                              bool
}

func (s *Store) GetReport(ctx context.Context, assignmentID string) (GitHubReport, error) {
	var r GitHubReport
	err := s.DB.QueryRowContext(ctx, `SELECT id,assignment_id,marker,state,comment_id,close_issue,reason FROM github_reports WHERE assignment_id=?`, assignmentID).Scan(&r.ID, &r.AssignmentID, &r.Marker, &r.State, &r.CommentID, &r.CloseIssue, &r.Reason)
	return r, err
}

func (s *Store) BeginReport(ctx context.Context, assignmentID string, closeIssue bool) (GitHubReport, error) {
	if old, err := s.GetReport(ctx, assignmentID); err == nil {
		return old, nil
	} else if err != sql.ErrNoRows {
		return GitHubReport{}, err
	}
	id := "report_" + assignmentID
	marker := "<!-- outpost-result:" + assignmentID + " -->"
	_, err := s.DB.ExecContext(ctx, `INSERT INTO github_reports(id,assignment_id,marker,state,close_issue,updated_at)
		SELECT ?,?,?,'pending',?,? WHERE EXISTS(SELECT 1 FROM assignments WHERE id=? AND state='succeeded')`, id, assignmentID, marker, closeIssue, time.Now().UTC().Format(time.RFC3339Nano), assignmentID)
	if err != nil {
		return GitHubReport{}, err
	}
	return s.GetReport(ctx, assignmentID)
}

func (s *Store) UpdateReport(ctx context.Context, id, state string, commentID int64, reason string) error {
	if state != "pending" && state != "comment_unknown" && state != "close_pending" && state != "close_unknown" && state != "settled" && state != "blocked" {
		return fmt.Errorf("invalid report state")
	}
	if len(reason) > 300 {
		reason = reason[:300]
	}
	r, err := s.DB.ExecContext(ctx, `UPDATE github_reports SET state=?,comment_id=CASE WHEN ?>0 THEN ? ELSE comment_id END,reason=?,updated_at=? WHERE id=? AND state!='settled'`, state, commentID, commentID, reason, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) DecideReview(ctx context.Context, assignmentID, candidate, decision, feedback string) error {
	if decision != "approved" && decision != "rejected" {
		return fmt.Errorf("invalid review decision")
	}
	if len(feedback) > 8192 {
		return fmt.Errorf("review feedback exceeds limit")
	}
	var actual string
	if err := s.DB.QueryRowContext(ctx, `SELECT c.candidate_digest FROM assignments a JOIN check_receipts c ON c.id=a.checks_receipt WHERE a.id=? AND a.state='running' AND c.status IN ('passed','reused')`, assignmentID).Scan(&actual); err != nil {
		return err
	}
	if actual != candidate {
		return ErrAdmissionChanged
	}
	var phase string
	if err := s.DB.QueryRowContext(ctx, `SELECT phase FROM assignment_attempts WHERE assignment_id=? AND outcome='running' ORDER BY ordinal DESC LIMIT 1`, assignmentID).Scan(&phase); err != nil {
		return err
	}
	if phase != "awaiting_review" {
		return fmt.Errorf("assignment is not awaiting review")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO review_decisions(assignment_id,candidate_digest,decision,feedback,decided_at) VALUES(?,?,?,?,?) ON CONFLICT(assignment_id,candidate_digest) DO NOTHING`, assignmentID, candidate, decision, feedback, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	var recorded, recordedFeedback string
	if err = s.DB.QueryRowContext(ctx, `SELECT decision,feedback FROM review_decisions WHERE assignment_id=? AND candidate_digest=?`, assignmentID, candidate).Scan(&recorded, &recordedFeedback); err != nil {
		return err
	}
	if recorded != decision || recordedFeedback != feedback {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) ReviewDecision(ctx context.Context, assignmentID, candidate string) (string, string, error) {
	var decision, feedback string
	err := s.DB.QueryRowContext(ctx, `SELECT decision,feedback FROM review_decisions WHERE assignment_id=? AND candidate_digest=?`, assignmentID, candidate).Scan(&decision, &feedback)
	return decision, feedback, err
}

func (s *Store) GetAssignment(ctx context.Context, id string) (Assignment, error) {
	var a Assignment
	err := s.DB.QueryRowContext(ctx, `SELECT id,run_id,project_id,issue_number,generation,attempt_number,state,snapshot_json FROM assignments WHERE id=?`, id).Scan(&a.ID, &a.RunID, &a.ProjectID, &a.IssueNumber, &a.Generation, &a.AttemptNumber, &a.State, &a.SnapshotJSON)
	return a, err
}

func (s *Store) UnreportedAssignments(ctx context.Context, runID string) ([]Assignment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT a.id,a.run_id,a.project_id,a.issue_number,a.generation,a.attempt_number,a.state,a.snapshot_json FROM assignments a LEFT JOIN github_reports r ON r.assignment_id=a.id WHERE a.run_id=? AND a.state='succeeded' AND (r.id IS NULL OR r.state!='settled') ORDER BY a.created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Assignment{}
	for rows.Next() {
		var a Assignment
		if err = rows.Scan(&a.ID, &a.RunID, &a.ProjectID, &a.IssueNumber, &a.Generation, &a.AttemptNumber, &a.State, &a.SnapshotJSON); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
