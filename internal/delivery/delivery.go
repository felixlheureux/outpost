// Package delivery owns the recorded Git effect after runner validation.
package delivery

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/validation"
)

type Result struct {
	Receipt store.DeliveryReceipt
	Head    string
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	timeout := 20 * time.Second
	if len(args) > 0 && (args[0] == "push" || args[0] == "fetch") {
		timeout = 2 * time.Minute
	}
	b, err := hostcmd.RunWithTimeout(ctx, timeout, dir, "git", nil, args...)
	return strings.TrimSpace(string(b)), err
}

func paths(ctx context.Context, dir string) ([]string, error) {
	a, err := hostcmd.Run(ctx, dir, "git", nil, "diff", "--no-renames", "--name-only", "-z", "HEAD")
	if err != nil {
		return nil, err
	}
	b, err := hostcmd.Run(ctx, dir, "git", nil, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, p := range strings.Split(string(append(a, b...)), "\x00") {
		if p != "" {
			seen[p] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	if len(out) > 5000 {
		return nil, fmt.Errorf("delivery has too many changed paths")
	}
	return out, nil
}

func policy(a store.Assignment, schedule store.Schedule) (string, string, error) {
	var raw struct {
		Requirements struct {
			Settings struct {
				Delivery string `json:"delivery"`
				Review   string `json:"review"`
			} `json:"settings"`
		} `json:"requirements"`
	}
	if err := json.Unmarshal([]byte(a.SnapshotJSON), &raw); err != nil {
		return "", "", err
	}
	var run struct {
		Delivery string `json:"delivery"`
		Review   string `json:"review"`
	}
	if err := json.Unmarshal([]byte(schedule.PolicyJSON), &run); err != nil {
		return "", "", err
	}
	delivery := run.Delivery
	if delivery == "" {
		delivery = "local_changes"
	}
	if raw.Requirements.Settings.Delivery != "" && raw.Requirements.Settings.Delivery != "inherit" {
		delivery = raw.Requirements.Settings.Delivery
	}
	review := run.Review
	if review == "" {
		review = "none"
	}
	if raw.Requirements.Settings.Review != "" && raw.Requirements.Settings.Review != "inherit" {
		review = raw.Requirements.Settings.Review
	}
	if delivery != "local_changes" && delivery != "commit" && delivery != "push" {
		return "", "", fmt.Errorf("unsupported delivery policy")
	}
	if review != "none" && review != "required" {
		return "", "", fmt.Errorf("unsupported review policy")
	}
	return delivery, review, nil
}

func Policy(a store.Assignment, s store.Schedule) (string, string, error) { return policy(a, s) }

func Deliver(ctx context.Context, db *store.Store, workspace string, a store.Assignment, schedule store.Schedule, candidate string) (Result, error) {
	mode, _, err := policy(a, schedule)
	if err != nil {
		return Result{}, err
	}
	var snapshot struct {
		Branch       string `json:"branch"`
		Head         string `json:"head"`
		Status       string `json:"status"`
		Upstream     string `json:"upstream"`
		RemoteDigest string `json:"remote_digest"`
	}
	if err = json.Unmarshal([]byte(a.SnapshotJSON), &snapshot); err != nil {
		return Result{}, err
	}
	if snapshot.Branch == "" || snapshot.Head == "" || snapshot.Status != "" {
		return Result{}, fmt.Errorf("assignment lacks a clean, attached Git baseline")
	}
	current, err := validation.CandidateIdentity(ctx, workspace, nil)
	if err != nil {
		return Result{}, err
	}
	if current != candidate {
		return Result{}, fmt.Errorf("candidate changed after validation; run checks again")
	}
	branch, err := git(ctx, workspace, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != snapshot.Branch {
		return Result{}, fmt.Errorf("delivery branch differs from pinned assignment")
	}
	changed, err := paths(ctx, workspace)
	if err != nil {
		return Result{}, err
	}
	var receipt store.DeliveryReceipt
	receipt, err = db.GetDelivery(ctx, a.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		if head, e := git(ctx, workspace, "rev-parse", "HEAD"); e != nil || head != snapshot.Head {
			return Result{}, fmt.Errorf("checkout HEAD changed before delivery intent")
		}
		if len(changed) == 0 {
			return Result{}, fmt.Errorf("candidate has no task changes to deliver")
		}
		upstream := ""
		remoteDigest := ""
		if mode == "push" {
			upstream = snapshot.Upstream
			actual, e := git(ctx, workspace, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
			if e != nil || upstream == "" || actual != upstream || !strings.Contains(upstream, "/") {
				return Result{}, fmt.Errorf("push policy requires an attached upstream")
			}
			remote, ref, _ := strings.Cut(upstream, "/")
			if _, e = git(ctx, workspace, "check-ref-format", "--branch", ref); e != nil {
				return Result{}, fmt.Errorf("invalid upstream branch")
			}
			url, e := git(ctx, workspace, "remote", "get-url", remote)
			if e != nil {
				return Result{}, e
			}
			remoteDigest = fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
			if remoteDigest != snapshot.RemoteDigest {
				return Result{}, fmt.Errorf("upstream remote URL changed since assignment")
			}
		}
		encoded, _ := json.Marshal(changed)
		receipt, err = db.BeginDelivery(ctx, store.DeliveryReceipt{AssignmentID: a.ID, Policy: mode, Candidate: candidate, BaselineHead: snapshot.Head, Branch: branch, Upstream: upstream, RemoteDigest: remoteDigest, PathsJSON: string(encoded)})
		if err != nil {
			return Result{}, err
		}
	}
	if receipt.Policy != mode || receipt.Candidate != candidate || receipt.BaselineHead != snapshot.Head || receipt.Branch != branch {
		return Result{}, store.ErrAdmissionChanged
	}
	if mode == "push" {
		remote, _, ok := strings.Cut(receipt.Upstream, "/")
		if !ok || snapshot.Upstream != receipt.Upstream {
			return Result{}, fmt.Errorf("pinned upstream changed")
		}
		url, e := git(ctx, workspace, "remote", "get-url", remote)
		if e != nil {
			return Result{}, e
		}
		if fmt.Sprintf("%x", sha256.Sum256([]byte(url))) != receipt.RemoteDigest {
			return Result{}, fmt.Errorf("upstream remote URL changed")
		}
	}
	if receipt.State == "blocked" {
		return Result{}, fmt.Errorf("delivery is blocked: %s", receipt.Reason)
	}
	var owned []string
	if err = json.Unmarshal([]byte(receipt.PathsJSON), &owned); err != nil {
		return Result{}, err
	}
	if receipt.State == "settled" {
		head, err := git(ctx, workspace, "rev-parse", "HEAD")
		if err != nil {
			return Result{}, err
		}
		if receipt.CommitSHA != "" && head != receipt.CommitSHA {
			return Result{}, fmt.Errorf("checkout moved after settled delivery")
		}
		if receipt.CommitSHA == "" && head != snapshot.Head {
			return Result{}, fmt.Errorf("local delivery baseline changed")
		}
		if mode == "push" {
			remote, ref, _ := strings.Cut(receipt.Upstream, "/")
			contains, e := remoteContains(ctx, workspace, remote, ref, receipt.CommitSHA)
			if e != nil {
				return Result{}, e
			}
			if !contains {
				return Result{}, fmt.Errorf("upstream no longer contains the delivery commit")
			}
		}
		return Result{receipt, head}, nil
	}
	if mode == "local_changes" {
		if !slices.Equal(owned, changed) {
			return Result{}, fmt.Errorf("changed paths differ from recorded delivery intent")
		}
		if err = db.UpdateDelivery(ctx, receipt.ID, "settled", "", ""); err != nil {
			return Result{}, err
		}
		receipt, err = db.GetDelivery(ctx, a.ID)
		if err != nil {
			return Result{}, err
		}
		return Result{Receipt: receipt, Head: snapshot.Head}, nil
	}
	marker := "Outpost-Assignment: " + a.ID
	head, err := git(ctx, workspace, "rev-parse", "HEAD")
	if err != nil {
		return Result{}, err
	}
	if receipt.CommitSHA == "" {
		message, e := git(ctx, workspace, "log", "-1", "--format=%B")
		if e != nil {
			return Result{}, e
		}
		if strings.Contains(message, marker) {
			parent, e := git(ctx, workspace, "rev-parse", "HEAD^")
			if e != nil || parent != snapshot.Head {
				return Result{}, fmt.Errorf("recorded commit marker has unexpected parent")
			}
			if len(changed) != 0 {
				return Result{}, fmt.Errorf("checkout has changes after task commit")
			}
		} else {
			if head != snapshot.Head || !slices.Equal(owned, changed) {
				return Result{}, fmt.Errorf("checkout changed after delivery intent; inspect ownership")
			}
			// HEAD and the exact path set were inspected before replaying an unknown commit.
			if err = db.UpdateDelivery(ctx, receipt.ID, "commit_unknown", "", "commit acknowledgement pending reconciliation"); err != nil {
				return Result{}, err
			}
			args := append([]string{"add", "-A", "--"}, owned...)
			if _, err = git(ctx, workspace, args...); err != nil {
				return Result{}, err
			}
			args = append([]string{"commit", "--only", "-m", fmt.Sprintf("Complete issue #%d", a.IssueNumber), "-m", marker, "--"}, owned...)
			if _, err = git(ctx, workspace, args...); err != nil {
				return Result{}, err
			}
			head, err = git(ctx, workspace, "rev-parse", "HEAD")
			if err != nil {
				return Result{}, err
			}
		}
		if err = db.UpdateDelivery(ctx, receipt.ID, "pending", head, ""); err != nil {
			return Result{}, err
		}
		receipt, err = db.GetDelivery(ctx, a.ID)
		if err != nil {
			return Result{}, err
		}
	}
	if head != receipt.CommitSHA {
		return Result{}, fmt.Errorf("checkout moved after task commit")
	}
	commitPaths, err := hostcmd.Run(ctx, workspace, "git", nil, "diff-tree", "--no-commit-id", "--no-renames", "--name-only", "-r", "-z", "HEAD")
	if err != nil {
		return Result{}, err
	}
	actualPaths := []string{}
	for _, p := range strings.Split(string(commitPaths), "\x00") {
		if p != "" {
			actualPaths = append(actualPaths, p)
		}
	}
	slices.Sort(actualPaths)
	if !slices.Equal(actualPaths, owned) {
		return Result{}, fmt.Errorf("task commit contains unexpected paths")
	}
	remaining, err := paths(ctx, workspace)
	if err != nil {
		return Result{}, err
	}
	if len(remaining) != 0 {
		return Result{}, fmt.Errorf("checkout changed after task commit")
	}
	if mode == "push" {
		remote, ref, ok := strings.Cut(receipt.Upstream, "/")
		if !ok || remote == "" || ref == "" {
			return Result{}, fmt.Errorf("invalid pinned upstream")
		}
		actual, e := git(ctx, workspace, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
		if e != nil || actual != receipt.Upstream {
			return Result{}, fmt.Errorf("upstream changed since delivery intent")
		}
		contains, e := remoteContains(ctx, workspace, remote, ref, head)
		if e != nil {
			return Result{}, e
		}
		if !contains {
			// The selected remote branch was fetched and inspected before replay.
			if err = db.UpdateDelivery(ctx, receipt.ID, "push_unknown", head, "push acknowledgement pending reconciliation"); err != nil {
				return Result{}, err
			}
			if _, err = git(ctx, workspace, "push", remote, "HEAD:refs/heads/"+ref); err != nil {
				return Result{}, err
			}
			contains, err = remoteContains(ctx, workspace, remote, ref, head)
			if err != nil {
				return Result{}, err
			}
			if !contains {
				return Result{}, fmt.Errorf("remote does not contain delivery commit")
			}
		}
	}
	if err = db.UpdateDelivery(ctx, receipt.ID, "settled", head, ""); err != nil {
		return Result{}, err
	}
	receipt, err = db.GetDelivery(ctx, a.ID)
	if err != nil {
		return Result{}, err
	}
	return Result{Receipt: receipt, Head: head}, nil
}

func remoteContains(ctx context.Context, dir, remote, ref, commit string) (bool, error) {
	// Fetch only the selected branch. This never checks out or merges it.
	if _, err := git(ctx, dir, "fetch", "--no-tags", remote, "refs/heads/"+ref); err != nil {
		return false, err
	}
	_, err := git(ctx, dir, "merge-base", "--is-ancestor", commit, "FETCH_HEAD")
	return err == nil, nil
}
