package issues

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
)

type Mutation struct {
	RequestKey       string `json:"request_key"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	ExpectedDigest   string `json:"expected_digest"`
	IssueNumber      int    `json:"issue_number"`
	ExpectedRevision int64  `json:"expected_revision"`
}

func Digest(title, body string) string {
	sum := sha256.Sum256([]byte(title + "\x00" + body))
	return hex.EncodeToString(sum[:])
}

func Mutate(ctx context.Context, db *store.Store, projectID, kind string, input Mutation) (store.IssueOperation, error) {
	project, err := db.GetProjectSetup(ctx, projectID)
	if err != nil {
		return store.IssueOperation{}, err
	}
	if kind != "create" && kind != "edit" {
		return store.IssueOperation{}, fmt.Errorf("unsupported issue operation")
	}
	if strings.TrimSpace(input.Title) == "" || len(input.Title) > 500 || len(input.Body) > maxBodyBytes || len(input.RequestKey) < 8 || len(input.RequestKey) > 100 {
		return store.IssueOperation{}, fmt.Errorf("invalid issue operation input")
	}
	if kind == "create" && (input.IssueNumber != 0 || input.ExpectedDigest != "" || input.ExpectedRevision != 0) {
		return store.IssueOperation{}, fmt.Errorf("create cannot target an existing issue")
	}
	if kind == "edit" && (input.IssueNumber <= 0 || input.ExpectedDigest == "" || input.ExpectedRevision <= 0) {
		return store.IssueOperation{}, fmt.Errorf("edit needs expected digest and revision")
	}
	proposed := store.IssueOperation{ProjectID: projectID, RequestKey: input.RequestKey, Kind: kind,
		IssueNumber: input.IssueNumber, ExpectedDigest: input.ExpectedDigest, ExpectedRevision: input.ExpectedRevision,
		Title: input.Title, Body: input.Body}
	op, fresh, err := db.ReserveIssueOperation(ctx, proposed)
	if err != nil {
		return store.IssueOperation{}, err
	}
	if !fresh {
		if op.State == "pending" || op.State == "unknown" {
			return reconcile(ctx, db, project.Owner, project.Name, op)
		}
		return op, nil
	}
	parseNumber := input.IssueNumber
	if kind == "create" {
		parseNumber = 1 << 30
	}
	parsed, err := Parse(parseNumber, input.Title, input.Body)
	if err != nil {
		return settle(db, op, "rejected", 0, err.Error())
	}
	if err := ValidateProfileReference(parsed); err != nil {
		return settle(db, op, "rejected", 0, err.Error())
	}
	if len(parsed.Settings.DependsOn) > 0 {
		remoteIssues, err := githubcli.ListIssues(ctx, project.Owner, project.Name)
		if err != nil {
			return settle(db, op, "rejected", 0, "Dependency source could not be checked before writing; retry with a new request key after source access recovers.")
		}
		requirements := make(map[int]Requirements, len(remoteIssues)+1)
		for _, issue := range remoteIssues {
			if issue.State != "open" {
				continue
			}
			if req, parseErr := Parse(issue.Number, issue.Title, issue.Body); parseErr == nil {
				requirements[issue.Number] = req
			}
		}
		requirements[parseNumber] = parsed
		if problem := ValidateDependencies(requirements)[parseNumber]; problem != "" {
			return settle(db, op, "rejected", 0, problem)
		}
	}
	if kind == "edit" {
		cached, err := db.GetIssueSource(ctx, projectID, input.IssueNumber)
		if errors.Is(err, sql.ErrNoRows) {
			return settle(db, op, "conflict", 0, "Issue is absent from the local source projection; sync before editing.")
		}
		if err != nil {
			return store.IssueOperation{}, err
		}
		if cached.ContentDigest != input.ExpectedDigest || cached.LocalRevision != input.ExpectedRevision {
			return settle(db, op, "conflict", 0, "Issue changed in Outpost; refresh the draft against the latest source.")
		}
		remote, err := githubcli.GetIssue(ctx, project.Owner, project.Name, input.IssueNumber)
		if err != nil {
			return settle(db, op, "rejected", 0, "Issue could not be re-read before editing; retry with a new request key after source access recovers.")
		}
		if Digest(remote.Title, remote.Body) != input.ExpectedDigest {
			return settle(db, op, "conflict", 0, "Issue changed on GitHub; keep the draft and compare revisions.")
		}
	}
	body := input.Body + "\n\n" + op.Marker
	written, err := githubcli.WriteIssue(ctx, project.Owner, project.Name, input.IssueNumber, input.Title, body)
	if err != nil {
		var command *hostcmd.CommandError
		if errors.As(err, &command) {
			switch command.Kind {
			case "authentication_required":
				return settle(db, op, "rejected", 0, "GitHub login is unavailable to the Outpost service user; sign in with gh and retry with a new request key.")
			case "permission_denied":
				return settle(db, op, "rejected", 0, "GitHub issue write permission was denied; check repository access and retry with a new request key.")
			case "rate_limited":
				return settle(db, op, "rejected", 0, "GitHub rate limit reached; wait for it to reset before retrying with a new request key.")
			}
		}
		return settle(db, op, "unknown", 0, "GitHub mutation outcome is unknown; inspect the operation before retrying.")
	}
	if !strings.Contains(written.Body, op.Marker) {
		return settle(db, op, "unknown", written.Number, "GitHub response did not contain the operation marker.")
	}
	observed, err := githubcli.GetIssue(ctx, project.Owner, project.Name, written.Number)
	if err != nil {
		return settle(db, op, "unknown", written.Number, "GitHub write returned but confirmation read failed.")
	}
	if observed.Title != input.Title || observed.Body != body {
		return settle(db, op, "unknown", written.Number, "Issue changed during confirmation; inspect before further edits.")
	}
	return settle(db, op, "confirmed", written.Number, "")
}

func reconcile(ctx context.Context, db *store.Store, owner, name string, op store.IssueOperation) (store.IssueOperation, error) {
	if op.Kind == "create" {
		remote, found, err := githubcli.FindIssueMarker(ctx, owner, name, op.Marker)
		if err != nil {
			return settle(db, op, "unknown", op.ResultNumber, "Could not inspect GitHub for the operation marker.")
		}
		if !found {
			return settle(db, op, "unknown", 0, "No matching issue found; creation may still have succeeded. Manual review is required before replay.")
		}
		if remote.Title == op.Title && remote.Body == op.Body+"\n\n"+op.Marker {
			return settle(db, op, "confirmed", remote.Number, "")
		}
		return settle(db, op, "unknown", remote.Number, "Matching issue changed after creation; inspect it before further edits.")
	}
	remote, err := githubcli.GetIssue(ctx, owner, name, op.IssueNumber)
	if err != nil {
		return settle(db, op, "unknown", op.ResultNumber, "Could not inspect the edited issue on GitHub.")
	}
	if remote.Title == op.Title && remote.Body == op.Body+"\n\n"+op.Marker {
		return settle(db, op, "confirmed", remote.Number, "")
	}
	return settle(db, op, "unknown", remote.Number, "Edit effect is not provable; compare the draft with GitHub before replay.")
}

// Inspect reads the remote effect of an uncertain operation without replaying it.
func Inspect(ctx context.Context, db *store.Store, projectID, requestKey string) (store.IssueOperation, error) {
	project, err := db.GetProjectSetup(ctx, projectID)
	if err != nil {
		return store.IssueOperation{}, err
	}
	op, err := db.GetIssueOperation(ctx, projectID, requestKey)
	if err != nil {
		return store.IssueOperation{}, err
	}
	if op.State == "pending" || op.State == "unknown" {
		return reconcile(ctx, db, project.Owner, project.Name, op)
	}
	return op, nil
}

func settle(db *store.Store, op store.IssueOperation, state string, number int, reason string) (store.IssueOperation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.SetIssueOperation(ctx, op.ID, state, number, reason); err != nil {
		return store.IssueOperation{}, err
	}
	op.State, op.ResultNumber, op.Reason = state, number, reason
	return op, nil
}
