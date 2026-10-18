package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServeCutoverReceiptSurvivesRetry(t *testing.T) {
	dir := t.TempDir()
	prepared := serveCutover{Origin: "https://host.tailnet.ts.net", Previous: "http://127.0.0.1:9000", Next: "http://127.0.0.1:8080", State: "prepared"}
	if err := saveServeCutover(dir, prepared); err != nil {
		t.Fatal(err)
	}
	observed := prepared
	observed.State = "observed"
	if err := saveServeCutover(dir, observed); err != nil {
		t.Fatal(err)
	}
	got, err := readServeCutover(dir)
	if err != nil || got != observed {
		t.Fatalf("receipt = %+v, %v", got, err)
	}
	changed := observed
	changed.Previous = "http://127.0.0.1:7331"
	if err := saveServeCutover(dir, changed); err == nil {
		t.Fatal("accepted a different previous route")
	}
	info, err := os.Stat(filepath.Join(dir, "serve-cutover.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("receipt mode: %v %v", info, err)
	}
}

func TestServeCutoverReceiptReconcilesUnchangedPreviousRoute(t *testing.T) {
	dir := t.TempDir()
	failed := serveCutover{Origin: "https://host.tailnet.ts.net", Previous: "http://127.0.0.1:8080", Next: "http://127.0.0.1:7331", State: "prepared"}
	if err := saveServeCutover(dir, failed); err != nil {
		t.Fatal(err)
	}
	current := serveCutover{Origin: failed.Origin, Previous: failed.Previous, Next: failed.Previous, State: "observed"}
	if err := saveServeCutover(dir, current); err != nil {
		t.Fatal(err)
	}
	got, err := readServeCutover(dir)
	if err != nil || got != current {
		t.Fatalf("current receipt = %+v, %v", got, err)
	}
	archives, err := filepath.Glob(filepath.Join(dir, "serve-cutover-abandoned-*.json"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("archived receipts = %v, %v", archives, err)
	}
	b, err := os.ReadFile(archives[0])
	if err != nil || string(b) != "{\"origin\":\"https://host.tailnet.ts.net\",\"previous\":\"http://127.0.0.1:8080\",\"next\":\"http://127.0.0.1:7331\",\"state\":\"prepared\"}\n" {
		t.Fatalf("archived receipt = %q, %v", b, err)
	}
}
