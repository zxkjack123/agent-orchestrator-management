// Package db provides SQLite bootstrap and migration support for AOM.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	migrationSchemaV1 = "schema-v1"
	migrationSchemaV2 = "schema-v2"
	migrationSchemaV3 = "schema-v3"
	migrationSchemaV4 = "schema-v4"
	migrationSchemaV5 = "schema-v5"
	migrationSchemaV6 = "schema-v6"
	migrationSchemaV7 = "schema-v7"
	migrationSchemaV8  = "schema-v8"
	migrationSchemaV9  = "schema-v9"
	migrationSchemaV10 = "schema-v10"

	// defaultBusyTimeoutMS is the time (ms) SQLite will retry a write before
	// returning SQLITE_BUSY. 30 s gives ample headroom for concurrent CLI
	// invocations under normal load. Pair with _txlock=immediate so that
	// busy_timeout fires at BEGIN rather than mid-transaction.
	defaultBusyTimeoutMS = 30000
)

// Open opens the SQLite database at the provided path and applies known migrations.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	// Pre-create the DB file and WAL companion files with group-writable
	// permissions (0664). The companion files (-wal, -shm) must exist with
	// correct permissions before WAL mode is activated; if SQLite creates them
	// on first open it uses the process umask (typically 0644) which causes
	// SQLITE_CANTOPEN (14) for other concurrent processes trying to write to
	// the shared memory file.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		p := path + suffix
		if _, err := os.Stat(p); os.IsNotExist(err) {
			f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o664)
			if err != nil {
				if suffix == "" {
					return nil, fmt.Errorf("create database file: %w", err)
				}
				// WAL/SHM companion creation failures are non-fatal;
				// SQLite will create them on first open.
			} else {
				_ = f.Close()
			}
		}
	}

	// Retry openAndMigrate on transient SQLite errors:
	//
	//   SQLITE_CANTOPEN (14): race on WAL/SHM file creation at first open.
	//   SQLITE_BUSY (5): another process holds the write lock before our
	//     busy_timeout PRAGMA is applied. Retrying opens a new connection
	//     with the DSN-level _busy_timeout already active.
	//   SQLITE_BUSY_SNAPSHOT (261): stale WAL snapshot — retrying forces a
	//     fresh snapshot and clears the condition in SQLite < 3.37.
	//
	// Five attempts with 100/200/400/800/1600 ms exponential backoff cover
	// any realistic parallel-CLI race window (total max wait ≈ 3 s).
	//
	// IMPORTANT: openAndMigrate calls db.Close() on failure, so we must open
	// a FRESH *sql.DB on every retry — reusing a closed db produces
	// "sql: database is closed" on the very next Ping() call.
	dsn := "file:" + path + "?_txlock=immediate&_busy_timeout=30000"
	const maxRetries = 5
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond)
		}
		// Open a fresh connection on each attempt; openAndMigrate closes it on
		// failure so we cannot reuse the same *sql.DB across retries.
		retryDB, openErr := sql.Open("sqlite", dsn)
		if openErr != nil {
			lastErr = fmt.Errorf("open sqlite database: %w", openErr)
			continue
		}
		if retryDB.Driver() == nil {
			_ = retryDB.Close()
			lastErr = fmt.Errorf("open sqlite database: driver is not available")
			continue
		}
		result, migrateErr := openAndMigrate(retryDB)
		if migrateErr == nil {
			return result, nil
		}
		if !isSQLiteCantopenError(migrateErr) && !isSQLiteBusyError(migrateErr) {
			return nil, migrateErr
		}
		lastErr = migrateErr
	}
	return nil, lastErr
}

// isSQLiteCantopenError reports whether err is a SQLite CANTOPEN error (code 14).
// This error occurs when concurrent processes race to create the WAL/SHM files
// on first open. modernc.org/sqlite surfaces it as a string containing "(14)".
func isSQLiteCantopenError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "unable to open database file") ||
		strings.Contains(err.Error(), "(14)")
}

// isSQLiteBusyError reports whether err is a transient SQLite contention error
// that is safe to retry. Two codes are handled:
//
//   - SQLITE_BUSY (5)         — another writer holds the lock; busy_timeout will
//     normally gate this, but application-level retry provides a safety net when
//     the C-level timeout is not yet active (e.g. before the first PRAGMA runs).
//
//   - SQLITE_BUSY_SNAPSHOT (261) — WAL-mode "stale snapshot" error that arises
//     when a reader tries to write while another connection already committed a
//     newer WAL snapshot. busy_timeout does NOT gate this error in SQLite < 3.37;
//     the only reliable fix is to retry so the connection opens a fresh snapshot.
func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// modernc.org/sqlite surfaces error codes as "(N)" in the error string.
	return strings.Contains(msg, "(5)") ||
		strings.Contains(msg, "(261)") ||
		strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "SQLITE_BUSY")
}

