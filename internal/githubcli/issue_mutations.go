package githubcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/felixlheureux/outpost/internal/hostcmd"
)

func GetIssue(ctx context.Context, owner, name string, number int) (Issue, error) {
	if !part.MatchString(owner) || !part.MatchString(name) || number <= 0 {
		return Issue{}, fmt.Errorf("invalid issue identity")
	}
	endpoint := "repos/" + owner + "/" + name + "/issues/" + strconv.Itoa(number)
	b, err := readAPI(ctx, endpoint)
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(b, &issue); err != nil {
		return Issue{}, fmt.Errorf("decode GitHub issue: %w", err)
	}
	if err := validateIssue(issue, owner, name); err != nil {
		return Issue{}, err
	}
	if issue.Number != number || len(issue.PullRequest) > 0 && string(issue.PullRequest) != "null" {
		return Issue{}, fmt.Errorf("GitHub returned a different issue identity")
	}
	return issue, nil
}

func WriteIssue(ctx context.Context, owner, name string, number int, title, body string) (Issue, error) {
	if !part.MatchString(owner) || !part.MatchString(name) || title == "" || len(title) > 500 || len(body) > 256<<10 || number < 0 {
		return Issue{}, fmt.Errorf("invalid issue mutation")
	}
	endpoint := "repos/" + owner + "/" + name + "/issues"
	method := "POST"
	if number != 0 {
		endpoint += "/" + strconv.Itoa(number)
		method = "PATCH"
	}
	payload, err := json.Marshal(map[string]string{"title": title, "body": body})
	if err != nil {
		return Issue{}, err
	}
	b, err := hostcmd.Run(ctx, "", "gh", bytes.NewReader(payload), "api", "--hostname", "github.com", "--method", method, "--input", "-", endpoint)
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(b, &issue); err != nil {
		return Issue{}, fmt.Errorf("decode GitHub issue mutation: %w", err)
	}
	if err := validateIssue(issue, owner, name); err != nil {
		return Issue{}, err
	}
	if number != 0 && issue.Number != number {
		return Issue{}, fmt.Errorf("GitHub returned a different issue identity")
	}
	return issue, nil
}

func FindIssueMarker(ctx context.Context, owner, name, marker string) (Issue, bool, error) {
	if !strings.HasPrefix(marker, "<!-- outpost-issue-operation:") || len(marker) > 100 {
		return Issue{}, false, fmt.Errorf("invalid operation marker")
	}
	list, err := ListIssues(ctx, owner, name)
	if err != nil {
		return Issue{}, false, err
	}
	var match Issue
	found := false
	for _, issue := range list {
		if strings.Contains(issue.Body, marker) {
			if found {
				return Issue{}, false, fmt.Errorf("operation marker occurs in multiple issues")
			}
			match, found = issue, true
		}
	}
	return match, found, nil
}
