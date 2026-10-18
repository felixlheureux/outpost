package projectsetup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/felixlheureux/outpost/internal/store"
)

func TestRealGitHubManagedCloneAndReplay(t *testing.T) {
	url := os.Getenv("OUTPOST_GH_REPO_TEST_URL")
	if url == "" {
		t.Skip("set OUTPOST_GH_REPO_TEST_URL for a real host GitHub clone")
	}
	if os.Getenv("OUTPOST_GH_QUEUE_TEST_WRITE") != "1" {
		t.Skip("set OUTPOST_GH_QUEUE_TEST_WRITE=1 for a disposable repository; setup may initialize its planning branch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	state := filepath.Join(t.TempDir(), "state")
	workspaces := filepath.Join(t.TempDir(), "workspaces")
	db, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := Setup(ctx, db, state, workspaces, url)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stage != "ready" || first.State != "ready" || !validClone(ctx, first.WorkspacePath, first.Owner, first.Name) {
		t.Fatalf("clone was not published: %+v", first)
	}
	observed, source, err := db.ListIssueSources(ctx, first.ID)
	if err != nil || source.State != "ready" || len(observed) == 0 {
		t.Fatalf("GitHub source was not observed: %d issues, %+v, %v", len(observed), source, err)
	}
	second, err := Setup(ctx, db, state, workspaces, strings.TrimSuffix(url, ".git")+".git")
	if err != nil || second.ID != first.ID || second.WorkspacePath != first.WorkspacePath {
		t.Fatalf("replay created another checkout: %+v %v", second, err)
	}
	otherDB, err := store.Open(ctx, filepath.Join(t.TempDir(), "existing-state"))
	if err != nil {
		t.Fatal(err)
	}
	defer otherDB.Close()
	if err := otherDB.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	registered, err := RegisterExisting(ctx, otherDB, first.WorkspacePath)
	if err != nil || registered.ID != first.ID || registered.WorkspacePath != first.WorkspacePath {
		t.Fatalf("existing checkout registration: %+v %v", registered, err)
	}
}
