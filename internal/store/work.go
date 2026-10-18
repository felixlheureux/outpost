package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrWorkConflict = errors.New("work request key has different content")
var ErrStaleWorkTarget = errors.New("work target is no longer current")

type WorkMessage struct {
	ID              string `json:"id"`
	RunID           string `json:"run_id"`
	AssignmentID    string `json:"assignment_id"`
	AttemptID       string `json:"attempt_id"`
	ThreadID        string `json:"thread_id"`
	ExpectedTurnID  string `json:"expected_turn_id"`
	Target          string `json:"target"`
	Body            string `json:"body"`
	Digest          string `json:"digest"`
	State           string `json:"state"`
	Reason          string `json:"reason"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	ContractChanged bool   `json:"contract_changed"`
}

type WorkControl struct {
	ID           string `json:"id"`
	RunID        string `json:"run_id"`
	AttemptID    string `json:"attempt_id"`
	Action       string `json:"action"`
	Mode         string `json:"mode"`
	Instructions string `json:"instructions"`
	Digest       string `json:"digest"`
	State        string `json:"state"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type WorkOverview struct {
	RunID, ProjectID, RunState, AssignmentID, AttemptID, AttemptPhase, AttemptOutcome               string
	ProviderState, ProviderFailureCode, ThreadID, TurnID, Model, Reasoning, Sandbox, ApprovalPolicy string
	SnapshotJSON                                                                                    string
	AssignmentStartedAt                                                                             string
	IssueNumber, SuccessfulCount, TaskLimit                                                         int
	StopRequested, TurnOpen                                                                         bool
	ScheduleOutcome, ScheduleReason                                                                 string
	Messages                                                                                        []WorkMessage
	Controls                                                                                        []WorkControl
	Attempts                                                                                        []WorkAttempt
}

type WorkAttempt struct {
	ID            string `json:"id"`
	Ordinal       int    `json:"ordinal"`
	Phase         string `json:"phase"`
	Outcome       string `json:"outcome"`
	PredecessorID string `json:"predecessor_id"`
	CreatedAt     string `json:"created_at"`
	SettledAt     string `json:"settled_at"`
}

func digest(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", len(part), part)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func validKey(id string) bool {
	return len(id) >= 8 && len(id) <= 128 && strings.TrimSpace(id) == id && !strings.ContainsAny(id, "\x00\r\n")
}

func scanWorkMessage(row *sql.Row) (WorkMessage, error) {
	var m WorkMessage
	var changed int
	err := row.Scan(&m.ID, &m.RunID, &m.AssignmentID, &m.AttemptID, &m.ThreadID, &m.ExpectedTurnID, &m.Target, &m.Body, &m.Digest, &m.State, &m.Reason, &changed, &m.CreatedAt, &m.UpdatedAt)
	m.ContractChanged = changed != 0
	return m, err
}

const messageColumns = "id,run_id,assignment_id,attempt_id,thread_id,expected_turn_id,target,body,digest,state,reason,contract_changed,created_at,updated_at"

func (s *Store) GetWorkMessage(ctx context.Context, id string) (WorkMessage, error) {
	return scanWorkMessage(s.DB.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM work_messages WHERE id=?", id))
}

func (s *Store) SubmitWorkMessage(ctx context.Context, m WorkMessage) (WorkMessage, error) {
	if !validKey(m.ID) || m.RunID == "" || m.AttemptID == "" || m.ThreadID == "" || strings.TrimSpace(m.Body) == "" || len(m.Body) > 32<<10 || (m.Target != "live" && m.Target != "next") || (m.Target == "live" && m.ExpectedTurnID == "") || (m.Target == "next" && m.ExpectedTurnID != "") {
		return WorkMessage{}, fmt.Errorf("invalid work message")
	}
	m.Digest = digest(m.RunID, m.AttemptID, m.ThreadID, m.ExpectedTurnID, m.Target, m.Body)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return WorkMessage{}, err
	}
	defer tx.Rollback()
	existing, err := scanWorkMessage(tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM work_messages WHERE id=?", m.ID))
	if err == nil {
		if existing.Digest != m.Digest {
			return WorkMessage{}, ErrWorkConflict
		}
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return WorkMessage{}, err
	}
	var assignment, runState, providerState, thread, turn string
	var open int
	err = tx.QueryRowContext(ctx, `SELECT a.id,r.state,p.state,p.thread_id,p.turn_id,p.turn_open
 FROM assignment_attempts t JOIN assignments a ON a.id=t.assignment_id JOIN runs r ON r.id=a.run_id
 JOIN provider_attempts p ON p.attempt_id=t.id
 WHERE t.id=? AND r.id=? AND r.reservation_state='reserved' AND a.state='running' AND t.outcome='running'`, m.AttemptID, m.RunID).Scan(&assignment, &runState, &providerState, &thread, &turn, &open)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkMessage{}, ErrStaleWorkTarget
		}
		return WorkMessage{}, err
	}
	if runState != "running" || thread != m.ThreadID || (m.Target == "live" && (providerState != "turn_started" || open == 0 || turn != m.ExpectedTurnID)) || (m.Target == "next" && providerState != "thread_started") {
		return WorkMessage{}, ErrStaleWorkTarget
	}
	m.AssignmentID = assignment
	m.State = "queued"
	m.ContractChanged = true
	m.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.UpdatedAt = m.CreatedAt
	_, err = tx.ExecContext(ctx, `INSERT INTO work_messages(id,run_id,assignment_id,attempt_id,thread_id,expected_turn_id,target,body,digest,state,contract_changed,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,'queued',1,?,?)`, m.ID, m.RunID, m.AssignmentID, m.AttemptID, m.ThreadID, m.ExpectedTurnID, m.Target, m.Body, m.Digest, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return WorkMessage{}, err
	}
	return m, tx.Commit()
}

