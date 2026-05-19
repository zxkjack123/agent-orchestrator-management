package provider

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// PolicyDirForSession returns the /tmp policy sandbox directory path for
// the given session. Returns "" on Windows where PATH-wrapper enforcement
// is not used.
func PolicyDirForSession(sessionID string) string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return fmt.Sprintf("/tmp/aom-policy-%s", sessionID)
}

// CaptureDirForSession returns the /tmp capture state file path for the
// given session. Returns "" on Windows.
func CaptureDirForSession(sessionID string) string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return fmt.Sprintf("/tmp/aom-capture-state-%s", sessionID)
}

// CleanupSession terminates any lingering policy-wrapper processes for the
// given session then removes its /tmp sandbox directory and capture state file.
// Safe to call unconditionally: returns nil immediately when the directories
// do not exist or the platform does not use wrapper enforcement.
func CleanupSession(sessionID string) error {
	var firstErr error

	if dir := PolicyDirForSession(sessionID); dir != "" {
		if _, statErr := os.Stat(dir); statErr == nil {
			killPolicyProcesses(dir)
			if err := os.RemoveAll(dir); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("remove policy dir %s: %w", dir, err)
			}
		}
	}

	if f := CaptureDirForSession(sessionID); f != "" {
		_ = os.Remove(f) // best-effort; file may not exist
	}

	return firstErr
}

// CleanupCaptureState removes only the capture state file for the session.
// Used when archiving a session that was already stopped (policy dir already
// cleaned by stop).
func CleanupCaptureState(sessionID string) {
	if f := CaptureDirForSession(sessionID); f != "" {
		_ = os.Remove(f)
	}
}

// ScanStalePolicyDirs returns /tmp/aom-policy-* directory paths whose session
// IDs are absent from activeSessionIDs. Returns nil on Windows.
func ScanStalePolicyDirs(activeSessionIDs map[string]bool) ([]string, error) {
	if runtime.GOOS == "windows" {
		return nil, nil
	}
	entries, err := os.ReadDir("/tmp")
	if err != nil {
		return nil, fmt.Errorf("read /tmp: %w", err)
	}
	const prefix = "aom-policy-"
	var stale []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		sessionID := strings.TrimPrefix(name, prefix)
		if !activeSessionIDs[sessionID] {
			stale = append(stale, "/tmp/"+name)
		}
	}
	return stale, nil
}

// CleanupStaleDir terminates processes referencing policyDir and removes it.
func CleanupStaleDir(policyDir string) error {
	killPolicyProcesses(policyDir)
	return os.RemoveAll(policyDir)
}

// killPolicyProcesses sends SIGTERM then SIGKILL to processes whose command
// line references policyDir. No-op when no matching processes are found.
func killPolicyProcesses(policyDir string) {
	pids, err := findProcessesByPath(policyDir)
	if err != nil || len(pids) == 0 {
		return
	}
	for _, pid := range pids {
		terminateProcess(pid, false) // SIGTERM
	}
	time.Sleep(2 * time.Second)
	for _, pid := range pids {
		if isProcessAlive(pid) {
			terminateProcess(pid, true) // SIGKILL
		}
	}
}
