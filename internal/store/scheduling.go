package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrAdmissionChanged = errors.New("admission inputs changed; refresh before retrying")

type Schedule struct {
	RunID, Kind, Mode, ScopeJSON, PolicyJSON, Instructions string
	TaskLimit, SuccessfulCount                             int
	DeadlineAt, Outcome, Reason                            string
	Revision                                               int64
	StopRequested                                          bool
}

type Assignment struct {
	ID, RunID, ProjectID, State, SnapshotJSON string
	IssueNumber, Generation, AttemptNumber    int
}

type Generation struct {
	IssueNumber, Number int
	State, BaselineHead string
	DependsOn           []int
}

func scanSchedule(row *sql.Row) (Schedule, error) {
	var v Schedule
	var stop int
	var deadline sql.NullString
	err := row.Scan(&v.RunID, &v.Kind, &v.Mode, &v.ScopeJSON, &v.PolicyJSON, &v.Instructions,
		&v.TaskLimit, &deadline, &v.SuccessfulCount, &v.Revision, &stop, &v.Outcome, &v.Reason)
	v.DeadlineAt, v.StopRequested = deadline.String, stop != 0
	return v, err
}

const scheduleColumns = "run_id,kind,mode,scope_json,policy_json,instructions,task_limit,deadline_at,successful_count,revision,stop_requested,outcome,reason"

func (s *Store) GetSchedule(ctx context.Context, runID string) (Schedule, error) {
	return scanSchedule(s.DB.QueryRowContext(ctx, "SELECT "+scheduleColumns+" FROM run_schedules WHERE run_id=?", runID))
}

