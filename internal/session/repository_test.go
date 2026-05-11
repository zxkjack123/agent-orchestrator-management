package session

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/db"
)

func TestRepositoryUpsertAndListByProjectID(t *testing.T) {
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	now := time.Now().UTC().Truncate(time.Second)

	record := Record{
		ID:              "SESS-100",
		ProjectID:       "my-app",
		AgentID:         "my-app:backend-main",
		AgentName:       "backend-main",
		RoleName:        "backend",
		Runtime:         "codex",
		Status:          "Created",
		RepoPath:        "C:/repo",
		WorktreePath:    "C:/repo",
		TmuxSessionName: "aom-my-app",
		TmuxWindow:      "@1",
		TmuxPane:        "%2",
		LastSeenAt:      &now,
	}

	if err := repo.Upsert(record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	loaded, err := repo.GetByID("SESS-100")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("GetByID returned nil record")
	}
	if loaded.AgentName != "backend-main" {
		t.Fatalf("AgentName = %q, want %q", loaded.AgentName, "backend-main")
	}
	if loaded.TmuxPane != "%2" {
		t.Fatalf("TmuxPane = %q, want %q", loaded.TmuxPane, "%2")
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set")
	}

	sessions, err := repo.ListByProjectID("my-app")
	if err != nil {
		t.Fatalf("ListByProjectID failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("session count = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "SESS-100" {
		t.Fatalf("session ID = %q, want %q", sessions[0].ID, "SESS-100")
	}
}

func TestRepositoryUpsertUpdatesExistingRecord(t *testing.T) {
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	record := Record{
		ID:        "SESS-200",
		ProjectID: "my-app",
		AgentName: "backend-main",
		RoleName:  "backend",
		Runtime:   "codex",
		Status:    "Created",
		RepoPath:  "C:/repo",
	}

	if err := repo.Upsert(record); err != nil {
		t.Fatalf("first Upsert failed: %v", err)
	}

	record.Status = "Working"
	record.TmuxSessionName = "aom-my-app"
	record.TmuxPane = "%10"
	if err := repo.Upsert(record); err != nil {
		t.Fatalf("second Upsert failed: %v", err)
	}

	loaded, err := repo.GetByID("SESS-200")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("GetByID returned nil record")
	}
	if loaded.Status != "Working" {
		t.Fatalf("Status = %q, want %q", loaded.Status, "Working")
	}
	if loaded.TmuxPane != "%10" {
		t.Fatalf("TmuxPane = %q, want %q", loaded.TmuxPane, "%10")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}

	return sqlDB
}
