package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type claudeProvider struct{}

func (p *claudeProvider) Name() string            { return "claude" }
func (p *claudeProvider) IdentityFilename() string { return "CLAUDE.md" }

func (p *claudeProvider) LaunchCommand(spec LaunchSpec, lookPath func(string) (string, error)) (string, error) {
	if _, err := lookPath("claude"); err != nil {
		return "", fmt.Errorf("real launch for runtime %q requires the %q CLI in PATH", "claude", "claude")
	}
	disallowedFlag := buildDisallowedToolsFlag(spec.DenyCommands)
	if spec.AgentSessionID != "" {
		if disallowedFlag != "" {
			return fmt.Sprintf("sh -lc 'exec claude --resume %s --dangerously-skip-permissions %s'", spec.AgentSessionID, disallowedFlag), nil
		}
		return fmt.Sprintf("sh -lc 'exec claude --resume %s --dangerously-skip-permissions'", spec.AgentSessionID), nil
	}
	if disallowedFlag != "" {
		return fmt.Sprintf("sh -lc 'exec claude --dangerously-skip-permissions %s'", disallowedFlag), nil
	}
	return "sh -lc 'exec claude --dangerously-skip-permissions'", nil
}

func (p *claudeProvider) ResumeInfo() ResumeInfo {
	return ResumeInfo{
		Supported:     true,
		FreshExample:  "claude --dangerously-skip-permissions",
		ResumeExample: "claude --resume <session-uuid> --dangerously-skip-permissions",
	}
}

func (p *claudeProvider) MCPConfigStyle() MCPStyle                 { return MCPStyleMarkdownAppend }
func (p *claudeProvider) PolicyEnforcementLevel() PolicyEnforcement { return PolicyEnforcementRuntimeFlag }

func (p *claudeProvider) NativeSessionDetection() *NativeSessionStrategy {
	return &NativeSessionStrategy{DetectFn: claudeSessionForWorktree}
}

// buildDisallowedToolsFlag converts deny_commands into a --disallowed-tools flag string.
// Each command cmd becomes 'Bash(cmd*)'. Returns "" when no commands are given.
func buildDisallowedToolsFlag(denyCommands []string) string {
	if len(denyCommands) == 0 {
		return ""
	}
	patterns := make([]string, len(denyCommands))
	for i, cmd := range denyCommands {
		patterns[i] = fmt.Sprintf("'Bash(%s*)'", cmd)
	}
	return "--disallowed-tools " + strings.Join(patterns, " ")
}

// claudeSessionForWorktree polls ~/.claude/projects/<path-hash>/ for the newest .jsonl
// session file whose mtime is at or after spawnedAt. Returns the UUID (filename without
// .jsonl extension) on success, or an empty string if none is found within timeout.
func claudeSessionForWorktree(worktreePath string, spawnedAt time.Time, timeout time.Duration) (string, error) {
	projectsDir, err := claudeProjectsDirForPath(worktreePath)
	if err != nil {
		return "", err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(projectsDir)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		var newest string
		var newestTime time.Time
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(spawnedAt) {
				continue
			}
			if newest == "" || info.ModTime().After(newestTime) {
				newest = strings.TrimSuffix(entry.Name(), ".jsonl")
				newestTime = info.ModTime()
			}
		}

		if newest != "" {
			return newest, nil
		}

		time.Sleep(time.Second)
	}

	return "", nil
}

// claudeProjectsDirForPath returns the ~/.claude/projects/ subdirectory that Claude
// uses for the given worktree path. Claude encodes the path by replacing every '/'
// and '.' character with '-'.
func claudeProjectsDirForPath(worktreePath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}

	encoded := strings.NewReplacer("/", "-", ".", "-").Replace(worktreePath)
	return filepath.Join(home, ".claude", "projects", encoded), nil
}
