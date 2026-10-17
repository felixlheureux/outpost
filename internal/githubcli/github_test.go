package githubcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	for _, url := range []string{"https://github.com/felixlheureux/outpost", " https://github.com/felixlheureux/outpost.git/ "} {
		owner, name, err := ParseURL(url)
		if err != nil || owner != "felixlheureux" || name != "outpost" {
			t.Fatalf("%q: %s/%s %v", url, owner, name, err)
		}
	}
	for _, url := range []string{"https://github.com/owner/repo/issues", "https://github.com/owner/..", "https://token@github.com/owner/repo", "http://github.com/owner/repo", "https://github.com.evil.test/a/b", "https://github.com:443/a/b", "https://github.com/a/%2e%2e", "https://github.com/a/b?q=x", "git@github.com:a/b.git"} {
		if _, _, err := ParseURL(url); err == nil {
			t.Fatalf("accepted %q", url)
		}
	}
}

func TestGHBoundary(t *testing.T) {
	dir := t.TempDir()
	// A fixture executable verifies argv; this does not prove actual GitHub account access.
	script := "#!/bin/sh\nif [ \"$*\" != \"api --hostname github.com --method GET repos/owner/repo\" ]; then exit 9; fi\nprintf '%s' '{\"id\":42,\"full_name\":\"owner/repo\",\"default_branch\":\"main\",\"private\":true}'\n"
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	repo, err := GetRepository(context.Background(), "https://github.com/owner/repo")
	if err != nil || repo.ID != 42 || !repo.Private {
		t.Fatalf("%+v %v", repo, err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s' 'fake-secret-from-tool' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err = GetRepository(context.Background(), "https://github.com/owner/repo")
	if err == nil || strings.Contains(err.Error(), "fake-secret") {
		t.Fatalf("unsafe tool error %v", err)
	}
}
