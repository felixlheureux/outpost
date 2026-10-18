package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/worker"
	"github.com/spf13/cobra"
)

func addWorker(root *cobra.Command, stateDir *string) {
	var runID, token string
	var fixtureHold bool
	cmd := &cobra.Command{Use: "worker", Short: "Execute one previously reserved run under user systemd", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !fixtureHold {
			return fmt.Errorf("task execution is not available until the provider adapter is integrated")
		}
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
