package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CheckSchema(ctx); err == nil {
		t.Fatal("startup accepted an unmigrated database")
	}
	var count int
	if err := db.DB.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='goose_db_version'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("startup mutated migration state")
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal("idempotent upgrade:", err)
	}
	if err := db.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}
	projects, err := db.Queries.ListProjects(ctx)
	if err != nil || len(projects) != 0 {
		t.Fatalf("empty projects: %v %v", projects, err)
	}
	for name, want := range map[string]int{"foreign_keys": 1, "busy_timeout": 5000} {
		var got int
		if err := db.DB.QueryRow("PRAGMA " + name).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s = %d", name, got)
		}
	}
	if _, err := db.DB.Exec("INSERT INTO goose_db_version(version_id,is_applied) VALUES(999,1)"); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckSchema(ctx); err == nil {
		t.Fatal("startup accepted future schema")
	}
	if err := db.Migrate(ctx); err == nil {
		t.Fatal("migration accepted future schema")
	}
}

func TestRejectPublicState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "public")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), dir); err == nil {
		t.Fatal("accepted public state directory")
	}
}
