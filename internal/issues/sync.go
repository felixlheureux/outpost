package issues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/hostcmd"
	"github.com/felixlheureux/outpost/internal/store"
)

// Sync reads GitHub first, then commits one source observation batch. Failed
// reads keep the prior projection and never imply an issue was deleted.
func Sync(ctx context.Context, db *store.Store, projectID string) (int, error) {
	project, err := db.GetProjectSetup(ctx, projectID)
	if err != nil {
		return 0, err
	}
	remote, err := githubcli.ListIssues(ctx, project.Owner, project.Name)
	if err != nil {
		message := "GitHub issue read unavailable; check host access and retry."
		var command *hostcmd.CommandError
		if errors.As(err, &command) {
			switch command.Kind {
			case "authentication_required":
				message = "GitHub login is unavailable to the Outpost service user; sign in with gh and retry."
			case "permission_denied":
				message = "GitHub issue read permission was denied; check repository access and retry."
			case "rate_limited":
				message = "GitHub rate limit reached; wait for it to reset, then retry."
			case "inaccessible_or_missing":
				message = "GitHub issues are inaccessible or missing; check the repository identity and host access."
			}
		}
		_ = db.RecordIssueSourceFailure(ctx, projectID, message)
		return 0, err
	}
	requirements := make(map[int]Requirements, len(remote))
	parseErrors := make(map[int]string)
	for _, issue := range remote {
		req, parseErr := Parse(issue.Number, issue.Title, issue.Body)
		if parseErr != nil {
			parseErrors[issue.Number] = parseErr.Error()
			continue
		}
		if policyErr := ValidateProfileReference(req); policyErr != nil {
			parseErrors[issue.Number] = policyErr.Error()
			continue
		}
		requirements[issue.Number] = req
	}
	for number, message := range ValidateDependencies(requirements) {
		parseErrors[number] = message
	}
	observations := make([]store.IssueObservation, 0, len(remote))
	for _, issue := range remote {
		requirementsJSON := "null"
		if req, found := requirements[issue.Number]; found {
			b, err := json.Marshal(req)
			if err != nil {
				return 0, err
			}
			requirementsJSON = string(b)
		}
		observations = append(observations, store.IssueObservation{
			NodeID: issue.NodeID, GithubID: issue.ID, Number: issue.Number, URL: issue.HTMLURL,
			Title: issue.Title, Body: issue.Body, State: issue.State, SourceUpdatedAt: issue.UpdatedAt,
			ContentDigest: Digest(issue.Title, issue.Body), RequirementsJSON: requirementsJSON,
			ParseError: parseErrors[issue.Number],
		})
	}
	if err := db.ApplyIssueObservations(ctx, projectID, observations); err != nil {
		return 0, fmt.Errorf("store GitHub issue observation: %w", err)
	}
	return len(remote), nil
}

func ValidateDependencies(requirements map[int]Requirements) map[int]string {
	problems := make(map[int]string)
	for number, req := range requirements {
		for _, dependency := range req.Settings.DependsOn {
			if _, found := requirements[dependency]; !found {
				problems[number] = fmt.Sprintf("dependency #%d is unavailable or invalid in this repository", dependency)
				break
			}
		}
	}
	color := make(map[int]int, len(requirements))
	var stack []int
	var walk func(int)
	walk = func(number int) {
		color[number] = 1
		stack = append(stack, number)
		for _, dependency := range requirements[number].Settings.DependsOn {
			if _, found := requirements[dependency]; !found {
				continue
			}
			if color[dependency] == 0 {
				walk(dependency)
			} else if color[dependency] == 1 {
				for index := len(stack) - 1; index >= 0; index-- {
					problems[stack[index]] = "dependency cycle in this repository"
					if stack[index] == dependency {
						break
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[number] = 2
	}
	for number := range requirements {
		if color[number] == 0 {
			walk(number)
		}
	}
	return problems
}
