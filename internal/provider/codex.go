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
		// Use a /tmp-based npm cache so npm install never hits EPERM from the
		// workspace-write sandbox, which restricts writes to paths outside the
		// worktree (including the default ~/.npm cache directory).
		`export npm_config_cache="/tmp/aom-npm-cache-$(id -u)"`,
		// GIT_OPTIONAL_LOCKS=0: prevents git from acquiring optional lock files
		// (e.g., commit-graph.lock, FETCH_LOCK) that it does not strictly need.
		// Without this, git can spin at 95%+ CPU on WSL2 when codex's Landlock
		// sandbox restricts creation of these lock files — git retries the lock
		// in a tight loop instead of proceeding without it.
		// GIT_TERMINAL_PROMPT=0: prevents git from blocking on credential prompts
		// (which would cause git to hang indefinitely in a non-interactive sandbox).
		"export GIT_OPTIONAL_LOCKS=0",
		"export GIT_TERMINAL_PROMPT=0",
		// IMPORTANT: printf format must use hex escapes for { " } — no single quotes allowed here.
		// This preamble is assembled inside sh -lc '...' by assembleLoginShellCommand; any single
		// quote inside the preamble would prematurely close the outer quoted string, truncating the
		// script and preventing the exec codex line from ever being reached.
		// \x7b={  \x22="  \x7d=}
		`[ -f "$HOME/.codex/version.json" ] || { mkdir -p "$HOME/.codex" && printf "\x7b\x22dismissed_version\x22:\x229999.0.0\x22\x7d\n" > "$HOME/.codex/version.json"; }`,
	}
	if len(spec.DenyCommands) > 0 {
		preamble = append(preamble, buildCodexWrapperPreamble(spec.SessionID, spec.DenyCommands)...)
	}
	// codexNiceExecPrefix runs codex at niceness 19 (the highest non-root
	// deprioritisation on Linux/macOS). Combined with agents.max_threads=1
	// this gives codex the lowest possible OS-scheduler priority so it yields
	// immediately to any interactive or normal-priority process. Codex still
	// runs at full throughput when cores are free — niceness only matters when
	// the CPU is contested.
	//
	// We use a codex-specific constant (not the shared NiceExecPrefix=10) so
	// the heavier AI workload from codex is throttled more aggressively than
	// lighter runtimes like claude.
	const codexNiceExecPrefix = "exec nice -n 19 "

	// -c flags: codex v0.133.0 silently accepts any -c key=value regardless of
	// whether the key is valid — even a completely made-up key returns no error.
	// Empirical testing shows these flags may not be enforced at runtime either
	// (background terminals ran 2+ minutes despite background_terminal_max_timeout=60000).
	// The reliable fallback is ~/.codex/config.toml — see "aom doctor" which checks
	// and advises on that file.
	//
	// We still pass these flags because they DO work on some codex versions / builds,
	// and they document the intent. The preamble env vars above (GIT_OPTIONAL_LOCKS,
	// GIT_TERMINAL_PROMPT) are the primary mitigation for git spinning on WSL2.
	//
	// Flags used:
	//   agents.max_threads=1             — serialise tool execution; prevents parallel
	//                                      fan-out from spiking CPU on the host.
	//   background_terminal_max_timeout=60000 — kill stalled background terminals after
	//                                      60 s (codex default is 3 600 000 ms = 1 hr).
	//   agents.job_max_runtime_seconds=120 — hard-kill each agent turn after 2 min.
	//
	// NOTE: the -c values must NOT be wrapped in single quotes here. The entire
	// ExecCmd is joined into sh -lc '...' by assembleLoginShellCommand, which uses
	// single quotes for the outer wrapper. Any inner single quote would prematurely
	// terminate the outer quoted string. The values here contain no special
	// characters, so no quoting is needed at all.
	const codexRuntimeFlags = " -c agents.max_threads=1 -c background_terminal_max_timeout=60000 -c agents.job_max_runtime_seconds=120"

	// sandboxFlag selects between two codex sandbox modes:
	//
	//   --sandbox danger-full-access (default)
	//       Runs all subprocesses through codex-linux-sandbox → bwrap.
	//       Provides namespace isolation but on WSL2 the bwrap overlay causes
	//       git to spin at 60–100% CPU in a tight retry loop for optional lock
	//       files.  GIT_OPTIONAL_LOCKS=0 partially mitigates this but cannot
	//       reach git processes inside bwrap's mount namespace.
	//
	//   --dangerously-bypass-approvals-and-sandbox (spec.BypassSandbox=true)
	//       Skips bwrap entirely — no namespace isolation, no lock-file spinning.
	//       Safe when AOM itself is the external control boundary (it is).
	//       Required on WSL2 to prevent CPU fan-out from bwrap overlay.
	//       Enable via policy.yaml:  codex_bypass_sandbox: true
	sandboxFlag := "--sandbox danger-full-access -a never"
	if spec.BypassSandbox {
		sandboxFlag = "--dangerously-bypass-approvals-and-sandbox"
	}

	var execCmd string
	if spec.AgentSessionID != "" {
		execCmd = fmt.Sprintf(codexNiceExecPrefix+"codex resume %s %s"+codexRuntimeFlags, spec.AgentSessionID, sandboxFlag)
	} else {
		execCmd = codexNiceExecPrefix + "codex " + sandboxFlag + codexRuntimeFlags
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

	// Open a single read-only connection for the entire polling window.
	// Previously this opened and closed a new connection every second (up to 90
	// times), which caused ~100% CPU sustained during native session detection.
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return "", nil // DB not ready yet; caller will get empty session ID
	}
	defer db.Close()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var id string
		err := db.QueryRow(
			`SELECT DISTINCT thread_id FROM logs
			 WHERE thread_id IS NOT NULL AND ts >= ?
			 ORDER BY ts ASC, ts_nanos ASC LIMIT 1`,
			spawnedAt.Unix(),
		).Scan(&id)
		if err == nil && id != "" {
			return id, nil
		}
		time.Sleep(3 * time.Second) // poll every 3s — codex needs a few seconds to boot anyway
	}
	return "", nil
}
