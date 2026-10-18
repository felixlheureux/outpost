package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/felixlheureux/outpost/migrations"
	"github.com/pressly/goose/v3"
)

func TestMigrationPreservesHistoricalTerminalRuns(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectSQLite3, db.DB, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','owner','repo','https://github.com/owner/repo','ready','2026-10-18T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"completed", "stopped"} {
		_, err = db.DB.ExecContext(ctx, `INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES(?,'p',?,?,?,?,?,'acknowledged','2026-10-18T00:00:00Z','2026-10-18T00:00:00Z')`, state, state, t.TempDir(), state, state+".service", state)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"completed", "stopped"} {
		run, err := db.GetRun(ctx, id)
		if err != nil || run.ReservationState != "released" {
			t.Fatalf("historical run %s: %+v %v", id, run, err)
		}
	}
	if _, _, err = db.ReserveRun(ctx, "p", t.TempDir(), "new-request"); err != nil {
		t.Fatalf("new reservation after migration: %v", err)
	}
}

func TestRunReservationAndClaim(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES ('project_one','owner','repo','https://github.com/owner/repo','ready','2026-10-18T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	first, replay, err := db.ReserveRun(ctx, "project_one", workspace, "request-one")
	if err != nil || replay || first.ID == "" || first.UnitName == "" || first.LaunchToken == "" {
		t.Fatalf("first reservation: %+v replay=%v err=%v", first, replay, err)
	}
	again, replay, err := db.ReserveRun(ctx, "project_one", workspace, "request-one")
	if err != nil || !replay || again.ID != first.ID {
		t.Fatalf("replay: %+v replay=%v err=%v", again, replay, err)
	}
	if _, _, err := db.ReserveRun(ctx, "project_one", workspace, "request-two"); !errors.Is(err, ErrSlotBusy) {
		t.Fatalf("second slot = %v", err)
	}
	if _, _, err := db.ReserveRun(ctx, "project_one", t.TempDir(), "request-one"); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("changed scope = %v", err)
	}
	if err := db.ClaimWorker(ctx, first.ID, "wrong-token", 101, "12345"); err == nil {
		t.Fatal("wrong launch token claimed worker")
	}
	if err := db.ClaimWorker(ctx, first.ID, first.LaunchToken, 101, "12345"); err != nil {
		t.Fatal(err)
	}
	if err := db.ClaimWorker(ctx, first.ID, first.LaunchToken, 102, "12346"); err == nil {
		t.Fatal("duplicate worker claimed run")
	}
	got, err := db.GetRun(ctx, first.ID)
	if err != nil || got.State != "running" || got.WorkerPID != 101 || got.WorkerStartTicks != "12345" {
		t.Fatalf("claimed run = %+v, %v", got, err)
	}
}

func TestReleasedRunAllowsAnotherProject(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		_, err = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES(?,?,?,?,'ready','2026-10-18T00:00:00Z')`, id, "owner", id, "https://github.com/owner/"+id)
		if err != nil {
			t.Fatal(err)
		}
	}
	a, _, err := db.ReserveRun(ctx, "a", t.TempDir(), "request-a")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.ClaimWorker(ctx, a.ID, a.LaunchToken, 123, "ticks"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.ReserveRun(ctx, "b", t.TempDir(), "request-b"); !errors.Is(err, ErrSlotBusy) {
		t.Fatalf("second project claimed slot: %v", err)
	}
	// The caller has proved the worker is gone and its workspace lock is free.
	if err = db.ReleaseReservation(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	b, _, err := db.ReserveRun(ctx, "b", t.TempDir(), "request-b")
	if err != nil || b.ProjectID != "b" {
		t.Fatalf("second project after settlement: %+v %v", b, err)
	}
}

func TestConcurrentReservationsAcrossConnections(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	first, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if err = first.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	second, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_, err = first.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','owner','repo','https://github.com/owner/repo','ready','2026-10-18T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i, db := range []*Store{first, second} {
		wg.Add(1)
		go func(i int, db *Store) {
			defer wg.Done()
			<-start
			_, _, err := db.ReserveRun(ctx, "p", workspace, fmt.Sprintf("request-%d", i))
			results <- err
		}(i, db)
	}
	close(start)
	wg.Wait()
	close(results)
	success, busy := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrSlotBusy) {
			busy++
		} else {
			t.Fatalf("unexpected reservation error: %v", err)
		}
	}
	if success != 1 || busy != 1 {
		t.Fatalf("concurrent reservations: success=%d busy=%d", success, busy)
	}
}
