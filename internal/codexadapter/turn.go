package codexadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const SupportedVersion = "codex-cli 0.163.0"

func FindExecutable() (string, error) {
	if path, err := exec.LookPath("codex"); err == nil {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".local", "bin", "codex")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("Codex is not installed for this user; run the official Codex installer")
}

func AuthStatus(ctx context.Context, executable string) error {
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(probe, executable, "login", "status").Run(); err != nil {
		return fmt.Errorf("Codex sign-in is unavailable; run codex login as the service user: %w", err)
	}
	return nil
}

type Capabilities struct {
	Version     string  `json:"version"`
	Models      []Model `json:"models"`
	Resume      bool    `json:"resume"`
	Steer       bool    `json:"steer"`
	Interrupt   bool    `json:"interrupt"`
	Structured  bool    `json:"structured_results"`
	Permissions bool    `json:"permission_controls"`
}

func Version(ctx context.Context, executable string) (string, error) {
	if executable == "" {
		return "", fmt.Errorf("Codex executable is missing")
	}
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := exec.CommandContext(probe, executable, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("Codex version could not be read: %w", err)
	}
	version := strings.TrimSpace(string(b))
	if version != SupportedVersion {
		return version, fmt.Errorf("unsupported Codex version %q; Outpost is tested with %s", version, SupportedVersion)
	}
	return version, nil
}

// Probe uses the installed protocol, not a hard-coded picker catalog.
func Probe(ctx context.Context, executable, workspace string) (Capabilities, error) {
	version, err := Version(ctx, executable)
	if err != nil {
		return Capabilities{Version: version}, err
	}
	if err := AuthStatus(ctx, executable); err != nil {
		return Capabilities{Version: version}, err
	}
	probe, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := Start(probe, executable, workspace)
	if err != nil {
		return Capabilities{Version: version}, err
	}
	defer c.Close()
	if err := c.Initialize(probe); err != nil {
		return Capabilities{Version: version}, err
	}
	models, err := c.Models(probe)
	if err != nil {
		return Capabilities{Version: version}, err
	}
	return Capabilities{Version: version, Models: models, Resume: true, Steer: true, Interrupt: true, Structured: true, Permissions: true}, nil
}

type Handoff struct {
	Summary       string   `json:"summary"`
	ChangedPaths  []string `json:"changed_paths"`
	ClaimedChecks []string `json:"claimed_checks"`
	Blockers      []string `json:"blockers"`
}

// The schema asks for a proposal. Outpost must independently verify every
// claimed check, changed file and delivery effect before completion.
var HandoffSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"summary":        map[string]string{"type": "string"},
		"changed_paths":  map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
		"claimed_checks": map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
		"blockers":       map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
	},
	"required":             []string{"summary", "changed_paths", "claimed_checks", "blockers"},
	"additionalProperties": false,
}

type Completion struct {
	ThreadID string  `json:"thread_id"`
	TurnID   string  `json:"turn_id"`
	Status   string  `json:"status"`
	Handoff  Handoff `json:"handoff"`
}

func (c *Client) WaitTurn(ctx context.Context, threadID, turnID string, observe func(Event) error) (Completion, error) {
	var lastMessage string
	for {
		select {
		case event := <-c.Events():
			if observe != nil {
				if err := observe(event); err != nil {
					return Completion{}, err
				}
			}
			if event.Method == "item/completed" {
				var item struct {
					ThreadID, TurnID string
					Item             struct{ Type, Text string }
				}
				if json.Unmarshal(event.Params, &item) == nil && item.ThreadID == threadID && item.TurnID == turnID && item.Item.Type == "agentMessage" {
					lastMessage = item.Item.Text
				}
			}
			if event.Method != "turn/completed" {
				continue
			}
			var notification struct {
				ThreadID string `json:"threadId"`
				Turn     struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Items  []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"items"`
				} `json:"turn"`
			}
			if err := json.Unmarshal(event.Params, &notification); err != nil || notification.ThreadID != threadID || notification.Turn.ID != turnID {
				return Completion{}, fmt.Errorf("%w: completion identity differs", ErrProtocol)
			}
			result := Completion{ThreadID: threadID, TurnID: turnID, Status: notification.Turn.Status}
			if result.Status != "completed" {
				return result, fmt.Errorf("Codex turn %s", result.Status)
			}
			for i := len(notification.Turn.Items) - 1; i >= 0; i-- {
				item := notification.Turn.Items[i]
				if item.Type != "agentMessage" {
					continue
				}
				lastMessage = item.Text
				break
			}
			if len(lastMessage) == 0 || len(lastMessage) > 64<<10 {
				return result, fmt.Errorf("%w: final handoff is missing or too large", ErrProtocol)
			}
			if err := json.Unmarshal([]byte(lastMessage), &result.Handoff); err != nil {
				return result, fmt.Errorf("%w: final handoff is not structured JSON", ErrProtocol)
			}
			if result.Handoff.Summary == "" || len(result.Handoff.ChangedPaths) > 100 || len(result.Handoff.ClaimedChecks) > 100 || len(result.Handoff.Blockers) > 100 {
				return result, fmt.Errorf("%w: invalid handoff", ErrProtocol)
			}
			for _, path := range result.Handoff.ChangedPaths {
				if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
					return result, fmt.Errorf("%w: handoff path escapes workspace", ErrProtocol)
				}
			}
			return result, nil
		case <-c.done:
			err := c.readerError()
			if errors.Is(err, context.Canceled) {
				return Completion{}, err
			}
			return Completion{}, fmt.Errorf("%w: app-server stopped before terminal turn notification: %v", ErrUnknown, err)
		case <-ctx.Done():
			return Completion{}, fmt.Errorf("%w: waiting for terminal turn: %v", ErrUnknown, ctx.Err())
		}
	}
}
