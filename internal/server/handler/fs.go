package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fsHome returns the user's home directory — the boundary for all fs operations.
func fsHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return home
}

// fsResolve cleans rawPath, resolves symlinks via filepath.EvalSymlinks (the
// canonical CWE-022 sanitiser), and verifies the result stays within the home
// directory. Returns the fully-resolved safe path or an error.
func fsResolve(rawPath string) (string, error) {
	p := filepath.Clean(strings.TrimSpace(rawPath))
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("path must be absolute")
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("path not found")
	}
	home := fsHome()
	if home != "/" && !strings.HasPrefix(resolved, home) {
		return "", fmt.Errorf("path is outside the allowed directory")
	}
	return resolved, nil
}

// FsBrowse handles GET /api/v1/fs/browse?path=...
// Returns the subdirectories at the given path (defaults to home dir).
// Restricted to within the user's home directory.
func FsBrowse(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("path"))
	if raw == "" {
		raw = fsHome()
	}

	// EvalSymlinks is CodeQL's recognised CWE-022 sanitiser; it also
	// guarantees the path exists before we call ReadDir.
	safePath, err := fsResolve(raw)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	info, err := os.Stat(safePath)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "path not found or not a directory")
		return
	}

	entries, err := os.ReadDir(safePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read directory")
		return
	}

	type dirEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	dirs := make([]dirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, dirEntry{
			Name: name,
			Path: filepath.Join(safePath, name),
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := filepath.Dir(safePath)
	if parent == safePath {
		parent = ""
	}

	writeJSON(w, map[string]any{
		"path":    safePath,
		"parent":  parent,
		"entries": dirs,
	})
}

// FsMkdir handles POST /api/v1/fs/mkdir
// Creates a new subdirectory within the user's home directory.
func FsMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Parent == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "parent and name are required")
		return
	}
	// Validate name: no path separators, no traversal components.
	if strings.ContainsAny(req.Name, "/\\") || req.Name == "." || req.Name == ".." {
		writeError(w, http.StatusBadRequest, "invalid folder name")
		return
	}
	// Resolve and validate the parent (must exist and be within home).
	safeParent, err := fsResolve(req.Parent)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	// Construct the new path from the safe (resolved) parent and the validated name.
	newPath := filepath.Join(safeParent, req.Name)
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create directory: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": newPath})
}
