// Package sqlite implements SQLite-backed persistence for the coordinator.
package sqlite

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) a SQLite database at path and sets required pragmas.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Set required runtime pragmas.
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}

	return db, nil
}

// Migrate applies embedded SQL migration files that have not yet been applied.
// Migrations are sorted lexicographically and applied in a single transaction
// each. Applied migrations are tracked in the _migrations table.
func Migrate(db *sql.DB, migrationsFS fs.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		name       text primary key,
		applied_at text not null
	)`); err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	entries, err := fs.Glob(migrationsFS, "*.sql")
	if err != nil {
		return fmt.Errorf("list migration files: %w", err)
	}
	sort.Strings(entries)

	for _, name := range entries {
		var already int
		if err := db.QueryRow("SELECT count(*) FROM _migrations WHERE name = ?", name).Scan(&already); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if already > 0 {
			continue
		}

		data, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := db.Exec("INSERT INTO _migrations (name, applied_at) VALUES (?, ?)", name, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	return nil
}
