// Package db provides SQLite bootstrap and migration support for AOM.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	migrationSchemaV1    = "schema-v1"
	migrationSchemaV2    = "schema-v2"
	migrationSchemaV3    = "schema-v3"
	migrationSchemaV4    = "schema-v4"
	migrationSchemaV5    = "schema-v5"
	migrationSchemaV6    = "schema-v6"
	migrationSchemaV7    = "schema-v7"
	migrationSchemaV8    = "schema-v8"
	defaultBusyTimeoutMS = 5000
)

// Open opens the SQLite database at the provided path and applies known migrations.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	// Pre-create the file with group-writable permissions (0664) so that sandboxed
	// runtimes (e.g. codex) running as a non-owner can still write to the DB.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o664)
		if err != nil {
			return nil, fmt.Errorf("create database file: %w", err)
		}
		_ = f.Close()
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if db.Driver() == nil {
		return nil, fmt.Errorf("open sqlite database: driver is not available")
	}

	return openAndMigrate(db)
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

	if _, err := db.Exec(fmt.Sprintf(`PRAGMA busy_timeout = %d`, defaultBusyTimeoutMS)); err != nil {
		return fmt.Errorf("configure sqlite busy timeout: %w", err)
	}

	// WAL mode allows concurrent readers while a writer holds the write lock,
	// eliminating most SQLITE_BUSY errors when operator runs parallel AOM commands.
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
