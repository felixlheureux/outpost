// Package queue owns the single planning-branch manifest and durable edit receipts.
package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
)

const MaxIssues = 100

type Manifest struct {
	Version int   `json:"version"`
	Issues  []int `json:"issues"`
}
type Source struct {
	SHA       string `json:"sha"`
	Issues    []int  `json:"issues"`
	State     string `json:"state"`
	FetchedAt string `json:"fetched_at"`
	LastError string `json:"last_error"`
}
type Edit struct {
	RequestKey  string `json:"request_key"`
	ExpectedSHA string `json:"expected_sha"`
	Kind        string `json:"kind"`
	Issues      []int  `json:"issues,omitempty"`
	Issue       int    `json:"issue,omitempty"`
	Before      int    `json:"before,omitempty"`
}

func Parse(data []byte) (Manifest, error) {
	if len(data) > 256<<10 {
		return Manifest{}, fmt.Errorf("queue manifest is oversized")
	}
	var m Manifest
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("invalid queue manifest: %w", err)
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("queue manifest has trailing content")
	}
	if m.Version != 1 || m.Issues == nil || len(m.Issues) > MaxIssues {
		return Manifest{}, fmt.Errorf("unsupported queue version or issue count")
	}
	seen := map[int]bool{}
	for _, n := range m.Issues {
		if n <= 0 || seen[n] {
			return Manifest{}, fmt.Errorf("queue has invalid or duplicate issue numbers")
		}
		seen[n] = true
	}
	return m, nil
}

func Encode(issues []int) (string, error) {
	if issues == nil {
		issues = []int{}
	}
	b, _ := json.Marshal(Manifest{Version: 1, Issues: issues})
	_, err := Parse(b)
	return string(b) + "\n", err
}

func Cached(ctx context.Context, db *store.Store, projectID string) (Source, error) {
	if _, err := db.GetProjectSetup(ctx, projectID); err != nil {
		return Source{}, err
	}
	s, err := db.QueueSource(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{Issues: []int{}, State: "not_synced"}, nil
	}
	if err != nil {
		return Source{}, err
	}
	var issues []int
	if err := json.Unmarshal([]byte(s.IssuesJSON), &issues); err != nil {
		return Source{}, err
	}
	return Source{SHA: s.SHA, Issues: issues, State: s.State, FetchedAt: s.FetchedAt, LastError: s.LastError}, nil
}

func Sync(ctx context.Context, db *store.Store, projectID string) (Source, error) {
	p, err := db.GetProjectSetup(ctx, projectID)
	if err != nil {
		return Source{}, err
	}
	content, sha, err := githubcli.ReadQueue(ctx, p.Owner, p.Name)
	if err == nil {
		var m Manifest
		m, err = Parse(content)
		if err == nil {
			err = validateReferences(ctx, p.Owner, p.Name, m.Issues)
		}
		if err == nil {
			b, _ := json.Marshal(m.Issues)
			err = db.SaveQueueSource(ctx, projectID, sha, string(b))
		}
	}
	if err != nil {
		_ = db.FailQueueSource(ctx, projectID, safeReason(err))
		return Cached(ctx, db, projectID)
	}
	return Cached(ctx, db, projectID)
}

func safeReason(err error) string {
	var command *hostcmd.CommandError
	if errors.As(err, &command) {
		return "GitHub planning source: " + command.Kind + "; check repository and planning branch access"
	}
	message := err.Error()
	if len(message) > 300 {
		return message[:300]
	}
	return message
}

func validateReferences(ctx context.Context, owner, name string, issues []int) error {
	for _, n := range issues {
		if _, err := githubcli.GetIssue(ctx, owner, name, n); err != nil {
			return fmt.Errorf("queue issue #%d is missing, inaccessible, or invalid: %w", n, err)
		}
	}
	return nil
}

