package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/felixlheureux/outpost/internal/codexadapter"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/supervisor"
	"github.com/spf13/cobra"
)

func addRun(root *cobra.Command, stateDir *string, out io.Writer) {
	var projectID, model, reasoning, instructions, requestKey string
	var fixedIssue, limit int
	cmd := &cobra.Command{Use: "run", Short: "Reserve and launch one owned Codex task", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if projectID == "" || model == "" || reasoning == "" || limit < 1 || limit > 100 || fixedIssue < 0 || !filepath.IsAbs(*stateDir) {
			return fmt.Errorf("run requires a project, discovered model/reasoning, valid task limit and absolute state directory")
		}
		ctx := cmd.Context()
		db, err := store.Open(ctx, *stateDir)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.CheckSchema(ctx); err != nil {
			return err
		}
		project, err := db.GetProjectSetup(ctx, projectID)
		if err != nil {
			return err
		}
		if project.State != "ready" {
			return fmt.Errorf("repository setup is not ready")
		}
		codexPath, err := codexadapter.FindExecutable()
		if err != nil {
			return err
		}
		capabilities, err := codexadapter.Probe(ctx, codexPath, project.WorkspacePath)
		if err != nil {
			return err
		}
		valid := false
		for _, item := range capabilities.Models {
			if item.ID == model {
				for _, effort := range item.ReasoningEfforts {
					if effort.Effort == reasoning {
						valid = true
					}
				}
			}
		}
		if !valid {
			return fmt.Errorf("model/reasoning is absent from the installed Codex catalog")
		}
		if requestKey == "" {
			b := make([]byte, 16)
			if _, err := rand.Read(b); err != nil {
				return err
			}
			requestKey = "cli-" + hex.EncodeToString(b)
		}
		mode, scope := "follow", []int{}
		if fixedIssue > 0 {
			mode, scope = "fixed", []int{fixedIssue}
		}
		scopeJSON, _ := json.Marshal(scope)
		policyJSON, _ := json.Marshal(map[string]any{"agent": map[string]string{"model": model, "reasoning": reasoning, "sandbox": "workspace-write", "approval_policy": "on-request"}})
		run, replay, err := db.ReserveScheduledRun(ctx, projectID, project.WorkspacePath, requestKey, store.Schedule{Kind: "queue", Mode: mode, ScopeJSON: string(scopeJSON), PolicyJSON: string(policyJSON), Instructions: instructions, TaskLimit: limit})
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Recorded run %s (request key %s).\n", run.ID, requestKey)
		if replay {
			return fmt.Errorf("run request already exists; inspect its recorded worker before acting again")
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		observed, err := supervisor.Launch(ctx, db, run, executable, *stateDir, false)
		if err != nil {
			return fmt.Errorf("run %s has an unresolved launch; inspect its unit: %w", run.ID, err)
		}
		fmt.Fprintf(out, "Worker unit %s: %s (PID %d). Provider completion still needs validation and delivery.\n", run.UnitName, observed.ActiveState, observed.MainPID)
		return nil
	}}
	cmd.Flags().StringVar(&projectID, "project", "", "Registered project ID")
	cmd.Flags().StringVar(&model, "model", "", "Codex model returned by capability discovery")
	cmd.Flags().StringVar(&reasoning, "reasoning", "", "Supported reasoning effort for the selected model")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Operator directions for the pinned assignment")
	cmd.Flags().StringVar(&requestKey, "request-key", "", "Idempotency key for this launch")
	cmd.Flags().IntVar(&fixedIssue, "fixed-issue", 0, "Run only this queued issue; otherwise follow queue")
	cmd.Flags().IntVar(&limit, "task-limit", 1, "Maximum completed tasks")
	root.AddCommand(cmd)
}
