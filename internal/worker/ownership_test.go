package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorkspaceLockLifetime(t *testing.T) {
	state := t.TempDir()
	workspace := t.TempDir()
	first, err := LockWorkspace(state, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := LockWorkspace(state, workspace); err == nil {
		second.Close()
		t.Fatal("second writer acquired a live workspace lock")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := LockWorkspace(state, workspace)
	if err != nil {
		t.Fatal("released lock remained owned:", err)
	}
	third.Close()
	entries, err := os.ReadDir(filepath.Join(state, "locks"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("lock file unexpectedly removed: %v %v", entries, err)
	}
}

func TestProcessStartIdentity(t *testing.T) {
	ticks, err := StartTicks(os.Getpid())
	if err != nil || ticks == "" {
		t.Fatalf("start ticks = %q, %v", ticks, err)
	}
}

func TestWorkspaceLockSurvivesParentExitWhileChildRuns(t *testing.T) {
	state := t.TempDir()
	workspace := t.TempDir()
	lock, err := LockWorkspace(state, workspace)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command("sleep", "10")
	child.ExtraFiles = []*os.File{lock}
	if err := child.Start(); err != nil {
		lock.Close()
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if contender, err := LockWorkspace(state, workspace); err == nil {
		contender.Close()
		t.Fatal("child did not retain workspace ownership")
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	if available, err := LockWorkspace(state, workspace); err != nil {
		t.Fatal("lock remained held after child exited:", err)
	} else {
		available.Close()
	}
}
