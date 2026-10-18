package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
)

type Reporter interface {
	Comments(context.Context, string, string, int) ([]Comment, error)
	Post(context.Context, string, string, int, string) (Comment, error)
	Issue(context.Context, string, string, int) (githubcli.Issue, error)
	Close(context.Context, string, string, int) error
}
type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}
type GitHubReporter struct{}

func (GitHubReporter) Comments(ctx context.Context, owner, name string, n int) ([]Comment, error) {
	out := []Comment{}
	for page := 1; page <= 10; page++ {
		endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/comments?per_page=100&page=%d", owner, name, n, page)
		b, err := hostcmd.Run(ctx, "", "gh", nil, "api", "--hostname", "github.com", "--method", "GET", endpoint)
		if err != nil {
			return nil, err
		}
		var batch []Comment
		if err = json.Unmarshal(b, &batch); err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < 100 {
			return out, nil
		}
	}
	return nil, fmt.Errorf("issue has more than 1000 comments; report marker cannot be reconciled")
}
func (GitHubReporter) Post(ctx context.Context, owner, name string, n int, body string) (Comment, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, name, n)
	b, err := hostcmd.Run(ctx, "", "gh", nil, "api", "--hostname", "github.com", "--method", "POST", "-f", "body="+body, endpoint)
	if err != nil {
		return Comment{}, err
	}
	var c Comment
	if err = json.Unmarshal(b, &c); err != nil {
		return Comment{}, err
	}
	if c.ID <= 0 {
		return Comment{}, fmt.Errorf("GitHub did not return a comment identity")
	}
	return c, nil
}
func (GitHubReporter) Issue(ctx context.Context, owner, name string, n int) (githubcli.Issue, error) {
	return githubcli.GetIssue(ctx, owner, name, n)
}
func (GitHubReporter) Close(ctx context.Context, owner, name string, n int) error {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d", owner, name, n)
	_, err := hostcmd.Run(ctx, "", "gh", nil, "api", "--hostname", "github.com", "--method", "PATCH", "-f", "state=closed", endpoint)
	return err
}

// Report is independently retryable after local settlement. An unknown comment
// is searched by its durable marker before another POST is allowed.
func Report(ctx context.Context, db *store.Store, a store.Assignment, closeIssue bool, remote Reporter) (store.GitHubReport, error) {
	if remote == nil {
		remote = GitHubReporter{}
	}
	project, err := db.GetProjectSetup(ctx, a.ProjectID)
	if err != nil {
		return store.GitHubReport{}, err
	}
	delivery, err := db.GetDelivery(ctx, a.ID)
	if err != nil {
		return store.GitHubReport{}, err
	}
	if delivery.State != "settled" {
		return store.GitHubReport{}, fmt.Errorf("Git delivery is not settled")
	}
	r, err := db.BeginReport(ctx, a.ID, closeIssue)
	if err != nil {
		return r, err
	}
	if r.State == "settled" {
		return r, nil
	}
	if r.CommentID == 0 {
		comments, readErr := remote.Comments(ctx, project.Owner, project.Name, a.IssueNumber)
		if readErr != nil {
			_ = db.UpdateReport(ctx, r.ID, "comment_unknown", 0, "could not inspect issue comments")
			return r, readErr
		}
		for _, c := range comments {
			if strings.Contains(c.Body, r.Marker) {
				if r.CommentID != 0 && r.CommentID != c.ID {
					return r, fmt.Errorf("conflicting report marker")
				}
				r.CommentID = c.ID
			}
		}
		if r.CommentID == 0 {
			checks, checkErr := db.ListChecks(ctx, a.ID)
			if checkErr != nil {
				return r, checkErr
			}
			passed := []string{}
			for _, c := range checks {
				if c.Required && c.CandidateDigest == delivery.Candidate && (c.Status == "passed" || c.Status == "reused") {
					passed = append(passed, fmt.Sprintf("%s (%s)", c.Name, c.ID))
				}
			}
			commit := delivery.CommitSHA
			if commit == "" {
				commit = "local changes"
			}
			body := fmt.Sprintf("Outpost completed issue #%d.\n\nRun: %s\nAssignment: %s\nDelivery: %s\nBranch: %s\nCommit: %s\nRequired checks: %s\nLimitations: none recorded.\n\n%s", a.IssueNumber, a.RunID, a.ID, delivery.Policy, delivery.Branch, commit, strings.Join(passed, ", "), r.Marker)
			if err = db.UpdateReport(ctx, r.ID, "comment_unknown", 0, "comment acknowledgement pending reconciliation"); err != nil {
				return r, err
			}
			comment, postErr := remote.Post(ctx, project.Owner, project.Name, a.IssueNumber, body)
			if postErr != nil {
				return r, postErr
			}
			r.CommentID = comment.ID
		}
		if err = db.UpdateReport(ctx, r.ID, "close_pending", r.CommentID, ""); err != nil {
			return r, err
		}
	}
	if !r.CloseIssue {
		if err = db.UpdateReport(ctx, r.ID, "settled", r.CommentID, ""); err != nil {
			return r, err
		}
		return db.GetReport(ctx, a.ID)
	}
	issue, err := remote.Issue(ctx, project.Owner, project.Name, a.IssueNumber)
	if err != nil {
		_ = db.UpdateReport(ctx, r.ID, "close_unknown", r.CommentID, "could not inspect issue state")
		return r, err
	}
	if issue.State != "closed" {
		if err = db.UpdateReport(ctx, r.ID, "close_unknown", r.CommentID, "close acknowledgement pending reconciliation"); err != nil {
			return r, err
		}
		if err = remote.Close(ctx, project.Owner, project.Name, a.IssueNumber); err != nil {
			return r, err
		}
		issue, err = remote.Issue(ctx, project.Owner, project.Name, a.IssueNumber)
		if err != nil {
			return r, err
		}
		if issue.State != "closed" {
			return r, fmt.Errorf("GitHub issue is not closed after close request")
		}
	}
	if err = db.UpdateReport(ctx, r.ID, "settled", r.CommentID, ""); err != nil {
		return r, err
	}
	return db.GetReport(ctx, a.ID)
}
