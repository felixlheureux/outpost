package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/felixlheureux/outpost/internal/codexadapter"
	"github.com/felixlheureux/outpost/internal/delivery"
	"github.com/felixlheureux/outpost/internal/providerwork"
	"github.com/felixlheureux/outpost/internal/scheduler"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/validation"
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
					var policy struct {
						CheckProfile validation.Profile `json:"check_profile"`
					}
					if err := json.Unmarshal([]byte(schedule.PolicyJSON), &policy); err != nil {
						return "", "", err
					}
					if policy.CheckProfile.RepairLimit == 0 {
						policy.CheckProfile.RepairLimit = 3
					}
					binary, binaryErr := os.Executable()
					if binaryErr != nil {
						return "", "", binaryErr
					}
					checks := validation.Runner{DB: db, StateDir: *stateDir, Workspace: run.WorkspacePath, Executable: binary, Lock: lock}
				validationCycle:
					for cycle := 0; ; cycle++ {
						result, runErr := checks.Run(ctx, a, schedule)
						if runErr != nil {
							return "", "", runErr
						}
						if result.Passed {
							if err := db.RecordCheckSuccess(ctx, a.ID, result.ReceiptID); err != nil {
								return "", "", err
							}
							if err := db.AdvanceAttemptPhase(ctx, a.ID, "validating", "closing"); err != nil {
								return "", "", err
							}
							_, review, err := delivery.Policy(a, schedule)
							if err != nil {
								return "", "", err
							}
							if review == "required" {
								if err := db.AdvanceAttemptPhase(ctx, a.ID, "closing", "awaiting_review"); err != nil {
									return "", "", err
								}
								for {
									decision, feedback, readErr := db.ReviewDecision(ctx, a.ID, result.Candidate)
									if readErr == nil {
										if decision == "approved" {
											break
										}
										if cycle >= policy.CheckProfile.RepairLimit-1 {
											return "", "", fmt.Errorf("review repair limit reached")
										}
										thread, e := db.BeginReviewRepair(ctx, a.ID, result.Candidate)
										if e != nil {
											return "", "", e
										}
										instruction := "Reviewer rejected the checked candidate. Revise the same pinned assignment and address the review feedback.\n" + feedback
										if _, e = providerwork.ExecuteContinuation(ctx, db, *stateDir, codexPath, a, schedule, run.WorkspacePath, thread, instruction); e != nil {
											return "", "", e
										}
										continue validationCycle
									}
									if !errors.Is(readErr, sql.ErrNoRows) {
										return "", "", readErr
									}
									select {
									case <-ctx.Done():
										return "", "", ctx.Err()
									case <-time.After(time.Second):
									}
								}
								if err := db.AdvanceAttemptPhase(ctx, a.ID, "awaiting_review", "delivering"); err != nil {
									return "", "", err
								}
							} else if err := db.AdvanceAttemptPhase(ctx, a.ID, "closing", "delivering"); err != nil {
								return "", "", err
							}
							delivered, err := delivery.Deliver(ctx, db, run.WorkspacePath, a, schedule, result.Candidate)
							if err != nil {
								return "", "", err
							}
							if err := db.RecordCompletionReceipt(ctx, a.ID, "delivery", delivered.Receipt.ID); err != nil {
								return "", "", err
							}
							return "succeeded", delivered.Head, nil
						}
						if result.Repeated {
							return "", "", fmt.Errorf("unchanged check failure: %s", result.Failure)
						}
						if cycle >= policy.CheckProfile.RepairLimit-1 {
							return "", "", fmt.Errorf("repair limit reached: %s", result.Failure)
						}
						if repeated, repeatErr := db.RepeatedCheckFailure(ctx, a.ID, result.ReceiptID); repeatErr != nil {
							return "", "", repeatErr
						} else if repeated {
							return "", "", fmt.Errorf("unchanged repeated check failure: %s", result.Failure)
						}
						evidence, evidenceErr := checks.FailureEvidence(ctx, a.ID)
						if evidenceErr != nil {
							return "", "", evidenceErr
						}
						_, thread, beginErr := db.BeginRepair(ctx, a.ID)
						if beginErr != nil {
							return "", "", beginErr
						}
						if _, err := providerwork.ExecuteContinuation(ctx, db, *stateDir, codexPath, a, schedule, run.WorkspacePath, thread, "Runner-owned check failure. Repair the same pinned assignment and preserve prior source identity.\n"+evidence); err != nil {
							return "", "", err
						}
					}
				})
				var reportPolicy struct {
					CloseIssue bool `json:"close_issue"`
				}
				_ = json.Unmarshal([]byte(schedule.PolicyJSON), &reportPolicy)
				completed, reportErr := db.UnreportedAssignments(cmd.Context(), runID)
				if reportErr == nil {
					for _, done := range completed {
						if _, sendErr := delivery.Report(cmd.Context(), db, done, reportPolicy.CloseIssue, nil); sendErr != nil {
							fmt.Fprintln(cmd.ErrOrStderr(), "GitHub result pending:", sendErr)
						}
					}
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "GitHub report lookup:", reportErr)
				}
				return err
			}()
			if err == nil {
				if scheduleNow, e := db.GetSchedule(cmd.Context(), runID); e == nil && scheduleNow.Outcome != "active" {
					if e = db.CompleteWorker(cmd.Context(), runID, pid, ticks); e == nil {
						return nil
					}
					err = e
				}
			}
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
