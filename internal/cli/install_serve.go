package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/privateaccess"
)

type serveCutover struct {
	Origin   string `json:"origin"`
	Previous string `json:"previous"`
	Next     string `json:"next"`
	State    string `json:"state"`
}

func configureServe(ctx context.Context, c privateaccess.Config, opts installOptions) error {
	next := "http://127.0.0.1:8080"
	if opts.replaceServeFrom != "" {
		u, err := url.Parse(opts.replaceServeFrom)
		if err != nil || u.Scheme != "http" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Port() == "" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
			return fmt.Errorf("--replace-serve-from must be an exact loopback HTTP origin")
		}
	}
	b, err := hostcmd.Run(ctx, "", "tailscale", nil, "serve", "status", "--json")
	if err != nil {
		return fmt.Errorf("inspect existing Serve configuration: %w", err)
	}
	if present, _ := privateaccess.InspectServeRoute(b, c, next); present {
		previous := opts.replaceServeFrom
		if old, err := readServeCutover(opts.stateDir); err == nil {
			previous = old.Previous
		} else if !os.IsNotExist(err) {
			return err
		}
		return completeServeCutover(opts.stateDir, c, previous, next)
	}
	if _, err := privateaccess.InspectServeRoute(b, c, next); err != nil {
		if opts.replaceServeFrom == "" {
			return fmt.Errorf("existing Serve route requires explicit --replace-serve-from: %w", err)
		}
		if err := privateaccess.InspectReplacement(b, c, opts.replaceServeFrom); err != nil {
			return err
		}
	} else if opts.replaceServeFrom != "" {
		return fmt.Errorf("expected previous Serve route is absent; inspect the current route before cutover")
	}
	receipt := serveCutover{Origin: c.Origin, Previous: opts.replaceServeFrom, Next: next, State: "prepared"}
	if err := saveServeCutover(opts.stateDir, receipt); err != nil {
		return err
	}
	_, commandErr := hostcmd.Run(ctx, "", "tailscale", nil, "serve", "--bg", "--https=443", "--yes", next)
	present, observeErr := privateaccess.ServeRoute(ctx, c, next)
	if observeErr == nil && present {
		return completeServeCutover(opts.stateDir, c, opts.replaceServeFrom, next)
	}
	if commandErr != nil {
		return fmt.Errorf("Serve cutover outcome is unknown after command failure; inspect the exact route before retrying: %w", commandErr)
	}
	if observeErr != nil {
		return fmt.Errorf("Serve cutover outcome is unknown after observation failure: %w", observeErr)
	}
	return fmt.Errorf("Serve cutover was not observed; inspect the exact route before retrying")
}

func readServeCutover(stateDir string) (serveCutover, error) {
	var receipt serveCutover
	b, err := os.ReadFile(filepath.Join(stateDir, "serve-cutover.json"))
	if err != nil {
		return receipt, err
	}
	if err := json.Unmarshal(b, &receipt); err != nil {
		return receipt, err
	}
	return receipt, nil
}

func completeServeCutover(stateDir string, c privateaccess.Config, previous, next string) error {
	return saveServeCutover(stateDir, serveCutover{Origin: c.Origin, Previous: previous, Next: next, State: "observed"})
}

func saveServeCutover(stateDir string, receipt serveCutover) error {
	path := filepath.Join(stateDir, "serve-cutover.json")
	if b, err := os.ReadFile(path); err == nil {
		var old serveCutover
		if err := json.Unmarshal(b, &old); err != nil {
			return fmt.Errorf("existing Serve cutover receipt is unreadable: %w", err)
		}
		if old.Origin != receipt.Origin || old.Previous != receipt.Previous || old.Next != receipt.Next {
			if old.State != "prepared" || receipt.State != "observed" || old.Origin != receipt.Origin || old.Previous != receipt.Next {
				return fmt.Errorf("existing Serve cutover targets a different route; reconcile it before retrying")
			}
			archive, err := os.CreateTemp(stateDir, "serve-cutover-abandoned-*.json")
			if err != nil {
				return err
			}
			if err := archive.Chmod(0600); err != nil {
				archive.Close()
				return err
			}
			if _, err := archive.Write(b); err != nil {
				archive.Close()
				return err
			}
			if err := archive.Sync(); err != nil {
				archive.Close()
				return err
			}
			if err := archive.Close(); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	b, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(stateDir, ".serve-cutover-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
