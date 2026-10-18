// Package projectsetup owns managed checkout creation after a durable project reservation.
package projectsetup

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/felixlheureux/outpost/internal/githubcli"
	"github.com/felixlheureux/outpost/internal/issues"
	"github.com/felixlheureux/outpost/internal/store"
)

func Setup(ctx context.Context, db *store.Store, stateDir, workspacesDir, repositoryURL string) (store.ProjectSetup, error) {
	if !filepath.IsAbs(workspacesDir) || !filepath.IsAbs(stateDir) {
		return store.ProjectSetup{}, fmt.Errorf("managed state and workspace paths must be absolute")
	}
	repo, err := githubcli.GetRepository(ctx, repositoryURL)
	if err != nil {
		return store.ProjectSetup{}, err
	}
	owner, name, err := githubcli.ParseURL("https://github.com/" + repo.FullName)
	if err != nil {
		return store.ProjectSetup{}, fmt.Errorf("GitHub returned an invalid repository identity")
	}
	if err := os.MkdirAll(workspacesDir, 0700); err != nil {
		return store.ProjectSetup{}, fmt.Errorf("managed workspace directory is unavailable: %w", err)
	}
	root, err := filepath.EvalSymlinks(workspacesDir)
	if err != nil {
		return store.ProjectSetup{}, fmt.Errorf("managed workspace directory is unavailable: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return store.ProjectSetup{}, fmt.Errorf("managed workspace directory must be private")
	}
	final := filepath.Join(root, strconv.FormatInt(repo.ID, 10))
	project, _, err := db.ReserveProject(ctx, repo.ID, owner, name, final)
	if err != nil {
		return store.ProjectSetup{}, err
	}
	if project.WorkspacePath != final {
		return project, fmt.Errorf("registered checkout path differs from this installation; inspect it before setup")
	}
	lockDir := filepath.Join(stateDir, "locks")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return project, err
	}
	lock, err := os.OpenFile(filepath.Join(lockDir, project.ID+".setup.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return project, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK {
			return db.GetProjectSetup(ctx, project.ID)
		}
		return project, fmt.Errorf("acquire repository setup lock: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if exists(final) {
		if !validClone(ctx, final, owner, name) {
			return project, fmt.Errorf("managed checkout path exists but does not match the recorded repository")
		}
		return sourcePending(ctx, db, project)
	}
	stage := filepath.Join(root, ".setup-"+strconv.FormatInt(repo.ID, 10))
	if exists(stage) {
		if validClone(ctx, stage, owner, name) {
			if err := os.Rename(stage, final); err != nil {
				return project, err
			}
			return sourcePending(ctx, db, project)
		}
		// Preserve interrupted staging for inspection. This owned path is
		// quarantined only after acquiring the project's exclusive setup lock.
		archive := stage + ".interrupted-" + time.Now().UTC().Format("20060102T150405")
		if err := os.Rename(stage, archive); err != nil {
			return project, fmt.Errorf("interrupted clone needs inspection: %w", err)
		}
	}
	if err := db.SetProjectStage(ctx, project.ID, "cloning", "pending", ""); err != nil {
		return project, err
	}
	cloneCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cloneCtx, "gh", "repo", "clone", repo.FullName, stage, "--", "--quiet")
	cmd.Stdout, cmd.Stderr, cmd.Stdin = io.Discard, io.Discard, nil
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GIT_TERMINAL_PROMPT=0")
	if err := cmd.Run(); err != nil {
		stage := "needs_attention"
		message := "Clone failed; check the host GitHub/Git access and retry. Staging files were retained."
		if cloneCtx.Err() != nil {
			stage = "interrupted"
			message = "Clone was interrupted; retry will inspect the retained staging checkout."
		}
		settle, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = db.SetProjectStage(settle, project.ID, stage, "failed", message)
		return project, fmt.Errorf("%s", message)
	}
	if !validClone(ctx, stage, owner, name) {
		_ = db.SetProjectStage(ctx, project.ID, "needs_attention", "failed", "Clone identity could not be verified; inspect staging checkout.")
		return project, fmt.Errorf("clone identity could not be verified")
	}
	if err := os.Rename(stage, final); err != nil {
		return project, fmt.Errorf("publish managed checkout: %w", err)
	}
	return sourcePending(ctx, db, project)
}

func sourcePending(ctx context.Context, db *store.Store, project store.ProjectSetup) (store.ProjectSetup, error) {
	if project.State == "ready" {
		return project, nil
	}
	if err := db.SetProjectStage(ctx, project.ID, "source_syncing", "pending", ""); err != nil {
		return project, err
	}
	if _, err := issues.Sync(ctx, db, project.ID); err != nil {
		settle, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = db.SetProjectStage(settle, project.ID, "source_needs_attention", "failed", "Checkout retained. GitHub issue access needs attention; retry setup after checking host access.")
		return project, fmt.Errorf("GitHub issue source unavailable: %w", err)
	}
	if err := db.SetProjectStage(ctx, project.ID, "queue_pending", "pending", ""); err != nil {
		return project, err
	}
	return db.GetProjectSetup(ctx, project.ID)
}

func exists(path string) bool { _, err := os.Lstat(path); return err == nil }

func validClone(ctx context.Context, path, owner, name string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", path, "remote", "get-url", "origin")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	b, err := cmd.Output()
	if err != nil {
		return false
	}
	remote := strings.TrimSpace(string(b))
	identity := strings.ToLower(owner + "/" + name)
	return strings.EqualFold(strings.TrimSuffix(strings.TrimPrefix(remote, "https://github.com/"), ".git"), identity) ||
		strings.EqualFold(strings.TrimSuffix(strings.TrimPrefix(remote, "git@github.com:"), ".git"), identity) ||
		strings.EqualFold(strings.TrimSuffix(strings.TrimPrefix(remote, "ssh://git@github.com/"), ".git"), identity)
}
