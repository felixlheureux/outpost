package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type CheckReceipt struct {
	ID              string        `json:"id"`
	AssignmentID    string        `json:"assignment_id"`
	AttemptID       string        `json:"attempt_id"`
	Generation      int           `json:"generation"`
	CheckID         string        `json:"check_id"`
	Name            string        `json:"name"`
	Required        bool          `json:"required"`
	CommandJSON     string        `json:"command_json"`
	EnvJSON         string        `json:"env_json"`
	ContractDigest  string        `json:"contract_digest"`
	InputDigest     string        `json:"input_digest"`
	CandidateDigest string        `json:"candidate_digest"`
	Status          string        `json:"status"`
	Reason          string        `json:"reason"`
	PID             int           `json:"pid"`
	StartTicks      string        `json:"start_ticks"`
	StartedAt       string        `json:"started_at"`
	FinishedAt      string        `json:"finished_at"`
	DurationMS      int64         `json:"duration_ms"`
	ExitCode        sql.NullInt64 `json:"exit_code"`
	Signal          string        `json:"signal"`
	CaseCount       sql.NullInt64 `json:"case_count"`
	RequireCases    bool          `json:"require_cases"`
	OutputPath      string        `json:"-"`
	TerminalPath    string        `json:"-"`
	OutputBytes     int64         `json:"output_bytes"`
	Truncated       bool          `json:"truncated"`
}

const checkColumns = `id,assignment_id,attempt_id,generation,check_id,name,required,command_json,env_json,contract_digest,input_digest,candidate_digest,status,reason,pid,start_ticks,started_at,finished_at,duration_ms,exit_code,signal,case_count,require_cases,output_path,terminal_path,output_bytes,truncated`

func scanCheck(rows interface{ Scan(...any) error }) (CheckReceipt, error) {
	var c CheckReceipt
	var required, cases, truncated int
	err := rows.Scan(&c.ID, &c.AssignmentID, &c.AttemptID, &c.Generation, &c.CheckID, &c.Name, &required, &c.CommandJSON, &c.EnvJSON, &c.ContractDigest, &c.InputDigest, &c.CandidateDigest, &c.Status, &c.Reason, &c.PID, &c.StartTicks, &c.StartedAt, &c.FinishedAt, &c.DurationMS, &c.ExitCode, &c.Signal, &c.CaseCount, &cases, &c.OutputPath, &c.TerminalPath, &c.OutputBytes, &truncated)
	c.Required = required != 0
	c.RequireCases = cases != 0
	c.Truncated = truncated != 0
	return c, err
}
func (s *Store) BeginCheck(ctx context.Context, c CheckReceipt) error {
	if c.ID == "" || c.AssignmentID == "" || c.AttemptID == "" || c.CheckID == "" || c.ContractDigest == "" || c.InputDigest == "" || c.OutputPath == "" || c.TerminalPath == "" {
		return fmt.Errorf("incomplete check intent")
	}
	r, err := s.DB.ExecContext(ctx, `INSERT INTO check_receipts(id,assignment_id,attempt_id,generation,check_id,name,required,command_json,env_json,contract_digest,input_digest,candidate_digest,status,reason,started_at,require_cases,output_path,terminal_path) SELECT ?,?,?,?,?,?,?,?,?,?,?,?,'pending',?,?,?,?,? WHERE EXISTS(SELECT 1 FROM assignment_attempts WHERE id=? AND assignment_id=? AND phase='validating' AND outcome='running')`, c.ID, c.AssignmentID, c.AttemptID, c.Generation, c.CheckID, c.Name, c.Required, c.CommandJSON, c.EnvJSON, c.ContractDigest, c.InputDigest, c.CandidateDigest, c.Reason, time.Now().UTC().Format(time.RFC3339Nano), c.RequireCases, c.OutputPath, c.TerminalPath, c.AttemptID, c.AssignmentID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}
func (s *Store) StartCheck(ctx context.Context, id string, pid int, ticks string) error {
	r, err := s.DB.ExecContext(ctx, `UPDATE check_receipts SET status='running',pid=?,start_ticks=? WHERE id=? AND status='pending'`, pid, ticks, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}
func (s *Store) UnknownCheck(ctx context.Context, id, reason string) error {
	r, err := s.DB.ExecContext(ctx, `UPDATE check_receipts SET status='unknown',reason=? WHERE id=? AND status IN ('pending','running')`, reason, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}
func (s *Store) FinishCheck(ctx context.Context, c CheckReceipt) error {
	r, err := s.DB.ExecContext(ctx, `UPDATE check_receipts SET status=?,reason=?,finished_at=?,duration_ms=?,exit_code=?,signal=?,case_count=?,output_bytes=?,truncated=? WHERE id=? AND status IN ('pending','running') AND contract_digest=? AND input_digest=?`, c.Status, c.Reason, c.FinishedAt, c.DurationMS, c.ExitCode, c.Signal, c.CaseCount, c.OutputBytes, c.Truncated, c.ID, c.ContractDigest, c.InputDigest)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}
func (s *Store) ReuseCheck(ctx context.Context, c CheckReceipt, newID, attemptID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO check_receipts(id,assignment_id,attempt_id,generation,check_id,name,required,command_json,env_json,contract_digest,input_digest,candidate_digest,status,reason,started_at,finished_at,duration_ms,exit_code,signal,case_count,require_cases,output_path,terminal_path,output_bytes,truncated) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'reused',?,?,?,?,?,?,?,?,?,?,?,?)`, newID, c.AssignmentID, attemptID, c.Generation, c.CheckID, c.Name, c.Required, c.CommandJSON, c.EnvJSON, c.ContractDigest, c.InputDigest, c.CandidateDigest, "Reused terminal receipt "+c.ID, now, now, c.DurationMS, c.ExitCode, c.Signal, c.CaseCount, c.RequireCases, c.OutputPath, c.TerminalPath, c.OutputBytes, c.Truncated)
	return err
}
func (s *Store) ListChecks(ctx context.Context, assignmentID string) ([]CheckReceipt, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+checkColumns+` FROM check_receipts WHERE assignment_id=? ORDER BY started_at,id`, assignmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CheckReceipt{}
	for rows.Next() {
		c, e := scanCheck(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) LatestCheck(ctx context.Context, assignmentID, checkID string) (CheckReceipt, error) {
	return scanCheck(s.DB.QueryRowContext(ctx, `SELECT `+checkColumns+` FROM check_receipts WHERE assignment_id=? AND check_id=? ORDER BY started_at DESC,id DESC LIMIT 1`, assignmentID, checkID))
}

func (s *Store) GetRunCheck(ctx context.Context, runID, receiptID string) (CheckReceipt, error) {
	return scanCheck(s.DB.QueryRowContext(ctx, `SELECT `+checkColumns+` FROM check_receipts WHERE id=? AND assignment_id IN (SELECT id FROM assignments WHERE run_id=?)`, receiptID, runID))
}

func (s *Store) RepeatedCheckFailure(ctx context.Context, assignmentID, receiptID string) (bool, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM check_receipts prior JOIN check_receipts current ON current.assignment_id=prior.assignment_id AND current.check_id=prior.check_id AND current.input_digest=prior.input_digest AND current.contract_digest=prior.contract_digest WHERE current.id=? AND current.assignment_id=? AND prior.id!=current.id AND prior.status='failed' AND current.status='failed'`, receiptID, assignmentID).Scan(&count)
	return count > 0, err
}
