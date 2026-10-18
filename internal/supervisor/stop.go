package supervisor

import (
	"context"
	"fmt"
	"time"

	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/worker"
)

// Stop interrupts only the currently recorded owner. The worker records its
// interrupted receipt; a live child or uncertain unit remains unresolved.
func Stop(ctx context.Context, db *store.Store, stateDir, runID string) error {
	run, err := db.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	unit, err := Observe(ctx, run.UnitName)
	if err != nil {
		return err
	}
	if unit.ActiveState != "active" || unit.MainPID == 0 || run.WorkerPID == 0 {
		return fmt.Errorf("recorded worker is not an active claimed process; inspect %s before stopping", run.UnitName)
	}
	ticks, err := worker.StartTicks(unit.MainPID)
	if err != nil || int64(unit.MainPID) != run.WorkerPID || ticks != run.WorkerStartTicks {
		return fmt.Errorf("recorded worker identity differs from systemd; inspect %s before stopping", run.UnitName)
	}
	if _, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "stop", run.UnitName); err != nil {
		return fmt.Errorf("stop outcome is unknown; inspect %s: %w", run.UnitName, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		unit, err = Observe(ctx, run.UnitName)
		if err != nil {
			return fmt.Errorf("stop outcome is unknown; inspect %s: %w", run.UnitName, err)
		}
		if unit.ActiveState != "active" && unit.MainPID == 0 {
			// A check may have inherited this advisory lock. Do not remove the
			// lock file or report the workspace free while it is still held.
			lock, err := worker.LockWorkspace(stateDir, run.WorkspacePath)
			if err != nil {
				return fmt.Errorf("worker unit stopped but workspace remains owned; inspect surviving check processes: %w", err)
			}
			return lock.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("stop acknowledgement did not establish settlement for %s", run.UnitName)
}
