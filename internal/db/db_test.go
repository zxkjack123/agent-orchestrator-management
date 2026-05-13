package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchemaV1(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	assertTableExists(t, db, "migrations")
	assertTableExists(t, db, "projects")
	assertTableExists(t, db, "agents")
	assertTableExists(t, db, "tasks")
	assertTableExists(t, db, "sessions")

	if count := migrationCount(t, db, migrationSchemaV1); count != 1 {
		t.Fatalf("migration count = %d, want 1", count)
	}
	if count := migrationCount(t, db, migrationSchemaV2); count != 1 {
		t.Fatalf("migration count = %d, want 1", count)
	}
	if count := migrationCount(t, db, migrationSchemaV3); count != 1 {
		t.Fatalf("migration count = %d, want 1", count)
	}
	if count := migrationCount(t, db, migrationSchemaV4); count != 1 {
		t.Fatalf("migration count = %d, want 1", count)
	}

	assertColumnExists(t, db, "sessions", "agent_name")
	assertColumnExists(t, db, "sessions", "role_name")
	assertColumnExists(t, db, "sessions", "repo_path")
	assertColumnExists(t, db, "sessions", "updated_at")
	assertColumnExists(t, db, "tasks", "preferred_role")
	assertColumnExists(t, db, "tasks", "preferred_agent")
	assertColumnExists(t, db, "tasks", "updated_at")
	assertTableExists(t, db, "steps")
	assertTableExists(t, db, "worktrees")
	assertBusyTimeout(t, db, defaultBusyTimeoutMS)
}

func TestOpenIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open failed: %v", err)
	}
	db.Close()

	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("second Open failed: %v", err)
	}
	defer db.Close()

	if count := migrationCount(t, db, migrationSchemaV1); count != 1 {
		t.Fatalf("migration count after reopen = %d, want 1", count)
	}
	if count := migrationCount(t, db, migrationSchemaV2); count != 1 {
		t.Fatalf("migration count after reopen = %d, want 1", count)
	}
	if count := migrationCount(t, db, migrationSchemaV3); count != 1 {
		t.Fatalf("migration count after reopen = %d, want 1", count)
	}
	if count := migrationCount(t, db, migrationSchemaV4); count != 1 {
		t.Fatalf("migration count after reopen = %d, want 1", count)
	}
	assertBusyTimeout(t, db, defaultBusyTimeoutMS)
}

func TestMigrateUpgradesSchemaV1DatabaseToLatest(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")

	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer rawDB.Close()

	if err := ensureMigrationsTable(rawDB); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	tx, err := rawDB.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := applySchemaV1(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("applySchemaV1 failed: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV1); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert migration schema-v1 failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := Migrate(rawDB); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if count := migrationCount(t, rawDB, migrationSchemaV1); count != 1 {
		t.Fatalf("migration schema-v1 count = %d, want 1", count)
	}
	if count := migrationCount(t, rawDB, migrationSchemaV2); count != 1 {
		t.Fatalf("migration schema-v2 count = %d, want 1", count)
	}
	if count := migrationCount(t, rawDB, migrationSchemaV3); count != 1 {
		t.Fatalf("migration schema-v3 count = %d, want 1", count)
	}
	if count := migrationCount(t, rawDB, migrationSchemaV4); count != 1 {
		t.Fatalf("migration schema-v4 count = %d, want 1", count)
	}

	assertColumnExists(t, rawDB, "sessions", "agent_name")
	assertColumnExists(t, rawDB, "sessions", "role_name")
	assertColumnExists(t, rawDB, "sessions", "repo_path")
	assertColumnExists(t, rawDB, "sessions", "updated_at")
	assertColumnExists(t, rawDB, "tasks", "preferred_role")
	assertColumnExists(t, rawDB, "tasks", "preferred_agent")
	assertColumnExists(t, rawDB, "tasks", "updated_at")
	assertTableExists(t, rawDB, "steps")
	assertTableExists(t, rawDB, "worktrees")
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()

	var count int
	err := db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master for %q failed: %v", table, err)
	}
	if count != 1 {
		t.Fatalf("table %q count = %d, want 1", table, count)
	}
}

func migrationCount(t *testing.T, db *sql.DB, id string) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM migrations WHERE id = ?`, id).Scan(&count); err != nil {
		t.Fatalf("count migration %q failed: %v", id, err)
	}

	return count
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%q) failed: %v", table, err)
	}
	defer rows.Close()

	var (
		cid        int
		name       string
		columnType string
		notNull    int
		defaultVal sql.NullString
		pk         int
	)

	for rows.Next() {
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			t.Fatalf("scan table_info row failed: %v", err)
		}
		if name == column {
			return
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info rows failed: %v", err)
	}

	t.Fatalf("column %q not found in table %q", column, table)
}

func assertBusyTimeout(t *testing.T, db *sql.DB, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&got); err != nil {
		t.Fatalf("PRAGMA busy_timeout failed: %v", err)
	}
	if got != want {
		t.Fatalf("busy_timeout = %d, want %d", got, want)
	}
}
