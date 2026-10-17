package gitcli

import (
	"context"
	"os/exec"
	"testing"
)

func TestReadRealGitHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git required")
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "--initial-branch=main"}, {"-c", "user.name=Outpost test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git fixture %v: %s", err, out)
		}
	}
	head, err := Head(context.Background(), dir)
	if err != nil || len(head) != 40 {
		t.Fatalf("head %q %v", head, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Head(ctx, dir); err == nil {
		t.Fatal("cancellation ignored")
	}
}
