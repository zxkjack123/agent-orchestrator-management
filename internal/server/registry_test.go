package server

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestRegistry creates a registry backed by a temp file so tests don't
// touch ~/.config/aom/web-registry.json.
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := &Registry{
		filePath: filepath.Join(t.TempDir(), "web-registry.json"),
	}
	return reg
}

func TestRegistryAddAndList(t *testing.T) {
	dir := t.TempDir()
	// EvalSymlinks resolves macOS /var → /private/var symlinks; the registry
	// stores resolved paths, so the expected value must match.
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	reg := newTestRegistry(t)

	proj, err := reg.Add(dir)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if proj.Path != resolvedDir {
		t.Errorf("path = %q, want %q", proj.Path, resolvedDir)
	}
	if proj.ID == "" {
		t.Error("ID should not be empty")
	}
	if proj.Name == "" {
		t.Error("Name should not be empty")
	}

	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("list length = %d, want 1", len(list))
	}
	if list[0].ID != proj.ID {
		t.Errorf("listed ID = %q, want %q", list[0].ID, proj.ID)
	}
}

func TestRegistryAddIdempotent(t *testing.T) {
	dir := t.TempDir()
	reg := newTestRegistry(t)

	p1, err := reg.Add(dir)
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}
	p2, err := reg.Add(dir)
	if err != nil {
		t.Fatalf("second Add: %v", err)
	}
	if p1.ID != p2.ID {
		t.Errorf("IDs differ: %q vs %q — Add should be idempotent", p1.ID, p2.ID)
	}
	if len(reg.List()) != 1 {
		t.Errorf("duplicate entry created on second Add")
	}
}

func TestRegistryGet(t *testing.T) {
	dir := t.TempDir()
	reg := newTestRegistry(t)

	proj, _ := reg.Add(dir)

	got, ok := reg.Get(proj.ID)
	if !ok {
		t.Fatal("Get returned not found for a registered project")
	}
	if got.Path != proj.Path {
		t.Errorf("got path %q, want %q", got.Path, proj.Path)
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("Get should return false for unknown ID")
	}
}

func TestRegistryRemove(t *testing.T) {
	dir := t.TempDir()
	reg := newTestRegistry(t)

	proj, _ := reg.Add(dir)
	if err := reg.Remove(proj.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(reg.List()) != 0 {
		t.Error("project still present after Remove")
	}
}

func TestRegistryPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	reg := newTestRegistry(t)
	filePath := reg.filePath

	proj, _ := reg.Add(dir)

	// Load a second registry from the same file — should see the project.
	reg2 := &Registry{filePath: filePath}
	if err := reg2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	list := reg2.List()
	if len(list) != 1 || list[0].ID != proj.ID {
		t.Errorf("second registry did not see persisted project")
	}
}

func TestRegistryAddNonExistentPath(t *testing.T) {
	reg := newTestRegistry(t)
	_, err := reg.Add("/does/not/exist/at/all")
	if err == nil {
		t.Error("expected error for non-existent path, got nil")
	}
}

func TestProjectIDStable(t *testing.T) {
	path := "/some/project/path"
	id1 := projectID(path)
	id2 := projectID(path)
	if id1 != id2 {
		t.Errorf("projectID not stable: %q vs %q", id1, id2)
	}
	if id1 == "" {
		t.Error("projectID returned empty string")
	}
}

func TestRegistryNameFromPath(t *testing.T) {
	dir := t.TempDir()
	// Create a sub-dir so the name is predictable.
	sub := filepath.Join(dir, "my-cool-project")
	_ = os.Mkdir(sub, 0o755)

	reg := newTestRegistry(t)
	proj, err := reg.Add(sub)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if proj.Name != "my-cool-project" {
		t.Errorf("name = %q, want %q", proj.Name, "my-cool-project")
	}
}