// ConfigureRun captures operator policy once, before the worker can claim it.
func (s *Store) ConfigureRun(ctx context.Context, v Schedule) error {
	if v.RunID == "" {
		return fmt.Errorf("run identity is required")
	}
	if err := validateSchedule(v); err != nil {
		return err
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO run_schedules(run_id,kind,mode,scope_json,policy_json,instructions,task_limit,deadline_at,created_at)
		SELECT id,?,?,?,?,?,?,?,? FROM runs WHERE id=? AND state='reserved' ON CONFLICT(run_id) DO NOTHING`,
		v.Kind, v.Mode, v.ScopeJSON, v.PolicyJSON, v.Instructions, v.TaskLimit, nullable(v.DeadlineAt), time.Now().UTC().Format(time.RFC3339Nano), v.RunID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func validateSchedule(v Schedule) error {
	if v.TaskLimit < 1 || v.TaskLimit > 100 || len(v.Instructions) > 32<<10 || len(v.PolicyJSON) > 32<<10 || len(v.ScopeJSON) > 8192 || !json.Valid([]byte(v.PolicyJSON)) || !json.Valid([]byte(v.ScopeJSON)) {
		return fmt.Errorf("invalid run schedule")
	}
	var policy map[string]any
	if err := json.Unmarshal([]byte(v.PolicyJSON), &policy); err != nil || policy == nil {
		return fmt.Errorf("run policy must be an object")
	}
	var scope []int
	if err := json.Unmarshal([]byte(v.ScopeJSON), &scope); err != nil || len(scope) > 100 {
		return fmt.Errorf("run scope must be an issue list")
	}
	if v.Mode == "fixed" || v.Kind == "task" {
		if len(scope) == 0 {
			return fmt.Errorf("fixed run scope is empty")
		}
	}
	if v.Mode == "follow" && len(scope) != 0 {
		return fmt.Errorf("follow run cannot capture a fixed scope")
	}
	seen := map[int]bool{}
	for _, n := range scope {
		if n <= 0 || seen[n] {
			return fmt.Errorf("run scope has invalid issue identities")
		}
		seen[n] = true
	}
	if v.Kind == "task" && len(scope) != 1 {
		return fmt.Errorf("task run requires one issue")
	}
	if v.DeadlineAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, v.DeadlineAt); err != nil {
			return fmt.Errorf("invalid run deadline")
		}
	}
	if (v.Kind == "queue" && v.Mode != "follow" && v.Mode != "fixed") || ((v.Kind == "task" || v.Kind == "instruction") && v.Mode != "single") {
		return fmt.Errorf("invalid run kind or mode")
	}
	return nil
}

// ReserveScheduledRun persists the global slot, launch identity, and captured
// schedule atomically. A replay with the same request key returns the original
// record only when its scope and policy match exactly.
func (s *Store) ReserveScheduledRun(ctx context.Context, projectID, workspacePath, requestKey string, schedule Schedule) (Run, bool, error) {
	var zero Run
	if projectID == "" || len(requestKey) < 8 || len(requestKey) > 128 || strings.TrimSpace(requestKey) != requestKey {
		return zero, false, fmt.Errorf("invalid run identity")
	}
	if err := validateSchedule(schedule); err != nil {
		return zero, false, err
	}
	workspace, err := canonicalWorkspace(workspacePath)
	if err != nil {
		return zero, false, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return zero, false, err
	}
	defer tx.Rollback()
	if existing, err := scanRun(tx.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE request_key=?", requestKey)); err == nil {
		if existing.ProjectID != projectID || existing.WorkspacePath != workspace {
			return zero, false, ErrRunConflict
		}
		captured, err := scanSchedule(tx.QueryRowContext(ctx, "SELECT "+scheduleColumns+" FROM run_schedules WHERE run_id=?", existing.ID))
		if err != nil || captured.Kind != schedule.Kind || captured.Mode != schedule.Mode || captured.ScopeJSON != schedule.ScopeJSON || captured.PolicyJSON != schedule.PolicyJSON || captured.Instructions != schedule.Instructions || captured.TaskLimit != schedule.TaskLimit || captured.DeadlineAt != schedule.DeadlineAt {
			return zero, false, ErrRunConflict
		}
		return existing, true, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return zero, false, err
	}
	id, err := newRandomID("run_", 16)
	if err != nil {
		return zero, false, err
	}
	token, err := newRandomID("", 32)
	if err != nil {
		return zero, false, err
	}
	unit := "outpost-" + id + ".service"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES(?,?,?,?,?,?,'reserved','pending',?,?)`, id, projectID, requestKey, workspace, token, unit, now, now)
	if err != nil {
		var occupied int
		if checkErr := tx.QueryRowContext(ctx, `SELECT count(*) FROM runs WHERE reservation_state='reserved'`).Scan(&occupied); checkErr == nil && occupied > 0 {
			return zero, false, ErrSlotBusy
		}
		return zero, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO run_schedules(run_id,kind,mode,scope_json,policy_json,instructions,task_limit,deadline_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, schedule.Kind, schedule.Mode, schedule.ScopeJSON, schedule.PolicyJSON, schedule.Instructions, schedule.TaskLimit, nullable(schedule.DeadlineAt), now)
	if err != nil {
		return zero, false, err
	}
	if err = tx.Commit(); err != nil {
		return zero, false, err
	}
	return Run{ID: id, ProjectID: projectID, RequestKey: requestKey, WorkspacePath: workspace, LaunchToken: token, UnitName: unit, State: "reserved", LaunchObservation: "pending", ReservationState: "reserved"}, false, nil
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Store) RequestFinishAfterTask(ctx context.Context, runID string, expectedRevision int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE run_schedules SET stop_requested=1,revision=revision+1 WHERE run_id=? AND revision=? AND outcome='active' AND stop_requested=0`, runID, expectedRevision)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) UndoFinishAfterTask(ctx context.Context, runID string, expectedRevision int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE run_schedules SET stop_requested=0,revision=revision+1 WHERE run_id=? AND revision=? AND outcome='active' AND stop_requested=1`, runID, expectedRevision)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

func (s *Store) ListGenerations(ctx context.Context, projectID string) (map[int]Generation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT g.issue_number,g.generation,g.state,g.baseline_head,a.snapshot_json FROM execution_generations g LEFT JOIN assignments a ON a.id=g.assignment_id WHERE g.project_id=? ORDER BY g.issue_number,g.generation DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int]Generation{}
	for rows.Next() {
		var g Generation
		var snapshot sql.NullString
		if err := rows.Scan(&g.IssueNumber, &g.Number, &g.State, &g.BaselineHead, &snapshot); err != nil {
			return nil, err
		}
		if snapshot.Valid {
			var pinned struct {
				Requirements struct {
					Settings struct {
						DependsOn []int `json:"depends_on"`
					} `json:"settings"`
				} `json:"requirements"`
			}
			if err := json.Unmarshal([]byte(snapshot.String), &pinned); err != nil {
				return nil, err
			}
			g.DependsOn = pinned.Requirements.Settings.DependsOn
		}
		if _, found := result[g.IssueNumber]; !found {
			result[g.IssueNumber] = g
		}
	}
	return result, rows.Err()
}

