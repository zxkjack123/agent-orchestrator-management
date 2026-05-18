package step

import (
	"path/filepath"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/db"
)

func TestRepositoryUpsertAndListByTaskID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	record := Record{
		ID:           "STEP-001",
		ProjectID:    "proj-1",
		TaskID:       "TASK-001",
		StepType:     "implementation",
		Title:        "Implement first slice",
		Status:       "Proposed",
		RoleName:     "backend",
		AgentName:    "backend-main",
		Dependencies: []string{"STEP-000"},
	}

	if err := repo.Upsert(record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	steps, err := repo.ListByTaskID("TASK-001")
	if err != nil {
		t.Fatalf("ListByTaskID failed: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("len(steps) = %d, want 1", len(steps))
	}

	got := steps[0]
	if got.StepType != "implementation" {
		t.Fatalf("StepType = %q, want implementation", got.StepType)
	}
	if got.RoleName != "backend" {
		t.Fatalf("RoleName = %q, want backend", got.RoleName)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0] != "STEP-000" {
		t.Fatalf("Dependencies = %#v, want [STEP-000]", got.Dependencies)
	}
}
