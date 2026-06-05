package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/db"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/session"
)

// projectFixture creates a minimal AOM project on disk and returns its path
// and an open session service so tests can seed records directly.
func projectFixture(t *testing.T) (projectPath string, svc *session.Service, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	aomDir := filepath.Join(dir, ".aom")
	_ = os.MkdirAll(aomDir, 0o755)

	projectYAML := "name: test-proj\nrepo: .\ndefault_branch: main\nruntime:\n  terminal: tmux\n  session_prefix: tp\ncontext:\n  state_dir: .aom/state\n"
	_ = os.WriteFile(filepath.Join(aomDir, "project.yaml"), []byte(projectYAML), 0o644)
	_ = os.WriteFile(filepath.Join(aomDir, "agents.yaml"), []byte("roles: {}\nagents: {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(aomDir, "resources.yaml"), []byte("skills: {}\nmcp_servers: {}\nrole_bindings: {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(aomDir, "policy.yaml"), []byte("policy:\n  deny_commands: []\n  require_approval: []\n  session_defaults:\n    approval_scope: per-session\n    yolo_mode: disabled\n  owner_exceptions:\n    enabled: true\n    log_required: true\n"), 0o644)

	dbPath := filepath.Join(aomDir, "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	svc = session.NewService(sqlDB)
	return dir, svc, func() { sqlDB.Close() }
}

func TestSessionsHandlerListEmpty(t *testing.T) {
	projPath, _, cleanup := projectFixture(t)
	defer cleanup()

	reg := newStubRegistry()
	proj, _ := reg.Add(projPath)

	h := NewSessionsHandler(app.New(), reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/sessions", nil)
	req.SetPathValue("id", proj.ID)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var sessions []dto.Session
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected empty list, got %d sessions", len(sessions))
	}
}

func TestSessionsHandlerListNotFound(t *testing.T) {
	reg := newStubRegistry()
	h := NewSessionsHandler(app.New(), reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/unknown/sessions", nil)
	req.SetPathValue("id", "unknown")
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSessionsHandlerActiveFilter(t *testing.T) {
	reg := newStubRegistry()
	h := NewSessionsHandler(app.New(), reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/x/sessions?active=true", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()

	h.List(w, req)

	// Project not in registry → 404, not a panic.
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestIsActiveStatus(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"Working", true},
		{"Idle", true},
		{"WaitingApproval", true},
		{"WaitingHandoff", true},
		{"Booting", true},
		{"Stopped", false},
		{"Archived", false},
		{"Created", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isActiveStatus(tc.status); got != tc.want {
			t.Errorf("isActiveStatus(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestSessionsHandlerStopNotFound(t *testing.T) {
	reg := newStubRegistry()
	h := NewSessionsHandler(app.New(), reg)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/x/sessions/s1", nil)
	req.SetPathValue("id", "x")
	req.SetPathValue("sid", "s1")
	w := httptest.NewRecorder()

	h.Stop(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSessionsHandlerResponseContentType(t *testing.T) {
	projPath, _, cleanup := projectFixture(t)
	defer cleanup()

	reg := newStubRegistry()
	proj, _ := reg.Add(projPath)

	h := NewSessionsHandler(app.New(), reg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/sessions", nil)
	req.SetPathValue("id", proj.ID)
	w := httptest.NewRecorder()

	h.List(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
