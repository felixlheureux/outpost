package supervisor

import (
	"context"
	"fmt"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/worker"
)

// ReconcileTerminal releases a terminal schedule's reservation only after its
// recorded unit is inactive and no inherited workspace lock remains.
func ReconcileTerminal(ctx context.Context, db *store.Store, stateDir string, run store.Run) error {
	if run.ReservationState != "reserved" {
		return nil
	}
	schedule, err := db.GetSchedule(ctx, run.ID)
	if err != nil {
		return err
	}
	if schedule.Outcome == "active" {
		return fmt.Errorf("schedule still has active work")
	}
	unit, err := Observe(ctx, run.UnitName)
	if err != nil {
		return err
	}
	if unit.ActiveState == "active" || unit.MainPID != 0 {
		return fmt.Errorf("completed worker unit remains active")
	}
	lock, err := worker.LockWorkspace(stateDir, run.WorkspacePath)
	if err != nil {
		return err
	}
	defer lock.Close()
	return db.ReleaseReservation(ctx, run.ID)
}
