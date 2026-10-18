package githubcli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Issue struct {
	ID          int64           `json:"id"`
	NodeID      string          `json:"node_id"`
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	HTMLURL     string          `json:"html_url"`
	State       string          `json:"state"`
	UpdatedAt   string          `json:"updated_at"`
	PullRequest json.RawMessage `json:"pull_request"`
}

// ListIssues reads a bounded, complete set through the host user's existing gh
// session. GitHub's issue listing includes pull requests, which are filtered.
func ListIssues(ctx context.Context, owner, name string) ([]Issue, error) {
	if !part.MatchString(owner) || !part.MatchString(name) {
		return nil, fmt.Errorf("invalid repository identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	issues := make([]Issue, 0)
	for page := 1; page <= 10; page++ {
		endpoint := fmt.Sprintf("repos/%s/%s/issues?state=all&per_page=50&page=%d", owner, name, page)
		b, err := readAPI(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("GitHub issue read failed on page %d: %w", page, err)
		}
		var batch []Issue
		if err := json.Unmarshal(b, &batch); err != nil {
			return nil, fmt.Errorf("decode GitHub issue page %d: %w", page, err)
		}
		if len(batch) > 50 {
			return nil, fmt.Errorf("GitHub returned an oversized issue page")
		}
		for _, issue := range batch {
			if len(issue.PullRequest) > 0 && string(issue.PullRequest) != "null" {
				continue
			}
			if err := validateIssue(issue, owner, name); err != nil {
				return nil, fmt.Errorf("issue page %d: %w", page, err)
			}
			issues = append(issues, issue)
		}
		if len(batch) < 50 {
			return issues, nil
		}
	}
	return nil, fmt.Errorf("repository has more than 500 issue and pull request records; narrow synchronization is required")
}

func validateIssue(issue Issue, owner, name string) error {
	if issue.ID <= 0 || issue.NodeID == "" || issue.Number <= 0 || len(issue.Title) > 500 || len(issue.Body) > 256<<10 || (issue.State != "open" && issue.State != "closed") {
		return fmt.Errorf("GitHub returned incomplete or oversized issue metadata")
	}
	if _, err := time.Parse(time.RFC3339, issue.UpdatedAt); err != nil {
		return fmt.Errorf("GitHub returned an invalid issue update time")
	}
	u, err := url.Parse(issue.HTMLURL)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.RawQuery != "" || u.Fragment != "" || !strings.EqualFold(u.Path, "/"+owner+"/"+name+"/issues/"+strconv.Itoa(issue.Number)) {
		return fmt.Errorf("GitHub returned an issue URL outside the selected repository")
	}
	return nil
}
