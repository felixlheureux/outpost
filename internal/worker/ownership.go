// Package worker establishes OS and database ownership before any task effects.
package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
)

// LockWorkspace leaves the lock file in place. Only the live advisory lock
// matters; a surviving check can inherit the descriptor through ExtraFiles.
func LockWorkspace(stateDir, canonicalWorkspace string) (*os.File, error) {
	if !filepath.IsAbs(canonicalWorkspace) {
		return nil, fmt.Errorf("workspace path is not absolute")
	}
	dir := filepath.Join(stateDir, "locks")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(canonicalWorkspace))
	path := filepath.Join(dir, hex.EncodeToString(sum[:])+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("workspace is owned by another process: %w", err)
	}
	return f, nil
}

// StartTicks distinguishes a current worker from a later process reusing its PID.
func StartTicks(pid int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	closeName := strings.LastIndexByte(string(b), ')')
	if closeName < 0 {
		return "", fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(string(b[closeName+1:]))
	if len(fields) < 20 {
		return "", fmt.Errorf("incomplete process stat")
	}
	// After comm (field 2), starttime is field 22, index 19 here.
	return fields[19], nil
}

func checkUnitProcess(ctx context.Context, unit string, pid int) error {
	b, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "show", unit, "--property=MainPID", "--value")
	if err != nil {
		return err
	}
	mainPID, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || mainPID != pid {
		return fmt.Errorf("worker process does not own its recorded systemd unit")
	}
	return nil
}

// Claim verifies both systemd process identity and workspace exclusion before
// changing the durable run. The returned lock must live through all owned work.
func Claim(ctx context.Context, db *store.Store, stateDir, runID, token string) (*os.File, int, string, error) {
	run, err := db.GetRun(ctx, runID)
	if err != nil {
		return nil, 0, "", err
	}
	if run.LaunchToken != token || run.State != "reserved" {
		return nil, 0, "", fmt.Errorf("worker launch identity does not match a reserved run")
	}
	pid := os.Getpid()
	if err := checkUnitProcess(ctx, run.UnitName, pid); err != nil {
		return nil, 0, "", err
	}
	lock, err := LockWorkspace(stateDir, run.WorkspacePath)
	if err != nil {
		return nil, 0, "", err
	}
	ticks, err := StartTicks(pid)
	if err != nil {
		lock.Close()
		return nil, 0, "", err
	}
	if err := db.ClaimWorker(ctx, run.ID, token, pid, ticks); err != nil {
		lock.Close()
		return nil, 0, "", err
	}
	return lock, pid, ticks, nil
}
