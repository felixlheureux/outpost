package cli

import (
	"fmt"
	"io"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/supervisor"
	"github.com/spf13/cobra"
)

func addStop(root *cobra.Command, stateDir *string, out io.Writer) {
	var runID string
	cmd := &cobra.Command{Use: "stop", Short: "Stop one recorded worker after checking its process identity", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if runID == "" {
			return fmt.Errorf("--run-id is required; use outpost status to find the active run")
		}
		db, err := store.Open(cmd.Context(), *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.CheckSchema(cmd.Context()); err != nil {
			return err
		}
		if err := supervisor.Stop(cmd.Context(), db, *stateDir, runID); err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "Stopped recorded worker %s; inspect its interrupted run before starting another.\n", runID)
		return err
	}}
	cmd.Flags().StringVar(&runID, "run-id", "", "Recorded run ID")
	root.AddCommand(cmd)
}
