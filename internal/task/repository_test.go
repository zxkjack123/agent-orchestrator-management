package task

import (
	"path/filepath"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/db"
)

func TestRepositoryUpsertGetAndCount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	record := Record{
		ID:             "TASK-001",
		ProjectID:      "proj-1",
		Title:          "Implement Milestone 3",
		Mode:           "Direct",
		Status:         "Planned",
		PreferredRole:  "backend",
		PreferredAgent: "backend-main",
	}

	if err := repo.Upsert(record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	got, err := repo.GetByID("TASK-001")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil record")
	}
	if got.Title != record.Title {
		t.Fatalf("Title = %q, want %q", got.Title, record.Title)
	}
	if got.PreferredAgent != record.PreferredAgent {
		t.Fatalf("PreferredAgent = %q, want %q", got.PreferredAgent, record.PreferredAgent)
	}

	count, err := repo.CountByProjectID("proj-1")
	if err != nil {
		t.Fatalf("CountByProjectID failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	records, err := repo.ListByProjectID("proj-1")
	if err != nil {
		t.Fatalf("ListByProjectID failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].ID != record.ID {
		t.Fatalf("records[0].ID = %q, want %q", records[0].ID, record.ID)
	}
}
