package sqlite

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

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

func TestMigrateEventSequencePreservesLegacyOrder(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}

	legacyMigrations := embeddedMigrations(t, "0001_schema_v1.sql", "0002_issue_type.sql", "0003_acceptance_criteria.sql", "0004_issue_external_key.sql")
	if err := Migrate(context.Background(), db, legacyMigrations); err != nil {
		t.Fatalf("apply legacy migrations: %v", err)
	}

	const sameSecond = "2026-07-13T20:00:00Z"
	if _, err := db.Exec(`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at) VALUES
		('event-z', NULL, 'legacy', 'event_z', '{}', ?),
		('event-a', NULL, 'legacy', 'event_a', '{}', ?),
		('event-next', NULL, 'legacy', 'event_next', '{}', '2026-07-13T20:00:01Z')`, sameSecond, sameSecond); err != nil {
		t.Fatal(err)
	}

	sequenceMigration := embeddedMigrations(t, "0005_event_sequence.sql")
	if err := Migrate(context.Background(), db, sequenceMigration); err != nil {
		t.Fatalf("apply event sequence migration: %v", err)
	}

	rows, err := db.Query(`SELECT sequence, id, event_type FROM events ORDER BY sequence`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var got []struct {
		sequence int64
		id       string
		event    string
	}
	for rows.Next() {
		var event struct {
			sequence int64
			id       string
			event    string
		}
		if err := rows.Scan(&event.sequence, &event.id, &event.event); err != nil {
			t.Fatal(err)
		}
		got = append(got, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := []struct {
		sequence int64
		id       string
		event    string
	}{
		{1, "event-a", "event_a"},
		{2, "event-z", "event_z"},
		{3, "event-next", "event_next"},
		{4, "", "event_ordering_enabled"},
	}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].sequence != want[i].sequence || got[i].event != want[i].event {
			t.Fatalf("event %d = %#v, want sequence=%d event=%q", i, got[i], want[i].sequence, want[i].event)
		}
		if want[i].id != "" && got[i].id != want[i].id {
			t.Fatalf("event %d id = %q, want %q", i, got[i].id, want[i].id)
		}
	}
}

func embeddedMigrations(t *testing.T, names ...string) fstest.MapFS {
	t.Helper()
	result := fstest.MapFS{}
	for _, name := range names {
		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatalf("read embedded migration %s: %v", name, err)
		}
		result[name] = &fstest.MapFile{Data: data}
	}
	return result
}
