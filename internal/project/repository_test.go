package project

import (
	"path/filepath"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/db"
)

func TestRepositoryUpsertIsIdempotent(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	record := Record{
		ID:            "my-app",
		Name:          "my-app",
		RepoPath:      t.TempDir(),
		DefaultBranch: "main",
	}

	if err := repo.Upsert(record); err != nil {
		t.Fatalf("first Upsert failed: %v", err)
	}
	if err := repo.Upsert(record); err != nil {
		t.Fatalf("second Upsert failed: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(1) FROM projects WHERE id = ?`, record.ID).Scan(&count); err != nil {
		t.Fatalf("count projects failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("project row count = %d, want 1", count)
	}
}

func TestRepositoryFindByRepoPath(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	record := Record{
		ID:            "my-app",
		Name:          "my-app",
		RepoPath:      filepath.Join(t.TempDir(), "repo"),
		DefaultBranch: "main",
	}

	if err := repo.Upsert(record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	got, err := repo.FindByRepoPath(record.RepoPath)
	if err != nil {
		t.Fatalf("FindByRepoPath failed: %v", err)
	}
	if got == nil {
		t.Fatal("FindByRepoPath returned nil, want record")
	}
	if got.ID != record.ID {
		t.Fatalf("record ID = %q, want %q", got.ID, record.ID)
	}
}
