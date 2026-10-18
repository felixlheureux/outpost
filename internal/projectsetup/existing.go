package projectsetup

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/store"
)

// RegisterExisting never changes an operator-owned checkout. It records its
// canonical identity and leaves any local work in place.
func RegisterExisting(ctx context.Context, db *store.Store, checkoutPath string) (store.ProjectSetup, error) {
	if !filepath.IsAbs(checkoutPath) {
		return store.ProjectSetup{}, fmt.Errorf("existing checkout path must be absolute")
	}
	canonical, err := filepath.EvalSymlinks(checkoutPath)
	if err != nil {
		return store.ProjectSetup{}, fmt.Errorf("existing checkout is unavailable")
	}
	root, err := gitOutput(ctx, canonical, "rev-parse", "--show-toplevel")
	if err != nil || root != canonical {
		return store.ProjectSetup{}, fmt.Errorf("select the root of an existing Git checkout")
	}
	remote, err := gitOutput(ctx, canonical, "remote", "get-url", "origin")
	if err != nil {
		return store.ProjectSetup{}, fmt.Errorf("existing checkout needs a GitHub origin remote")
	}
	url, err := githubURLFromRemote(remote)
	if err != nil {
		return store.ProjectSetup{}, err
	}
	repo, err := githubcli.GetRepository(ctx, url)
	if err != nil {
		return store.ProjectSetup{}, err
	}
	owner, name, err := githubcli.ParseURL("https://github.com/" + repo.FullName)
	if err != nil {
		return store.ProjectSetup{}, fmt.Errorf("GitHub returned an invalid repository identity")
	}
	project, _, err := db.ReserveExistingProject(ctx, repo.ID, owner, name, canonical)
	if err != nil {
		return store.ProjectSetup{}, err
	}
	dirty, err := gitOutput(ctx, canonical, "status", "--porcelain")
	if err != nil {
		return project, fmt.Errorf("could not inspect existing checkout changes")
	}
	stage, message := "source_pending", ""
	if dirty != "" {
		stage, message = "source_pending_dirty", "Existing checkout has local changes; inspect before execution."
	}
	if err := db.SetProjectStage(ctx, project.ID, stage, "pending", message); err != nil {
		return project, err
	}
	return db.GetProjectSetup(ctx, project.ID)
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Stderr = io.Discard
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}

func githubURLFromRemote(remote string) (string, error) {
	var path string
	for _, prefix := range []string{"https://github.com/", "git@github.com:", "ssh://git@github.com/"} {
		if strings.HasPrefix(remote, prefix) {
			path = strings.TrimPrefix(remote, prefix)
			break
		}
	}
	path = strings.TrimSuffix(path, ".git")
	url := "https://github.com/" + path
	if _, _, err := githubcli.ParseURL(url); err != nil {
		return "", fmt.Errorf("origin must identify a GitHub repository")
	}
	return url, nil
}
