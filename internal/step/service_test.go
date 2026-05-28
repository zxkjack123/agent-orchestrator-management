package step

import (
	"path/filepath"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/db"
)

func TestServiceUpdateStatusAndOwnership(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	if err := repo.Upsert(Record{
		ID:        "STEP-001",
		ProjectID: "proj-1",
		TaskID:    "TASK-001",
		StepType:  "implementation",
		Title:     "Implement first slice",
		Status:    "Proposed",
	}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	service := NewService(sqlDB)
	record, err := service.Update("STEP-001", UpdateParams{
		Status:    "confirmed",
		RoleName:  "backend",
		AgentName: "backend-main",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if record.Status != "Confirmed" {
		t.Fatalf("Status = %q, want Confirmed", record.Status)
	}
	if record.AgentName != "backend-main" {
		t.Fatalf("AgentName = %q, want backend-main", record.AgentName)
	}
}

func TestServiceUpdateRejectsReadyWithoutOwner(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	if err := repo.Upsert(Record{
		ID:        "STEP-001",
		ProjectID: "proj-1",
		TaskID:    "TASK-001",
		StepType:  "implementation",
		Title:     "Implement first slice",
		Status:    "Confirmed",
	}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	service := NewService(sqlDB)
	if _, err := service.Update("STEP-001", UpdateParams{Status: "ready"}); err == nil {
		t.Fatal("Update should fail without owner")
	}
}

func TestServiceAssignOwnerAllowsClearingAssignedAgent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewRepository(sqlDB)
	if err := repo.Upsert(Record{
		ID:        "STEP-001",
		ProjectID: "proj-1",
		TaskID:    "TASK-001",
		StepType:  "review",
		Title:     "Review current diff",
		Status:    "Confirmed",
		RoleName:  "backend",
		AgentName: "backend-main",
	}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	service := NewService(sqlDB)
	record, err := service.AssignOwner("STEP-001", "reviewer", "")
	if err != nil {
		t.Fatalf("AssignOwner failed: %v", err)
	}
	if record.RoleName != "reviewer" {
		t.Fatalf("RoleName = %q, want reviewer", record.RoleName)
	}
	if record.AgentName != "" {
		t.Fatalf("AgentName = %q, want empty", record.AgentName)
	}
}

func TestServiceCreateReadyStepRequiresOwnerAndPersistsDependencies(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewServiceWithIDGenerator(sqlDB, func() string { return "STEP-NEW" })
	if _, err := service.Create(CreateParams{
		ProjectID: "proj-1",
		TaskID:    "TASK-001",
		StepType:  "review",
		Title:     "Review current diff",
		Status:    "ready",
	}); err == nil {
		t.Fatal("Create should fail for Ready step without owner")
	}

	record, err := service.Create(CreateParams{
		ProjectID:    "proj-1",
		TaskID:       "TASK-001",
		StepType:     "review",
		Title:        "Review current diff",
		Status:       "ready",
		RoleName:     "reviewer",
		AgentName:    "reviewer-main",
		Dependencies: []string{"STEP-001"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if record.ID != "STEP-NEW" {
		t.Fatalf("ID = %q, want STEP-NEW", record.ID)
	}
	if record.Status != "Ready" {
		t.Fatalf("Status = %q, want Ready", record.Status)
	}
	if len(record.Dependencies) != 1 || record.Dependencies[0] != "STEP-001" {
		t.Fatalf("Dependencies = %#v, want [STEP-001]", record.Dependencies)
	}
}
