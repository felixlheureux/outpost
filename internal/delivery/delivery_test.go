package delivery

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felixlheureux/outpost/internal/gitcli"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/validation"
)

func command(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}

func setupDelivery(t *testing.T, mode string) (*store.Store, string, store.Assignment, store.Schedule, string) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	command(t, dir, "init", "-q", "--initial-branch=main")
	command(t, dir, "config", "user.name", "Outpost test")
	command(t, dir, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "task.txt"), []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command(t, dir, "add", "task.txt")
	command(t, dir, "commit", "-qm", "base")
	head := command(t, dir, "rev-parse", "HEAD")
	if mode == "push" {
		remote := t.TempDir()
		command(t, remote, "init", "-q", "--bare", "--initial-branch=main")
		command(t, dir, "remote", "add", "origin", remote)
		command(t, dir, "push", "-q", "-u", "origin", "main")
	}
	state := filepath.Join(t.TempDir(), "state")
	if e := os.Mkdir(state, 0700); e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(ctx, state)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = db.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	_, e = db.DB.ExecContext(ctx, `INSERT INTO projects(id,github_owner,github_name,repository_url,setup_state,created_at,github_id,workspace_path,setup_stage) VALUES('p','owner','repo','https://github.com/owner/repo','ready','now',1,?,'ready')`, dir)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.DB.ExecContext(ctx, `INSERT INTO runs(id,project_id,request_key,workspace_path,launch_token,unit_name,state,launch_observation,created_at,updated_at) VALUES('r','p','request-key',?,'token','unit','running','acknowledged','now','now')`, dir)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.DB.ExecContext(ctx, `INSERT INTO execution_generations(project_id,issue_number,generation,state) VALUES('p',1,1,'owned')`)
	if e != nil {
		t.Fatal(e)
	}
	upstream := gitcli.Upstream(ctx, dir)
	snap, _ := json.Marshal(map[string]any{"branch": "main", "head": head, "status": "", "upstream": upstream, "remote_digest": gitcli.RemoteDigest(ctx, dir, upstream), "requirements": map[string]any{"settings": map[string]any{"delivery": "inherit", "review": "inherit"}}})
	_, e = db.DB.ExecContext(ctx, `INSERT INTO assignments(id,run_id,project_id,issue_number,generation,state,snapshot_json,created_at) VALUES('a','r','p',1,1,'running',?,'now')`, string(snap))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.DB.ExecContext(ctx, `INSERT INTO assignment_attempts(id,assignment_id,ordinal,phase,outcome,created_at) VALUES('t','a',1,'delivering','running','now')`)
	if e != nil {
		t.Fatal(e)
	}
	if err := os.WriteFile(filepath.Join(dir, "task.txt"), []byte("done\n"), 0600); err != nil {
		t.Fatal(err)
	}
	candidate, e := validation.CandidateIdentity(ctx, dir, nil)
	if e != nil {
		t.Fatal(e)
	}
	policy, _ := json.Marshal(map[string]string{"delivery": mode, "review": "none"})
	schedule := store.Schedule{RunID: "r", PolicyJSON: string(policy)}
	return db, dir, store.Assignment{ID: "a", RunID: "r", ProjectID: "p", IssueNumber: 1, SnapshotJSON: string(snap)}, schedule, candidate
}

func TestDeliveryModesAndCommitRecovery(t *testing.T) {
	for _, mode := range []string{"local_changes", "commit", "push"} {
		t.Run(mode, func(t *testing.T) {
			db, dir, a, s, candidate := setupDelivery(t, mode)
			ctx := context.Background()
			r, e := Deliver(ctx, db, dir, a, s, candidate)
			if e != nil {
				t.Fatal(e)
			}
			if r.Receipt.State != "settled" {
				t.Fatalf("%+v", r.Receipt)
			}
			if mode == "local_changes" {
				if r.Receipt.CommitSHA != "" {
					t.Fatal("local delivery created a commit")
				}
			} else {
				if r.Receipt.CommitSHA == "" {
					t.Fatal("missing commit")
				}
				if again, e := Deliver(ctx, db, dir, a, s, candidate); e != nil || again.Receipt.CommitSHA != r.Receipt.CommitSHA {
					t.Fatalf("replay: %+v %v", again, e)
				}
				if mode == "push" {
					remote := command(t, dir, "rev-parse", "@{upstream}")
					if remote != r.Receipt.CommitSHA {
						t.Fatalf("remote %s != %s", remote, r.Receipt.CommitSHA)
					}
				}
			}
		})
	}
}

