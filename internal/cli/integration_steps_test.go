package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/db"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/step"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/tmux"
)

func TestAutoSkipPlaceholderIntegrationSteps(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := step.NewRepository(sqlDB)
	records := []step.Record{
		{ID: "STEP-001", ProjectID: "proj-1", TaskID: "TASK-001", StepType: "integration", Title: "Merge branch", Status: "Proposed", RoleName: "operator"},
		{ID: "STEP-002", ProjectID: "proj-1", TaskID: "TASK-001", StepType: "integration", Title: "Finalize merge", Status: "Ready", RoleName: "operator"},
		{ID: "STEP-003", ProjectID: "proj-1", TaskID: "TASK-001", StepType: "integration", Title: "Already in progress", Status: "InProgress", RoleName: "operator"},
		{ID: "STEP-004", ProjectID: "proj-1", TaskID: "TASK-001", StepType: "implementation", Title: "Ship feature", Status: "Proposed", RoleName: "backend"},
	}
	for _, record := range records {
		if err := repo.Upsert(record); err != nil {
			t.Fatalf("Upsert(%s) failed: %v", record.ID, err)
		}
	}

	service := step.NewService(sqlDB)
	steps, err := service.ListByTask("TASK-001")
	if err != nil {
		t.Fatalf("ListByTask failed: %v", err)
	}

	updated, err := autoSkipPlaceholderIntegrationSteps(service, steps)
	if err != nil {
		t.Fatalf("autoSkipPlaceholderIntegrationSteps failed: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("updated len = %d, want 2", len(updated))
	}

	got, err := service.ListByTask("TASK-001")
	if err != nil {
		t.Fatalf("ListByTask after skip failed: %v", err)
	}
	statusByID := map[string]string{}
	for _, item := range got {
		statusByID[item.ID] = item.Status
	}
	if statusByID["STEP-001"] != "Skipped" || statusByID["STEP-002"] != "Skipped" {
		t.Fatalf("integration placeholder steps = %#v, want skipped", statusByID)
	}
	if statusByID["STEP-003"] != "InProgress" {
		t.Fatalf("STEP-003 status = %q, want InProgress", statusByID["STEP-003"])
	}
	if statusByID["STEP-004"] != "Proposed" {
		t.Fatalf("STEP-004 status = %q, want Proposed", statusByID["STEP-004"])
	}
}

func TestAutoCompleteIntegrationSteps(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := step.NewRepository(sqlDB)
	records := []step.Record{
		{ID: "STEP-001", ProjectID: "proj-1", TaskID: "TASK-001", StepType: "integration", Title: "Merge branch", Status: "Proposed", RoleName: "operator"},
		{ID: "STEP-002", ProjectID: "proj-1", TaskID: "TASK-001", StepType: "integration", Title: "Finalize merge", Status: "InProgress", RoleName: "operator"},
		{ID: "STEP-003", ProjectID: "proj-1", TaskID: "TASK-001", StepType: "implementation", Title: "Ship feature", Status: "Ready", RoleName: "backend"},
	}
	for _, record := range records {
		if err := repo.Upsert(record); err != nil {
			t.Fatalf("Upsert(%s) failed: %v", record.ID, err)
		}
	}

	service := step.NewService(sqlDB)
	steps, err := service.ListByTask("TASK-001")
	if err != nil {
		t.Fatalf("ListByTask failed: %v", err)
	}

	updated, err := autoCompleteIntegrationSteps(service, steps)
	if err != nil {
		t.Fatalf("autoCompleteIntegrationSteps failed: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("updated len = %d, want 2", len(updated))
	}

	got, err := service.ListByTask("TASK-001")
	if err != nil {
		t.Fatalf("ListByTask after complete failed: %v", err)
	}
	statusByID := map[string]string{}
	for _, item := range got {
		statusByID[item.ID] = item.Status
	}
	if statusByID["STEP-001"] != "Completed" || statusByID["STEP-002"] != "Completed" {
		t.Fatalf("integration steps = %#v, want completed", statusByID)
	}
	if statusByID["STEP-003"] != "Ready" {
		t.Fatalf("STEP-003 status = %q, want Ready", statusByID["STEP-003"])
	}
}

func TestExecuteTaskCloseAutoSkipsPlaceholderIntegrationStep(t *testing.T) {
	repoRoot := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	restoreAppFactory := stubAppFactory(t, tmux.NewManagerWithDeps(
		func(string) (string, error) { return "", errors.New("not found") },
		func(string, ...string) ([]byte, error) { return nil, nil },
		func(string, ...string) error { return nil },
	))
	defer restoreAppFactory()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Execute([]string{"project", "init", "my-app", "--repo", repoRoot}, &stdout, &stderr); err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "create", "Close integration task", "--role", "backend", "--agent", "backend-main"}, &stdout, &stderr); err != nil {
		t.Fatalf("task create failed: %v", err)
	}
	taskID := extractEntityID(stdout.String(), "Task: ")

	projectResult, err := app.New().Projects.Open(".")
	if err != nil {
		t.Fatalf("project open failed: %v", err)
	}
	stepService, sqlDB, err := app.New().OpenStepService(projectResult.DBPath)
	if err != nil {
		t.Fatalf("open step service failed: %v", err)
	}
	integrationStep, err := stepService.Create(step.CreateParams{
		ProjectID: projectResult.Project.ID,
		TaskID:    taskID,
		StepType:  "integration",
		Title:     "Merge task branch",
		RoleName:  "operator",
	})
	sqlDB.Close()
	if err != nil {
		t.Fatalf("create integration step failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"step", "list", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("step list failed: %v", err)
	}
	initialStepID := extractStepID(stdout.String())
	if initialStepID == "" {
		t.Fatalf("could not extract initial step id from %q", stdout.String())
	}

	stdout.Reset()
	if err := Execute([]string{"step", "update", initialStepID, "--status", "confirmed"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to confirmed failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"step", "update", initialStepID, "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("step update to ready failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "ready"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to ready failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "update", taskID, "--status", "in-progress"}, &stdout, &stderr); err != nil {
		t.Fatalf("task update to in-progress failed: %v", err)
	}

	stdout.Reset()
	if err := Execute([]string{"task", "close", taskID}, &stdout, &stderr); err != nil {
		t.Fatalf("task close failed: %v", err)
	}
	if out := stdout.String(); !strings.Contains(out, "Auto-skipped 1 placeholder integration step(s):") {
		t.Fatalf("stdout = %q, want auto-skip message", out)
	}

	stepService, sqlDB, err = app.New().OpenStepService(projectResult.DBPath)
	if err != nil {
		t.Fatalf("re-open step service failed: %v", err)
	}
	defer sqlDB.Close()

	record, err := stepService.Get(integrationStep.ID)
	if err != nil {
		t.Fatalf("step get failed: %v", err)
	}
	if record == nil || record.Status != "Skipped" {
		t.Fatalf("integration step status = %#v, want Skipped", record)
	}
}
