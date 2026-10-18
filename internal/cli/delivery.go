package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/felixlheureux/outpost/internal/delivery"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/supervisor"
	"github.com/felixlheureux/outpost/internal/worker"
	"github.com/spf13/cobra"
)

func addDelivery(root *cobra.Command, stateDir *string) {
	cmd := &cobra.Command{Use: "delivery", Short: "Inspect and recover a recorded Git delivery"}
	var runID string
	recoverCmd := &cobra.Command{Use: "recover", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if runID == "" {
			return fmt.Errorf("--run-id is required")
		}
		db, err := store.Open(c.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = db.CheckSchema(c.Context()); err != nil {
			return err
		}
		run, err := db.GetRun(c.Context(), runID)
		if err != nil {
			return err
		}
		if run.WorkerPID > 0 {
			if ticks, e := worker.StartTicks(int(run.WorkerPID)); e == nil && ticks == run.WorkerStartTicks {
				return fmt.Errorf("recorded worker is still alive")
			}
		}
		lock, err := worker.LockWorkspace(*stateDir, run.WorkspacePath)
		if err != nil {
			return err
		}
		defer lock.Close()
		a, err := db.CurrentAssignment(c.Context(), runID)
		if err != nil {
			return err
		}
		schedule, err := db.GetSchedule(c.Context(), runID)
		if err != nil {
			return err
		}
		var phase, checkID string
		err = db.DB.QueryRowContext(c.Context(), `SELECT t.phase,a.checks_receipt FROM assignment_attempts t JOIN assignments a ON a.id=t.assignment_id WHERE a.id=? AND t.outcome='running' ORDER BY t.ordinal DESC LIMIT 1`, a.ID).Scan(&phase, &checkID)
		if err != nil {
			return err
		}
		if checkID == "" {
			return fmt.Errorf("required validation is not settled")
		}
		if phase != "closing" && phase != "delivering" && phase != "awaiting_review" {
			return fmt.Errorf("assignment is in %s", phase)
		}
		check, err := db.GetRunCheck(c.Context(), runID, checkID)
		if err != nil {
			return err
		}
		_, review, err := delivery.Policy(a, schedule)
		if err != nil {
			return err
		}
		if phase == "closing" && review == "required" {
			if err = db.AdvanceAttemptPhase(c.Context(), a.ID, "closing", "awaiting_review"); err != nil {
				return err
			}
			phase = "awaiting_review"
		}
		if review == "required" {
			decision, _, e := db.ReviewDecision(c.Context(), a.ID, check.CandidateDigest)
			if errors.Is(e, sql.ErrNoRows) {
				return fmt.Errorf("review approval is pending for candidate %s", check.CandidateDigest)
			}
			if e != nil {
				return e
			}
			if decision != "approved" {
				return fmt.Errorf("review rejected; resume linked repair through the worker")
			}
		}
		if phase == "closing" {
			if err = db.AdvanceAttemptPhase(c.Context(), a.ID, "closing", "delivering"); err != nil {
				return err
			}
		}
		if phase == "awaiting_review" {
			if err = db.AdvanceAttemptPhase(c.Context(), a.ID, "awaiting_review", "delivering"); err != nil {
				return err
			}
		}
		result, err := delivery.Deliver(c.Context(), db, run.WorkspacePath, a, schedule, check.CandidateDigest)
		if err != nil {
			return err
		}
		var recorded string
		if err = db.DB.QueryRowContext(c.Context(), `SELECT delivery_receipt FROM assignments WHERE id=?`, a.ID).Scan(&recorded); err != nil {
			return err
		}
		if recorded == "" {
			if err = db.RecordCompletionReceipt(c.Context(), a.ID, "delivery", result.Receipt.ID); err != nil {
				return err
			}
		} else if recorded != result.Receipt.ID {
			return store.ErrAdmissionChanged
		}
		if err = db.SettleAssignment(c.Context(), a.ID, "succeeded", result.Head); err != nil {
			return err
		}
		schedule, err = db.GetSchedule(c.Context(), runID)
		if err != nil {
			return err
		}
		if schedule.Outcome == "active" {
			outcome, reason := "blocked", "manual delivery recovery settled; start a new run for remaining work"
			if schedule.SuccessfulCount >= schedule.TaskLimit {
				outcome, reason = "complete", "task limit reached"
			}
			if err = db.SettleSchedule(c.Context(), runID, schedule.Revision, outcome, reason); err != nil {
				return err
			}
		}
		if err = lock.Close(); err != nil {
			return err
		}
		run, err = db.GetRun(c.Context(), runID)
		if err != nil {
			return err
		}
		if err = supervisor.ReconcileTerminal(c.Context(), db, *stateDir, run); err != nil {
			return fmt.Errorf("delivery settled, but run ownership needs reconciliation: %w", err)
		}
		var reportPolicy struct {
			CloseIssue bool `json:"close_issue"`
		}
		_ = json.Unmarshal([]byte(schedule.PolicyJSON), &reportPolicy)
		if _, err = delivery.Report(c.Context(), db, a, reportPolicy.CloseIssue, nil); err != nil {
			fmt.Fprintln(c.ErrOrStderr(), "GitHub report pending:", err)
		}
		fmt.Fprintf(c.OutOrStdout(), "Delivery settled for %s (%s).\n", a.ID, result.Receipt.ID)
		return nil
	}}
	recoverCmd.Flags().StringVar(&runID, "run-id", "", "Recorded run ID")
	cmd.AddCommand(recoverCmd)
	var releaseRun string
	release := &cobra.Command{Use: "release", Args: cobra.NoArgs, Short: "Reconcile an inactive terminal worker and release its slot", RunE: func(c *cobra.Command, _ []string) error {
		if releaseRun == "" {
			return fmt.Errorf("--run-id is required")
		}
		db, err := store.Open(c.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = db.CheckSchema(c.Context()); err != nil {
			return err
		}
		run, err := db.GetRun(c.Context(), releaseRun)
		if err != nil {
			return err
		}
		if err = supervisor.ReconcileTerminal(c.Context(), db, *stateDir, run); err != nil {
			return err
		}
		_, err = fmt.Fprintln(c.OutOrStdout(), "Terminal run ownership reconciled.")
		return err
	}}
	release.Flags().StringVar(&releaseRun, "run-id", "", "Recorded run ID")
	cmd.AddCommand(release)
	root.AddCommand(cmd)

	var assignmentID, candidate, decision, feedback string
	reviewCmd := &cobra.Command{Use: "review", Short: "Record an explicit candidate review decision"}
	decide := &cobra.Command{Use: "decide", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if assignmentID == "" || candidate == "" {
			return fmt.Errorf("--assignment and --candidate are required")
		}
		db, err := store.Open(c.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = db.CheckSchema(c.Context()); err != nil {
			return err
		}
		if err = db.DecideReview(c.Context(), assignmentID, candidate, decision, feedback); err != nil {
			return err
		}
		_, err = fmt.Fprintln(c.OutOrStdout(), "Review decision recorded.")
		return err
	}}
	decide.Flags().StringVar(&assignmentID, "assignment", "", "Pinned assignment ID")
	decide.Flags().StringVar(&candidate, "candidate", "", "Validated candidate digest")
	decide.Flags().StringVar(&decision, "decision", "approved", "approved or rejected")
	decide.Flags().StringVar(&feedback, "feedback", "", "Review feedback for a rejected candidate")
	reviewCmd.AddCommand(decide)
	root.AddCommand(reviewCmd)

	var reportRun string
	reports := &cobra.Command{Use: "reports", Short: "Retry pending GitHub result operations"}
	retry := &cobra.Command{Use: "retry", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if reportRun == "" {
			return fmt.Errorf("--run-id is required")
		}
		db, err := store.Open(c.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = db.CheckSchema(c.Context()); err != nil {
			return err
		}
		schedule, err := db.GetSchedule(c.Context(), reportRun)
		if err != nil {
			return err
		}
		var policy struct {
			CloseIssue bool `json:"close_issue"`
		}
		if err = json.Unmarshal([]byte(schedule.PolicyJSON), &policy); err != nil {
			return err
		}
		pending, err := db.UnreportedAssignments(c.Context(), reportRun)
		if err != nil {
			return err
		}
		for _, a := range pending {
			r, e := delivery.Report(c.Context(), db, a, policy.CloseIssue, nil)
			if e != nil {
				return fmt.Errorf("report %s pending: %w", a.ID, e)
			}
			fmt.Fprintf(c.OutOrStdout(), "Report %s settled.\n", r.ID)
		}
		return nil
	}}
	retry.Flags().StringVar(&reportRun, "run-id", "", "Recorded run ID")
	reports.AddCommand(retry)
	root.AddCommand(reports)
}
