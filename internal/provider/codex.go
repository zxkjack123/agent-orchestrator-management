package provider

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type codexProvider struct{}

func (p *codexProvider) Name() string            { return "codex" }
func (p *codexProvider) IdentityFilename() string { return "AGENTS.md" }

func (p *codexProvider) LaunchShellSpec(spec LaunchSpec, lookPath func(string) (string, error)) (ShellSpec, error) {
	if _, err := lookPath("codex"); err != nil {
		return ShellSpec{}, fmt.Errorf("real launch for runtime %q requires the %q CLI in PATH", "codex", "codex")
	}
	preamble := []string{
		"export AOM_RUNTIME=codex",
		"export PYTHONDONTWRITEBYTECODE=1",
		`[ -f "$HOME/.codex/version.json" ] || { mkdir -p "$HOME/.codex" && printf '{"dismissed_version":"9999.0.0"}\n' > "$HOME/.codex/version.json"; }`,
	}
	if len(spec.DenyCommands) > 0 {
		preamble = append(preamble, buildCodexWrapperPreamble(spec.SessionID, spec.DenyCommands)...)
	}
	var execCmd string
	if spec.AgentSessionID != "" {
		execCmd = fmt.Sprintf("exec codex resume %s --sandbox workspace-write -a never -c 'sandbox_workspace_write.network_access=true'", spec.AgentSessionID)
	} else {
		execCmd = "exec codex --sandbox workspace-write -a never -c 'sandbox_workspace_write.network_access=true'"
	}
	if spec.Model != "" {
		execCmd += " -m " + spec.Model
	}
	return ShellSpec{
		Preamble: preamble,
		ExecCmd:  execCmd,
	}, nil
}

// buildCodexWrapperPreamble generates preamble statements that create lightweight
// shell wrapper scripts blocking each denied command. The wrapper bin dir is
// prepended to PATH before exec, so codex and its subprocesses intercept the
// blocked commands at the shell level.
//
// For base commands where ALL deny entries are multi-word (e.g. "git push --force"),
// a smart wrapper is generated that checks $1 against the blocked subcommand and
// passes through to the real binary for non-matching args.
//
// For base commands where ANY deny entry is single-word (e.g. "rm"), a full-block
// wrapper is generated that always exits 1.
//
// The bin dir is session-scoped under /tmp so the OS cleans it up on reboot.
func buildCodexWrapperPreamble(sessionID string, denyCommands []string) []string {
	binDir := fmt.Sprintf("/tmp/aom-policy-%s/bin", sessionID)
	stmts := []string{
		fmt.Sprintf(`mkdir -p "%s"`, binDir),
	}

	// Group entries by base command.
	type denyEntry struct {
		raw     string
		baseCmd string
		subArg  string // first arg after base command (empty for single-word entries)
	}
	var entries []denyEntry
	for _, rawCmd := range denyCommands {
		cmd := strings.TrimSpace(rawCmd)
		if cmd == "" {
			continue
		}
		fields := strings.Fields(cmd)
		e := denyEntry{raw: cmd, baseCmd: fields[0]}
		if len(fields) > 1 {
			e.subArg = fields[1]
		}
		entries = append(entries, e)
	}

	// For each base command, determine whether ALL entries are multi-word.
	baseCmdAllMulti := make(map[string]bool)
	baseCmdSeen := make(map[string]bool)
	for _, e := range entries {
		if !baseCmdSeen[e.baseCmd] {
			baseCmdAllMulti[e.baseCmd] = true
			baseCmdSeen[e.baseCmd] = true
		}
		if e.subArg == "" {
			baseCmdAllMulti[e.baseCmd] = false
		}
	}

	// Build wrapper for each base command (deduplicated by base command).
	wrapperBuilt := make(map[string]bool)
	for _, e := range entries {
		if wrapperBuilt[e.baseCmd] {
			continue
		}
		wrapperBuilt[e.baseCmd] = true

		if !baseCmdAllMulti[e.baseCmd] {
			// Full-block wrapper.
			// printf format: double quotes wrap the format so \n and \" are shell-processed.
			stmts = append(stmts,
				fmt.Sprintf(
					`printf "#!/bin/sh\necho \"AOM policy: %s blocked by project policy\" >&2\nexit 1\n" > "%s/%s" && chmod +x "%s/%s"`,
					e.baseCmd, binDir, e.baseCmd, binDir, e.baseCmd,
				),
			)
		} else {
			// Smart wrapper: check $1 against each blocked subcommand.
			// Collect all unique subargs for this base command.
			seenSubArg := make(map[string]bool)
			var checkLines []string
			for _, e2 := range entries {
				if e2.baseCmd != e.baseCmd {
					continue
				}
				if seenSubArg[e2.subArg] {
					continue
				}
				seenSubArg[e2.subArg] = true
				// Use \$1 and \$@ so they are not expanded when the printf command itself is evaluated.
				// Use \"...\" for literal double-quotes inside the format string.
				checkLines = append(checkLines,
					fmt.Sprintf(`if [ \"\$1\" = \"%s\" ]; then echo \"AOM policy: %s %s blocked by project policy\" >&2; exit 1; fi`,
						e2.subArg, e.baseCmd, e2.subArg),
				)
			}
			// Pass-through line: strip our wrapper binDir from PATH to avoid infinite recursion.
			passThroughLine := fmt.Sprintf(`exec env \"PATH=\${PATH#%s:}\" %s \"\$@\"`, binDir, e.baseCmd)

			body := strings.Join(checkLines, `\n`) + `\n` + passThroughLine

			stmts = append(stmts,
				fmt.Sprintf(
					`printf "#!/bin/sh\n%s\n" > "%s/%s" && chmod +x "%s/%s"`,
					body, binDir, e.baseCmd, binDir, e.baseCmd,
				),
			)
		}
	}

	stmts = append(stmts, fmt.Sprintf(`export PATH="%s:$PATH"`, binDir))
	return stmts
}

func (p *codexProvider) ResumeInfo() ResumeInfo {
	return ResumeInfo{
		Supported:     true,
		FreshExample:  "codex --sandbox workspace-write -c 'sandbox_workspace_write.network_access=true'",
		ResumeExample: "codex resume <session-id> --sandbox workspace-write -c 'sandbox_workspace_write.network_access=true'",
	}
}

func (p *codexProvider) MCPConfigStyle() MCPStyle                  { return MCPStyleJSONFile }
func (p *codexProvider) PolicyEnforcementLevel() PolicyEnforcement { return PolicyEnforcementWrapperScript }
// StartupDialogResponse returns "1" to accept codex's directory trust dialog
// ("1. Yes, continue") shown on fresh starts in new or untrusted directories.
func (p *codexProvider) StartupDialogResponse() string { return "1" }

func (p *codexProvider) ModelHint() string {
	return "Known slugs for ChatGPT account: gpt-5.5, gpt-5.4, gpt-5.4-mini, gpt-5.3-codex, gpt-5.2. " +
		"Note: gpt-4.x series (gpt-4o, gpt-4.1, gpt-4.1-mini) require an OpenAI API account, not a ChatGPT account. " +
		"Full list cached at ~/.codex/models_cache.json (auto-refreshed by codex on startup)."
}

func (p *codexProvider) KnownModels() []string {
	return []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"}
}

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
