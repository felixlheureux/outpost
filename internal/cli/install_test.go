package cli

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpgradeRequiresMatchingBackupAndPreservesPriorBinary(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "backup")
	if err := os.Mkdir(backup, 0700); err != nil {
		t.Fatal(err)
	}
	opts := installOptions{
		stateDir: filepath.Join(root, "state"), configPath: filepath.Join(root, "config.json"),
		workspacesDir: filepath.Join(root, "workspaces"), backupDir: backup,
	}
	manifest, _ := json.Marshal(map[string]string{"state_dir": opts.stateDir, "config": opts.configPath, "workspaces_dir": opts.workspacesDir})
	if err := os.WriteFile(filepath.Join(backup, "manifest.json"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyUpgradeBackup(opts); err == nil {
		t.Fatal("accepted incomplete backup")
	}
	if err := os.Mkdir(filepath.Join(backup, "state"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"state/outpost.sqlite", "config.json"} {
		if err := os.WriteFile(filepath.Join(backup, path), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(backup, "workspaces"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := verifyUpgradeBackup(opts); err != nil {
		t.Fatal(err)
	}
	opts.stateDir = "different"
	if err := verifyUpgradeBackup(opts); err == nil {
		t.Fatal("accepted a backup for another state directory")
	}
	target := filepath.Join(root, "outpost")
	prior := []byte("prior binary")
	if err := os.WriteFile(target, prior, 0700); err != nil {
		t.Fatal(err)
	}
	if err := installBinary(target, false, backup); err == nil {
		t.Fatal("replaced a differing binary without upgrade")
	}
	if err := installBinary(target, true, backup); err != nil {
		t.Fatal(err)
	}
	gotPrior, err := os.ReadFile(filepath.Join(backup, "previous-outpost"))
	if err != nil || string(gotPrior) != string(prior) {
		t.Fatalf("preserved prior binary = %q, %v", gotPrior, err)
	}
	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil || sha256.Sum256(installed) != sha256.Sum256(source) {
		t.Fatalf("installed binary differs from source: %v", err)
	}
	if err := installBinary(target, true, backup); err != nil {
		t.Fatalf("idempotent replay failed: %v", err)
	}
}

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
