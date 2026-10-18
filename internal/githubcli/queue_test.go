package githubcli

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureQueueReusesOrInitializesSource(t *testing.T) {
	dir := t.TempDir()
	content := base64.StdEncoding.EncodeToString([]byte("{\"version\":1,\"issues\":[7]}\n"))
	script := `#!/bin/sh
for arg in "$@"; do endpoint="$arg"; done
case "$endpoint" in
  *'/git/ref/heads/outpost/queue')
    if test "$QUEUE_MODE" = existing || test -f "$QUEUE_DIR/branch"; then printf '{"object":{"sha":"branchsha"}}'; else echo 'HTTP 404' >&2; exit 1; fi;;
  *'/git/ref/heads/main') printf '{"object":{"sha":"basesha"}}';;
  *'/git/refs')
    cat > "$QUEUE_DIR/branch-payload"
    touch "$QUEUE_DIR/branch"
    printf '{"ref":"refs/heads/outpost/queue"}';;
  *'/contents/queue.json?ref='*)
    if test "$QUEUE_MODE" = existing; then cat "$QUEUE_DIR/existing"; elif test -f "$QUEUE_DIR/file"; then cat "$QUEUE_DIR/created"; else echo 'HTTP 404' >&2; exit 1; fi;;
  *'/contents/queue.json')
    cat > "$QUEUE_DIR/file-payload"
    touch "$QUEUE_DIR/file"
    printf '{"content":{"sha":"createdsha"}}';;
  *) exit 1;;
esac
`
	for name, data := range map[string]string{"gh": script, "existing": fmt.Sprintf(`{"type":"file","encoding":"base64","sha":"existing","content":"%s"}`, content), "created": fmt.Sprintf(`{"type":"file","encoding":"base64","sha":"createdsha","content":"%s"}`, base64.StdEncoding.EncodeToString([]byte("{\"version\":1,\"issues\":[]}\n")))} {
		mode := os.FileMode(0600)
		if name == "gh" {
			mode = 0700
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("QUEUE_DIR", dir)
	t.Setenv("QUEUE_MODE", "existing")
	if err := EnsureQueue(context.Background(), "owner", "repo", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "branch")); !os.IsNotExist(err) {
		t.Fatal("existing branch was rewritten")
	}
	if _, err := os.Stat(filepath.Join(dir, "file")); !os.IsNotExist(err) {
		t.Fatal("existing file was rewritten")
	}
	t.Setenv("QUEUE_MODE", "missing")
	if err := EnsureQueue(context.Background(), "owner", "repo", "main"); err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(filepath.Join(dir, "branch-payload"))
	if err != nil || !strings.Contains(string(base), `"sha":"basesha"`) {
		t.Fatalf("base ref = %s, %v", base, err)
	}
	file, err := os.ReadFile(filepath.Join(dir, "file-payload"))
	if err != nil || !strings.Contains(string(file), `"branch":"outpost/queue"`) {
		t.Fatalf("file target = %s, %v", file, err)
	}
}
