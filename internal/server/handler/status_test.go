package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

func TestStatusHandlerNotFound(t *testing.T) {
	reg := newStubRegistry()
	h := NewStatusHandler(app.New(), reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/missing/status", nil)
	req.SetPathValue("id", "missing")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestStatusHandlerReturnsProjectInfo(t *testing.T) {
	projPath, _, cleanup := projectFixture(t)
	defer cleanup()

	reg := newStubRegistry()
	proj, _ := reg.Add(projPath)

	h := NewStatusHandler(app.New(), reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/status", nil)
	req.SetPathValue("id", proj.ID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var status dto.ProjectStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.ProjectName == "" {
		t.Error("ProjectName should not be empty")
	}
	// agents list may be empty for a skeleton project, but must not be nil.
	if status.Agents == nil {
		t.Error("Agents field should not be nil")
	}
}

func TestStatusHandlerActiveIdleCounts(t *testing.T) {
	projPath, _, cleanup := projectFixture(t)
	defer cleanup()

	reg := newStubRegistry()
	proj, _ := reg.Add(projPath)

	h := NewStatusHandler(app.New(), reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/status", nil)
	req.SetPathValue("id", proj.ID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	var status dto.ProjectStatus
	_ = json.NewDecoder(w.Body).Decode(&status)

	// With no sessions seeded, both counts must be zero — not negative.
	if status.ActiveCount < 0 {
		t.Errorf("ActiveCount = %d, should be >= 0", status.ActiveCount)
	}
	if status.IdleCount < 0 {
		t.Errorf("IdleCount = %d, should be >= 0", status.IdleCount)
	}
}

func TestStatusHandlerContentType(t *testing.T) {
	projPath, _, cleanup := projectFixture(t)
	defer cleanup()

	reg := newStubRegistry()
	proj, _ := reg.Add(projPath)

	h := NewStatusHandler(app.New(), reg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/status", nil)
	req.SetPathValue("id", proj.ID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
