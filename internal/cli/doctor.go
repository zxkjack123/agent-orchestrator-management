package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/provider"
)

type doctorResult struct {
	label   string
	detail  string
	ok      bool
	warning bool
}

func (d doctorResult) prefix() string {
	if d.ok {
		return "[PASS]"
	}
	if d.warning {
		return "[WARN]"
	}
	return "[FAIL]"
}

func (r Runner) executeDoctor(args []string) error {
	globalOnly := false
	fixMode := false
	for _, arg := range args {
		switch arg {
		case "--global":
			globalOnly = true
		case "--fix":
			fixMode = true
		}
	}

	if fixMode {
		return r.executeDoctorFix()
	}

	var results []doctorResult

	// ── Environment ──────────────────────────────────────────────────────────
	avail := r.app.Tmux.Availability()
	if avail.Available {
		results = append(results, doctorResult{
			label:  "tmux",
			detail: avail.BinaryPath,
			ok:     true,
		})
	} else {
		results = append(results, doctorResult{
			label:  "tmux",
			detail: "not found in PATH — required for session management",
		})
	}

	aomPath, aomLookErr := exec.LookPath("aom")
	if aomLookErr != nil {
		results = append(results, doctorResult{
			label:   "aom in PATH",
			detail:  "not found — agents cannot run \"aom\" commands; add the binary to your PATH or symlink to /usr/local/bin/aom",
			warning: true,
		})
	} else {
		results = append(results, doctorResult{
			label:  "aom in PATH",
			detail: aomPath,
			ok:     true,
		})
	}

	// Multi-binary check: warn when the running binary and the PATH-resolved aom
	// are in different directories — agents will then use a different (possibly
	// older) build than the operator, causing version-skew failures.
	if aomLookErr == nil {
		if exePath, err := os.Executable(); err == nil {
			if warn := multiBinaryCheck(exePath, aomPath); warn != nil {
				results = append(results, *warn)
			}
		}
	}

	if globalOnly {
		// Check all 4 known provider runtimes in PATH.
		for _, rt := range []string{"claude", "codex", "gemini", "kiro"} {
			path, err := exec.LookPath(rt)
			if err != nil {
				results = append(results, doctorResult{
					label:  fmt.Sprintf("runtime: %s", rt),
					detail: "not found in PATH",
				})
			} else {
				results = append(results, doctorResult{
					label:  fmt.Sprintf("runtime: %s", rt),
					detail: path,
					ok:     true,
				})
			}
		}

		// Print results and return early.
		fmt.Fprintln(r.stdout, "AOM Doctor")
		fmt.Fprintln(r.stdout, "==========")
		fmt.Fprintln(r.stdout, "")

		passed, failed, warned := 0, 0, 0
		for _, res := range results {
			fmt.Fprintf(r.stdout, "  %-6s %-22s %s\n", res.prefix(), res.label, res.detail)
			switch {
			case res.ok:
				passed++
			case res.warning:
				warned++
			default:
				failed++
			}
		}

		fmt.Fprintln(r.stdout, "")
		summary := fmt.Sprintf("Summary: %d passed", passed)
		if warned > 0 {
			summary += fmt.Sprintf(", %d warning", warned)
			if warned > 1 {
				summary += "s"
			}
		}
		if failed > 0 {
			summary += fmt.Sprintf(", %d failed", failed)
		}
		fmt.Fprintln(r.stdout, summary)

		if failed > 0 {
			return fmt.Errorf("doctor found %d issue(s)", failed)
		}
		return nil
	}

	var cfg *config.ProjectConfig

	// ── Project config ────────────────────────────────────────────────────────
	aomDir := filepath.Join(".", ".aom")
	if _, err := os.Stat(aomDir); os.IsNotExist(err) {
		results = append(results, doctorResult{
			label:  "project config",
			detail: ".aom/ directory not found — run \"aom project init\" first",
		})
	} else {
		loaded, err := config.LoadProjectConfig(".")
		if err != nil {
			results = append(results, doctorResult{
				label:  "project config",
				detail: fmt.Sprintf("failed to load: %v", err),
			})
		} else {
			cfg = loaded
			results = append(results, doctorResult{
				label:  "project config",
				detail: fmt.Sprintf("project=%q  branch=%s", cfg.Project.Name, cfg.Project.DefaultBranch),
				ok:     true,
			})
		}
	}

	// ── Git: initial commit ───────────────────────────────────────────────────
	if cfg != nil {
		if _, lookErr := exec.LookPath("git"); lookErr == nil {
			out, err := exec.Command("git", "-C", ".", "rev-parse", "--verify", "HEAD").Output()
			if err != nil {
				results = append(results, doctorResult{
					label:  "git: initial commit",
					detail: `none — task worktrees cannot be provisioned; fix: git commit --allow-empty -m "initial"`,
				})
			} else {
				sha := strings.TrimSpace(string(out))
				if len(sha) > 8 {
					sha = sha[:8]
				}
				results = append(results, doctorResult{label: "git: initial commit", detail: sha, ok: true})
			}

			// ── Git: identity ────────────────────────────────────────────────────
			emailOut, emailErr := exec.Command("git", "config", "--get", "user.email").Output()
			nameOut, nameErr := exec.Command("git", "config", "--get", "user.name").Output()
			emailSet := emailErr == nil && len(strings.TrimSpace(string(emailOut))) > 0
			nameSet := nameErr == nil && len(strings.TrimSpace(string(nameOut))) > 0
			if !emailSet || !nameSet {
				results = append(results, doctorResult{
					label:  "git: identity",
					detail: `user.name or user.email not set — commits will fail; fix: git config --global user.name "Your Name" && git config --global user.email "you@example.com"`,
				})
			} else {
				results = append(results, doctorResult{
					label:  "git: identity",
					detail: fmt.Sprintf("%s <%s>", strings.TrimSpace(string(nameOut)), strings.TrimSpace(string(emailOut))),
					ok:     true,
				})
			}
		}
	}

	// ── .aom/ files tracked in git ───────────────────────────────────────────
	// If .aom/sessions.db or .aom/channel.md are tracked in git, git add -A
	// will stage the binary SQLite DB (often 50-200KB) on every commit.
	// This causes git operations to be slow and makes codex background terminals
	// accumulate while waiting for git — leading to high system load.
	if _, lookErr := exec.LookPath("git"); lookErr == nil {
		tracked, lsErr := exec.Command("git", "-C", ".", "ls-files", ".aom/sessions.db", ".aom/channel.md").Output()
		if lsErr == nil && len(strings.TrimSpace(string(tracked))) > 0 {
			results = append(results, doctorResult{
				label:   "git: .aom/ tracked",
				detail:  ".aom/sessions.db or .aom/channel.md is committed — will be staged on every git add -A, causing slow git and agent background terminal accumulation; fix: git rm --cached .aom/sessions.db .aom/channel.md && echo '.aom/' >> .gitignore",
				warning: true,
			})
		}
	}

	// ── NTFS mount detection ──────────────────────────────────────────────────
	// WSL2 mounts Windows NTFS volumes under /mnt/. Git lock files on NTFS are
	// read-only, which causes "index.lock: Read-only file system" inside worktrees.
	if cfg != nil {
		repoSlash := filepath.ToSlash(cfg.RootPath)
		if strings.HasPrefix(repoSlash, "/mnt/") {
			results = append(results, doctorResult{
				label:   "git: NTFS mount",
				detail:  "repo is under /mnt/ (WSL2→NTFS) — use \"aom worktree commit <task-id>\" instead of git commit in worktrees",
				warning: true,
			})
		}
	}

	// ── .aom/ writable ────────────────────────────────────────────────────────
	if cfg != nil {
		probe := filepath.Join(cfg.AOMPath, ".doctor-probe")
		if err := os.WriteFile(probe, []byte(""), 0o644); err != nil {
			results = append(results, doctorResult{
				label:  ".aom/ writable",
				detail: fmt.Sprintf("write failed: %v", err),
			})
		} else {
			_ = os.Remove(probe)
			results = append(results, doctorResult{
				label:  ".aom/ writable",
				detail: cfg.AOMPath,
				ok:     true,
			})
		}
	}

	// ── Database ──────────────────────────────────────────────────────────────
	var dbPath string
	if cfg != nil {
		dbPath = filepath.Join(cfg.AOMPath, "sessions.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			results = append(results, doctorResult{
				label:   "database",
				detail:  "sessions.db not found — run \"aom project init\" to bootstrap",
				warning: true,
			})
		} else {
			f, werr := os.OpenFile(dbPath, os.O_WRONLY, 0)
			if werr != nil {
				results = append(results, doctorResult{
					label:   "database",
					detail:  fmt.Sprintf("sessions.db not writable — agent sandbox commands will fail (fix: chmod 664 %s)", dbPath),
					warning: true,
				})
			} else {
				_ = f.Close()
				results = append(results, doctorResult{
					label:  "database",
					detail: "sessions.db present and writable",
					ok:     true,
				})
			}
		}
	}

	// ── Runtime binaries ──────────────────────────────────────────────────────
	if cfg != nil {
		runtimeAgents := buildRuntimeAgentMap(cfg)
		for _, rt := range sortedKeys(runtimeAgents) {
			agents := runtimeAgents[rt]
			agentList := strings.Join(agents, ", ")
			path, err := exec.LookPath(rt)
			if err != nil {
				results = append(results, doctorResult{
					label:  fmt.Sprintf("runtime: %s", rt),
					detail: fmt.Sprintf("not found in PATH  (used by: %s)", agentList),
				})
			} else {
				results = append(results, doctorResult{
					label:  fmt.Sprintf("runtime: %s", rt),
					detail: fmt.Sprintf("%s  (used by: %s)", path, agentList),
					ok:     true,
				})
			}
		}
	}

	// ── Codex: update dialog suppression ─────────────────────────────────────
	if cfg != nil {
		if _, lookErr := exec.LookPath("codex"); lookErr == nil {
			home, _ := os.UserHomeDir()
			versionFile := filepath.Join(home, ".codex", "version.json")
			data, readErr := os.ReadFile(versionFile)
			hasDismissed := false
			if readErr == nil {
				var v map[string]any
				if jsonErr := json.Unmarshal(data, &v); jsonErr == nil {
					if val, ok := v["dismissed_version"]; ok && fmt.Sprintf("%v", val) != "" {
						hasDismissed = true
					}
				}
			}
			if !hasDismissed {
				results = append(results, doctorResult{
					label:   "codex: update dialog",
					detail:  `dismissed_version not set — update prompt may block session spawn; fix: printf '{"dismissed_version":"9999.0.0"}\n' > ~/.codex/version.json`,
					warning: true,
				})
			} else {
				results = append(results, doctorResult{
					label:  "codex: update dialog",
					detail: "dismissed_version set",
					ok:     true,
				})
			}

			// ── Codex: background_terminal_max_timeout ────────────────────────────
			// codex v0.133.0 silently ignores -c flags passed on the command line.
			// The only reliable way to enforce background_terminal_max_timeout is via
			// ~/.codex/config.toml. Without it, stuck background bash terminals (e.g.
			// a git command spinning on WSL2) survive for the codex default of
			// 3,600,000 ms (1 hour), continuously consuming 40–100% CPU.
			home, _ = os.UserHomeDir()
			globalCfgPath := filepath.Join(home, ".codex", "config.toml")
			globalCfgData, cfgReadErr := os.ReadFile(globalCfgPath)
			hasBgTimeout := cfgReadErr == nil && strings.Contains(string(globalCfgData), "background_terminal_max_timeout")
			if !hasBgTimeout {
				results = append(results, doctorResult{
					label:   "codex: bg terminal timeout",
					detail:  `background_terminal_max_timeout not set in ~/.codex/config.toml — stuck git/shell processes can spin at 100% CPU for up to 1 hour; fix: add 'background_terminal_max_timeout = 60000' to ~/.codex/config.toml`,
					warning: true,
				})
			} else {
				results = append(results, doctorResult{
					label:  "codex: bg terminal timeout",
					detail: "background_terminal_max_timeout set in ~/.codex/config.toml",
					ok:     true,
				})
			}

			// ── Codex: WSL2 bwrap bypass ──────────────────────────────────────────
			// AOM auto-detects WSL2 at codex launch time (provider reads /proc/version)
			// and applies --dangerously-bypass-approvals-and-sandbox automatically.
			// No policy.yaml entry is required. codex_bypass_sandbox: true is still
			// honoured for non-WSL2 environments that also want to skip bwrap.
			procVersion, pvErr := os.ReadFile("/proc/version")
			isWSL2 := pvErr == nil && (strings.Contains(strings.ToLower(string(procVersion)), "microsoft") ||
				strings.Contains(strings.ToLower(string(procVersion)), "wsl"))
			if isWSL2 {
				detail := "WSL2 detected — bwrap bypass applied automatically (no policy.yaml change needed)"
				if cfg.Policy.Policy.CodexBypassSandbox {
					detail = "WSL2 detected — bwrap bypass applied automatically (also set explicitly in policy.yaml)"
				}
				results = append(results, doctorResult{
					label:  "codex: wsl2 bypass",
					detail: detail,
					ok:     true,
				})
			}
		}
	}

	// ── Active worktrees ──────────────────────────────────────────────────────
	if cfg != nil && dbPath != "" {
		wtService, sqlDB, err := r.app.OpenWorktreeService(dbPath)
		if err == nil {
			defer sqlDB.Close()
			projectID := sanitizeProjectID(cfg.Project.Name)
			records, err := wtService.ListByProject(projectID)
			if err == nil {
				for _, wt := range records {
					if wt.Status != "Active" && wt.Status != "Ready" {
						continue
					}
					if _, err := os.Stat(wt.WorktreePath); os.IsNotExist(err) {
						results = append(results, doctorResult{
							label:  fmt.Sprintf("worktree: %s", wt.TaskID),
							detail: fmt.Sprintf("%s  (status: %s, path missing — run \"aom worktree repair %s\")", wt.WorktreePath, wt.Status, wt.TaskID),
						})
					} else {
						results = append(results, doctorResult{
							label:  fmt.Sprintf("worktree: %s", wt.TaskID),
							detail: fmt.Sprintf("%s  (status: %s)", wt.WorktreePath, wt.Status),
							ok:     true,
						})
					}
				}
			}
		}
	}

	// ── Hooks ─────────────────────────────────────────────────────────────────
	if cfg != nil {
		hooksDir := filepath.Join(cfg.AOMPath, "hooks")
		if entries, err := os.ReadDir(hooksDir); err == nil {
			exampleOnly := []string{}
			for _, e := range entries {
				name := e.Name()
				if !strings.HasSuffix(name, ".sh.example") {
					continue
				}
				live := strings.TrimSuffix(name, ".example")
				livePath := filepath.Join(hooksDir, live)
				if _, statErr := os.Stat(livePath); os.IsNotExist(statErr) {
					exampleOnly = append(exampleOnly, name)
				}
			}
			if len(exampleOnly) > 0 {
				results = append(results, doctorResult{
					label:   "hooks",
					detail:  fmt.Sprintf("%d .sh.example file(s) not activated: %s — copy without .example suffix and chmod +x to enable", len(exampleOnly), strings.Join(exampleOnly, ", ")),
					warning: true,
				})
			} else {
				// Check that at least on-task-done.sh exists
				livePath := filepath.Join(hooksDir, "on-task-done.sh")
				if _, err := os.Stat(livePath); err == nil {
					results = append(results, doctorResult{
						label:  "hooks",
						detail: "on-task-done.sh present",
						ok:     true,
					})
				}
			}
		}
	}

	// ── Agent model field ─────────────────────────────────────────────────────
	if cfg != nil {
		agentsPath := filepath.Join(cfg.AOMPath, "agents.yaml")
		if rawData, readErr := os.ReadFile(agentsPath); readErr == nil {
			modelFieldCount := strings.Count(string(rawData), "\n    model:")
			agentCount := len(cfg.Agents.Agents)
			if agentCount > 0 && modelFieldCount < agentCount {
				missing := agentCount - modelFieldCount
				results = append(results, doctorResult{
					label:   "agents: model field",
					detail:  fmt.Sprintf("%d agent(s) missing model: field — run \"aom doctor --fix\" to auto-repair agents.yaml", missing),
					warning: true,
				})
			} else if agentCount > 0 {
				results = append(results, doctorResult{
					label:  "agents: model field",
					detail: "all agents have model: field",
					ok:     true,
				})
			}
		}
	}

	// ── Same-runtime workspace isolation ─────────────────────────────────────
	// Multiple enabled agents sharing the same runtime (e.g., two claude agents)
	// must each have a dedicated workspace.  Without it they both write CLAUDE.md /
	// AGENTS.md to the repo root and overwrite each other's identity files.
	if cfg != nil && dbPath != "" {
		agentRepo, agentDB, agentRepoErr := r.app.OpenAgentRepository(dbPath)
		if agentRepoErr == nil {
			projectID := sanitizeProjectID(cfg.Project.Name)
			agentRecs, listErr := agentRepo.ListByProjectID(projectID)
			agentDB.Close()
			if listErr == nil {
				// Group enabled agents by runtime; track which lack a workspace.
				type runtimeGroup struct {
					total    int
					noWs     []string
				}
				groups := make(map[string]*runtimeGroup)
				for _, ag := range agentRecs {
					if !ag.Enabled {
						continue
					}
					if groups[ag.Runtime] == nil {
						groups[ag.Runtime] = &runtimeGroup{}
					}
					groups[ag.Runtime].total++
					if strings.TrimSpace(ag.WorkspacePath) == "" {
						groups[ag.Runtime].noWs = append(groups[ag.Runtime].noWs, ag.Name)
					}
				}
				for rt, grp := range groups {
					if grp.total > 1 && len(grp.noWs) > 0 {
						cmds := make([]string, len(grp.noWs))
						for i, n := range grp.noWs {
							cmds[i] = "aom agent provision " + n
						}
						results = append(results, doctorResult{
							label:   fmt.Sprintf("workspace: %s", rt),
							detail:  fmt.Sprintf("%d %s agent(s) lack a workspace: %s — run: %s", len(grp.noWs), rt, strings.Join(grp.noWs, ", "), strings.Join(cmds, " && ")),
							warning: true,
						})
					} else if grp.total > 1 {
						results = append(results, doctorResult{
							label:  fmt.Sprintf("workspace: %s", rt),
							detail: fmt.Sprintf("all %d %s agents have dedicated workspaces", grp.total, rt),
							ok:     true,
						})
					}
				}
			}
		}
	}

	// ── Stale policy dirs + session count ────────────────────────────────────
	if cfg != nil && dbPath != "" {
		sessService, sessDB, sessErr := r.app.OpenSessionService(dbPath)
		if sessErr == nil {
			defer sessDB.Close()
			activeIDs := make(map[string]bool)
			if sessions, listErr := sessService.ListByProject(sanitizeProjectID(cfg.Project.Name)); listErr == nil {
				for _, s := range sessions {
					switch s.Status {
					case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked", "NeedsAttention":
						activeIDs[s.ID] = true
					}
				}
			}
			if staleDirs, scanErr := provider.ScanStalePolicyDirs(activeIDs); scanErr == nil && len(staleDirs) > 0 {
				results = append(results, doctorResult{
					label:   "policy-dirs",
					detail:  fmt.Sprintf("%d stale policy dir(s) in /tmp — run \"aom session cleanup --stale\" to remove", len(staleDirs)),
					warning: true,
				})
			}

			// Warn when active sessions outnumber enabled agents — a sign that orphan
			// sessions are accumulating and consuming RAM/CPU unnecessarily.
			enabledAgents := 0
			for _, a := range cfg.Agents.Agents {
				if a.Enabled {
					enabledAgents++
				}
			}
			activeSessCount := len(activeIDs)
			if enabledAgents > 0 && activeSessCount > enabledAgents {
				results = append(results, doctorResult{
					label:   "session-count",
					detail:  fmt.Sprintf("%d active session(s) for %d enabled agent(s) — possible orphans; run \"aom session list\" then \"aom session stop <id>\" for any unneeded sessions", activeSessCount, enabledAgents),
					warning: true,
				})
			}
		}
	}

	// ── Print results ─────────────────────────────────────────────────────────
	fmt.Fprintln(r.stdout, "AOM Doctor")
	fmt.Fprintln(r.stdout, "==========")
	fmt.Fprintln(r.stdout, "")

	passed, failed, warned := 0, 0, 0
	for _, res := range results {
		fmt.Fprintf(r.stdout, "  %-6s %-22s %s\n", res.prefix(), res.label, res.detail)
		switch {
		case res.ok:
			passed++
		case res.warning:
			warned++
		default:
			failed++
		}
	}

	fmt.Fprintln(r.stdout, "")
	summary := fmt.Sprintf("Summary: %d passed", passed)
	if warned > 0 {
		summary += fmt.Sprintf(", %d warning", warned)
		if warned > 1 {
			summary += "s"
		}
	}
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}
	fmt.Fprintln(r.stdout, summary)

	if failed > 0 {
		return fmt.Errorf("doctor found %d issue(s)", failed)
	}
	return nil
}

