package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestIssueObservationPreservesRevisionAndCacheOnFailure(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	project, _, err := db.ReserveProject(ctx, 10, "owner", "repo", filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	issue := IssueObservation{NodeID: "I_1", GithubID: 20, Number: 1, URL: "https://github.com/owner/repo/issues/1", Title: "Task", Body: "Do it", State: "open", SourceUpdatedAt: "2026-10-18T00:00:00Z", ContentDigest: "digest-a", RequirementsJSON: "{}"}
	if err := db.ApplyIssueObservations(ctx, project.ID, []IssueObservation{issue}); err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyIssueObservations(ctx, project.ID, []IssueObservation{issue}); err != nil {
		t.Fatal(err)
	}
	issue.ContentDigest = "digest-b"
	issue.Title = "Task revised"
	if err := db.ApplyIssueObservations(ctx, project.ID, []IssueObservation{issue}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordIssueSourceFailure(ctx, project.ID, "GitHub temporarily unavailable"); err != nil {
		t.Fatal(err)
	}
	issues, status, err := db.ListIssueSources(ctx, project.ID)
	if err != nil || len(issues) != 1 || issues[0].LocalRevision != 2 || issues[0].Title != "Task revised" || status.State != "needs_attention" || status.LastSuccessAt == "" {
		t.Fatalf("cached observation = %+v, status = %+v, error = %v", issues, status, err)
	}
}
