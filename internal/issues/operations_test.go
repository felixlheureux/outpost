package issues

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felixlheureux/outpost/internal/store"
)

func TestCreateUnknownReconcilesWithoutDuplicate(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	project, _, err := db.ReserveProject(ctx, 31, "owner", "repo", filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	script := `#!/bin/sh
last=''
method=''
for arg in "$@"; do
  last="$arg"
  if [ "$arg" = POST ]; then method=POST; fi
done
emit_issue() {
  printf '{"id":31,"node_id":"I_31","number":31,"html_url":"https://github.com/owner/repo/issues/31","state":"open","updated_at":"2026-10-17T12:00:00Z",'
  tail -c +2 "$GH_FIXTURE_FILE"
}
if [ "$method" = POST ]; then
  cat > "$GH_FIXTURE_FILE"
  printf x >> "$GH_FIXTURE_COUNT"
  exit 1
fi
case "$last" in
  *'?state='*) printf '['; emit_issue; printf ']';;
  *'/issues/31') emit_issue;;
  *) exit 1;;
esac
`
	if err := os.WriteFile(filepath.Join(fixture, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fixture+":"+os.Getenv("PATH"))
	t.Setenv("GH_FIXTURE_FILE", filepath.Join(fixture, "issue.json"))
	t.Setenv("GH_FIXTURE_COUNT", filepath.Join(fixture, "count"))
	input := Mutation{RequestKey: "create-123456", Title: "New task", Body: "Describe the outcome."}
	first, err := Mutate(ctx, db, project.ID, "create", input)
	if err != nil || first.State != "unknown" {
		t.Fatalf("first operation = %+v, %v", first, err)
	}
	second, err := Mutate(ctx, db, project.ID, "create", input)
	if err != nil || second.State != "confirmed" || second.ResultNumber != 31 || second.ID != first.ID {
		t.Fatalf("reconciled operation = %+v, %v", second, err)
	}
	count, err := os.ReadFile(filepath.Join(fixture, "count"))
	if err != nil || string(count) != "x" {
		t.Fatalf("create requests = %q, %v", count, err)
	}
	receipts, err := db.ListIssueOperations(ctx, project.ID)
	if err != nil || len(receipts) != 1 || receipts[0].State != "confirmed" {
		t.Fatalf("inspectable receipts = %+v, %v", receipts, err)
	}
	inspected, err := Inspect(ctx, db, project.ID, input.RequestKey)
	if err != nil || inspected.ResultNumber != 31 {
		t.Fatalf("receipt inspection = %+v, %v", inspected, err)
	}
	changed := input
	changed.Title = "Another task"
	if _, err := Mutate(ctx, db, project.ID, "create", changed); err == nil || !strings.Contains(err.Error(), "request key") {
		t.Fatalf("accepted changed request key payload: %v", err)
	}
}

func TestEditConflictKeepsDraftInOperation(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	project, _, err := db.ReserveProject(ctx, 31, "owner", "repo", filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	observation := store.IssueObservation{NodeID: "I_31", GithubID: 31, Number: 31, URL: "https://github.com/owner/repo/issues/31", Title: "Current", Body: "Changed", State: "open", SourceUpdatedAt: "2026-10-17T12:00:00Z", ContentDigest: Digest("Current", "Changed"), RequirementsJSON: "{}"}
	if err := db.ApplyIssueObservations(ctx, project.ID, []store.IssueObservation{observation}); err != nil {
		t.Fatal(err)
	}
	input := Mutation{RequestKey: "edit-123456", Title: "Desired", Body: "Keep this draft", IssueNumber: 31, ExpectedDigest: Digest("Old", "Body"), ExpectedRevision: 1}
	op, err := Mutate(ctx, db, project.ID, "edit", input)
	if err != nil || op.State != "conflict" {
		t.Fatalf("edit result = %+v, %v", op, err)
	}
	stored, err := db.GetIssueOperation(ctx, project.ID, input.RequestKey)
	if err != nil || stored.Body != input.Body || stored.State != "conflict" {
		t.Fatalf("stored draft = %+v, %v", stored, err)
	}
}

func TestCreatePermissionDenialHasReceipt(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	project, _, err := db.ReserveProject(ctx, 31, "owner", "repo", filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	script := "#!/bin/sh\nprintf 'HTTP 403: Resource not accessible by integration\\n' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(fixture, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fixture+":"+os.Getenv("PATH"))
	op, err := Mutate(ctx, db, project.ID, "create", Mutation{RequestKey: "create-denied", Title: "Task", Body: "Outcome"})
	if err != nil || op.State != "rejected" || !strings.Contains(op.Reason, "write permission") {
		t.Fatalf("permission receipt = %+v, %v", op, err)
	}
}