func (s *Store) CurrentAssignment(ctx context.Context, runID string) (Assignment, error) {
	var a Assignment
	err := s.DB.QueryRowContext(ctx, `SELECT id,run_id,project_id,issue_number,generation,attempt_number,state,snapshot_json FROM assignments WHERE run_id=? AND state!='succeeded' ORDER BY created_at DESC LIMIT 1`, runID).
		Scan(&a.ID, &a.RunID, &a.ProjectID, &a.IssueNumber, &a.Generation, &a.AttemptNumber, &a.State, &a.SnapshotJSON)
	return a, err
}

// BeginAttempt is the provider handoff. Only the current pending assignment
// can enter implementation, and the attempt retains a separate phase receipt.
func (s *Store) BeginAttempt(ctx context.Context, assignmentID string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE assignments SET state='running' WHERE id=? AND state='pending'`, assignmentID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	result, err = tx.ExecContext(ctx, `UPDATE assignment_attempts SET phase='implementing',outcome='running' WHERE assignment_id=? AND ordinal=1 AND phase='preparing' AND outcome='pending'`, assignmentID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return tx.Commit()
}

func (s *Store) AdvanceAttemptPhase(ctx context.Context, assignmentID, expected, next string) error {
	valid := (expected == "implementing" && next == "validating") || (expected == "validating" && (next == "repairing" || next == "closing")) || (expected == "repairing" && next == "validating") || (expected == "closing" && (next == "delivering" || next == "awaiting_review")) || (expected == "awaiting_review" && next == "delivering")
	if !valid {
		return fmt.Errorf("invalid attempt phase transition")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE assignment_attempts SET phase=? WHERE assignment_id=? AND phase=? AND outcome='running'`, next, assignmentID, expected)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

