package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/privateaccess"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/supervisor"
	"github.com/felixlheureux/outpost/internal/worker"
	"github.com/spf13/cobra"
)

func addStatus(root *cobra.Command, stateDir *string, out io.Writer) {
	configPath := privateaccess.DefaultPath()
	cmd := &cobra.Command{Use: "status", Short: "Observe the web service and private access route", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return showStatus(cmd.Context(), configPath, *stateDir, out)
	}}
	cmd.Flags().StringVar(&configPath, "config", configPath, "Private access configuration file")
	root.AddCommand(cmd)
}

func showStatus(ctx context.Context, configPath, stateDir string, out io.Writer) error {
	b, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "show", "outpost.service", "--property=ActiveState,SubState,MainPID", "--no-pager")
	if err != nil {
		fmt.Fprintln(out, "Web service: unavailable (user systemd could not be observed)")
	} else {
		fmt.Fprintf(out, "Web service:\n%s", b)
		if !strings.HasSuffix(string(b), "\n") {
			fmt.Fprintln(out)
		}
	}
	config, configErr := privateaccess.Load(configPath)
	if configErr != nil {
		fmt.Fprintln(out, "Private access: not configured")
	} else {
		fmt.Fprintf(out, "Configured URL: %s\nAllowed login: %s\n", config.Origin, config.AllowedLogin)
		if err := privateaccess.CheckNode(ctx, config); err != nil {
			fmt.Fprintln(out, "Tailscale node: unavailable or differs from config")
		} else {
			fmt.Fprintln(out, "Tailscale node: matches config")
		}
		present, err := privateaccess.ServeRoute(ctx, config, "http://127.0.0.1:8080")
		if err != nil || !present {
			fmt.Fprintln(out, "Serve route: not pointing to Outpost")
		} else if err := verifyLocalWeb(ctx, config); err != nil {
			fmt.Fprintln(out, "Serve route: points to port 8080, but the Outpost API is unavailable")
		} else {
			fmt.Fprintln(out, "Serve route: points to Outpost")
		}
	}
	if _, err := os.Stat(filepath.Join(stateDir, "outpost.sqlite")); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "Execution record: no local database")
		return nil
	} else if err != nil {
		fmt.Fprintf(out, "Execution record: unavailable (%v)\n", err)
		return nil
	}
	db, err := store.Open(ctx, stateDir)
	if err != nil {
		fmt.Fprintf(out, "Execution record: unavailable (%v)\n", err)
		return nil
	}
	defer db.Close()
	if err := db.CheckSchema(ctx); err != nil {
		fmt.Fprintf(out, "Execution record: unavailable (%v)\n", err)
		return nil
	}
	var runID string
	err = db.DB.QueryRowContext(ctx, "SELECT id FROM runs WHERE reservation_state='reserved' LIMIT 1").Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		fmt.Fprintln(out, "Execution record: no active or unresolved run")
		return nil
	}
	if err != nil {
		fmt.Fprintf(out, "Execution record: query failed (%v)\n", err)
		return nil
	}
	run, err := db.GetRun(ctx, runID)
	if err != nil {
		fmt.Fprintf(out, "Execution record: unreadable (%v)\n", err)
		return nil
	}
	fmt.Fprintf(out, "Execution record: %s (%s; launch %s)\n", run.ID, run.State, run.LaunchObservation)
	unit, err := supervisor.Observe(ctx, run.UnitName)
	if err != nil {
		fmt.Fprintf(out, "Worker unit: unknown (%v)\n", err)
		return nil
	}
	fmt.Fprintf(out, "Worker unit: %s; main PID %d\n", unit.ActiveState, unit.MainPID)
	if unit.MainPID > 0 && run.WorkerPID > 0 {
		ticks, err := worker.StartTicks(unit.MainPID)
		if err != nil || int64(unit.MainPID) != run.WorkerPID || ticks != run.WorkerStartTicks {
			fmt.Fprintln(out, "Worker identity: differs from recorded owner; inspect before starting work")
		} else {
			fmt.Fprintln(out, "Worker identity: matches recorded owner")
		}
	}
	return nil
}
