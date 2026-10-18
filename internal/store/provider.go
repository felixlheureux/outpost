package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type ProviderAttempt struct {
	AssignmentID, AttemptID, State, Version, Model, Reasoning string
	Sandbox, ApprovalPolicy, Prompt, ThreadID, TurnID         string
	HandoffJSON, FailureCode                                  string
}

func (s *Store) BeginProvider(ctx context.Context, p ProviderAttempt) error {
	if p.AssignmentID == "" || p.Version == "" || p.Model == "" || p.Reasoning == "" || p.Sandbox == "" || p.ApprovalPolicy == "" || len(p.Prompt) == 0 || len(p.Prompt) > 128<<10 {
		return fmt.Errorf("invalid provider intent")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return s.DB.QueryRowContext(ctx, `INSERT INTO provider_attempts(assignment_id,attempt_id,state,version,model,reasoning,sandbox,approval_policy,prompt,created_at,updated_at)
		SELECT a.id,t.id,'pending',?,?,?,?,?,?,?,? FROM assignments a JOIN assignment_attempts t ON t.assignment_id=a.id
		WHERE a.id=? AND a.state='running' AND t.phase='implementing' AND t.outcome='running'
		RETURNING attempt_id`, p.Version, p.Model, p.Reasoning, p.Sandbox, p.ApprovalPolicy, p.Prompt, now, now, p.AssignmentID).Scan(&p.AttemptID)
}

func (s *Store) ProviderThread(ctx context.Context, assignmentID, threadID string) error {
	if threadID == "" || len(threadID) > 160 {
		return fmt.Errorf("invalid provider thread")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE provider_attempts SET thread_id=?,state='thread_started',updated_at=? WHERE assignment_id=? AND state='pending'`, threadID, time.Now().UTC().Format(time.RFC3339Nano), assignmentID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) ProviderTurn(ctx context.Context, assignmentID, turnID string) error {
	if turnID == "" || len(turnID) > 160 {
		return fmt.Errorf("invalid provider turn")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE provider_attempts SET turn_id=?,state='turn_started',updated_at=? WHERE assignment_id=? AND state='thread_started'`, turnID, time.Now().UTC().Format(time.RFC3339Nano), assignmentID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) FinishProvider(ctx context.Context, assignmentID, state, handoffJSON, failureCode string) error {
	if state != "completed" && state != "failed" && state != "interrupted" && state != "unknown" {
		return fmt.Errorf("invalid provider outcome")
	}
	if state == "completed" && (len(handoffJSON) > 64<<10 || !json.Valid([]byte(handoffJSON))) {
		return fmt.Errorf("invalid provider handoff")
	}
	if len(failureCode) > 100 {
		return fmt.Errorf("invalid provider failure code")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE provider_attempts SET state=?,handoff_json=?,failure_code=?,updated_at=? WHERE assignment_id=? AND state IN ('pending','thread_started','turn_started')`, state, handoffJSON, failureCode, time.Now().UTC().Format(time.RFC3339Nano), assignmentID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) GetProviderAttempt(ctx context.Context, assignmentID string) (ProviderAttempt, error) {
	var p ProviderAttempt
	err := s.DB.QueryRowContext(ctx, `SELECT assignment_id,attempt_id,state,version,model,reasoning,sandbox,approval_policy,prompt,thread_id,turn_id,handoff_json,failure_code FROM provider_attempts WHERE assignment_id=?`, assignmentID).
		Scan(&p.AssignmentID, &p.AttemptID, &p.State, &p.Version, &p.Model, &p.Reasoning, &p.Sandbox, &p.ApprovalPolicy, &p.Prompt, &p.ThreadID, &p.TurnID, &p.HandoffJSON, &p.FailureCode)
	return p, err
}
