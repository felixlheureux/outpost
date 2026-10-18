package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/validation"
	"github.com/felixlheureux/outpost/internal/worker"
	"github.com/spf13/cobra"
)

func addChecks(root *cobra.Command, stateDir *string) {
	var projectID, file string
	var makeDefault bool
	cmd := &cobra.Command{Use: "checks", Short: "Configure operator-owned project check profiles"}
	set := &cobra.Command{Use: "set", Short: "Set the default check profile for a project", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if projectID == "" || file == "" {
			return fmt.Errorf("--project and --file are required")
		}
		profile, err := validation.Load(file)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(profile)
		if err != nil {
			return err
		}
		db, err := store.Open(c.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.CheckSchema(c.Context()); err != nil {
			return err
		}
		if err := db.SetCheckProfile(c.Context(), projectID, profile.ID, string(raw), makeDefault); err != nil {
			return err
		}
		_, err = fmt.Fprintf(c.OutOrStdout(), "Configured check profile %s for project %s.\n", profile.ID, projectID)
		return err
	}}
	set.Flags().StringVar(&projectID, "project", "", "Registered project ID")
	set.Flags().StringVar(&file, "file", "", "Operator-owned JSON check profile")
	set.Flags().BoolVar(&makeDefault, "default", true, "Use this profile for inherited checks")
	cmd.AddCommand(set)
	var runID string
	recoverCmd := &cobra.Command{Use: "recover", Short: "Reconcile a stopped worker's exact check child or terminal receipt", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if runID == "" {
			return fmt.Errorf("--run-id is required")
		}
		db, err := store.Open(c.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.CheckSchema(c.Context()); err != nil {
			return err
		}
		run, err := db.GetRun(c.Context(), runID)
		if err != nil {
			return err
		}
		if run.WorkerPID > 0 {
			if ticks, e := worker.StartTicks(int(run.WorkerPID)); e == nil && ticks == run.WorkerStartTicks {
				return fmt.Errorf("recorded worker is still alive; inspect its unit before recovery")
			}
		}
		lock, err := worker.LockWorkspace(*stateDir, run.WorkspacePath)
		if err != nil {
			return err
		}
		defer lock.Close()
		assignment, err := db.CurrentAssignment(c.Context(), runID)
		if err != nil {
			return err
		}
		schedule, err := db.GetSchedule(c.Context(), runID)
		if err != nil {
			return err
		}
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		checks := validation.Runner{DB: db, StateDir: *stateDir, Workspace: run.WorkspacePath, Executable: binary, Lock: lock}
		result, err := checks.Run(c.Context(), assignment, schedule)
		if err != nil {
			return err
		}
		if !result.Passed {
			return fmt.Errorf("validation remains failed: %s", result.Failure)
		}
		if err := db.RecordCheckSuccess(c.Context(), assignment.ID, result.ReceiptID); err != nil {
			return err
		}
		if err := db.AdvanceAttemptPhase(c.Context(), assignment.ID, "validating", "closing"); err != nil {
			return err
		}
		_, err = fmt.Fprintf(c.OutOrStdout(), "Validation passed for candidate %s (receipt %s); delivery remains pending.\n", result.Candidate, result.ReceiptID)
		return err
	}}
	recoverCmd.Flags().StringVar(&runID, "run-id", "", "Recorded run ID")
	cmd.AddCommand(recoverCmd)
	root.AddCommand(cmd)
}
