// Package gitcli keeps native Git operations separate from GitHub metadata.
package gitcli

import (
	"context"
	"strings"

	"github.com/felixlheureux/outpost/internal/hostcmd"
)

func Version(ctx context.Context) (string, error) {
	b, err := hostcmd.Run(ctx, "", "git", nil, "--version")
	return strings.TrimSpace(string(b)), err
}

// Head reads an existing checkout. Clone/delivery operations belong to their later tasks.
func Head(ctx context.Context, dir string) (string, error) {
	b, err := hostcmd.Run(ctx, dir, "git", nil, "rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(string(b)), err
}

// Baseline captures the current branch and every staged, unstaged, and
// untracked path. Porcelain output is retained verbatim in the assignment.
func Baseline(ctx context.Context, dir string) (branch, status string, err error) {
	b, err := hostcmd.Run(ctx, dir, "git", nil, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", "", err
	}
	branch = strings.TrimSpace(string(b))
	b, err = hostcmd.Run(ctx, dir, "git", nil, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return "", "", err
	}
	return branch, string(b), nil
}

// ContainsCommit reports whether a verified predecessor commit is in HEAD's
// ancestry. A missing or unrelated commit is not usable dependency evidence.
func ContainsCommit(ctx context.Context, dir, predecessor string) bool {
	if predecessor == "" {
		return false
	}
	b, err := hostcmd.Run(ctx, dir, "git", nil, "merge-base", predecessor, "HEAD")
	return err == nil && strings.TrimSpace(string(b)) == predecessor
}
