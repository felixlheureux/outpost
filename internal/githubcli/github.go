// Package githubcli reuses the signed-in host user's gh credentials. No token is copied into Outpost.
package githubcli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/felixlheureux/outpost/internal/hostcmd"
)

var part = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

type Repository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// ParseURL deliberately supports github.com HTTPS links only in the first release.
func ParseURL(raw string) (owner, name string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return "", "", fmt.Errorf("use a repository URL such as https://github.com/owner/repo")
	}
	parts := strings.Split(strings.TrimSuffix(u.Path, "/"), "/")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("URL must identify one GitHub repository")
	}
	owner, name = parts[1], strings.TrimSuffix(parts[2], ".git")
	if !part.MatchString(owner) || !part.MatchString(name) || name == "." || name == ".." {
		return "", "", fmt.Errorf("invalid repository name")
	}
	return owner, name, nil
}

func CheckAuth(ctx context.Context) error {
	// gh stores authentication in its own credential store; never call `gh auth token`.
	_, err := hostcmd.Run(ctx, "", "gh", nil, "auth", "status", "--hostname", "github.com", "--active")
	return err
}

func GetRepository(ctx context.Context, rawURL string) (Repository, error) {
	owner, name, err := ParseURL(rawURL)
	if err != nil {
		return Repository{}, err
	}
	b, err := hostcmd.Run(ctx, "", "gh", nil, "api", "--hostname", "github.com", "--method", "GET", "repos/"+owner+"/"+name)
	if err != nil {
		return Repository{}, err
	}
	var repo Repository
	if err := json.Unmarshal(b, &repo); err != nil {
		return Repository{}, fmt.Errorf("decode GitHub repository response: %w", err)
	}
	if repo.ID == 0 || repo.FullName == "" {
		return Repository{}, fmt.Errorf("GitHub returned incomplete repository metadata")
	}
	return repo, nil
}