// buildRuntimeAgentMap returns a map of runtime name → slice of agent names.
func buildRuntimeAgentMap(cfg *config.ProjectConfig) map[string][]string {
	m := make(map[string][]string)
	for agentName, agentCfg := range cfg.Agents.Agents {
		if !agentCfg.Enabled {
			continue
		}
		m[agentCfg.Runtime] = append(m[agentCfg.Runtime], agentName)
	}
	return m
}

// sanitizeProjectID mirrors project.sanitizeName to derive the DB project ID.
func sanitizeProjectID(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")

	var b strings.Builder
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			b.WriteRune(ch)
		}
	}

	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "aom-project"
	}
	return result
}

// executeDoctorFix auto-fixes known permission issues:
//   - .agent/ directories inside worktrees: chmod 755
//   - .agent/*.md files inside worktrees: chmod 664
//   - sessions.db: chmod 664
func (r Runner) executeDoctorFix() error {
	cfg, err := config.LoadProjectConfig(".")
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}

	fixed := 0
	failed := 0

	fix := func(path string, mode os.FileMode) {
		if err := os.Chmod(path, mode); err != nil {
			fmt.Fprintf(r.stdout, "  FAIL  %s: %v\n", path, err)
			failed++
		} else {
			fmt.Fprintf(r.stdout, "  FIXED %s → %04o\n", path, mode)
			fixed++
		}
	}

	fmt.Fprintln(r.stdout, "AOM Doctor --fix")
	fmt.Fprintln(r.stdout, "================")
	fmt.Fprintln(r.stdout, "")

	// Re-serialize agents.yaml to ensure model: field is present for all agents.
	if err := project.RepairAgentsFile(cfg.AOMPath); err != nil {
		fmt.Fprintf(r.stdout, "  FAIL  agents.yaml repair: %v\n", err)
		failed++
	} else {
		fmt.Fprintf(r.stdout, "  FIXED agents.yaml — model: field ensured for all agents\n")
		fixed++
	}

	// Fix sessions.db permissions.
	dbPath := filepath.Join(cfg.AOMPath, "sessions.db")
	if _, err := os.Stat(dbPath); err == nil {
		fix(dbPath, 0o664)
	}

	// Walk worktree directories and fix .agent/ dirs and their files.
	worktreesRoot := filepath.Join(cfg.AOMPath, "worktrees")
	entries, err := os.ReadDir(worktreesRoot)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read worktrees dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentDir := filepath.Join(worktreesRoot, entry.Name(), ".agent")
		if _, err := os.Stat(agentDir); os.IsNotExist(err) {
			continue
		}
		fix(agentDir, 0o755)

		files, err := os.ReadDir(agentDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() {
				fix(filepath.Join(agentDir, f.Name()), 0o664)
			}
		}
	}

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Fixed: %d  Failed: %d\n", fixed, failed)
	if failed > 0 {
		return fmt.Errorf("doctor --fix: %d item(s) could not be fixed", failed)
	}
	return nil
}

// multiBinaryCheck returns a warning result when the running executable and the
// PATH-resolved aom binary live in different directories, indicating that agents
// will pick up a different (possibly stale) build than the operator is using.
// Returns nil when the directories match or when either path is empty.
func multiBinaryCheck(exePath, aomPath string) *doctorResult {
	if exePath == "" || aomPath == "" {
		return nil
	}
	exeDir := filepath.Dir(exePath)
	aomDir := filepath.Dir(aomPath)
	if exeDir == aomDir {
		return nil
	}
	return &doctorResult{
		label:   "aom: multi-binary",
		detail:  fmt.Sprintf("running binary is %s but PATH resolves to %s — agents may run a different aom version; fix: update PATH or reinstall aom to %s", exePath, aomPath, exeDir),
		warning: true,
	}
}

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// simple insertion sort — small maps only
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
