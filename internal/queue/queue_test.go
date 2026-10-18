package queue

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felixlheureux/outpost/internal/store"
)

func TestManifestValidationAndMoves(t *testing.T) {
	for _, invalid := range []string{`{"version":2,"issues":[]}`, `{"version":1,"issues":[2,2]}`, `{"version":1,"issues":[0]}`, `{"version":1,"issues":[] ,"rank":1}`, `{"version":1,"issues":[1]} trailing`} {
		if _, err := Parse([]byte(invalid)); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
	current := []int{3, 4, 5}
	got, err := Apply(current, Edit{Kind: "move", Issue: 5, Before: 4})
	if err != nil || !equal(got, []int{3, 5, 4}) || !equal(current, []int{3, 4, 5}) {
		t.Fatalf("move = %v, %v", got, err)
	}
	got, err = Apply(current, Edit{Kind: "append", Issue: 4})
	if err != nil || !equal(got, current) {
		t.Fatalf("duplicate append = %v, %v", got, err)
	}
}

func TestStaleSHAAndVerifiedWrite(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p, _, err := db.ReserveProject(ctx, 12, "owner", "repo", filepath.Join(dir, "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	oldManifest, _ := Encode([]int{2})
	newManifest, _ := Encode([]int{2, 3})
	fixture := filepath.Join(dir, "fixture")
	if err := os.Mkdir(fixture, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"old": fmt.Sprintf(`{"type":"file","encoding":"base64","sha":"oldsha","content":"%s"}`, base64.StdEncoding.EncodeToString([]byte(oldManifest))),
		"new": fmt.Sprintf(`{"type":"file","encoding":"base64","sha":"newsha","content":"%s"}`, base64.StdEncoding.EncodeToString([]byte(newManifest))),
	} {
		if err := os.WriteFile(filepath.Join(fixture, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
for arg in "$@"; do endpoint="$arg"; done
case "$endpoint" in
  *'/contents/queue.json?ref='*)
    if test -f "$FIXTURE_DIR/marker"; then cat "$FIXTURE_DIR/new"; else cat "$FIXTURE_DIR/old"; fi;;
  *'/issues/2'|*'/issues/3')
    number="${endpoint##*/}"
    printf '{"id":%s,"node_id":"I_%s","number":%s,"title":"Task","body":"Outcome","html_url":"https://github.com/owner/repo/issues/%s","state":"open","updated_at":"2026-10-17T12:00:00Z"}' "$number" "$number" "$number" "$number";;
  *'/contents/queue.json')
    read -r payload
    printf '%s' "$payload" | sed -n 's/.*outpost queue operation \([a-f0-9]*\).*/\1/p' > "$FIXTURE_DIR/marker"
    if test "$LOSE_RESPONSE" = 1; then exit 1; fi
    printf '{"content":{"sha":"newsha"}}';;
  *'/commits?'*)
    printf '{"commit":{"message":"outpost queue operation %s"}}\n' "$(cat "$FIXTURE_DIR/marker")" | sed '1s/^/[/' | sed '$s/$/]/';;
  *) exit 1;;
esac
`
	if err := os.WriteFile(filepath.Join(fixture, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fixture+":"+os.Getenv("PATH"))
	t.Setenv("FIXTURE_DIR", fixture)
	conflict, err := Mutate(ctx, db, p.ID, Edit{RequestKey: "stale", ExpectedSHA: "stale-sha", Kind: "append", Issue: 3})
	if err != nil || conflict.State != "conflict" || conflict.ObservedJSON != "[2]" {
		t.Fatalf("conflict = %+v, %v", conflict, err)
	}
	if _, err := os.Stat(filepath.Join(fixture, "marker")); !os.IsNotExist(err) {
		t.Fatal("stale edit wrote the file")
	}
	t.Setenv("LOSE_RESPONSE", "1")
	uncertain, err := Mutate(ctx, db, p.ID, Edit{RequestKey: "lost", ExpectedSHA: "oldsha", Kind: "append", Issue: 3})
	if err != nil || uncertain.State != "unknown" {
		t.Fatalf("lost response = %+v, %v", uncertain, err)
	}
	confirmedAfterInspect, err := Inspect(ctx, db, p.ID, "lost")
	if err != nil || confirmedAfterInspect.State != "confirmed" {
		t.Fatalf("inspected write = %+v, %v", confirmedAfterInspect, err)
	}
	if err := os.Remove(filepath.Join(fixture, "marker")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOSE_RESPONSE", "0")
	confirmed, err := Mutate(ctx, db, p.ID, Edit{RequestKey: "fresh", ExpectedSHA: "oldsha", Kind: "append", Issue: 3})
	if err != nil || confirmed.State != "confirmed" || confirmed.ObservedJSON != "[2,3]" {
		t.Fatalf("write = %+v, %v", confirmed, err)
	}
	marker, err := os.ReadFile(filepath.Join(fixture, "marker"))
	if err != nil || strings.TrimSpace(string(marker)) != confirmed.ID {
		t.Fatalf("write marker = %q, %v", marker, err)
	}
}