// RecordCompletionReceipt records a verified delivery receipt. Check receipts
// use RecordCheckSuccess, which validates the runner-owned terminal row.
func (s *Store) RecordCompletionReceipt(ctx context.Context, assignmentID, kind, receiptID string) error {
	if receiptID == "" || len(receiptID) > 160 {
		return fmt.Errorf("completion receipt identity is required")
	}
	if kind != "delivery" {
		return fmt.Errorf("invalid completion receipt kind")
	}
	column, phase := "delivery_receipt", "delivering"
	result, err := s.DB.ExecContext(ctx, `UPDATE assignments SET `+column+`=? WHERE id=? AND state='running' AND `+column+`='' AND EXISTS(SELECT 1 FROM assignment_attempts WHERE assignment_id=assignments.id AND phase=? AND outcome='running') AND EXISTS(SELECT 1 FROM delivery_receipts WHERE id=? AND assignment_id=assignments.id AND state='settled')`, receiptID, assignmentID, phase, receiptID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

type ClaimInput struct {
	RunID, ProjectID, QueueSHA, IssueDigest, SnapshotJSON string
	SourceDigests                                         map[int]string
	IssueNumber, TaskLimit, Generation                    int
	ExpectedRevision                                      int64
}

// ClaimAssignment compares the observed local source and control revision in
// one transaction. Remote changes after that observation belong to the next
// admission boundary; GitHub and SQLite do not form a distributed transaction.
func (s *Store) ClaimAssignment(ctx context.Context, input ClaimInput) (Assignment, error) {
	var zero Assignment
	if input.RunID == "" || input.ProjectID == "" || input.QueueSHA == "" || input.IssueDigest == "" || input.IssueNumber <= 0 || input.Generation <= 0 || !json.Valid([]byte(input.SnapshotJSON)) {
		return zero, fmt.Errorf("invalid assignment claim")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	var revision int64
	var stop, count, limit int
	var deadline sql.NullString
	var outcome, runState, projectID, queueSHA, queueState, digest, issueState, issueStatus string
	err = tx.QueryRowContext(ctx, `SELECT s.revision,s.stop_requested,s.successful_count,s.task_limit,s.deadline_at,s.outcome,r.state,r.project_id,q.blob_sha,q.state,i.content_digest,i.state,iss.state
		FROM run_schedules s JOIN runs r ON r.id=s.run_id JOIN queue_sources q ON q.project_id=r.project_id
		JOIN issue_sources i ON i.project_id=r.project_id AND i.number=? JOIN issue_source_status iss ON iss.project_id=r.project_id
		WHERE s.run_id=?`, input.IssueNumber, input.RunID).Scan(&revision, &stop, &count, &limit, &deadline, &outcome, &runState, &projectID, &queueSHA, &queueState, &digest, &issueState, &issueStatus)
	if err != nil {
		return zero, err
	}
	if revision != input.ExpectedRevision || stop != 0 || outcome != "active" || runState != "running" || projectID != input.ProjectID || queueState != "ready" || issueStatus != "ready" || queueSHA != input.QueueSHA || digest != input.IssueDigest || issueState != "open" || count >= limit || (deadline.Valid && deadline.String <= time.Now().UTC().Format(time.RFC3339Nano)) {
		return zero, ErrAdmissionChanged
	}
	var occupied int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM assignments WHERE run_id=? AND state!='succeeded'`, input.RunID).Scan(&occupied); err != nil {
		return zero, err
	}
	if occupied != 0 {
		return zero, ErrAdmissionChanged
	}
	for number, expected := range input.SourceDigests {
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT content_digest FROM issue_sources WHERE project_id=? AND number=?`, input.ProjectID, number).Scan(&current); err != nil || current != expected {
			return zero, ErrAdmissionChanged
		}
	}
	var priorState string
	err = tx.QueryRowContext(ctx, `SELECT state FROM execution_generations WHERE project_id=? AND issue_number=? AND generation=?`, input.ProjectID, input.IssueNumber, input.Generation).Scan(&priorState)
	if err != nil && err != sql.ErrNoRows {
		return zero, err
	}
	if err == nil && priorState != "available" {
		return zero, ErrAdmissionChanged
	}
	if err == sql.ErrNoRows && input.Generation != 1 {
		return zero, ErrAdmissionChanged
	}
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, `INSERT INTO execution_generations(project_id,issue_number,generation,state) VALUES(?,?,?,'available')`, input.ProjectID, input.IssueNumber, input.Generation); err != nil {
			return zero, err
		}
	}
	id, err := newRandomID("assignment_", 16)
	if err != nil {
		return zero, err
	}
	attemptID, err := newRandomID("attempt_", 16)
	if err != nil {
		return zero, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES(?,?,?,?,?,'pending',?,?)`, id, input.RunID, input.ProjectID, input.IssueNumber, input.Generation, input.SnapshotJSON, now); err != nil {
		return zero, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES(?,?,1,'preparing','pending',?)`, attemptID, id, now); err != nil {
		return zero, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE execution_generations SET state='owned',assignment_id=? WHERE project_id=? AND issue_number=? AND generation=?`, id, input.ProjectID, input.IssueNumber, input.Generation); err != nil {
		return zero, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE run_schedules SET revision=revision+1 WHERE run_id=? AND revision=?`, input.RunID, input.ExpectedRevision); err != nil {
		return zero, err
	}
	if err = tx.Commit(); err != nil {
		return zero, err
	}
	return Assignment{ID: id, RunID: input.RunID, ProjectID: input.ProjectID, IssueNumber: input.IssueNumber, Generation: input.Generation, AttemptNumber: 1, State: "pending", SnapshotJSON: input.SnapshotJSON}, nil
}

