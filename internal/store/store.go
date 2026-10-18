// Package store owns SQLite connections and deliberate schema upgrades.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/felixlheureux/outpost/internal/store/dbgen"
	"github.com/felixlheureux/outpost/migrations"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const SchemaVersion int64 = 11

type Store struct {
	DB      *sql.DB
	Queries *dbgen.Queries
}

// Open creates private local state, but never upgrades an existing schema.
func Open(ctx context.Context, stateDir string) (*Store, error) {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	info, err := os.Stat(stateDir)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("state directory must be private (chmod 700 %s)", stateDir)
	}
	path, err := filepath.Abs(filepath.Join(stateDir, "outpost.sqlite"))
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("database file must be private (chmod 600 %s)", path)
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	for _, pragma := range []string{"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(WAL)"} {
		q.Add("_pragma", pragma)
	}
	// Admission reads and writes in one short transaction. Acquire SQLite's
	// writer lock at BEGIN so concurrent connections wait instead of both
	// reading and then failing an attempted lock upgrade with SQLITE_BUSY.
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db, Queries: dbgen.New(db)}, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	p, err := goose.NewProvider(goose.DialectSQLite3, s.DB, migrations.Files)
	if err != nil {
		return err
	}
	current, err := p.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	if current > SchemaVersion {
		return fmt.Errorf("schema %d is newer than supported schema %d", current, SchemaVersion)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// CheckSchema is read-only: startup must not race a worker to migrate.
func (s *Store) CheckSchema(ctx context.Context) error {
	var exists int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'").Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("database is not initialized; run outpost migrate with the same --state-dir")
	}
	var version int64
	if err := s.DB.QueryRowContext(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied = 1 ORDER BY id DESC LIMIT 1").Scan(&version); err != nil {
		return err
	}
	if version != SchemaVersion {
		return fmt.Errorf("schema %d differs from supported schema %d; stop services and use the matching binary to migrate", version, SchemaVersion)
	}
	return nil
}