func openAndMigrate(db *sql.DB) (*sql.DB, error) {
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if err := configureConnection(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func configureConnection(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("configure sqlite connection: db is required")
	}

	// Limit the connection pool to a single connection within one process.
	// Combined with _txlock=immediate in the DSN, this eliminates in-process
	// write contention: only one goroutine holds the connection at a time, and
	// when it starts a transaction the write lock is claimed immediately so
	// busy_timeout can gate any cross-process contention cleanly.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(fmt.Sprintf(`PRAGMA busy_timeout = %d`, defaultBusyTimeoutMS)); err != nil {
		return fmt.Errorf("configure sqlite busy timeout: %w", err)
	}

	// WAL mode allows concurrent readers while a writer holds the write lock.
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		return fmt.Errorf("configure sqlite WAL mode: %w", err)
	}

	// Synchronous=NORMAL is safe with WAL and significantly reduces fsync overhead.
	if _, err := db.Exec(`PRAGMA synchronous = NORMAL`); err != nil {
		return fmt.Errorf("configure sqlite synchronous mode: %w", err)
	}

	return nil
}

// Migrate applies known schema migrations to the database.
func Migrate(db *sql.DB) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	applied, err := hasMigration(db, migrationSchemaV1)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV1, err)
	}
	if applied {
	} else {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}

		if err := applySchemaV1(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v1: %w", err)
		}

		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV1); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV1, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV2)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV2, err)
	}
	if applied {
	} else {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}

		if err := applySchemaV2(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v2: %w", err)
		}

		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV2); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV2, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV3)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV3, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV3(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v3: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV3); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV3, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV4)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV4, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV4(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v4: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV4); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV4, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV5)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV5, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV5(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v5: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV5); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV5, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV6)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV6, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV6(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v6: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV6); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV6, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV7)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV7, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV7(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v7: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV7); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV7, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV8)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV8, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV8(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v8: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV8); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV8, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV9)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV9, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV9(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v9: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV9); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV9, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	applied, err = hasMigration(db, migrationSchemaV10)
	if err != nil {
		return fmt.Errorf("check migration %q: %w", migrationSchemaV10, err)
	}
	if !applied {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration transaction: %w", err)
		}
		if err := applySchemaV10(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema v10: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO migrations (id) VALUES (?)`, migrationSchemaV10); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", migrationSchemaV10, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration transaction: %w", err)
		}
	}

	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS migrations (
	id TEXT PRIMARY KEY,
	applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
	return err
}

func hasMigration(db *sql.DB, id string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM migrations WHERE id = ?`, id).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

func applySchemaV1(tx *sql.Tx) error {
	stmts := []string{
		`
CREATE TABLE projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	repo_path TEXT NOT NULL,
	default_branch TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`,
		`
CREATE TABLE agents (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	name TEXT NOT NULL,
	runtime TEXT NOT NULL,
	role TEXT NOT NULL,
	enabled INTEGER NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(id)
);
`,
		`
CREATE TABLE tasks (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	title TEXT NOT NULL,
	mode TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(id)
);
`,
		`
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	agent_id TEXT,
	task_id TEXT,
	runtime TEXT NOT NULL,
	status TEXT NOT NULL,
	worktree_path TEXT NOT NULL DEFAULT '',
	tmux_session_name TEXT NOT NULL DEFAULT '',
	tmux_window TEXT NOT NULL DEFAULT '',
	tmux_pane TEXT NOT NULL DEFAULT '',
	vendor_session_id TEXT NOT NULL DEFAULT '',
	last_seen_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(id),
	FOREIGN KEY(agent_id) REFERENCES agents(id),
	FOREIGN KEY(task_id) REFERENCES tasks(id)
);
`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func applySchemaV2(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE sessions ADD COLUMN agent_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN role_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN repo_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func applySchemaV3(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE tasks ADD COLUMN preferred_role TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tasks ADD COLUMN preferred_agent TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tasks ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`,
		`
CREATE TABLE steps (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	step_type TEXT NOT NULL,
	title TEXT NOT NULL,
	status TEXT NOT NULL,
	role_name TEXT NOT NULL DEFAULT '',
	agent_name TEXT NOT NULL DEFAULT '',
	dependencies TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(id),
	FOREIGN KEY(task_id) REFERENCES tasks(id)
);
`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func applySchemaV7(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE sessions ADD COLUMN model TEXT NOT NULL DEFAULT ''`)
	return err
}

func applySchemaV8(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE tasks ADD COLUMN description TEXT NOT NULL DEFAULT ''`)
	return err
}

func applySchemaV6(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE agents ADD COLUMN model TEXT NOT NULL DEFAULT ''`)
	return err
}

func applySchemaV5(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE tasks ADD COLUMN priority INTEGER NOT NULL DEFAULT 0`,
		`
CREATE TABLE task_dependencies (
	dependent_task_id TEXT NOT NULL,
	blocking_task_id  TEXT NOT NULL,
	created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (dependent_task_id, blocking_task_id),
	FOREIGN KEY(dependent_task_id) REFERENCES tasks(id),
	FOREIGN KEY(blocking_task_id)  REFERENCES tasks(id)
);
`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func applySchemaV4(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE worktrees (
	task_id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	status TEXT NOT NULL,
	base_branch TEXT NOT NULL,
	branch_name TEXT NOT NULL,
	worktree_path TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(id),
	FOREIGN KEY(task_id) REFERENCES tasks(id)
);
`)
	return err
}

func applySchemaV9(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE agents ADD COLUMN workspace_path TEXT NOT NULL DEFAULT ''`)
	return err
}

func applySchemaV10(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE sessions ADD COLUMN persistent INTEGER NOT NULL DEFAULT 0`)
	return err
}
