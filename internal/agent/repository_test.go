package agent

import (
	"path/filepath"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/db"
)

func TestRepositorySyncUpsertsAgents(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	cfg := config.AgentsFile{
		Agents: map[string]config.AgentConfig{
			"backend-main": {
				Runtime: "codex",
				Role:    "backend",
				Enabled: true,
			},
			"reviewer-main": {
				Runtime: "claude",
				Role:    "reviewer",
				Enabled: true,
			},
		},
	}

	if err := repo.Sync("my-app", cfg); err != nil {
		t.Fatalf("first Sync failed: %v", err)
	}
	if err := repo.Sync("my-app", cfg); err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(1) FROM agents WHERE project_id = ?`, "my-app").Scan(&count); err != nil {
		t.Fatalf("count agents failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("agent row count = %d, want 2", count)
	}
}

func TestRepositoryListByProjectID(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	cfg := config.AgentsFile{
		Agents: map[string]config.AgentConfig{
			"reviewer-main": {
				Runtime: "claude",
				Role:    "reviewer",
				Enabled: true,
			},
			"backend-main": {
				Runtime: "codex",
				Role:    "backend",
				Enabled: true,
			},
		},
	}

	if err := repo.Sync("my-app", cfg); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	records, err := repo.ListByProjectID("my-app")
	if err != nil {
		t.Fatalf("ListByProjectID failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	if records[0].Name != "backend-main" {
		t.Fatalf("first record name = %q, want %q", records[0].Name, "backend-main")
	}
}
