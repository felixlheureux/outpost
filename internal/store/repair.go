package store

import (
	"context"
	"fmt"
	"time"
)

// BeginRepair links another provider attempt to the same immutable assignment.
// The validation receipt and failure evidence remain attached to the old attempt.
func (s *Store) BeginRepair(ctx context.Context, assignmentID string) (string, string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var oldID, phase, outcome, provider, thread string
	var ordinal int
	err = tx.QueryRowContext(ctx, `SELECT t.id,t.phase,t.outcome,p.state,p.thread_id,t.ordinal FROM assignment_attempts t JOIN provider_attempts p ON p.attempt_id=t.id JOIN assignments a ON a.id=t.assignment_id WHERE t.assignment_id=? AND a.state='running' AND t.outcome='running' ORDER BY t.ordinal DESC LIMIT 1`, assignmentID).Scan(&oldID, &phase, &outcome, &provider, &thread, &ordinal)
	if err != nil {
		return "", "", err
	}
	if phase != "validating" || outcome != "running" || provider != "completed" {
		return "", "", ErrAdmissionChanged
	}
	id, err := newRandomID("attempt_", 16)
	if err != nil {
		return "", "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r, err := tx.ExecContext(ctx, `UPDATE assignment_attempts SET phase='settled',outcome='failed',settled_at=? WHERE id=? AND phase='validating' AND outcome='running'`, now, oldID)
	if err != nil {
		return "", "", err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return "", "", ErrAdmissionChanged
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,predecessor_id,created_at) VALUES(?,?,?,'implementing','running',?,?)`, id, assignmentID, ordinal+1, oldID, now)
	if err != nil {
		return "", "", err
	}
	_, err = tx.ExecContext(ctx, `UPDATE assignments SET attempt_number=? WHERE id=?`, ordinal+1, assignmentID)
	if err != nil {
		return "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	return id, thread, nil
}
func (s *Store) RecordCheckSuccess(ctx context.Context, assignmentID, receiptID string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var required, missing int
	if err = tx.QueryRowContext(ctx, `SELECT count(*),sum(CASE WHEN status NOT IN ('passed','reused') OR candidate_digest='' THEN 1 ELSE 0 END) FROM check_receipts WHERE assignment_id=? AND id=?`, assignmentID, receiptID).Scan(&required, &missing); err != nil {
		return err
	}
	if required != 1 || missing != 0 {
		return fmt.Errorf("check receipt is not passed")
	}
	r, err := tx.ExecContext(ctx, `UPDATE assignments SET checks_receipt=? WHERE id=? AND state='running' AND checks_receipt='' AND EXISTS(SELECT 1 FROM assignment_attempts WHERE assignment_id=assignments.id AND phase='validating' AND outcome='running')`, receiptID, assignmentID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrAdmissionChanged
	}
	return tx.Commit()
}
