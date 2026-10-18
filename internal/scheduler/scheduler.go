// Package scheduler selects the next task from a fresh GitHub observation.
// It does not start a provider or let source text grant process permissions.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/felixlheureux/outpost/internal/gitcli"
	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/issues"
	"github.com/felixlheureux/outpost/internal/queue"
	"github.com/felixlheureux/outpost/internal/store"
	"github.com/felixlheureux/outpost/internal/validation"
)

type Reader interface {
	Queue(context.Context, string, string) ([]byte, string, error)
	Issue(context.Context, string, string, int) (githubcli.Issue, error)
}

type githubReader struct{}

func (githubReader) Queue(ctx context.Context, owner, name string) ([]byte, string, error) {
	return githubcli.ReadQueue(ctx, owner, name)
}
func (githubReader) Issue(ctx context.Context, owner, name string, n int) (githubcli.Issue, error) {
	return githubcli.GetIssue(ctx, owner, name, n)
}

type GitHubReader = githubReader

type Waiting struct {
	Issue  int    `json:"issue"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}
type Projection struct {
	Selected  int       `json:"selected"`
	Waiting   []Waiting `json:"waiting"`
	Outcome   string    `json:"outcome"`
	QueueSHA  string    `json:"queue_sha"`
	FetchedAt string    `json:"fetched_at"`
}

type Snapshot struct {
	IssueNumber     int                 `json:"issue_number"`
	IssueNodeID     string              `json:"issue_node_id"`
	IssueURL        string              `json:"issue_url"`
	Title           string              `json:"title"`
	Body            string              `json:"body"`
	Requirements    issues.Requirements `json:"requirements"`
	IssueDigest     string              `json:"issue_digest"`
	IssueRevision   int64               `json:"issue_revision"`
	QueueSHA        string              `json:"queue_sha"`
	QueuePosition   int                 `json:"queue_position"`
	FetchedAt       string              `json:"fetched_at"`
	WorkspacePath   string              `json:"workspace_path"`
	Branch          string              `json:"branch"`
	Head            string              `json:"head"`
	Status          string              `json:"status"`
	Instructions    string              `json:"instructions"`
	PolicyJSON      json.RawMessage     `json:"policy"`
	DependencyHeads map[int]string      `json:"dependency_heads"`
	Skipped         []Waiting           `json:"skipped"`
}

type Engine struct {
	DB     *store.Store
	Source Reader
}

func (e Engine) reader() Reader {
	if e.Source != nil {
		return e.Source
	}
	return githubReader{}
}

// PreviewProject is a fresh, read-only admission projection for the start
// panel. It does not reserve the global slot or authorize execution.
func (e Engine) PreviewProject(ctx context.Context, projectID, mode string, selected int) (Projection, error) {
	project, err := e.DB.GetProjectSetup(ctx, projectID)
	if err != nil {
		return Projection{}, err
	}
	if project.State != "ready" {
		return Projection{Outcome: "project_not_ready", Waiting: []Waiting{}}, nil
	}
	manifest, sha, observed, fetchedAt, err := e.Observe(ctx, project, nil)
	if err != nil {
		return Projection{Outcome: "source_unavailable", Waiting: []Waiting{{Code: "source_unavailable", Reason: err.Error()}}}, nil
	}
	schedule := store.Schedule{Kind: "queue", Mode: mode, ScopeJSON: "[]", TaskLimit: 20}
	if mode == "fixed" {
		if selected <= 0 {
			return Projection{Outcome: "invalid_scope", Waiting: []Waiting{}}, nil
		}
		b, _ := json.Marshal([]int{selected})
		schedule.ScopeJSON = string(b)
	}
	if mode != "fixed" && mode != "follow" {
		return Projection{}, fmt.Errorf("invalid preview mode")
	}
	gens, err := e.DB.ListGenerations(ctx, projectID)
	if err != nil {
		return Projection{}, err
	}
	usable := usableOutputs(ctx, project.WorkspacePath, gens)
	branch, status, err := gitcli.Baseline(ctx, project.WorkspacePath)
	if err != nil || branch == "" {
		return Projection{Outcome: "baseline_changed", Waiting: []Waiting{}}, nil
	}
	p := evaluate(schedule, manifest, sha, fetchedAt, observed, gens, usable)
	if status != "" {
		p.Outcome = "baseline_changed"
		p.Selected = 0
		p.Waiting = append(p.Waiting, Waiting{Code: "baseline_changed", Reason: "Checkout has staged, unstaged, or untracked work."})
	}
	return p, nil
}

type observed struct {
	source     githubcli.Issue
	req        issues.Requirements
	digest     string
	parseError string
}

func usableOutputs(ctx context.Context, workspace string, gens map[int]store.Generation) map[int]bool {
	result := map[int]bool{}
	visiting := map[int]bool{}
	var visit func(int) bool
	visit = func(n int) bool {
		if v, found := result[n]; found {
			return v
		}
		if visiting[n] {
			return false
		}
		visiting[n] = true
		g := gens[n]
		ok := g.State == "succeeded" && gitcli.ContainsCommit(ctx, workspace, g.BaselineHead)
		if ok {
			for _, dep := range g.DependsOn {
				if !visit(dep) {
					ok = false
					break
				}
			}
		}
		visiting[n] = false
		result[n] = ok
		return ok
	}
	for n := range gens {
		visit(n)
	}
	return result
}

// Observe fetches every queued issue and recursive dependency from GitHub.
// Source calls complete before a SQLite transaction begins.
func (e Engine) Observe(ctx context.Context, project store.ProjectSetup, extra []int) (queue.Manifest, string, map[int]observed, string, error) {
	content, sha, err := e.reader().Queue(ctx, project.Owner, project.Name)
	if err != nil {
		_ = e.DB.FailQueueSource(ctx, project.ID, "Planning source unavailable; check GitHub access.")
		return queue.Manifest{}, "", nil, "", fmt.Errorf("planning source unavailable: %w", err)
	}
	manifest, err := queue.Parse(content)
	if err != nil {
		_ = e.DB.FailQueueSource(ctx, project.ID, err.Error())
		return queue.Manifest{}, "", nil, "", err
	}
	all := map[int]observed{}
	var load func(int) error
	load = func(number int) error {
		if _, found := all[number]; found {
			return nil
		}
		if len(all) >= 500 {
			return fmt.Errorf("dependency graph exceeds 500 issues")
		}
		issue, err := e.reader().Issue(ctx, project.Owner, project.Name, number)
		if err != nil {
			return fmt.Errorf("issue #%d is inaccessible: %w", number, err)
		}
		if issue.Number != number || issue.ID <= 0 || issue.NodeID == "" || issue.HTMLURL == "" || (issue.State != "open" && issue.State != "closed") {
			return fmt.Errorf("issue #%d has invalid source identity", number)
		}
		req, parseErr := issues.Parse(number, issue.Title, issue.Body)
		item := observed{source: issue, req: req, digest: issues.Digest(issue.Title, issue.Body)}
		if parseErr != nil {
			item.parseError = parseErr.Error()
		}
		all[number] = item
		if parseErr == nil {
			for _, dep := range req.Settings.DependsOn {
				if err := load(dep); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, n := range manifest.Issues {
		if err := load(n); err != nil {
			_ = e.DB.RecordIssueSourceFailure(ctx, project.ID, "Queued issue or dependency is unavailable; check GitHub access.")
			return queue.Manifest{}, "", nil, "", err
		}
	}
	for _, n := range extra {
		if err := load(n); err != nil {
			_ = e.DB.RecordIssueSourceFailure(ctx, project.ID, "Selected issue is unavailable; check GitHub access.")
			return queue.Manifest{}, "", nil, "", err
		}
	}
	observations := make([]store.IssueObservation, 0, len(all))
	for _, v := range all {
		reqJSON, _ := json.Marshal(v.req)
		observations = append(observations, store.IssueObservation{NodeID: v.source.NodeID, GithubID: v.source.ID, Number: v.source.Number, URL: v.source.HTMLURL, Title: v.source.Title, Body: v.source.Body, State: v.source.State, SourceUpdatedAt: v.source.UpdatedAt, ContentDigest: v.digest, RequirementsJSON: string(reqJSON), ParseError: v.parseError})
	}
	if err := e.DB.ApplyIssueObservations(ctx, project.ID, observations); err != nil {
		return queue.Manifest{}, "", nil, "", err
	}
	encoded, _ := json.Marshal(manifest.Issues)
	if err := e.DB.SaveQueueSource(ctx, project.ID, sha, string(encoded)); err != nil {
		return queue.Manifest{}, "", nil, "", err
	}
	return manifest, sha, all, time.Now().UTC().Format(time.RFC3339Nano), nil
}

func scope(schedule store.Schedule, manifest queue.Manifest) ([]int, error) {
	if schedule.Kind == "instruction" {
		return nil, fmt.Errorf("instruction runs do not admit queue tasks")
	}
	if schedule.Mode == "follow" {
		return manifest.Issues, nil
	}
	var fixed []int
	if err := json.Unmarshal([]byte(schedule.ScopeJSON), &fixed); err != nil || len(fixed) == 0 || len(fixed) > queue.MaxIssues {
		return nil, fmt.Errorf("invalid captured scope")
	}
	seen := map[int]bool{}
	for _, n := range fixed {
		if n <= 0 || seen[n] {
			return nil, fmt.Errorf("invalid captured scope")
		}
		seen[n] = true
	}
	if schedule.Kind == "task" && len(fixed) != 1 {
		return nil, fmt.Errorf("single-task run needs exactly one issue")
	}
	return fixed, nil
}

func evaluate(schedule store.Schedule, manifest queue.Manifest, sha, fetchedAt string, observed map[int]observed, generations map[int]store.Generation, usable map[int]bool) Projection {
	p := Projection{Waiting: []Waiting{}, QueueSHA: sha, FetchedAt: fetchedAt}
	if schedule.StopRequested {
		p.Outcome = "stopped"
		return p
	}
	if schedule.SuccessfulCount >= schedule.TaskLimit {
		p.Outcome = "budget_reached"
		return p
	}
	if schedule.DeadlineAt != "" && schedule.DeadlineAt <= time.Now().UTC().Format(time.RFC3339Nano) {
		p.Outcome = "budget_reached"
		return p
	}
	for _, g := range generations {
		if g.State == "owned" || g.State == "failed" || g.State == "blocked" || g.State == "interrupted" {
			p.Outcome = "unfinished_assignment"
			p.Waiting = append(p.Waiting, Waiting{Issue: g.IssueNumber, Code: "unfinished_assignment", Reason: "An earlier assignment in this checkout needs explicit recovery."})
			return p
		}
	}
	order, err := scope(schedule, manifest)
	if err != nil {
		p.Outcome = "invalid_scope"
		return p
	}
	requirements := map[int]issues.Requirements{}
	for n, v := range observed {
		if v.parseError == "" {
			requirements[n] = v.req
		}
	}
	graphProblems := issues.ValidateDependencies(requirements)
	closed := 0
	for _, number := range order {
		item, ok := observed[number]
		wait := Waiting{Issue: number}
		switch {
		case schedule.Kind == "queue" && !slices.Contains(manifest.Issues, number):
			wait.Code = "not_queued"
			wait.Reason = "Issue is no longer in the shared queue."
		case !ok:
			wait.Code = "source_unavailable"
			wait.Reason = "Issue source was not observed."
		case item.parseError != "":
			wait.Code = "invalid_issue"
			wait.Reason = item.parseError
		case graphProblems[number] != "":
			wait.Code = "invalid_dependencies"
			wait.Reason = graphProblems[number]
		case item.source.State == "closed":
			wait.Code = "externally_closed"
			wait.Reason = "GitHub issue is closed; closure is not completion evidence."
			closed++
		case generations[number].State == "succeeded":
			wait.Code = "settled"
			wait.Reason = "This execution generation already succeeded."
		case item.req.Settings.HoldReason != nil && *item.req.Settings.HoldReason != "":
			wait.Code = "held"
			wait.Reason = *item.req.Settings.HoldReason
		case !selectedProfile(schedule, item.req.Settings.CheckProfile):
			wait.Code = "invalid_check_profile"
			wait.Reason = "Issue check profile is not configured for this project."
		default:
			for _, dep := range item.req.Settings.DependsOn {
				g := generations[dep]
				if g.State != "succeeded" || !usable[dep] {
					wait.Code = "dependency_incomplete"
					wait.Reason = fmt.Sprintf("Dependency #%d has no verified output in this checkout baseline.", dep)
					break
				}
			}
		}
		if wait.Code != "" {
			p.Waiting = append(p.Waiting, wait)
			continue
		}
		p.Selected = number
		return p
	}
	if len(order) == 0 {
		p.Outcome = "drained"
	} else if closed == len(order) {
		p.Outcome = "externally_closed"
	} else {
		p.Outcome = "drained"
		for _, w := range p.Waiting {
			if w.Code != "settled" && w.Code != "externally_closed" && w.Code != "not_queued" {
				p.Outcome = "blocked"
				break
			}
		}
	}
	return p
}

func selectedProfile(schedule store.Schedule, selected string) bool {
	var policy struct {
		CheckProfile validation.Profile `json:"check_profile"`
	}
	if json.Unmarshal([]byte(schedule.PolicyJSON), &policy) != nil || policy.CheckProfile.ID == "" {
		return selected == "inherit"
	}
	return selected == "inherit" || selected == policy.CheckProfile.ID
}

// Next refreshes the source, computes a projection, and atomically claims one
// immutable assignment when claim is true. The caller must hold the workspace
// advisory lock and be the recorded worker before invoking a claim.
func (e Engine) Next(ctx context.Context, runID string, claim bool) (Projection, *store.Assignment, error) {
	run, err := e.DB.GetRun(ctx, runID)
	if err != nil {
		return Projection{}, nil, err
	}
	schedule, err := e.DB.GetSchedule(ctx, runID)
	if err != nil {
		return Projection{}, nil, err
	}
	if schedule.Outcome != "active" {
		return Projection{Outcome: schedule.Outcome}, nil, nil
	}
	if schedule.Kind == "instruction" {
		return Projection{Outcome: "instruction_mode", Waiting: []Waiting{}}, nil, nil
	}
	if existing, err := e.DB.CurrentAssignment(ctx, runID); err == nil {
		return Projection{Outcome: "unfinished_assignment", Waiting: []Waiting{{Issue: existing.IssueNumber, Code: "unfinished_assignment", Reason: "Resume or settle the current assignment."}}}, &existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Projection{}, nil, err
	}
	project, err := e.DB.GetProjectSetup(ctx, run.ProjectID)
	if err != nil {
		return Projection{}, nil, err
	}
	if project.State != "ready" || project.WorkspacePath != run.WorkspacePath {
		return Projection{Outcome: "project_not_ready"}, nil, nil
	}
	var extra []int
	if schedule.Kind == "task" {
		if err := json.Unmarshal([]byte(schedule.ScopeJSON), &extra); err != nil {
			return Projection{}, nil, err
		}
	}
	manifest, sha, observed, fetchedAt, err := e.Observe(ctx, project, extra)
	if err != nil {
		return Projection{Outcome: "source_unavailable", Waiting: []Waiting{{Code: "source_unavailable", Reason: err.Error()}}}, nil, nil
	}
	head, err := gitcli.Head(ctx, run.WorkspacePath)
	if err != nil {
		return Projection{Outcome: "baseline_changed"}, nil, nil
	}
	gens, err := e.DB.ListGenerations(ctx, run.ProjectID)
	if err != nil {
		return Projection{}, nil, err
	}
	usable := usableOutputs(ctx, run.WorkspacePath, gens)
	projection := evaluate(schedule, manifest, sha, fetchedAt, observed, gens, usable)
	if !claim || projection.Selected == 0 {
		return projection, nil, nil
	}
	branch, status, err := gitcli.Baseline(ctx, run.WorkspacePath)
	if err != nil {
		return Projection{}, nil, err
	}
	if status != "" {
		projection.Outcome = "baseline_changed"
		projection.Waiting = append(projection.Waiting, Waiting{Issue: projection.Selected, Code: "baseline_changed", Reason: "Checkout has staged, unstaged, or untracked work."})
		projection.Selected = 0
		return projection, nil, nil
	}
	chosen := observed[projection.Selected]
	var revision int64
	all, _, err := e.DB.ListIssueSources(ctx, run.ProjectID)
	if err != nil {
		return Projection{}, nil, err
	}
	for _, v := range all {
		if v.Number == projection.Selected {
			revision = v.LocalRevision
			break
		}
	}
	position := slices.Index(manifest.Issues, projection.Selected) + 1
	deps := map[int]string{}
	for _, n := range chosen.req.Settings.DependsOn {
		deps[n] = gens[n].BaselineHead
	}
	snapshot := Snapshot{IssueNumber: projection.Selected, IssueNodeID: chosen.source.NodeID, IssueURL: chosen.source.HTMLURL, Title: chosen.source.Title, Body: chosen.source.Body, Requirements: chosen.req, IssueDigest: chosen.digest, IssueRevision: revision, QueueSHA: sha, QueuePosition: position, FetchedAt: fetchedAt, WorkspacePath: run.WorkspacePath, Branch: branch, Head: head, Status: status, Instructions: schedule.Instructions, PolicyJSON: json.RawMessage(schedule.PolicyJSON), DependencyHeads: deps, Skipped: projection.Waiting}
	b, _ := json.Marshal(snapshot)
	generation := 1
	if g, found := gens[projection.Selected]; found {
		generation = g.Number
	}
	digests := make(map[int]string, len(observed))
	for n, v := range observed {
		digests[n] = v.digest
	}
	a, err := e.DB.ClaimAssignment(ctx, store.ClaimInput{RunID: runID, ProjectID: run.ProjectID, QueueSHA: sha, IssueDigest: chosen.digest, SourceDigests: digests, SnapshotJSON: string(b), IssueNumber: projection.Selected, Generation: generation, ExpectedRevision: schedule.Revision})
	if err != nil {
		return Projection{}, nil, err
	}
	return projection, &a, nil
}
