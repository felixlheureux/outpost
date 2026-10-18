// Package supervisor launches recorded worker identities through user systemd.
package supervisor

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
)

type UnitObservation struct {
	LoadState, ActiveState string
	MainPID                int
}

func Observe(ctx context.Context, unit string) (UnitObservation, error) {
	var observed UnitObservation
	b, err := hostcmd.Run(ctx, "", "systemctl", nil, "--user", "show", unit, "--property=LoadState,ActiveState,MainPID", "--no-pager")
	if err != nil {
		return observed, fmt.Errorf("observe recorded worker unit: %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "LoadState":
			observed.LoadState = value
		case "ActiveState":
			observed.ActiveState = value
		case "MainPID":
			observed.MainPID, _ = strconv.Atoi(value)
		}
	}
	if observed.LoadState == "" || observed.ActiveState == "" {
		return UnitObservation{}, fmt.Errorf("systemd returned an incomplete unit observation")
	}
	return observed, nil
}

// Launch starts only a fresh, persisted reservation. An unknown previous
// launch is observed but never automatically replayed.
func Launch(ctx context.Context, db *store.Store, run store.Run, executable, stateDir string, fixtureHold bool) (UnitObservation, error) {
	observed, err := Observe(ctx, run.UnitName)
	if err != nil {
		return observed, err
	}
	if observed.ActiveState == "active" && observed.MainPID > 0 {
		if err := db.ObserveLaunch(ctx, run.ID, "acknowledged"); err != nil {
			return observed, err
		}
		return observed, nil
	}
	if run.LaunchObservation != "pending" {
		return observed, fmt.Errorf("launch outcome is unresolved; inspect %s before another start", run.UnitName)
	}
	if observed.LoadState != "not-found" {
		return observed, fmt.Errorf("recorded unit already exists in state %s", observed.ActiveState)
	}
	args := []string{"--user", "--collect", "--unit=" + strings.TrimSuffix(run.UnitName, ".service"), "--property=Type=exec", "--property=KillMode=control-group", executable, "--state-dir", stateDir, "worker", "--run-id", run.ID, "--launch-token", run.LaunchToken}
	if fixtureHold {
		args = append(args, "--fixture-hold")
	}
	_, launchErr := hostcmd.Run(ctx, "", "systemd-run", nil, args...)
	observed, observeErr := Observe(ctx, run.UnitName)
	if observeErr == nil && observed.ActiveState == "active" && observed.MainPID > 0 {
		if err := db.ObserveLaunch(ctx, run.ID, "acknowledged"); err != nil {
			return observed, err
		}
		return observed, nil
	}
	if err := db.ObserveLaunch(ctx, run.ID, "unknown"); err != nil {
		return observed, err
	}
	if launchErr != nil {
		return observed, fmt.Errorf("systemd launch outcome is unknown: %w", launchErr)
	}
	if observeErr != nil {
		return observed, fmt.Errorf("systemd launch outcome is unknown: %w", observeErr)
	}
	return observed, fmt.Errorf("systemd accepted launch but no owned worker was observed")
}
