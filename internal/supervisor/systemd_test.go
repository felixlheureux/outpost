package supervisor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/felixlheureux/outpost/internal/store"
)

func TestRealUserSystemdWorkerSurvivesWebRestart(t *testing.T) {
	binary := os.Getenv("OUTPOST_SYSTEMD_TEST_BINARY")
	if binary == "" {
		t.Skip("set OUTPOST_SYSTEMD_TEST_BINARY for real user-systemd acceptance")
	}
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
	_, err = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES ('fixture_project','fixture','repo','https://github.com/fixture/repo','ready','2026-10-18T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := db.ReserveRun(ctx, "fixture_project", t.TempDir(), "systemd-fixture-run")
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Command("systemctl", "--user", "stop", run.UnitName).Run()
	observed, err := Launch(ctx, db, run, binary, state, true)
	if err != nil {
		t.Fatalf("launch: %+v %v", observed, err)
	}
	waitState := func(want string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			current, err := db.GetRun(ctx, run.ID)
			if err == nil && current.State == want {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		current, _ := db.GetRun(ctx, run.ID)
		t.Fatalf("run state %s, wanted %s", current.State, want)
	}
	waitState("running")
	// A lost database acknowledgement must reconcile the recorded unit instead
	// of creating a second process for the same reservation.
	if err := db.ObserveLaunch(ctx, run.ID, "unknown"); err != nil {
		t.Fatal(err)
	}
	run, err = db.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := Launch(ctx, db, run, binary, state, true)
	if err != nil || replayed.MainPID != observed.MainPID {
		t.Fatalf("launch replay created or lost worker: original=%+v replay=%+v err=%v", observed, replayed, err)
	}
	if err := exec.Command("systemctl", "--user", "restart", "outpost.service").Run(); err != nil {
		t.Fatal("restart web:", err)
	}
	observed, err = Observe(ctx, run.UnitName)
	if err != nil || observed.ActiveState != "active" || observed.MainPID == 0 {
		t.Fatalf("worker lost after web restart: %+v %v", observed, err)
	}
	if err := Stop(ctx, db, state, run.ID); err != nil {
		t.Fatal("stop fixture worker:", err)
	}
	waitState("stopped")
	stopped, err := db.GetRun(ctx, run.ID)
	if err != nil || stopped.ReservationState != "released" {
		t.Fatalf("fixture worker did not release its slot: %+v %v", stopped, err)
	}
}
