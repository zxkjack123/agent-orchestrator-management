package session

import (
	"fmt"
	"testing"
)

func TestServiceCreateAndListByProject(t *testing.T) {
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	nextID := 0
	service := NewServiceWithIDGenerator(sqlDB, func() string {
		nextID++
		return fmt.Sprintf("SESS-TEST-%d", nextID)
	})

	record, err := service.Create(CreateParams{
		ProjectID:       "my-app",
		AgentID:         "my-app:backend-main",
		AgentName:       "backend-main",
		RoleName:        "backend",
		Runtime:         "codex",
		RepoPath:        "C:/repo",
		TmuxSessionName: "aom-my-app",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if record.Status != "Created" {
		t.Fatalf("Status = %q, want %q", record.Status, "Created")
	}
	if record.ID != "SESS-TEST-1" {
		t.Fatalf("ID = %q, want %q", record.ID, "SESS-TEST-1")
	}

	loaded, err := service.Get(record.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Get returned nil record")
	}
	if loaded.AgentName != "backend-main" {
		t.Fatalf("AgentName = %q, want %q", loaded.AgentName, "backend-main")
	}

	sessions, err := service.ListByProject("my-app")
	if err != nil {
		t.Fatalf("ListByProject failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("session count = %d, want 1", len(sessions))
	}
}

func TestServiceCreateValidatesRequiredFields(t *testing.T) {
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	service := NewServiceWithIDGenerator(sqlDB, func() string { return "SESS-TEST-1" })

	_, err := service.Create(CreateParams{
		ProjectID: "my-app",
		Runtime:   "codex",
		RepoPath:  "C:/repo",
	})
	if err == nil {
		t.Fatal("Create should fail when agent name and role are missing")
	}
}
