package githubcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestValidateIssueIdentityAndBounds(t *testing.T) {
	issue := Issue{ID: 10, NodeID: "I_123", Number: 4, Title: "Task", Body: "Outcome", HTMLURL: "https://github.com/Owner/Repo/issues/4", State: "open", UpdatedAt: "2026-10-17T12:00:00Z"}
	if err := validateIssue(issue, "owner", "repo"); err != nil {
		t.Fatal(err)
	}
	changed := issue
	changed.HTMLURL = "https://github.com/other/repo/issues/4"
	if err := validateIssue(changed, "owner", "repo"); err == nil {
		t.Fatal("accepted an issue outside the selected repository")
	}
	changed = issue
	changed.Body = string(make([]byte, (256<<10)+1))
	if err := validateIssue(changed, "owner", "repo"); err == nil {
		t.Fatal("accepted an oversized issue body")
	}
}

func TestListIssuesPaginatesAndExcludesPullRequests(t *testing.T) {
	fixture := t.TempDir()
	makeIssue := func(number int) Issue {
		return Issue{ID: int64(number), NodeID: "I_" + strconv.Itoa(number), Number: number,
			Title: "Task", Body: "Outcome", HTMLURL: "https://github.com/owner/repo/issues/" + strconv.Itoa(number),
			State: "open", UpdatedAt: "2026-10-17T12:00:00Z"}
	}
	page1 := make([]Issue, 50)
	for i := range page1 {
		page1[i] = makeIssue(i + 1)
	}
	page1[49].PullRequest = json.RawMessage(`{"url":"https://api.github.com/repos/owner/repo/pulls/50"}`)
	for name, data := range map[string]any{"page1": page1, "page2": []Issue{makeIssue(51)}} {
		b, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fixture, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
for arg in "$@"; do last="$arg"; done
case "$last" in
  *'page=1') cat "$GH_FIXTURE_PAGE1";;
  *'page=2') cat "$GH_FIXTURE_PAGE2";;
  *) exit 1;;
esac
`
	if err := os.WriteFile(filepath.Join(fixture, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fixture+":"+os.Getenv("PATH"))
	t.Setenv("GH_FIXTURE_PAGE1", filepath.Join(fixture, "page1"))
	t.Setenv("GH_FIXTURE_PAGE2", filepath.Join(fixture, "page2"))
	issues, err := ListIssues(context.Background(), "owner", "repo")
	if err != nil || len(issues) != 50 || issues[49].Number != 51 {
		t.Fatalf("listed issues = %d, last = %+v, error = %v", len(issues), issues, err)
	}
}
