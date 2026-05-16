package provider

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type codexProvider struct{}

func (p *codexProvider) Name() string            { return "codex" }
func (p *codexProvider) IdentityFilename() string { return "AGENTS.md" }

func (p *codexProvider) LaunchCommand(spec LaunchSpec, lookPath func(string) (string, error)) (string, error) {
	if _, err := lookPath("codex"); err != nil {
		return "", fmt.Errorf("real launch for runtime %q requires the %q CLI in PATH", "codex", "codex")
	}
	if spec.AgentSessionID != "" {
		return fmt.Sprintf("sh -lc 'exec codex resume %s --sandbox workspace-write'", spec.AgentSessionID), nil
	}
	return "sh -lc 'exec codex --sandbox workspace-write'", nil
}

func (p *codexProvider) ResumeInfo() ResumeInfo {
	return ResumeInfo{
		Supported:     true,
		FreshExample:  "codex --sandbox workspace-write",
		ResumeExample: "codex resume <session-id> --sandbox workspace-write",
	}
}

func (p *codexProvider) MCPConfigStyle() MCPStyle                  { return MCPStyleJSONFile }
func (p *codexProvider) PolicyEnforcementLevel() PolicyEnforcement { return PolicyEnforcementInstructionOnly }

func (p *codexProvider) NativeSessionDetection() *NativeSessionStrategy {
	return &NativeSessionStrategy{DetectFn: codexSessionAfterSpawn}
}

// codexSessionAfterSpawn polls ~/.codex/logs_2.sqlite for the first thread_id
// that appears at or after spawnedAt. Returns the session UUID on success, or
// an empty string if none is found within timeout.
func codexSessionAfterSpawn(_ string, spawnedAt time.Time, timeout time.Duration) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	dbPath := filepath.Join(home, ".codex", "logs_2.sqlite")

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if id := queryNewestCodexSession(dbPath, spawnedAt); id != "" {
			return id, nil
		}
		time.Sleep(time.Second)
	}
	return "", nil
}

// queryNewestCodexSession opens codex's logs_2.sqlite read-only and returns the
// first thread_id logged at or after spawnedAt, or "" if none found yet.
func queryNewestCodexSession(dbPath string, spawnedAt time.Time) string {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return ""
	}
	defer db.Close()

	var id string
	if err := db.QueryRow(
		`SELECT DISTINCT thread_id FROM logs
		 WHERE thread_id IS NOT NULL AND ts >= ?
		 ORDER BY ts ASC, ts_nanos ASC LIMIT 1`,
		spawnedAt.Unix(),
	).Scan(&id); err != nil {
		return ""
	}
	return id
}