// SettleAssignment is called only after the owning worker has evidence from
// validation and delivery. A failed assignment remains owned and stops advance.
func (s *Store) SettleAssignment(ctx context.Context, id, state, baselineHead string) error {
	if state != "succeeded" && state != "failed" && state != "blocked" && state != "interrupted" {
		return fmt.Errorf("invalid assignment outcome")
	}
	if state == "succeeded" && baselineHead == "" {
		return fmt.Errorf("successful assignment requires a verified checkout head")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var runID, projectID string
	var issue, generation int
	var prior string
	err = tx.QueryRowContext(ctx, `SELECT run_id,project_id,issue_number,generation,state FROM assignments WHERE id=?`, id).Scan(&runID, &projectID, &issue, &generation, &prior)
	if err != nil {
		return err
	}
	if prior != "pending" && prior != "running" {
		return ErrAdmissionChanged
	}
	if state == "succeeded" {
		var checks, delivery string
		if err := tx.QueryRowContext(ctx, `SELECT checks_receipt,delivery_receipt FROM assignments WHERE id=?`, id).Scan(&checks, &delivery); err != nil {
			return err
		}
		var verified int
		if checks != "" {
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM check_receipts WHERE id=? AND assignment_id=? AND status IN ('passed','reused') AND candidate_digest!=''`, checks, id).Scan(&verified); err != nil {
				return err
			}
		}
		var delivered int
		if delivery != "" {
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM delivery_receipts d JOIN check_receipts c ON c.id=? WHERE d.id=? AND d.assignment_id=? AND d.state='settled' AND d.candidate_digest=c.candidate_digest`, checks, delivery, id).Scan(&delivered); err != nil {
				return err
			}
		}
		if prior != "running" || checks == "" || delivery == "" || verified != 1 || delivered != 1 {
			return fmt.Errorf("successful assignment requires settled check and delivery receipts")
		}
		var phase string
		if err := tx.QueryRowContext(ctx, `SELECT phase FROM assignment_attempts WHERE assignment_id=? AND outcome='running' ORDER BY ordinal DESC LIMIT 1`, id).Scan(&phase); err != nil {
			return err
		}
		if phase != "delivering" {
			return fmt.Errorf("assignment is not in delivery phase")
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `UPDATE assignments SET state=?,settled_at=? WHERE id=?`, state, now, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE assignment_attempts SET phase='settled',outcome=?,settled_at=? WHERE assignment_id=? AND outcome IN ('pending','running')`, state, now, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE execution_generations SET state=?,baseline_head=? WHERE project_id=? AND issue_number=? AND generation=?`, state, baselineHead, projectID, issue, generation); err != nil {
		return err
	}
	if state == "succeeded" {
		_, err = tx.ExecContext(ctx, `UPDATE run_schedules SET successful_count=successful_count+1,revision=revision+1 WHERE run_id=?`, runID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE run_schedules SET outcome=?,reason='assigned task did not settle successfully',revision=revision+1 WHERE run_id=?`, state, runID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SettleSchedule(ctx context.Context, runID string, revision int64, outcome, reason string) error {
	if outcome != "complete" && outcome != "blocked" && outcome != "stopped" {
		return fmt.Errorf("invalid schedule outcome")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE run_schedules SET outcome=?,reason=?,revision=revision+1 WHERE run_id=? AND revision=? AND outcome='active' AND NOT EXISTS(SELECT 1 FROM assignments WHERE run_id=? AND state!='succeeded')`, outcome, reason, runID, revision, runID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrAdmissionChanged
	}
	return nil
}

// ReopenGeneration is an explicit operator action. Source edits and GitHub
// reopen events never call it and cannot erase prior assignment receipts.
func (s *Store) ReopenGeneration(ctx context.Context, projectID string, issueNumber, expectedGeneration int) (Generation, error) {
	if projectID == "" || issueNumber <= 0 || expectedGeneration <= 0 {
		return Generation{}, fmt.Errorf("invalid generation identity")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Generation{}, err
	}
	defer tx.Rollback()
	var number int
	var state string
	err = tx.QueryRowContext(ctx, `SELECT generation,state FROM execution_generations WHERE project_id=? AND issue_number=? ORDER BY generation DESC LIMIT 1`, projectID, issueNumber).Scan(&number, &state)
	if err != nil {
		return Generation{}, err
	}
	if number != expectedGeneration || state != "succeeded" {
		return Generation{}, ErrAdmissionChanged
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM runs WHERE project_id=? AND reservation_state='reserved'`, projectID).Scan(&active); err != nil {
		return Generation{}, err
	}
	if active != 0 {
		return Generation{}, ErrSlotBusy
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO execution_generations(project_id,issue_number,generation,state) VALUES(?,?,?,'available')`, projectID, issueNumber, number+1); err != nil {
		return Generation{}, err
	}
	if err = tx.Commit(); err != nil {
		return Generation{}, err
	}
	return Generation{IssueNumber: issueNumber, Number: number + 1, State: "available"}, nil
}
