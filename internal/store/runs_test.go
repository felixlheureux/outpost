package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

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
