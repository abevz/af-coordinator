package sqlite

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/abevz/af-coordinator/migrations"
	_ "modernc.org/sqlite"
)

// newTestDB creates a temp-file-backed SQLite database with the schema applied.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	f, err := os.CreateTemp("", "af-coordinator-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	f.Close()
	_ = os.Remove(dbPath) // SQLite will keep the file handle open after open

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); os.Remove(dbPath) })

	// Enable foreign keys and set pragmas.
	for _, p := range []string{
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(p); err != nil {
			t.Fatal(err)
		}
	}

	// Apply schema using real migrations.
	if err := Migrate(context.Background(), db, migrations.FS); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return db
}