func Apply(current []int, edit Edit) ([]int, error) {
	result := append([]int(nil), current...)
	switch edit.Kind {
	case "replace":
		if edit.Issues == nil {
			return nil, fmt.Errorf("replacement order is required")
		}
		result = append([]int(nil), edit.Issues...)
	case "append":
		if edit.Issue <= 0 {
			return nil, fmt.Errorf("select an issue")
		}
		for _, n := range result {
			if n == edit.Issue {
				return result, nil
			}
		}
		result = append(result, edit.Issue)
	case "remove", "move":
		if edit.Issue <= 0 {
			return nil, fmt.Errorf("select an issue")
		}
		index := -1
		for i, n := range result {
			if n == edit.Issue {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("issue is not in the queue")
		}
		result = append(result[:index], result[index+1:]...)
		if edit.Kind == "move" {
			at := len(result)
			if edit.Before != 0 {
				at = -1
				for i, n := range result {
					if n == edit.Before {
						at = i
						break
					}
				}
				if at < 0 {
					return nil, fmt.Errorf("move target is not in the queue")
				}
			}
			result = append(result, 0)
			copy(result[at+1:], result[at:])
			result[at] = edit.Issue
		}
	default:
		return nil, fmt.Errorf("invalid queue edit")
	}
	if _, err := Encode(result); err != nil {
		return nil, err
	}
	return result, nil
}

// Mutate only writes against a fresh remote SHA. A stale client receives the
// current order in its receipt and must deliberately submit a new request key.
func Mutate(ctx context.Context, db *store.Store, projectID string, edit Edit) (store.QueueOperation, error) {
	if edit.RequestKey == "" || len(edit.RequestKey) > 100 || edit.ExpectedSHA == "" {
		return store.QueueOperation{}, fmt.Errorf("request key and expected SHA are required")
	}
	p, err := db.GetProjectSetup(ctx, projectID)
	if err != nil {
		return store.QueueOperation{}, err
	}
	intent, err := json.Marshal(edit)
	if err != nil {
		return store.QueueOperation{}, err
	}
	op, fresh, err := db.ReserveQueueOperation(ctx, store.QueueOperation{ProjectID: projectID, RequestKey: edit.RequestKey, Kind: edit.Kind, ExpectedSHA: edit.ExpectedSHA, IntendedJSON: string(intent)})
	if err != nil {
		return op, err
	}
	if !fresh {
		return op, nil
	}
	content, sha, err := githubcli.ReadQueue(ctx, p.Owner, p.Name)
	if err != nil {
		_ = db.FailQueueSource(ctx, projectID, safeReason(err))
		return settle(ctx, db, op, "unknown", safeReason(err), nil)
	}
	current, err := Parse(content)
	if err != nil {
		_ = db.FailQueueSource(ctx, projectID, safeReason(err))
		return settle(ctx, db, op, "rejected", safeReason(err), nil)
	}
	if err = validateReferences(ctx, p.Owner, p.Name, current.Issues); err != nil {
		_ = db.FailQueueSource(ctx, projectID, safeReason(err))
		return settle(ctx, db, op, "rejected", safeReason(err), current.Issues)
	}
	if err = db.SaveQueueSource(ctx, projectID, sha, jsonIssues(current.Issues)); err != nil {
		return op, err
	}
	if edit.Kind == "append" {
		for _, n := range current.Issues {
			if n == edit.Issue {
				return settle(ctx, db, op, "confirmed", "Issue is already in the queue.", current.Issues)
			}
		}
	}
	if sha != edit.ExpectedSHA {
		return settle(ctx, db, op, "conflict", "Planning file changed; review the current order and submit a new edit.", current.Issues)
	}
	result, err := Apply(current.Issues, edit)
	if err != nil {
		return settle(ctx, db, op, "rejected", err.Error(), current.Issues)
	}
	if err = validateReferences(ctx, p.Owner, p.Name, result); err != nil {
		return settle(ctx, db, op, "rejected", safeReason(err), current.Issues)
	}
	if equal(result, current.Issues) {
		return settle(ctx, db, op, "confirmed", "Queue already has the requested order.", result)
	}
	encoded, _ := Encode(result)
	if err := db.SetQueueIntentResult(ctx, op.ID, jsonIssues(result)); err != nil {
		return op, err
	}
	op.ResultJSON = jsonIssues(result)
	_, err = githubcli.WriteQueue(ctx, p.Owner, p.Name, sha, op.ID, []byte(encoded))
	if err != nil {
		var command *hostcmd.CommandError
		if errors.As(err, &command) {
			if command.Kind == "conflict" {
				latest, syncErr := Sync(ctx, db, projectID)
				if syncErr == nil && latest.State == "ready" {
					return settle(ctx, db, op, "conflict", "Planning file changed; review current order before a new edit.", latest.Issues)
				}
			}
			switch command.Kind {
			case "authentication_required", "permission_denied", "rate_limited", "inaccessible_or_missing":
				_ = db.FailQueueSource(ctx, projectID, safeReason(err))
				return settle(ctx, db, op, "rejected", safeReason(err), current.Issues)
			}
		}
		_ = db.FailQueueSource(ctx, projectID, safeReason(err))
		return settle(ctx, db, op, "unknown", safeReason(err), result)
	}
	return inspect(ctx, db, p, op)
}

func Inspect(ctx context.Context, db *store.Store, projectID, key string) (store.QueueOperation, error) {
	p, err := db.GetProjectSetup(ctx, projectID)
	if err != nil {
		return store.QueueOperation{}, err
	}
	op, err := db.QueueOperation(ctx, projectID, key)
	if err != nil {
		return op, err
	}
	if op.State != "pending" && op.State != "unknown" {
		return op, nil
	}
	return inspect(ctx, db, p, op)
}

func inspect(ctx context.Context, db *store.Store, p store.ProjectSetup, op store.QueueOperation) (store.QueueOperation, error) {
	content, sha, err := githubcli.ReadQueue(ctx, p.Owner, p.Name)
	if err != nil {
		return settle(ctx, db, op, "unknown", safeReason(err), nil)
	}
	m, err := Parse(content)
	if err != nil {
		return settle(ctx, db, op, "unknown", safeReason(err), nil)
	}
	if err := validateReferences(ctx, p.Owner, p.Name, m.Issues); err != nil {
		_ = db.FailQueueSource(ctx, p.ID, safeReason(err))
		return settle(ctx, db, op, "unknown", safeReason(err), m.Issues)
	}
	if err = db.SaveQueueSource(ctx, p.ID, sha, jsonIssues(m.Issues)); err != nil {
		return op, err
	}
	message, err := githubcli.LatestQueueCommitMessage(ctx, p.Owner, p.Name)
	if err != nil {
		return settle(ctx, db, op, "unknown", safeReason(err), m.Issues)
	}
	// A commit marker proves this operation wrote the latest file. An identical
	// payload from another planner alone is insufficient proof.
	if strings.Contains(message, "outpost queue operation "+op.ID) && op.ResultJSON == jsonIssues(m.Issues) {
		return settle(ctx, db, op, "confirmed", "Verified planning commit.", m.Issues)
	}
	return settle(ctx, db, op, "unknown", "No matching planning commit and payload observed; write outcome remains uncertain.", m.Issues)
}

func settle(ctx context.Context, db *store.Store, op store.QueueOperation, state, reason string, result []int) (store.QueueOperation, error) {
	if result == nil {
		result = []int{}
	}
	encoded := jsonIssues(result)
	if err := db.SetQueueOperation(ctx, op.ID, state, reason, encoded); err != nil {
		return op, err
	}
	return db.QueueOperation(ctx, op.ProjectID, op.RequestKey)
}
func jsonIssues(issues []int) string {
	if issues == nil {
		issues = []int{}
	}
	b, _ := json.Marshal(issues)
	return string(b)
}
func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
