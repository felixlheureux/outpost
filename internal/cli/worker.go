package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/felixlheureux/outpost/internal/codexadapter"
	"github.com/felixlheureux/outpost/internal/providerwork"
	"github.com/felixlheureux/outpost/internal/scheduler"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/worker"
	"github.com/spf13/cobra"
)

func addWorker(root *cobra.Command, stateDir *string) {
	var runID, token string
	var fixtureHold bool
	cmd := &cobra.Command{Use: "worker", Short: "Execute one previously reserved run under user systemd", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if runID == "" || token == "" || !filepath.IsAbs(*stateDir) {
			return fmt.Errorf("worker requires a recorded run, launch token and absolute state directory")
		}
		db, err := store.Open(cmd.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.CheckSchema(cmd.Context()); err != nil {
			return err
		}
		lock, pid, ticks, err := worker.Claim(cmd.Context(), db, *stateDir, runID, token)
		if err != nil {
			return err
		}
		defer lock.Close()
		if !fixtureHold {
			var codexPath string
			var run store.Run
			var schedule store.Schedule
			err := func() error {
				codexPath, err = codexadapter.FindExecutable()
				if err != nil {
					return fmt.Errorf("Codex is not installed for the worker user: %w", err)
				}
				run, err = db.GetRun(cmd.Context(), runID)
				if err != nil {
					return err
				}
				schedule, err = db.GetSchedule(cmd.Context(), runID)
				if err != nil {
					return err
				}
				_, err = (scheduler.Engine{DB: db}).Drive(cmd.Context(), runID, func(ctx context.Context, a store.Assignment) (string, string, error) {
					_, err := providerwork.Execute(ctx, db, *stateDir, codexPath, a, schedule, run.WorkspacePath)
					if err != nil {
						return "", "", err
					}
					// OP-011 validates the provider's proposed result. Hold the
					// assignment and workspace; a model claim cannot settle it.
					return "", "", fmt.Errorf("provider handoff is ready; validation and delivery have not settled")
				})
				return err
			}()
			if err != nil {
				// Keep the worker and lock alive for explicit stop/recovery.
				fmt.Fprintln(cmd.ErrOrStderr(), err)
			}
			// A failed or interrupted provider attempt remains owned. Continue
			// requests are consumed only by this worker after Execute has closed
			// the old App Server process and released its turn.
			poll := time.NewTicker(300 * time.Millisecond)
			defer poll.Stop()
			for {
				select {
				case <-cmd.Context().Done():
					goto stopped
				case <-poll.C:
				}
				control, claimErr := db.ClaimWorkControl(cmd.Context(), runID, "continue")
				if errors.Is(claimErr, sql.ErrNoRows) {
					continue
				}
				if claimErr != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), claimErr)
					continue
				}
				_, threadID, beginErr := db.BeginContinuation(cmd.Context(), control)
				if beginErr != nil {
					_ = db.SettleWorkControl(cmd.Context(), control.ID, "rejected", beginErr.Error())
					continue
				}
				assignment, assignmentErr := db.CurrentAssignment(cmd.Context(), runID)
				if assignmentErr != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), assignmentErr)
					continue
				}
				resume := ""
				if control.Mode == "resume" {
					resume = threadID
				}
				_, executeErr := providerwork.ExecuteContinuation(cmd.Context(), db, *stateDir, codexPath, assignment, schedule, run.WorkspacePath, resume, control.Instructions)
				if executeErr != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), executeErr)
				}
			}
		}
	stopped:
		<-cmd.Context().Done()
		settleCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return db.InterruptWorker(settleCtx, runID, pid, ticks)
	}}
	cmd.Flags().StringVar(&runID, "run-id", "", "Recorded run ID")
	cmd.Flags().StringVar(&token, "launch-token", "", "Immutable launch token")
	cmd.Flags().BoolVar(&fixtureHold, "fixture-hold", false, "Hold worker ownership for a supervision fixture")
	_ = cmd.Flags().MarkHidden("fixture-hold")
	root.AddCommand(cmd)
}