func TestDeliveryBlocksChangedCandidateAndAmbiguousHead(t *testing.T) {
	db, dir, a, s, candidate := setupDelivery(t, "commit")
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, "task.txt"), []byte("after checks\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Deliver(ctx, db, dir, a, s, candidate); err == nil {
		t.Fatal("changed candidate delivered")
	}
	command(t, dir, "add", "task.txt")
	command(t, dir, "commit", "-qm", "foreign")
	newCandidate, err := validation.CandidateIdentity(ctx, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Deliver(ctx, db, dir, a, s, newCandidate); err == nil {
		t.Fatal("foreign HEAD delivered")
	}
}

func TestLostCommitAcknowledgementUsesExistingCommit(t *testing.T) {
	db, dir, a, s, candidate := setupDelivery(t, "commit")
	ctx := context.Background()
	_, err := db.BeginDelivery(ctx, store.DeliveryReceipt{AssignmentID: a.ID, Policy: "commit", Candidate: candidate, BaselineHead: command(t, dir, "rev-parse", "HEAD"), Branch: "main", PathsJSON: `["task.txt"]`})
	if err != nil {
		t.Fatal(err)
	}
	command(t, dir, "add", "task.txt")
	command(t, dir, "commit", "-qm", "Complete issue #1", "-m", "Outpost-Assignment: a")
	committed := command(t, dir, "rev-parse", "HEAD")
	r, err := Deliver(ctx, db, dir, a, s, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if r.Receipt.CommitSHA != committed || command(t, dir, "rev-list", "--count", "HEAD") != "2" {
		t.Fatalf("recovery created another commit: %+v", r.Receipt)
	}
}

func TestLostPushAcknowledgementInspectsRemote(t *testing.T) {
	db, dir, a, s, candidate := setupDelivery(t, "push")
	ctx := context.Background()
	r, err := Deliver(ctx, db, dir, a, s, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.ExecContext(ctx, `UPDATE delivery_receipts SET state='push_unknown' WHERE id=?`, r.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	again, err := Deliver(ctx, db, dir, a, s, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if again.Receipt.State != "settled" || again.Receipt.CommitSHA != r.Receipt.CommitSHA {
		t.Fatalf("remote receipt did not reconcile: %+v", again.Receipt)
	}
}

func TestRejectedReviewKeepsCandidateAndLinksRepair(t *testing.T) {
	db, _, a, _, candidate := setupDelivery(t, "local_changes")
	ctx := context.Background()
	_, err := db.DB.ExecContext(ctx, `UPDATE assignment_attempts SET phase='awaiting_review' WHERE id='t'`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO check_receipts(id,assignment_id,attempt_id,generation,check_id,name,required,command_json,contract_digest,input_digest,candidate_digest,status,started_at,output_path,terminal_path) VALUES('check','a','t',1,'test','Test',1,'{}','contract','input',?,'passed','now','/tmp/check','/tmp/terminal')`, candidate)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.DB.ExecContext(ctx, `UPDATE assignments SET checks_receipt='check' WHERE id='a'`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO provider_attempts(attempt_id,assignment_id,state,version,model,reasoning,sandbox,approval_policy,prompt,thread_id,created_at,updated_at) VALUES('t','a','completed','fixture','model','low','workspace-write','on-request','prompt','thread','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.DecideReview(ctx, a.ID, candidate, "rejected", "fix edge case"); err != nil {
		t.Fatal(err)
	}
	if err = db.DecideReview(ctx, a.ID, candidate, "approved", ""); err == nil {
		t.Fatal("conflicting review accepted")
	}
	thread, err := db.BeginReviewRepair(ctx, a.ID, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if thread != "thread" {
		t.Fatalf("wrong thread: %s", thread)
	}
	var phase, checkID string
	if err = db.DB.QueryRowContext(ctx, `SELECT t.phase,a.checks_receipt FROM assignment_attempts t JOIN assignments a ON a.id=t.assignment_id WHERE t.assignment_id='a' AND t.outcome='running'`).Scan(&phase, &checkID); err != nil {
		t.Fatal(err)
	}
	if phase != "implementing" || checkID != "" {
		t.Fatalf("repair phase=%s check=%s", phase, checkID)
	}
}
