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
