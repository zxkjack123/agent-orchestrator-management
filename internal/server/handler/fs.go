package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fsAllowedRoot returns the boundary that all fs operations must stay within.
// Restricting to the user's home directory prevents traversal of sensitive
// system paths (e.g. /etc, /proc) by anyone who can reach the local server.
func fsAllowedRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return home
}

// fsGuard cleans path and verifies it is absolute and within the allowed root.
// Returns the cleaned path and true on success; writes an error and false on failure.
func fsGuard(w http.ResponseWriter, rawPath string) (string, bool) {
	p := filepath.Clean(strings.TrimSpace(rawPath))
	if !filepath.IsAbs(p) {
		writeError(w, http.StatusBadRequest, "path must be absolute")
		return "", false
	}
	root := fsAllowedRoot()
	if root != "/" && !strings.HasPrefix(p, root) {
		writeError(w, http.StatusForbidden, "path is outside the allowed directory")
		return "", false
	}
	return p, true
}

// FsBrowse handles GET /api/v1/fs/browse?path=...
// Returns the directories at the given path (defaults to home dir).
// Restricted to within the user's home directory.
func FsBrowse(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if reqPath == "" {
		reqPath = fsAllowedRoot()
	}

	reqPath, ok := fsGuard(w, reqPath)
	if !ok {
		return
	}

	info, err := os.Stat(reqPath)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "path not found or not a directory")
		return
	}

	entries, err := os.ReadDir(reqPath)
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
		// Skip hidden dirs except at root level.
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, dirEntry{
			Name: name,
			Path: filepath.Join(reqPath, name),
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := filepath.Dir(reqPath)
	if parent == reqPath {
		parent = "" // at root
	}

	writeJSON(w, map[string]any{
		"path":    reqPath,
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
	if strings.ContainsAny(req.Name, "/\\") || req.Name == "." || req.Name == ".." {
		writeError(w, http.StatusBadRequest, "invalid folder name")
		return
	}
	parent, ok := fsGuard(w, req.Parent)
	if !ok {
		return
	}
	newPath := filepath.Join(parent, req.Name)
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create directory: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": newPath})
}
