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

// claudeEnvResetPreamble clears environment variables that Claude Code sets on
// its own process. Without this, a nested Claude agent inherits CLAUDECODE=1
// and exits immediately thinking it's already running inside a Claude Code session.
const claudeEnvResetPreamble = "unset CLAUDECODE CLAUDE_CODE_SESSION_ID CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_EXECPATH AI_AGENT CLAUDE_CODE_IS_INNER_CLAUDE_CODE 2>/dev/null"

func (p *claudeProvider) LaunchShellSpec(spec LaunchSpec, lookPath func(string) (string, error)) (ShellSpec, error) {
	if _, err := lookPath("claude"); err != nil {
		return ShellSpec{}, fmt.Errorf("real launch for runtime %q requires the %q CLI in PATH", "claude", "claude")
	}
	disallowedFlag := buildDisallowedToolsFlag(spec.DenyCommands)
	var execCmd string
	if spec.AgentSessionID != "" {
		execCmd = fmt.Sprintf("exec claude --resume %s --dangerously-skip-permissions", spec.AgentSessionID)
	} else {
		execCmd = "exec claude --dangerously-skip-permissions"
	}
	if disallowedFlag != "" {
		execCmd += " " + disallowedFlag
	}
	return ShellSpec{
		Preamble: []string{claudeEnvResetPreamble},
		ExecCmd:  execCmd,
	}, nil
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
		// Double quotes are used here so the pattern is compatible with the
		// single-quoted outer sh -lc wrapper assembled by runtime.Builder.
		patterns[i] = fmt.Sprintf(`"Bash(%s*)"`, cmd)
	}
	return "--disallowed-tools " + strings.Join(patterns, " ")
}

// claudeSessionForWorktree polls ~/.claude/projects/<path-hash>/ for the newest .jsonl
// session file whose mtime is at or after spawnedAt. Returns the UUID (filename without
// .jsonl extension) on success, or an empty string if none is found within timeout.
func claudeSessionForWorktree(worktreePath string, spawnedAt time.Time, timeout time.Duration) (string, error) {
	// Resolve the base path for the encoded directory name (used by the truncated-name fallback).
	encoded := strings.NewReplacer("/", "-", ".", "-").Replace(worktreePath)
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	projectsBase := filepath.Join(home, ".claude", "projects")

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Re-resolve each iteration: Claude may not have created the directory yet on
		// the first few polls, and the truncated-name variant only appears once Claude
		// has initialised its project state.
		projectsDir, dirErr := claudeProjectsDirForPath(worktreePath)
		if dirErr != nil {
			time.Sleep(time.Second)
			continue
		}

		entries, err := os.ReadDir(projectsDir)
		if err != nil {
			// Directory not yet created — also scan projectsBase for the truncated variant.
			if candidates, scanErr := scanForTruncatedDir(projectsBase, encoded); scanErr == nil && candidates != "" {
				projectsDir = candidates
				entries, err = os.ReadDir(projectsDir)
			}
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
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

// scanForTruncatedDir searches ~/.claude/projects/ for a directory whose name
// (after stripping a trailing "-XXXXXX" hash) is a prefix of encoded.
func scanForTruncatedDir(projectsBase, encoded string) (string, error) {
	entries, err := os.ReadDir(projectsBase)
	if err != nil {
		return "", err
	}
	var best string
	var bestLen int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) <= 7 || name[len(name)-7] != '-' {
			continue
		}
		prefix := name[:len(name)-7]
		if strings.HasPrefix(encoded, prefix) && len(prefix) > bestLen {
			best = filepath.Join(projectsBase, name)
			bestLen = len(prefix)
		}
	}
	if best == "" {
		return "", fmt.Errorf("no matching truncated directory found")
	}
	return best, nil
}

// claudeProjectsDirForPath returns the ~/.claude/projects/ subdirectory that Claude
// uses for the given worktree path. Claude encodes the path by replacing every '/'
// and '.' character with '-'. For long paths Claude truncates the name and appends
// a short hash suffix, so we fall back to prefix matching when the exact name is absent.
func claudeProjectsDirForPath(worktreePath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}

	encoded := strings.NewReplacer("/", "-", ".", "-").Replace(worktreePath)
	projectsBase := filepath.Join(home, ".claude", "projects")
	exact := filepath.Join(projectsBase, encoded)

	// Fast path: exact match exists.
	if _, err := os.Stat(exact); err == nil {
		return exact, nil
	}

	// Fallback: Claude may have truncated the directory name and appended a hash.
	// Find the longest-prefix match inside ~/.claude/projects/.
	entries, err := os.ReadDir(projectsBase)
	if err != nil {
		// Directory doesn't exist yet; return the exact path so the caller can poll.
		return exact, nil
	}
	// Claude truncates long directory names and appends a "-XXXXXX" 6-char hash.
	// Strip that suffix and check whether the remaining prefix is a prefix of our
	// full encoded name, picking the longest match to avoid false positives.
	var best string
	var bestLen int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) <= 7 || name[len(name)-7] != '-' {
			continue
		}
		prefix := name[:len(name)-7]
		if strings.HasPrefix(encoded, prefix) && len(prefix) > bestLen {
			best = filepath.Join(projectsBase, name)
			bestLen = len(prefix)
		}
	}
	if best != "" {
		return best, nil
	}

	return exact, nil
}
