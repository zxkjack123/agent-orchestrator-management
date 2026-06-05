package handler

import (
	"net/http"
	"os/exec"
	"strings"
)

// TerminalHistory handles GET /api/v1/terminal/{pane}/history.
// Returns the tmux pane scrollback as plain text (no ANSI), trailing spaces trimmed per line.
func TerminalHistory(w http.ResponseWriter, r *http.Request) {
	pane := strings.TrimSpace(r.PathValue("pane"))
	if pane == "" {
		writeError(w, http.StatusBadRequest, "pane ID required")
		return
	}
	// No -e: plain text only. tmux pads every line to terminal width with spaces;
	// we trim those so the browser doesn't render hundreds of trailing blanks.
	out, err := exec.Command("tmux", "capture-pane", "-p", "-J", "-S", "-5000", "-t", pane).Output()
	if err != nil {
		writeError(w, http.StatusNotFound, "pane not found or capture failed")
		return
	}

	lines := strings.Split(string(out), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	// Drop trailing blank lines at the end of the capture.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	result := strings.Join(lines, "\n")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(result))
}
