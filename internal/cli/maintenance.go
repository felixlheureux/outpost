package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/felixlheureux/outpost/internal/store"
)

// ensureMigrationIdle refuses a reserved worker/check and an active web unit
// that owns this state directory. Disposable development state can be migrated
// even while the installed production web unit serves a different database.
func ensureMigrationIdle(ctx context.Context, db *store.Store, stateDir string) error {
	var runsTable int
	if err := db.DB.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='runs'").Scan(&runsTable); err != nil {
		return fmt.Errorf("cannot establish execution state before migration: %w", err)
	}
	if runsTable != 0 {
		if _, err := db.ActiveRun(ctx); err == nil {
			return fmt.Errorf("an execution reservation remains; settle its worker and check children before migration")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("cannot establish execution state before migration: %w", err)
		}
	}
	active, err := outpostWebActive(ctx)
	if err != nil {
		if filepath.Clean(stateDir) == filepath.Clean(defaultStateDir()) {
			return fmt.Errorf("cannot establish web service state before migration: %w", err)
		}
		return nil
	}
	if !active {
		return nil
	}
	output, err := exec.CommandContext(ctx, "systemctl", "--user", "show", "outpost.service", "--property=ExecStart", "--value").Output()
	if err != nil {
		return fmt.Errorf("cannot establish active web service database before migration: %w", err)
	}
	unit := string(output)
	marker := " --state-dir "
	start := strings.Index(unit, marker)
	if start < 0 {
		if filepath.Clean(stateDir) == filepath.Clean(defaultStateDir()) {
			return fmt.Errorf("stop outpost.service before migrating its database")
		}
		return fmt.Errorf("active web service state directory is unknown; stop it before migration")
	}
	configured := strings.Fields(unit[start+len(marker):])
	if len(configured) == 0 {
		return fmt.Errorf("active web service state directory is unknown; stop it before migration")
	}
	if filepath.Clean(configured[0]) == filepath.Clean(stateDir) {
		return fmt.Errorf("stop outpost.service before migrating its database")
	}
	return nil
}