func (s *Store) ClaimWorkMessage(ctx context.Context, attemptID, threadID, turnID string) (WorkMessage, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return WorkMessage{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT m.id FROM work_messages m JOIN provider_attempts p ON p.attempt_id=m.attempt_id WHERE m.attempt_id=? AND m.thread_id=? AND ((m.target='live' AND m.expected_turn_id=?) OR (m.target='next' AND m.expected_turn_id='')) AND m.state='queued' AND p.state='turn_started' AND p.turn_open=1 AND p.turn_id=? ORDER BY m.created_at,m.id LIMIT 1`, attemptID, threadID, turnID, turnID).Scan(&id)
	if err != nil {
		return WorkMessage{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE work_messages SET state='sending',expected_turn_id=?,updated_at=? WHERE id=? AND state='queued'`, turnID, now, id)
	if err != nil {
		return WorkMessage{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return WorkMessage{}, ErrStaleWorkTarget
	}
	m, err := scanWorkMessage(tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM work_messages WHERE id=?", id))
	if err != nil {
		return WorkMessage{}, err
	}
	return m, tx.Commit()
}

func (s *Store) SettleWorkMessage(ctx context.Context, id, state, reason string) error {
	if state != "accepted" && state != "rejected" && state != "unknown" {
		return fmt.Errorf("invalid message outcome")
	}
	r, err := s.DB.ExecContext(ctx, `UPDATE work_messages SET state=?,reason=?,updated_at=? WHERE id=? AND state='sending'`, state, reason, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return ErrStaleWorkTarget
	}
	return nil
}

// Closing a turn is the durable boundary used by both browser submission and worker dispatch.
func (s *Store) CloseProviderTurn(ctx context.Context, attemptID, turnID string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE provider_attempts SET turn_open=0,updated_at=? WHERE attempt_id=? AND turn_id=? AND turn_open=1`, time.Now().UTC().Format(time.RFC3339Nano), attemptID, turnID)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return ErrStaleWorkTarget
	}
	_, err = tx.ExecContext(ctx, `UPDATE work_messages SET state='rejected',reason='target turn finished before delivery',updated_at=? WHERE attempt_id=? AND state='queued' AND target='live' AND expected_turn_id=?`, time.Now().UTC().Format(time.RFC3339Nano), attemptID, turnID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReconcileWorkMessages(ctx context.Context, attemptID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE work_messages SET state=CASE WHEN state='sending' THEN 'unknown' ELSE 'rejected' END,reason=CASE WHEN state='sending' THEN 'provider acknowledgement was lost' ELSE 'target attempt ended before delivery' END,updated_at=? WHERE attempt_id=? AND state IN ('queued','sending')`, time.Now().UTC().Format(time.RFC3339Nano), attemptID)
	return err
}

func scanWorkControl(row *sql.Row) (WorkControl, error) {
	var c WorkControl
	err := row.Scan(&c.ID, &c.RunID, &c.AttemptID, &c.Action, &c.Mode, &c.Instructions, &c.Digest, &c.State, &c.Reason, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

const controlColumns = "id,run_id,attempt_id,action,mode,instructions,digest,state,reason,created_at,updated_at"

func (s *Store) GetWorkControl(ctx context.Context, id string) (WorkControl, error) {
	return scanWorkControl(s.DB.QueryRowContext(ctx, "SELECT "+controlColumns+" FROM work_controls WHERE id=?", id))
}

func (s *Store) SubmitWorkControl(ctx context.Context, c WorkControl) (WorkControl, error) {
	if !validKey(c.ID) || c.RunID == "" || c.AttemptID == "" || (c.Action != "finish" && c.Action != "undo" && c.Action != "interrupt" && c.Action != "continue") || len(c.Instructions) > 32<<10 {
		return WorkControl{}, fmt.Errorf("invalid work control")
	}
	if c.Action == "continue" {
		if c.Mode != "resume" && c.Mode != "fresh" {
			return WorkControl{}, fmt.Errorf("continuation mode required")
		}
	} else if c.Mode != "" || c.Instructions != "" {
		return WorkControl{}, fmt.Errorf("unexpected control content")
	}
	c.Digest = digest(c.RunID, c.AttemptID, c.Action, c.Mode, c.Instructions)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return WorkControl{}, err
	}
	defer tx.Rollback()
	existing, err := scanWorkControl(tx.QueryRowContext(ctx, "SELECT "+controlColumns+" FROM work_controls WHERE id=?", c.ID))
	if err == nil {
		if existing.Digest != c.Digest {
			return WorkControl{}, ErrWorkConflict
		}
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return WorkControl{}, err
	}
	var attemptOutcome, phase, runState, scheduleOutcome, providerState, thread string
	var stop, turnOpen int
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT t.outcome,t.phase,r.state,s.outcome,s.stop_requested,s.revision,COALESCE(p.state,''),COALESCE(p.thread_id,''),COALESCE(p.turn_open,0) FROM assignment_attempts t JOIN assignments a ON a.id=t.assignment_id JOIN runs r ON r.id=a.run_id JOIN run_schedules s ON s.run_id=r.id LEFT JOIN provider_attempts p ON p.attempt_id=t.id WHERE t.id=? AND r.id=? AND r.reservation_state='reserved'`, c.AttemptID, c.RunID).Scan(&attemptOutcome, &phase, &runState, &scheduleOutcome, &stop, &revision, &providerState, &thread, &turnOpen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkControl{}, ErrStaleWorkTarget
		}
		return WorkControl{}, err
	}
	if runState != "running" || scheduleOutcome != "active" || attemptOutcome != "running" {
		return WorkControl{}, ErrStaleWorkTarget
	}
	if c.Action == "undo" {
		var immediate int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM work_controls WHERE run_id=? AND action='interrupt' AND state!='rejected'`, c.RunID).Scan(&immediate); err != nil {
			return WorkControl{}, err
		}
		if immediate != 0 {
			return WorkControl{}, ErrStaleWorkTarget
		}
	}
	var revisionResult sql.Result
	switch c.Action {
	case "finish":
		if stop != 0 {
			return WorkControl{}, ErrStaleWorkTarget
		}
		revisionResult, err = tx.ExecContext(ctx, `UPDATE run_schedules SET stop_requested=1,revision=revision+1 WHERE run_id=? AND revision=? AND outcome='active'`, c.RunID, revision)
	case "undo":
		if stop == 0 {
			return WorkControl{}, ErrStaleWorkTarget
		}
		revisionResult, err = tx.ExecContext(ctx, `UPDATE run_schedules SET stop_requested=0,revision=revision+1 WHERE run_id=? AND revision=? AND outcome='active'`, c.RunID, revision)
	case "interrupt":
		if attemptOutcome != "running" || providerState != "turn_started" || turnOpen == 0 {
			return WorkControl{}, ErrStaleWorkTarget
		}
		revisionResult, err = tx.ExecContext(ctx, `UPDATE run_schedules SET stop_requested=1,revision=revision+1 WHERE run_id=? AND revision=? AND outcome='active'`, c.RunID, revision)
	case "continue":
		if attemptOutcome != "running" || phase != "implementing" || (providerState != "failed" && providerState != "interrupted" && providerState != "unknown" && providerState != "completed") || (c.Mode == "resume" && thread == "") {
			return WorkControl{}, ErrStaleWorkTarget
		}
	}
	if err != nil {
		return WorkControl{}, err
	}
	if revisionResult != nil {
		if n, _ := revisionResult.RowsAffected(); n != 1 {
			return WorkControl{}, ErrStaleWorkTarget
		}
	}
	c.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	c.UpdatedAt = c.CreatedAt
	c.State = "accepted"
	if c.Action == "interrupt" || c.Action == "continue" {
		c.State = "queued"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO work_controls(id,run_id,attempt_id,action,mode,instructions,digest,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, c.ID, c.RunID, c.AttemptID, c.Action, c.Mode, c.Instructions, c.Digest, c.State, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return WorkControl{}, err
	}
	return c, tx.Commit()
}

func (s *Store) ClaimWorkControl(ctx context.Context, runID, action string) (WorkControl, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return WorkControl{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM work_controls WHERE run_id=? AND action=? AND state='queued' ORDER BY created_at,id LIMIT 1`, runID, action).Scan(&id)
	if err != nil {
		return WorkControl{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE work_controls SET state='sending',updated_at=? WHERE id=? AND state='queued'`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return WorkControl{}, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return WorkControl{}, ErrStaleWorkTarget
	}
	c, err := scanWorkControl(tx.QueryRowContext(ctx, "SELECT "+controlColumns+" FROM work_controls WHERE id=?", id))
	if err != nil {
		return WorkControl{}, err
	}
	return c, tx.Commit()
}
func (s *Store) SettleWorkControl(ctx context.Context, id, state, reason string) error {
	if state != "accepted" && state != "rejected" && state != "unknown" {
		return fmt.Errorf("invalid control outcome")
	}
	r, err := s.DB.ExecContext(ctx, `UPDATE work_controls SET state=?,reason=?,updated_at=? WHERE id=? AND state='sending'`, state, reason, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return ErrStaleWorkTarget
	}
	return nil
}

func (s *Store) ReconcileInterruptControls(ctx context.Context, attemptID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE work_controls SET state=CASE WHEN state='sending' THEN 'unknown' ELSE 'rejected' END,reason=CASE WHEN state='sending' THEN 'provider interruption acknowledgement was lost' ELSE 'target turn finished before interruption' END,updated_at=? WHERE attempt_id=? AND action='interrupt' AND state IN ('queued','sending')`, time.Now().UTC().Format(time.RFC3339Nano), attemptID)
	return err
}

func (s *Store) WorkOverview(ctx context.Context, runID string) (WorkOverview, error) {
	var w WorkOverview
	var stop, open int
	err := s.DB.QueryRowContext(ctx, `SELECT r.id,r.project_id,r.state,s.successful_count,s.task_limit,s.stop_requested,s.outcome,s.reason FROM runs r JOIN run_schedules s ON s.run_id=r.id WHERE r.id=?`, runID).Scan(&w.RunID, &w.ProjectID, &w.RunState, &w.SuccessfulCount, &w.TaskLimit, &stop, &w.ScheduleOutcome, &w.ScheduleReason)
	if err != nil {
		return w, err
	}
	w.StopRequested = stop != 0
	var phase, outcome, provider, failure, thread, turn, model, reasoning, sandbox, approval sql.NullString
	err = s.DB.QueryRowContext(ctx, `SELECT a.id,a.issue_number,a.snapshot_json,a.created_at,t.id,t.phase,t.outcome,p.state,p.failure_code,p.thread_id,p.turn_id,p.model,p.reasoning,p.sandbox,p.approval_policy,COALESCE(p.turn_open,0)
 FROM assignments a JOIN assignment_attempts t ON t.assignment_id=a.id LEFT JOIN provider_attempts p ON p.attempt_id=t.id WHERE a.run_id=? ORDER BY a.created_at DESC,t.ordinal DESC LIMIT 1`, runID).Scan(&w.AssignmentID, &w.IssueNumber, &w.SnapshotJSON, &w.AssignmentStartedAt, &w.AttemptID, &phase, &outcome, &provider, &failure, &thread, &turn, &model, &reasoning, &sandbox, &approval, &open)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return w, err
	}
	w.AttemptPhase = phase.String
	w.AttemptOutcome = outcome.String
	w.ProviderState = provider.String
	w.ProviderFailureCode = failure.String
	w.ThreadID = thread.String
	w.TurnID = turn.String
	w.Model = model.String
	w.Reasoning = reasoning.String
	w.Sandbox = sandbox.String
	w.ApprovalPolicy = approval.String
	w.TurnOpen = open != 0
	w.Messages = []WorkMessage{}
	w.Controls = []WorkControl{}
	w.Attempts = []WorkAttempt{}
	if w.AssignmentID != "" {
		attempts, err := s.DB.QueryContext(ctx, `SELECT id,ordinal,phase,outcome,predecessor_id,created_at,settled_at FROM assignment_attempts WHERE assignment_id=? ORDER BY ordinal DESC LIMIT 100`, w.AssignmentID)
		if err != nil {
			return w, err
		}
		for attempts.Next() {
			var a WorkAttempt
			var predecessor, settled sql.NullString
			if err = attempts.Scan(&a.ID, &a.Ordinal, &a.Phase, &a.Outcome, &predecessor, &a.CreatedAt, &settled); err != nil {
				break
			}
			a.PredecessorID = predecessor.String
			a.SettledAt = settled.String
			w.Attempts = append(w.Attempts, a)
		}
		if err == nil {
			err = attempts.Err()
		}
		attempts.Close()
		if err != nil {
			return w, err
		}
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT "+messageColumns+" FROM work_messages WHERE run_id=? ORDER BY created_at DESC LIMIT 50", runID)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var m WorkMessage
		var changed int
		if err = rows.Scan(&m.ID, &m.RunID, &m.AssignmentID, &m.AttemptID, &m.ThreadID, &m.ExpectedTurnID, &m.Target, &m.Body, &m.Digest, &m.State, &m.Reason, &changed, &m.CreatedAt, &m.UpdatedAt); err != nil {
			break
		}
		m.ContractChanged = changed != 0
		w.Messages = append(w.Messages, m)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return w, err
	}
	controls, err := s.DB.QueryContext(ctx, "SELECT "+controlColumns+" FROM work_controls WHERE run_id=? ORDER BY created_at DESC LIMIT 30", runID)
	if err != nil {
		return w, err
	}
	for controls.Next() {
		var c WorkControl
		if err = controls.Scan(&c.ID, &c.RunID, &c.AttemptID, &c.Action, &c.Mode, &c.Instructions, &c.Digest, &c.State, &c.Reason, &c.CreatedAt, &c.UpdatedAt); err != nil {
			break
		}
		w.Controls = append(w.Controls, c)
	}
	if err == nil {
		err = controls.Err()
	}
	controls.Close()
	return w, err
}

// A continuation is linked to the same immutable assignment and can only be
// admitted after the prior provider is terminal and its process has settled.
func (s *Store) BeginContinuation(ctx context.Context, c WorkControl) (string, string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var controlState, controlMode string
	if err := tx.QueryRowContext(ctx, `SELECT state,mode FROM work_controls WHERE id=? AND run_id=? AND attempt_id=? AND action='continue'`, c.ID, c.RunID, c.AttemptID).Scan(&controlState, &controlMode); err != nil {
		return "", "", err
	}
	if controlState != "sending" || controlMode != c.Mode {
		return "", "", ErrStaleWorkTarget
	}
	var assignment, phase, outcome, provider, thread string
	var ordinal int
	err = tx.QueryRowContext(ctx, `SELECT t.assignment_id,t.phase,t.outcome,p.state,p.thread_id,t.ordinal FROM assignment_attempts t JOIN assignments a ON a.id=t.assignment_id JOIN provider_attempts p ON p.attempt_id=t.id JOIN runs r ON r.id=a.run_id WHERE t.id=? AND r.id=? AND a.state='running' AND r.state='running' AND r.reservation_state='reserved'`, c.AttemptID, c.RunID).Scan(&assignment, &phase, &outcome, &provider, &thread, &ordinal)
	if err != nil {
		return "", "", err
	}
	if outcome != "running" || phase != "implementing" || (provider != "failed" && provider != "interrupted" && provider != "unknown" && provider != "completed") || (c.Mode == "resume" && thread == "") {
		return "", "", ErrStaleWorkTarget
	}
	id, err := newRandomID("attempt_", 16)
	if err != nil {
		return "", "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r, err := tx.ExecContext(ctx, `UPDATE assignment_attempts SET phase='settled',outcome='interrupted',settled_at=? WHERE id=? AND outcome='running'`, now, c.AttemptID)
	if err != nil {
		return "", "", err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return "", "", ErrStaleWorkTarget
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,predecessor_id,created_at) VALUES(?,?,?,'implementing','running',?,?)`, id, assignment, ordinal+1, c.AttemptID, now)
	if err != nil {
		return "", "", err
	}
	_, err = tx.ExecContext(ctx, `UPDATE assignments SET attempt_number=? WHERE id=?`, ordinal+1, assignment)
	if err != nil {
		return "", "", err
	}
	result, err := tx.ExecContext(ctx, `UPDATE work_controls SET state='accepted',reason=?,updated_at=? WHERE id=? AND state='sending'`, "linked attempt "+id+" created", now, c.ID)
	if err != nil {
		return "", "", err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return "", "", ErrStaleWorkTarget
	}
	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	return id, thread, nil
}
