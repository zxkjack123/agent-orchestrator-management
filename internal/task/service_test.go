package task

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/db"
)

func TestServiceCreateCreatesTaskAndInitialStep(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewServiceWithGenerators(
		sqlDB,
		func() string { return "TASK-001" },
		func() string { return "STEP-001" },
	)

	result, err := service.Create(CreateParams{
		ProjectID:      "proj-1",
		Title:          "Implement task flow",
		PreferredRole:  "backend",
		PreferredAgent: "backend-main",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if result.Task.ID != "TASK-001" {
		t.Fatalf("Task.ID = %q, want TASK-001", result.Task.ID)
	}
	if result.Task.Mode != "Direct" {
		t.Fatalf("Task.Mode = %q, want Direct", result.Task.Mode)
	}
	if result.Task.Status != "Planned" {
		t.Fatalf("Task.Status = %q, want Planned", result.Task.Status)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("len(result.Steps) = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].ID != "STEP-001" {
		t.Fatalf("Step.ID = %q, want STEP-001", result.Steps[0].ID)
	}
	if result.Steps[0].Status != "Proposed" {
		t.Fatalf("Step.Status = %q, want Proposed", result.Steps[0].Status)
	}
}

func TestServiceUpdateValidatesTransitions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewServiceWithGenerators(
		sqlDB,
		func() string { return "TASK-001" },
		func() string { return "STEP-001" },
	)

	result, err := service.Create(CreateParams{
		ProjectID: "proj-1",
		Title:     "Implement task flow",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(result.Task.ID, UpdateParams{Status: "ready", Mode: "bugfix"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Status != "Ready" {
		t.Fatalf("Status = %q, want Ready", updated.Status)
	}
	if updated.Mode != "Bugfix" {
		t.Fatalf("Mode = %q, want Bugfix", updated.Mode)
	}

	if _, err := service.Update(result.Task.ID, UpdateParams{Status: "planned"}); err == nil {
		t.Fatal("Update should reject Ready -> Planned")
	}

	records, err := service.ListByProject("proj-1")
	if err != nil {
		t.Fatalf("ListByProject failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].ID != result.Task.ID {
		t.Fatalf("records[0].ID = %q, want %q", records[0].ID, result.Task.ID)
	}
}

func TestServiceCloseMarksDoneWhenAllowed(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewServiceWithGenerators(
		sqlDB,
		func() string { return "TASK-001" },
		func() string { return "STEP-001" },
	)

	result, err := service.Create(CreateParams{
		ProjectID: "proj-1",
		Title:     "Implement task flow",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := service.Update(result.Task.ID, UpdateParams{Status: "ready"}); err != nil {
		t.Fatalf("Update to Ready failed: %v", err)
	}
	if _, err := service.Update(result.Task.ID, UpdateParams{Status: "in-progress"}); err != nil {
		t.Fatalf("Update to InProgress failed: %v", err)
	}

	closed, err := service.Close(result.Task.ID)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if closed.Status != "Done" {
		t.Fatalf("Status = %q, want Done", closed.Status)
	}
}

func TestServiceCreateFromPlanCreatesMultipleSequentialSteps(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	stepIDs := []string{"STEP-001", "STEP-002"}
	stepIndex := 0
	service := NewServiceWithGenerators(
		sqlDB,
		func() string { return "TASK-001" },
		func() string {
			id := stepIDs[stepIndex]
			stepIndex++
			return id
		},
	)

	result, err := service.CreateFromPlan(
		CreateParams{
			ProjectID:      "proj-1",
			Title:          "Fix login bug",
			Mode:           "Bugfix",
			PreferredRole:  "backend",
			PreferredAgent: "backend-main",
		},
		[]StepSeed{
			{Type: "research", Title: "Confirm current behavior and likely root cause", RoleName: "backend", AgentName: "backend-main"},
			{Type: "implementation", Title: "Apply the fix for Fix login bug", RoleName: "backend", AgentName: "backend-main"},
		},
	)
	if err != nil {
		t.Fatalf("CreateFromPlan failed: %v", err)
	}

	if len(result.Steps) != 2 {
		t.Fatalf("len(result.Steps) = %d, want 2", len(result.Steps))
	}
	if result.Steps[0].StepType != "research" {
		t.Fatalf("Step[0].StepType = %q, want research", result.Steps[0].StepType)
	}
	if len(result.Steps[1].Dependencies) != 1 || result.Steps[1].Dependencies[0] != "STEP-001" {
		t.Fatalf("Step[1].Dependencies = %#v, want [STEP-001]", result.Steps[1].Dependencies)
	}
}

func TestDefaultTaskIDGeneratorProducesDistinctIDs(t *testing.T) {
	gen := defaultTaskIDGenerator("STEP")

	first := gen()
	second := gen()

	if first == second {
		t.Fatalf("generated duplicate ids: %q", first)
	}
	if !strings.HasPrefix(first, "STEP-") {
		t.Fatalf("first id = %q, want STEP- prefix", first)
	}
	if !strings.HasPrefix(second, "STEP-") {
		t.Fatalf("second id = %q, want STEP- prefix", second)
	}
}

func TestServiceAssignOwnerAllowsClearingPreferredAgent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewServiceWithGenerators(
		sqlDB,
		func() string { return "TASK-001" },
		func() string { return "STEP-001" },
	)

	result, err := service.Create(CreateParams{
		ProjectID:      "proj-1",
		Title:          "Implement task flow",
		PreferredRole:  "backend",
		PreferredAgent: "backend-main",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	record, err := service.AssignOwner(result.Task.ID, "reviewer", "")
	if err != nil {
		t.Fatalf("AssignOwner failed: %v", err)
	}
	if record.PreferredRole != "reviewer" {
		t.Fatalf("PreferredRole = %q, want reviewer", record.PreferredRole)
	}
	if record.PreferredAgent != "" {
		t.Fatalf("PreferredAgent = %q, want empty", record.PreferredAgent)
	}
}
