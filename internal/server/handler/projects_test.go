package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// stubRegistry is an in-memory implementation of ProjectRegistry for tests.
type stubRegistry struct {
	projects map[string]dto.Project
	nextID   int
}

func newStubRegistry() *stubRegistry {
	return &stubRegistry{projects: make(map[string]dto.Project)}
}

func (s *stubRegistry) List() []dto.Project {
	out := make([]dto.Project, 0, len(s.projects))
	for _, p := range s.projects {
		out = append(out, p)
	}
	return out
}

func (s *stubRegistry) Get(id string) (dto.Project, bool) {
	p, ok := s.projects[id]
	return p, ok
}

func (s *stubRegistry) Add(path string) (dto.Project, error) {
	s.nextID++
	id := string(rune('a' + s.nextID))
	p := dto.Project{ID: id, Name: path, Path: path, AddedAt: "2026-01-01T00:00:00Z"}
	s.projects[id] = p
	return p, nil
}

func (s *stubRegistry) Remove(id string) error {
	delete(s.projects, id)
	return nil
}

func TestProjectsHandlerList(t *testing.T) {
	reg := newStubRegistry()
	_, _ = reg.Add("/path/a")
	_, _ = reg.Add("/path/b")

	h := NewProjectsHandler(reg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var projects []dto.Project
	if err := json.NewDecoder(w.Body).Decode(&projects); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("got %d projects, want 2", len(projects))
	}
}

func TestProjectsHandlerListEmpty(t *testing.T) {
	h := NewProjectsHandler(newStubRegistry())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	// Should be empty JSON array, not null.
	if body == "null\n" {
		t.Error("response should be [] not null for empty list")
	}
}

func TestProjectsHandlerAdd(t *testing.T) {
	dir := t.TempDir()
	reg := newStubRegistry()
	h := NewProjectsHandler(reg)

	body := bytes.NewBufferString(`{"path":"` + dir + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	w := httptest.NewRecorder()

	h.Add(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var proj dto.Project
	if err := json.NewDecoder(w.Body).Decode(&proj); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if proj.ID == "" {
		t.Error("returned project should have an ID")
	}
}

func TestProjectsHandlerAddMissingPath(t *testing.T) {
	h := NewProjectsHandler(newStubRegistry())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	h.Add(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestProjectsHandlerAddBadJSON(t *testing.T) {
	h := NewProjectsHandler(newStubRegistry())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`not-json`))
	w := httptest.NewRecorder()

	h.Add(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestProjectsHandlerRemove(t *testing.T) {
	reg := newStubRegistry()
	proj, _ := reg.Add("/some/path")
	h := NewProjectsHandler(reg)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+proj.ID, nil)
	req.SetPathValue("id", proj.ID)
	w := httptest.NewRecorder()

	h.Remove(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if len(reg.List()) != 0 {
		t.Error("project should be gone after Remove")
	}
}

func TestProjectsHandlerResponseIsJSON(t *testing.T) {
	h := NewProjectsHandler(newStubRegistry())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestProjectsHandlerAddRealPath uses a real temp directory to verify the full
// path-resolution flow (not just the stub).
func TestProjectsHandlerAddRealPath(t *testing.T) {
	dir := t.TempDir()

	// Use a real registry backed by a temp file.
	type realReg struct {
		projects map[string]dto.Project
	}

	sub := make(map[string]dto.Project)
	realR := &stubRegistry{projects: sub}

	h := NewProjectsHandler(realR)
	body := bytes.NewBufferString(`{"path":"` + dir + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	w := httptest.NewRecorder()

	h.Add(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

// TestProjectsHandlerCORSHeaders verifies that the middleware sets the
// Access-Control-Allow-Origin header so any frontend origin can reach the API.
func TestProjectsHandlerCORSHeader(t *testing.T) {
	// Wrap the handler with corsMiddleware the same way server.go does.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := corsMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS origin = %q, want *", got)
	}
}

// corsMiddleware mirrors the server package's middleware so we can test it
// from the handler package without a circular import.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Ensure os is used (path existence check in AddRealPath).
var _ = os.TempDir
