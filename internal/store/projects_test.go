package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReserveProjectReusesGitHubIdentity(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "123")
	first, reused, err := db.ReserveProject(ctx, 123, "Owner", "Repo", path)
	if err != nil || reused {
		t.Fatalf("first reservation: %+v %v %v", first, reused, err)
	}
	second, reused, err := db.ReserveProject(ctx, 123, "owner", "repo", path)
	if err != nil || !reused || first.ID != second.ID {
		t.Fatalf("retry created another project: %+v %v %v", second, reused, err)
	}
	if _, _, err := db.ReserveExistingProject(ctx, 123, "Owner", "Repo", filepath.Join(t.TempDir(), "other")); err == nil {
		t.Fatal("registered repository was silently retargeted")
	}
	if _, _, err := db.ReserveExistingProject(ctx, 456, "Other", "Repo", path); err == nil {
		t.Fatal("registered checkout was assigned to a second repository")
	}
	if err := db.SetProjectStage(ctx, first.ID, "source_pending", "pending", "Issue and queue setup follows."); err != nil {
		t.Fatal(err)
	}
	current, err := db.GetProjectSetup(ctx, first.ID)
	if err != nil || current.Stage != "source_pending" {
		t.Fatalf("stage receipt: %+v %v", current, err)
	}
}
