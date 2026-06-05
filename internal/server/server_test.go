package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
)

// newTestServer creates a Server with a temp-file registry for integration tests.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	reg := &Registry{
		filePath: t.TempDir() + "/web-registry.json",
	}
	s := &Server{
		app:      app.New(),
		registry: reg,
		mux:      http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func TestServerProjectsEndpointReturnsJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/projects = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestServerCORSHeaderOnEveryResponse(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS header = %q, want *", got)
	}
}

func TestServerPreflightRequest(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", w.Code)
	}
}

func TestServerAddAndListProject(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer(t)

	// Add project.
	body := strings.NewReader(`{"path":"` + dir + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/projects = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	// List — should contain the added project.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)

	var projects []map[string]any
	if err := json.NewDecoder(w2.Body).Decode(&projects); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("project count = %d, want 1", len(projects))
	}
}

func TestServerDeleteProject(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer(t)

	// Add.
	body := strings.NewReader(`{"path":"` + dir + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var proj map[string]any
	_ = json.NewDecoder(w.Body).Decode(&proj)
	id := proj["id"].(string)

	// Delete.
	req2 := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+id, nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want 204", w2.Code)
	}

	// List — should be empty.
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w3, req3)

	var projects []any
	_ = json.NewDecoder(w3.Body).Decode(&projects)
	if len(projects) != 0 {
		t.Errorf("expected empty list after delete, got %d", len(projects))
	}
}

func TestServerUnknownRoute(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("unknown route status = %d, want 404", w.Code)
	}
}
