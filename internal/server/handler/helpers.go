package handler

import (
	"encoding/json"
	"net/http"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// ProjectLookup is the minimal interface handlers need to resolve a project by ID.
type ProjectLookup interface {
	Get(id string) (dto.Project, bool)
}

// writeJSON serialises v as JSON and writes it with status 200.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error body with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// resolveProject extracts the {id} path value and looks it up in the registry.
// Returns false and writes a 404 if not found.
func resolveProject(reg ProjectLookup, w http.ResponseWriter, r *http.Request) (dto.Project, bool) {
	id := r.PathValue("id")
	proj, ok := reg.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return dto.Project{}, false
	}
	return proj, true
}
