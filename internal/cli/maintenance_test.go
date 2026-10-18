package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felixlheureux/outpost/internal/store"
)

func TestMigrationGuardRejectsReservedRun(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','owner','repo','https://github.com/owner/repo','ready','now')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.ReserveRun(ctx, "p", t.TempDir(), "migration-guard-test"); err != nil {
		t.Fatal(err)
	}
	if err := ensureMigrationIdle(ctx, db, state); err == nil || !strings.Contains(err.Error(), "execution reservation") {
		t.Fatalf("guard = %v, want reserved-run refusal", err)
	}
}
