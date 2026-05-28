package agent

import (
	"path/filepath"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/db"
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

// TestRepositorySyncPrunesRemovedAgents verifies that agents deleted from
// agents.yaml are removed from the DB on the next Sync call. This prevents
// stale default agents (backend-main, frontend-main, reviewer-main) from
// appearing in aom status after the operator trims agents.yaml.
func TestRepositorySyncPrunesRemovedAgents(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)

	// Initial sync: 3 agents (simulates the default project init template).
	initialCfg := config.AgentsFile{
		Agents: map[string]config.AgentConfig{
			"backend-main":  {Runtime: "codex", Role: "backend", Enabled: true},
			"frontend-main": {Runtime: "claude", Role: "frontend", Enabled: true},
			"reviewer-main": {Runtime: "claude", Role: "reviewer", Enabled: true},
		},
	}
	if err := repo.Sync("proj", initialCfg); err != nil {
		t.Fatalf("initial Sync: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(1) FROM agents WHERE project_id = ?`, "proj").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("before prune: count = %d, want 3", count)
	}

	// Second sync: operator removed backend-main and frontend-main, added codex-be.
	trimmedCfg := config.AgentsFile{
		Agents: map[string]config.AgentConfig{
			"codex-be":      {Runtime: "codex", Role: "backend", Enabled: true},
			"reviewer-main": {Runtime: "claude", Role: "reviewer", Enabled: true},
		},
	}
	if err := repo.Sync("proj", trimmedCfg); err != nil {
		t.Fatalf("trimmed Sync: %v", err)
	}

	records, err := repo.ListByProjectID("proj")
	if err != nil {
		t.Fatalf("ListByProjectID: %v", err)
	}
	if len(records) != 2 {
		names := make([]string, len(records))
		for i, r := range records {
			names[i] = r.Name
		}
		t.Fatalf("after prune: count = %d (agents: %v), want 2", len(records), names)
	}

	// The two surviving agents should be codex-be and reviewer-main.
	for _, r := range records {
		if r.Name != "codex-be" && r.Name != "reviewer-main" {
			t.Fatalf("unexpected agent %q survived prune", r.Name)
		}
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
