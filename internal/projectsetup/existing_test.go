package projectsetup

import "testing"

func TestGitHubOriginVariants(t *testing.T) {
	for _, remote := range []string{
		"https://github.com/Owner/Repo.git",
		"git@github.com:Owner/Repo.git",
		"ssh://git@github.com/Owner/Repo.git",
	} {
		url, err := githubURLFromRemote(remote)
		if err != nil || url != "https://github.com/Owner/Repo" {
			t.Fatalf("origin %q: %q %v", remote, url, err)
		}
	}
	for _, remote := range []string{"https://evil.example/Owner/Repo", "https://github.com/Owner/Repo/extra", "https://github.com/user:token@github.com/Owner/Repo"} {
		if _, err := githubURLFromRemote(remote); err == nil {
			t.Fatalf("accepted unsafe origin %q", remote)
		}
	}
}
