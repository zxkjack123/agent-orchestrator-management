package handler

import (
	"encoding/json"
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

// FsBrowse handles GET /api/v1/fs/browse?path=...
// Returns the subdirectories at the given path (defaults to home dir).
// Restricted to within the user's home directory.
func FsBrowse(w http.ResponseWriter, r *http.Request) {
	home := fsHome()
	raw := strings.TrimSpace(r.URL.Query().Get("path"))
	if raw == "" {
		raw = home
	}

	// filepath.EvalSymlinks is the canonical CWE-022 sanitiser recognised by
	// CodeQL. It resolves all symlinks and verifies the path exists, breaking
	// the taint chain from user input.
	safePath, err := filepath.EvalSymlinks(filepath.Clean(raw))
	if err != nil {
		writeError(w, http.StatusNotFound, "path not found")
		return
	}
	if home != "/" && !strings.HasPrefix(safePath, home) {
		writeError(w, http.StatusForbidden, "path is outside the allowed directory")
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
	home := fsHome()

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

	// Resolve the parent via EvalSymlinks — breaks the taint chain.
	safeParent, err := filepath.EvalSymlinks(filepath.Clean(req.Parent))
	if err != nil {
		writeError(w, http.StatusNotFound, "parent path not found")
		return
	}
	if home != "/" && !strings.HasPrefix(safeParent, home) {
		writeError(w, http.StatusForbidden, "path is outside the allowed directory")
		return
	}

	// newPath is constructed from the sanitised parent and a validated name
	// that contains no separators or traversal sequences.
	newPath := filepath.Join(safeParent, req.Name)
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create directory: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": newPath})
}
