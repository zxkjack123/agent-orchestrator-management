package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/dto"
)

// Registry manages the list of AOM projects known to the web server.
// Persisted at ~/.config/aom/web-registry.json so it survives restarts.
type Registry struct {
	mu       sync.RWMutex
	filePath string
	data     registryFile
}

type registryFile struct {
	Projects []dto.Project `json:"projects"`
}

// NewRegistry loads an existing registry or starts a fresh one.
func NewRegistry() (*Registry, error) {
	path, err := defaultRegistryPath()
	if err != nil {
		return nil, err
	}
	r := &Registry{filePath: path}
	_ = r.load() // ok when file doesn't exist yet
	return r, nil
}

func defaultRegistryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "aom", "web-registry.json"), nil
}

// projectID returns a stable short ID derived from the absolute path.
func projectID(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return fmt.Sprintf("%x", h[:4])
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &r.data)
}

func (r *Registry) save() error {
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, data, 0o644)
}

// List returns all registered projects.
func (r *Registry) List() []dto.Project {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]dto.Project, len(r.data.Projects))
	copy(out, r.data.Projects)
	return out
}

// Get returns a project by ID.
func (r *Registry) Get(id string) (dto.Project, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.data.Projects {
		if p.ID == id {
			return p, true
		}
	}
	return dto.Project{}, false
}

// Add registers a project by its filesystem path. Idempotent — returns the
// existing entry if the path was already registered.
func (r *Registry) Add(path string) (dto.Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return dto.Project{}, fmt.Errorf("resolve path: %w", err)
	}
	// EvalSymlinks resolves symlinks and verifies the path exists — the
	// canonical sanitiser for CWE-022 path-traversal in CodeQL's taint model.
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return dto.Project{}, fmt.Errorf("path does not exist: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	id := projectID(abs)
	for _, p := range r.data.Projects {
		if p.ID == id {
			return p, nil
		}
	}

	proj := dto.Project{
		ID:      id,
		Name:    filepath.Base(abs),
		Path:    abs,
		AddedAt: time.Now().UTC().Format(time.RFC3339),
	}
	r.data.Projects = append(r.data.Projects, proj)
	return proj, r.save()
}

// Remove removes a project from the registry by ID.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := r.data.Projects[:0]
	for _, p := range r.data.Projects {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	r.data.Projects = filtered
	return r.save()
}
