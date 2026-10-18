package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/store"
)

type fixtureSource struct {
	mu         sync.Mutex
	order      []int
	issues     map[int]githubcli.Issue
	sha        string
	afterIssue func(int)
	queueErr   error
}

func (f *fixtureSource) Queue(context.Context, string, string) ([]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queueErr != nil {
		return nil, "", f.queueErr
	}
	b, _ := json.Marshal(map[string]any{"version": 1, "issues": f.order})
	return b, f.sha, nil
}
func (f *fixtureSource) Issue(_ context.Context, _, _ string, n int) (githubcli.Issue, error) {
	f.mu.Lock()
	v, ok := f.issues[n]
	hook := f.afterIssue
	f.mu.Unlock()
	if hook != nil {
		hook(n)
	}
	if !ok {
		return v, fmt.Errorf("missing issue")
	}
	return v, nil
}

func TestStopAcceptedDuringSourceReadRejectsClaim(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	ctx := context.Background()
	s, err := db.GetSchedule(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	f.afterIssue = func(n int) {
		if n == 41 {
			if err := db.RequestFinishAfterTask(ctx, runID, s.Revision); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, _, err = (Engine{DB: db, Source: f}).Next(ctx, runID, true)
	if err != store.ErrAdmissionChanged {
		t.Fatalf("admission after stop = %v", err)
	}
	var count int
	if err := db.DB.QueryRowContext(ctx, "SELECT count(*) FROM assignments").Scan(&count); err != nil || count != 0 {
		t.Fatalf("stop admitted %d assignments: %v", count, err)
	}
}
func fixtureIssue(n int, deps ...int) githubcli.Issue {
	body := "Task body"
	if len(deps) > 0 {
		body = fmt.Sprintf("## Outpost settings\n```yaml\noutpost_task: 1\nkind: implementation\ndepends_on: %v\ncheck_profile: inherit\ndelivery: inherit\nreview: inherit\n```", deps)
	}
	return githubcli.Issue{ID: int64(n), NodeID: fmt.Sprintf("node%d", n), Number: n, Title: fmt.Sprintf("Task %d", n), Body: body, HTMLURL: fmt.Sprintf("https://github.com/owner/repo/issues/%d", n), State: "open", UpdatedAt: "2026-10-18T00:00:00Z"}
}
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if b, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
}
func setup(t *testing.T, mode string, scope []int) (*store.Store, *fixtureSource, string, string) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	git(t, workspace, "init", "-q")
	git(t, workspace, "config", "user.name", "Fixture")
	git(t, workspace, "config", "user.email", "fixture@example.com")
	if err := os.WriteFile(filepath.Join(workspace, "README"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, workspace, "add", "README")
	git(t, workspace, "commit", "-qm", "baseline")
	_, err = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at,github_id,workspace_path,setup_stage) VALUES('p','owner','repo','https://github.com/owner/repo','ready',?,1,?,'ready')`, time.Now().UTC().Format(time.RFC3339Nano), workspace)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(scope)
	if scope == nil {
		b = []byte("[]")
	}
	schedule := store.Schedule{Kind: "queue", Mode: mode, ScopeJSON: string(b), PolicyJSON: `{"model":"fixture"}`, TaskLimit: 20}
	run, _, err := db.ReserveScheduledRun(ctx, "p", workspace, "fixture-request", schedule)
	if err != nil {
		t.Fatal(err)
	}
	if again, replay, err := db.ReserveScheduledRun(ctx, "p", workspace, "fixture-request", schedule); err != nil || !replay || again.ID != run.ID {
		t.Fatalf("scheduled replay: %+v %v %v", again, replay, err)
	}
	if err := db.ClaimWorker(ctx, run.ID, run.LaunchToken, 101, "fixture-ticks"); err != nil {
		t.Fatal(err)
	}
	f := &fixtureSource{order: []int{72, 41, 88}, sha: "sha-one", issues: map[int]githubcli.Issue{72: fixtureIssue(72, 41), 41: fixtureIssue(41), 88: fixtureIssue(88)}}
	return db, f, run.ID, workspace
}

func fixtureReceipts(ctx context.Context, db *store.Store, id string) error {
	if err := db.AdvanceAttemptPhase(ctx, id, "implementing", "validating"); err != nil {
		return err
	}
	receiptID := "fixture-checks-" + id
	_, err := db.DB.ExecContext(ctx, `INSERT INTO check_receipts(id,assignment_id,attempt_id,generation,check_id,name,required,command_json,contract_digest,input_digest,candidate_digest,status,started_at,output_path,terminal_path) SELECT ?,?,t.id,1,'fixture','Fixture',1,'{}','contract','input','candidate','passed','now',?,? FROM assignment_attempts t WHERE t.assignment_id=? AND t.phase='validating'`, receiptID, id, receiptID, receiptID, id)
	if err != nil {
		return err
	}
	if err := db.RecordCheckSuccess(ctx, id, receiptID); err != nil {
		return err
	}
	if err := db.AdvanceAttemptPhase(ctx, id, "validating", "closing"); err != nil {
		return err
	}
	if err := db.AdvanceAttemptPhase(ctx, id, "closing", "delivering"); err != nil {
		return err
	}
	return db.RecordCompletionReceipt(ctx, id, "delivery", "fixture-delivery-verified")
}

func succeedFixture(t *testing.T, ctx context.Context, db *store.Store, a *store.Assignment, head string) {
	t.Helper()
	if err := db.BeginAttempt(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := fixtureReceipts(ctx, db, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.SettleAssignment(ctx, a.ID, "succeeded", head); err != nil {
		t.Fatal(err)
	}
}

func TestFollowQueueAdmissionAndPinnedSnapshot(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	engine := Engine{DB: db, Source: f}
	ctx := context.Background()
	p, a, err := engine.Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Selected != 41 || a == nil || len(p.Waiting) != 1 || p.Waiting[0].Code != "dependency_incomplete" {
		t.Fatalf("first selection: %+v %+v", p, a)
	}
	f.mu.Lock()
	f.order = []int{99, 72, 41, 88}
	f.sha = "sha-two"
	f.issues[99] = fixtureIssue(99)
	f.issues[41] = fixtureIssue(41)
	f.issues[41] = githubcli.Issue{ID: 41, NodeID: "node41", Number: 41, Title: "Changed", Body: "New body", HTMLURL: "https://github.com/owner/repo/issues/41", State: "open", UpdatedAt: "2026-10-18T01:00:00Z"}
	f.mu.Unlock()
	var snap Snapshot
	if err := json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Title != "Task 41" || snap.QueueSHA != "sha-one" || snap.QueuePosition != 2 {
		t.Fatalf("assignment changed: %+v", snap)
	}
	succeedFixture(t, ctx, db, a, snap.Head)
	p, a, err = engine.Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Selected != 99 || a == nil {
		t.Fatalf("follow addition: %+v %+v", p, a)
	}
}

func TestFixedScopeAndStopBoundary(t *testing.T) {
	db, f, runID, _ := setup(t, "fixed", []int{72, 41, 88})
	engine := Engine{DB: db, Source: f}
	ctx := context.Background()
	f.mu.Lock()
	f.order = append([]int{99}, f.order...)
	f.issues[99] = fixtureIssue(99)
	f.sha = "sha-two"
	f.mu.Unlock()
	p, _, err := engine.Next(ctx, runID, false)
	if err != nil || p.Selected != 41 {
		t.Fatalf("fixed selection: %+v %v", p, err)
	}
	s, err := db.GetSchedule(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RequestFinishAfterTask(ctx, runID, s.Revision); err != nil {
		t.Fatal(err)
	}
	_, _, err = engine.Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB.QueryRowContext(ctx, "SELECT count(*) FROM assignments").Scan(&count); err != nil || count != 0 {
		t.Fatalf("stop admitted %d assignments: %v", count, err)
	}
}

func TestFixedScopeExcludesNewIssue(t *testing.T) {
	db, f, runID, _ := setup(t, "fixed", []int{72, 41, 88})
	engine := Engine{DB: db, Source: f}
	ctx := context.Background()
	f.mu.Lock()
	f.order = append([]int{99}, f.order...)
	f.issues[99] = fixtureIssue(99)
	f.sha = "sha-two"
	f.mu.Unlock()
	_, a, err := engine.Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err = json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil {
		t.Fatal(err)
	}
	succeedFixture(t, ctx, db, a, snap.Head)
	p, _, err := engine.Next(ctx, runID, false)
	if err != nil || p.Selected != 72 {
		t.Fatalf("fixed scope selected new issue: %+v %v", p, err)
	}
}

func TestFailedAssignmentBlocksAnotherTask(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	engine := Engine{DB: db, Source: f}
	ctx := context.Background()
	_, a, err := engine.Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SettleAssignment(ctx, a.ID, "failed", ""); err != nil {
		t.Fatal(err)
	}
	p, existing, err := engine.Next(ctx, runID, true)
	if err != nil || p.Outcome != "failed" || existing != nil {
		t.Fatalf("failed run advanced: %+v %+v %v", p, existing, err)
	}
}

func TestSuccessRequiresCompletionReceipts(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	ctx := context.Background()
	_, a, err := (Engine{DB: db, Source: f}).Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err = json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil {
		t.Fatal(err)
	}
	if err = db.BeginAttempt(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.SettleAssignment(ctx, a.ID, "succeeded", snap.Head); err == nil {
		t.Fatal("unverified completion was counted")
	}
	if err = fixtureReceipts(ctx, db, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.SettleAssignment(ctx, a.ID, "succeeded", snap.Head); err != nil {
		t.Fatal(err)
	}
}

func TestOtherProjectStartsAfterFailedAssignmentSettles(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	ctx := context.Background()
	_, a, err := (Engine{DB: db, Source: f}).Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.SettleAssignment(ctx, a.ID, "failed", ""); err != nil {
		t.Fatal(err)
	}
	if err = db.ReleaseReservation(ctx, runID); err != nil {
		t.Fatal(err)
	}
	workspaceB := t.TempDir()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at,github_id,workspace_path,setup_stage) VALUES('b','owner','second','https://github.com/owner/second','ready',?,2,?,'ready')`, time.Now().UTC().Format(time.RFC3339Nano), workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := db.ReserveScheduledRun(ctx, "b", workspaceB, "second-request", store.Schedule{Kind: "queue", Mode: "follow", ScopeJSON: "[]", PolicyJSON: "{}", TaskLimit: 20})
	if err != nil || b.ProjectID != "b" {
		t.Fatalf("other project reservation: %+v %v", b, err)
	}
	gens, err := db.ListGenerations(ctx, "p")
	if err != nil || gens[41].State != "failed" {
		t.Fatalf("first project lost unfinished work: %+v %v", gens, err)
	}
}

func TestManualTaskDoesNotNeedQueueMembership(t *testing.T) {
	db, f, prior, workspace := setup(t, "follow", nil)
	ctx := context.Background()
	if err := db.ReleaseReservation(ctx, prior); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.issues[99] = fixtureIssue(99)
	f.mu.Unlock()
	run, _, err := db.ReserveScheduledRun(ctx, "p", workspace, "manual-task-request", store.Schedule{Kind: "task", Mode: "single", ScopeJSON: "[99]", PolicyJSON: "{}", TaskLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.ClaimWorker(ctx, run.ID, run.LaunchToken, 102, "manual-ticks"); err != nil {
		t.Fatal(err)
	}
	p, a, err := (Engine{DB: db, Source: f}).Next(ctx, run.ID, true)
	if err != nil || p.Selected != 99 || a == nil {
		t.Fatalf("manual task: %+v %+v %v", p, a, err)
	}
	var snap Snapshot
	if err := json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil || snap.QueuePosition != 0 {
		t.Fatalf("manual task snapshot: %+v %v", snap, err)
	}
}

func TestFixtureWorkerFollowsAdditionWithinBudget(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx, `UPDATE run_schedules SET task_limit=2 WHERE run_id=?`, runID); err != nil {
		t.Fatal(err)
	}
	var selected []int
	p, err := (Engine{DB: db, Source: f}).Drive(ctx, runID, func(_ context.Context, a store.Assignment) (string, string, error) {
		selected = append(selected, a.IssueNumber)
		if len(selected) == 1 {
			f.mu.Lock()
			f.order = append([]int{99}, f.order...)
			f.issues[99] = fixtureIssue(99)
			f.sha = "sha-two"
			f.mu.Unlock()
		}
		var snap Snapshot
		if err := json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil {
			return "", "", err
		}
		if err := fixtureReceipts(ctx, db, a.ID); err != nil {
			return "", "", err
		}
		return "succeeded", snap.Head, nil
	})
	if err != nil || p.Outcome != "budget_reached" || len(selected) != 2 || selected[0] != 41 || selected[1] != 99 {
		t.Fatalf("fixture drive: selected=%v projection=%+v err=%v", selected, p, err)
	}
	s, err := db.GetSchedule(ctx, runID)
	if err != nil || s.Outcome != "complete" || s.SuccessfulCount != 2 {
		t.Fatalf("schedule=%+v err=%v", s, err)
	}
	if err := db.ReleaseReservation(ctx, runID); err != nil {
		t.Fatal(err)
	}
	r, err := db.GetRun(ctx, runID)
	if err != nil || r.State != "completed" || r.ReservationState != "released" {
		t.Fatalf("completed reservation: %+v %v", r, err)
	}
}

func TestFixtureWorkerStopAfterTask(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	ctx := context.Background()
	count := 0
	p, err := (Engine{DB: db, Source: f}).Drive(ctx, runID, func(_ context.Context, a store.Assignment) (string, string, error) {
		count++
		s, err := db.GetSchedule(ctx, runID)
		if err != nil {
			return "", "", err
		}
		if err = db.RequestFinishAfterTask(ctx, runID, s.Revision); err != nil {
			return "", "", err
		}
		var snap Snapshot
		if err = json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil {
			return "", "", err
		}
		if err := fixtureReceipts(ctx, db, a.ID); err != nil {
			return "", "", err
		}
		return "succeeded", snap.Head, nil
	})
	if err != nil || p.Outcome != "stopped" || count != 1 {
		t.Fatalf("stop after task: count=%d projection=%+v err=%v", count, p, err)
	}
}

func TestSourceOutageAfterPinnedAssignmentStopsAdvance(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	ctx := context.Background()
	engine := Engine{DB: db, Source: f}
	_, a, err := engine.Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.queueErr = fmt.Errorf("offline")
	f.mu.Unlock()
	p, pinned, err := engine.Next(ctx, runID, true)
	if err != nil || pinned == nil || pinned.ID != a.ID || p.Outcome != "unfinished_assignment" {
		t.Fatalf("active assignment lost on outage: %+v %+v %v", p, pinned, err)
	}
	var snap Snapshot
	if err = json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil {
		t.Fatal(err)
	}
	succeedFixture(t, ctx, db, a, snap.Head)
	p, next, err := engine.Next(ctx, runID, true)
	if err != nil || next != nil || p.Outcome != "source_unavailable" {
		t.Fatalf("outage admitted task: %+v %+v %v", p, next, err)
	}
}

func TestClosedIssueAndExplicitReopenGeneration(t *testing.T) {
	db, f, runID, _ := setup(t, "follow", nil)
	ctx := context.Background()
	engine := Engine{DB: db, Source: f}
	_, a, err := engine.Next(ctx, runID, true)
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err = json.Unmarshal([]byte(a.SnapshotJSON), &snap); err != nil {
		t.Fatal(err)
	}
	succeedFixture(t, ctx, db, a, snap.Head)
	f.mu.Lock()
	changed := fixtureIssue(41)
	changed.Title = "Edited after success"
	f.issues[41] = changed
	closed := f.issues[72]
	closed.State = "closed"
	f.issues[72] = closed
	f.mu.Unlock()
	p, _, err := engine.Next(ctx, runID, false)
	if err != nil || p.Selected != 88 {
		t.Fatalf("completed/closed selection: %+v %v", p, err)
	}
	if err = db.ReleaseReservation(ctx, runID); err != nil {
		t.Fatal(err)
	}
	g, err := db.ReopenGeneration(ctx, "p", 41, 1)
	if err != nil || g.Number != 2 || g.State != "available" {
		t.Fatalf("explicit generation: %+v %v", g, err)
	}
}

func TestDirtyCheckoutBlocksClaim(t *testing.T) {
	db, f, runID, workspace := setup(t, "follow", nil)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(workspace, "untracked"), []byte("work"), 0600); err != nil {
		t.Fatal(err)
	}
	p, a, err := (Engine{DB: db, Source: f}).Next(ctx, runID, true)
	if err != nil || a != nil || p.Outcome != "baseline_changed" {
		t.Fatalf("dirty checkout admitted: %+v %+v %v", p, a, err)
	}
}
