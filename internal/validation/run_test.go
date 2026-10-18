package validation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/worker"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "check-child" {
		if len(os.Args) != 4 || os.Args[2] != "--spec" {
			os.Exit(2)
		}
		if err := Child(os.Args[3]); err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func fixture(t *testing.T, check Check) (Runner, store.Assignment, store.Schedule) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	workspace := filepath.Join(root, "repo")
	state := filepath.Join(root, "state")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "a@example.test"}, {"config", "user.name", "A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = workspace
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git: %v %s", e, b)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, "code.txt"), []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "code.txt")
	cmd.Dir = workspace
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("git add: %v %s", e, b)
	}
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at) VALUES('p','o','r','https://github.com/o/r','ready','now')`,
		`INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','k',?, 'token','unit','running','acknowledged','now','now')`,
		`INSERT INTO execution_generations(project_id,issue_number,generation,state,assignment_id) VALUES('p',8,1,'owned','a')`,
		`INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',8,1,'running',?, 'now')`,
		`INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('attempt','a',1,'validating','running','now')`,
	}
	snapshot := `{"requirements":{"outcome":"Do work","criteria":["Pass"],"notes":"","settings":{"check_profile":"inherit"}}}`
	for i, s := range statements {
		_, err = db.DB.ExecContext(ctx, s, func() []any {
			if i == 1 {
				return []any{workspace}
			}
			if i == 3 {
				return []any{snapshot}
			}
			return nil
		}()...)
		if err != nil {
			t.Fatal(err)
		}
	}
	profile := Profile{ID: "standard", Checks: []Check{check}, RepairLimit: 3}
	policy, _ := json.Marshal(map[string]any{"check_profile": profile})
	lock, err := worker.LockWorkspace(state, workspace)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lock.Close() })
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Runner{DB: db, StateDir: state, Workspace: workspace, Executable: binary, Lock: lock}, store.Assignment{ID: "a", RunID: "r", ProjectID: "p", SnapshotJSON: snapshot}, store.Schedule{PolicyJSON: string(policy)}
}
func TestReceiptReuseAndChangedInputs(t *testing.T) {
	c := Check{ID: "test", Name: "Test", Argv: []string{"/bin/sh", "-c", "echo '3 tests'"}, Cwd: ".", Required: true, ExecutionClass: "local", RequireCases: true, CaseCountPattern: `(\d+) tests`}
	r, a, s := fixture(t, c)
	ctx := context.Background()
	first, err := r.Run(ctx, a, s)
	if err != nil || !first.Passed {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := r.Run(ctx, a, s)
	if err != nil || !second.Passed {
		t.Fatalf("reuse: %+v %v", second, err)
	}
	receipts, err := r.DB.ListChecks(ctx, a.ID)
	if err != nil || len(receipts) != 2 || receipts[1].Status != "reused" {
		t.Fatalf("receipts: %+v %v", receipts, err)
	}
	if err := os.WriteFile(filepath.Join(r.Workspace, "code.txt"), []byte("two"), 0600); err != nil {
		t.Fatal(err)
	}
	third, err := r.Run(ctx, a, s)
	if err != nil || !third.Passed || third.Candidate == first.Candidate {
		t.Fatalf("changed input: %+v %v", third, err)
	}
	receipts, _ = r.DB.ListChecks(ctx, a.ID)
	if len(receipts) != 3 || receipts[2].Status != "passed" {
		t.Fatalf("stale receipt reused: %+v", receipts)
	}
}
func TestZeroCasesAndUnchangedFailure(t *testing.T) {
	c := Check{ID: "test", Argv: []string{"/bin/sh", "-c", "echo '0 tests'"}, Cwd: ".", Required: true, ExecutionClass: "local", RequireCases: true, CaseCountPattern: `(\d+) tests`}
	r, a, s := fixture(t, c)
	first, err := r.Run(context.Background(), a, s)
	if err != nil || first.Passed || !strings.Contains(first.Failure, "zero") {
		t.Fatalf("zero cases: %+v %v", first, err)
	}
	second, err := r.Run(context.Background(), a, s)
	if err != nil || !second.Repeated {
		t.Fatalf("unchanged failure: %+v %v", second, err)
	}
}
func TestSurvivingChildAndTerminalRecovery(t *testing.T) {
	c := Check{ID: "test", Argv: []string{"/bin/sh", "-c", "sleep 0.25; echo done"}, Cwd: ".", Required: true, ExecutionClass: "local"}
	r, a, s := fixture(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := r.Run(ctx, a, s)
	if !errors.Is(err, ErrActiveChild) {
		t.Fatalf("expected active child: %v", err)
	}
	_, err = r.Run(context.Background(), a, s)
	if !errors.Is(err, ErrActiveChild) {
		t.Fatalf("duplicate child was not blocked: %v", err)
	}
	time.Sleep(350 * time.Millisecond)
	result, err := r.Run(context.Background(), a, s)
	if err != nil || !result.Passed {
		t.Fatalf("terminal recovery: %+v %v", result, err)
	}
	receipts, _ := r.DB.ListChecks(context.Background(), a.ID)
	if len(receipts) != 2 || receipts[0].Status != "passed" || receipts[1].Status != "reused" {
		t.Fatalf("unexpected recovery receipts: %+v", receipts)
	}
}

func TestOutputBoundAndTimeout(t *testing.T) {
	large := Check{ID: "test", Argv: []string{"/bin/sh", "-c", "yes x | head -c 1500000"}, Cwd: ".", Required: true, ExecutionClass: "local"}
	r, a, s := fixture(t, large)
	got, err := r.Run(context.Background(), a, s)
	if err != nil || !got.Passed {
		t.Fatalf("large output: %+v %v", got, err)
	}
	items, err := r.DB.ListChecks(context.Background(), a.ID)
	if err != nil || len(items) != 1 || !items[0].Truncated || items[0].OutputBytes > 1<<20 {
		t.Fatalf("unbounded output: %+v %v", items, err)
	}
	slow := Check{ID: "test", Argv: []string{"/bin/sh", "-c", "sleep 5"}, Cwd: ".", Required: true, ExecutionClass: "local", TimeoutSeconds: 1}
	r2, a2, s2 := fixture(t, slow)
	got, err = r2.Run(context.Background(), a2, s2)
	if err != nil || got.Passed || !strings.Contains(got.Failure, "deadline") {
		t.Fatalf("deadline: %+v %v", got, err)
	}
}

func TestUntrackedCandidateAndMetadataOutsideWorkspace(t *testing.T) {
	c := Check{ID: "test", Argv: []string{"/usr/bin/true"}, Cwd: ".", Required: true, ExecutionClass: "local"}
	r, _, _ := fixture(t, c)
	before, err := CandidateIdentity(context.Background(), r.Workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.StateDir, "note"), []byte("metadata"), 0600); err != nil {
		t.Fatal(err)
	}
	unchanged, err := CandidateIdentity(context.Background(), r.Workspace, nil)
	if err != nil || unchanged != before {
		t.Fatalf("outside metadata invalidated candidate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(r.Workspace, "fixture.txt"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := CandidateIdentity(context.Background(), r.Workspace, nil)
	if err != nil || changed == before {
		t.Fatalf("untracked fixture omitted: %v", err)
	}
}

func TestNoopPreparationAndContractChange(t *testing.T) {
	c := Check{ID: "test", Argv: []string{"/usr/bin/true"}, Cwd: ".", Required: true, ExecutionClass: "local"}
	r, a, s := fixture(t, c)
	profile := Profile{ID: "standard", Prepare: []Check{{ID: "format", Argv: []string{"/usr/bin/true"}, Cwd: ".", Required: true, ExecutionClass: "local"}}, Checks: []Check{c}, RepairLimit: 3}
	b, _ := json.Marshal(map[string]any{"check_profile": profile})
	s.PolicyJSON = string(b)
	first, err := r.Run(context.Background(), a, s)
	if err != nil || !first.Passed {
		t.Fatalf("first validation: %+v %v", first, err)
	}
	second, err := r.Run(context.Background(), a, s)
	if err != nil || !second.Passed {
		t.Fatalf("noop preparation: %+v %v", second, err)
	}
	items, _ := r.DB.ListChecks(context.Background(), a.ID)
	if len(items) != 4 || items[3].Status != "reused" {
		t.Fatalf("noop preparation reran code gate: %+v", items)
	}
	s.Instructions = "New accepted execution criterion"
	third, err := r.Run(context.Background(), a, s)
	if err != nil || !third.Passed {
		t.Fatalf("changed contract: %+v %v", third, err)
	}
	items, _ = r.DB.ListChecks(context.Background(), a.ID)
	if len(items) != 6 || items[5].Status != "passed" {
		t.Fatalf("changed contract reused stale gate: %+v", items)
	}
}
