# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Project Skills

The following skills are installed via `skills-lock.json`. When the user types the trigger, invoke the Skill tool immediately before doing anything else.

- **software-engineering-core** — One evidence-first workflow covering clarification, planning, debugging, and implementation. Use for most engineering work. Trigger: `/software-engineering-core`
  When the user types `/software-engineering-core`, invoke the Skill tool with `skill: "software-engineering-core"` before doing anything else.

- **change-review** — Review repository changes with an evidence-first, impact-aware workflow. Use after implementation is complete. Trigger: `/change-review`
  When the user types `/change-review`, invoke the Skill tool with `skill: "change-review"` before doing anything else.

---

## Role in This Repository

You are operating as an **implementation partner**. You may read, edit, and create
files as needed to implement tasks. Follow the engineering guidelines in
`docs/engineering-guidelines.md` and `AGENTS.md` before making changes. Always
read the relevant documentation before advising or implementing any area.

---

## Commands

```bash
# Build (embeds version metadata — bare go build leaves Version=dev/Commit=unknown)
VERSION=$(git describe --tags --match 'v[0-9]*' --dirty --always --abbrev=8 2>/dev/null || echo dev)
COMMIT=$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
PKG=github.com/lattapon-aek/agent-orchestrator-management/internal/cli
go build \
  -ldflags "-X $PKG.Version=$VERSION -X $PKG.Commit=$COMMIT -X $PKG.BuiltAt=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X $PKG.GoVersion=$(go env GOVERSION)" \
  -o aom cmd/aom/main.go

# Run
go run cmd/aom/main.go <command>

# Test all packages (cli integration tests run real git ops — needs extra time on Windows)
go test -timeout 20m ./...

# Test a single package
go test ./internal/<package>/...

# Environment (recommended for consistent builds)
export GOTOOLCHAIN='local'
export GOCACHE="$PWD/.cache/gocache"
export GOMODCACHE="$PWD/.cache/gomodcache"
export GOTELEMETRY='off'
```

---

## Architecture Overview

**AOM** (Agents-Orchestrator-Management) is a project-level control plane for managing multiple CLI-based AI agents (Claude Code, Codex, Kiro) as a coordinated team. A single operator runs `aom` to dispatch tasks, manage agent sessions, and maintain durable state across sessions and git worktrees.

### Dependency Direction

```
cmd/aom → internal/cli → internal/app → internal/{project,agent,task,step,session,worktree,artifact,plan}
                                       → internal/{config,db,tmux}
```

- `cmd/aom/main.go` — process entrypoint only
- `internal/cli/` — Cobra command definitions; must stay thin (parse + call domain)
- `internal/app/` — dependency wiring; constructs and connects domain services
- `internal/config/` — YAML loading and validation; no CLI or DB knowledge
- `internal/db/` — SQLite bootstrap and migrations only; no CLI types
- `internal/tmux/` — all tmux concerns isolated here; nothing else touches tmux directly
- `internal/plan/` — orchestrator planning, mode inference
- `internal/artifact/` — generates and updates `.agent/*.md` files
- All other packages own their own state transitions and persistence

### Operational Memory Model

Three layers of truth (in order of authority):
1. **`.agent/*.md` artifacts** — durable, primary source of truth (task.md, state.md, index.md, log.md, handoff.md)
2. **SQLite DB** (`.aom/sessions.db`) — structured relational state for queries and transitions
3. **Live tmux sessions** — ephemeral, replaceable

If `log.md` conflicts with any in-memory state, prefer `log.md`.

### State Machines

Defined in full in `docs/state-machine.md`. Summary:

| Entity | States |
|--------|--------|
| Task | Draft → Planned → Ready → InProgress → Blocked/NeedsAttention → Done → Archived |
| Step | Proposed → Confirmed → Ready → InProgress → Completed/Blocked/NeedsAttention/Skipped/Canceled |
| Session | Created → Booting → Idle → Working/WaitingApproval/WaitingHandoff → Detached/Failed/Stopped → Archived |
| Worktree | Planned → Ready → Active → NeedsRepair |

### Task Modes

- `Direct` — default; small, well-scoped changes
- `Bugfix` — structured root-cause analysis
- `Requirements-first` — requirements → design → tasks
- `Design-first` — design/constraints → requirements → tasks

---

## Key Design Rules (from `docs/engineering-guidelines.md` and `AGENTS.md`)

- **CLI handlers must be thin**: parse input, call domain service, print result — no business logic
- **Domain logic must not import Cobra** or any CLI type
- **DB helpers must not know about CLI types**
- **No speculative flexibility**: implement the minimum for the current milestone
- **Surgical changes only**: touch only what the task requires; no opportunistic refactoring
- **No giant util packages**: prefer small, focused files by responsibility
- **Orchestrator is a dispatcher and state gate**, not a hidden or non-inspectable manager — when the orchestrator is an AI session it still drives explicit CLI commands, never hidden state mutations
- State transitions belong in domain packages, not in CLI handlers
- A task may outlive many sessions; a session may be replaced without replacing the worktree

---

## Milestone Status

| Milestone | Status |
|-----------|--------|
| 0 — Foundation specs | Complete |
| 1 — Project init, config, basic status | Complete |
| 2 — tmux management, session spawning | Complete |
| 3 — Task/step management, planning, mode inference | Complete |
| 4 — Canonical task artifacts, worktree mapping, session lifecycle | Complete |
| 5 — Git worktree continuity, one-writer guardrails | Complete |
| 6 — Checkpoint, handoff, review workflow | Complete |
| 7 — Manual intervention, re-analysis (`aom task reanalyze`) | Complete |
| 8 — Session approval/deny, recovery | Complete |
| 9 — Project governance: skills, MCP, policy injection | Complete |
| 10 — Runtime adapters (codex + claude) | Partial — codex and claude live; gemini/kiro pending |
| 11 — Operator UX refinement | Partial — ANSI color status, section headers done |
| 12 — Agent team collaboration (channel, broadcast, watch) | Complete |
| 13 — Task graph & priority | Complete |
| 14 — Agent self-service & team briefing | Complete |
| 15 — Merge coordination | Complete |
| 16 — Communication & feedback upgrade | Complete |
| 17 — Observability (cross-worktree read, velocity metrics) | Complete |
| Post-M17 — Bug fixes, UX, merge commit, runtime policy | Complete |
| E2E feedback rounds 1–5 — operator UX, agent profiles, SQLite | Complete |
| E2E feedback rounds 6–7 — readiness labels, invariants, shared brief, JSON output | Complete |
| E2E feedback rounds 8–11 — codex background terminal cleanup, profile trim, auto-stop | Complete |
| Per-Agent Workspace (Free-Roam) — A1–A8 + guards G1/G2/G3 + resume fix + task.md fix | Complete |
| WSL2 bwrap bypass + wrapper loop fix — codex E2E hardening (2026-05-22) | Complete |
| Single-quote `sh -lc` pane crash fix — deny_commands spawn failure (2026-05-22) | Complete |
| E2E feedback fixes (2026-05-22) — completion gate, reviewer guard, workspace hard error, repo layout | Complete |
| Phase 2 completion — F2–F8 fixes, `aom task signal`, E2E 2-agent test (builder→reviewer), master plan doc | Complete |
| Phase 4 Operator UX — `aom switch`, `aom dashboard`, `aom task verify --watch`, `aom status --action-items` | Complete |
| Phase 5 Guided Autonomy — `aom task accept --auto`, `aom session watch --auto-spawn`, `aom run-pipeline` + timeout/escalation | Complete |
| Messaging & communication hardening — claude workspace CWD, auto-provision deadlock, channel sender, notification ordering, message watch exit-on-receipt | Complete |
| Role & Class management — `aom role/class/system-template`, 3-zone profile system, Web UI Roles view, 12 REST API endpoints | Complete |

**Immediate next work** (see `docs/dev/current-status.md` for full detail):

*Deferred:*
- gemini and kiro runtime launch (`internal/provider/gemini.go`, `internal/provider/kiro.go`) — blocked on confirmed CLI flags

*Ready to start:*
- Phase 3.3: **DONE** (2026-05-26) — cross-provider E2E v2 completed; codex-be + claude-fe both 5/5 verify checks; merged to main; residual: codex sometimes uses step.completed instead of task.completed as final signal (operator unblock: `aom task signal task.completed`)
- Next phase TBD: Orchestrator Agent Mode, real-world project trial, or profile/compliance hardening

**Recent additions** (see `docs/dev/current-status.md` for full detail):
- M13: `aom task link/unlink`, cross-task dependency graph with BFS cycle detection, `--priority` flag, `aom next`
- M14: `aom task request/list-requests/approve-request/reject-request`, `aom team brief` (`.aom/team-brief.md`)
- M15: `aom merge check/prepare`, `internal/merge` package, `merge-plan.md` artifact
- M16: `aom message send/read/clear`, `aom task record-result`, `aom session health`, `aom pause-all/resume-all`
- M17: `aom worktree read-file` (cross-worktree read with path-traversal guard), `aom metrics` (velocity report from log events)
- Post-M17: `aom merge commit`, `aom task list/claim/cancel/accept/ready`, runtime-level policy enforcement (`--disallowed-tools` for claude), `project-board.md` auto-refresh
- E2E feedback: smart codex deny-command wrapper, `aom capture --follow/--diff`, `aom status --active/--graph`, `aom doctor --global`, `--step-type` flag, branch name truncation, auto-flush outbox on capture
- Agent profile system: profiles moved from hardcoded Go strings to embedded `.md.tmpl` template files with 3-level lookup (templateDir → `.aom/templates/profiles/` → embedded); built-in classes: builder, frontend, reviewer, orchestrator
- Team building: `aom agent add --class <class>`, Team Building section in all agent profiles, operator workflow in `--help` with Option A (operator-as-orchestrator) and Option B (delegate to orchestrator agent)
- E2E feedback (third round): `aom doctor` git identity check, `aom agent list` model column, `task.md` worktree path annotated as CWD-only, `agents.yaml` templates include commented `model:` examples
- Stability & observability: `aom session recover` (diagnose + recommend recovery action), `aom events tail` (stream log.md events live), codex auto-commit reminder at spawn
- Cross-platform polish: `project.yaml → repo: .`, NTFS `mkdir` stat-check fallback, `aom doctor` NTFS mount warning, NTFS hint in agent profile template, live `on-task-done.sh` generated by `project init`, `aom doctor` warns on unactivated `.sh.example` hooks, model spawn soft-warn, `CLAUDE.md` add/add conflict auto-resolve, `--prefer-branch` for `aom merge commit`, `aom task ready/cancel`
- Session UX: `aom session list --active` (filter to live sessions), `aom session resume` smart auto-recovery without `--task` (4-path: rebind pane → native resume → spawn hint → archive hint)
- E2E feedback (fourth round): codex commit loop fix (foreground-only + universal fallback + no retry loops), `sessions.db` created with `0664` permissions, `aom doctor` adds PATH check + DB writable check, `aom agent set-model <name> <model>` (safe model update without overwriting agents.yaml), codex ModelHint clarifies ChatGPT vs OpenAI API account, builder profile adds Sandbox Constraints section (network + package manager guidance)
- E2E feedback (fifth round): `ensureDir` fixes `.agent/` permission (umask-independent 0755), `aom watch` now waits for tasks instead of returning immediately, `aom policy list [--task <id>]` shows deny commands + per-task enforcement level, `aom session stop` is idempotent (no-op if already Stopped), dependent tasks auto-promote to Ready when all blockers Done, `aom doctor --fix` auto-corrects permissions, `aom broadcast --file <path>` for Markdown briefs
- E2E feedback (sixth round — from AOM_FEEDBACK.md): SQLite WAL mode, `aom task verify`, commit guard in `aom task show`, `aom worktree prune`, spawn channel announcement, mini model warning, session `readiness=` label, `aom status --json`, collaboration step gate, task invariants (`--invariant` flag + `aom task verify`), `aom project share <file>` (broadcast to all active worktrees), `session replace --mock` bug fix
- E2E feedback (rounds 8–9): SQLite `_txlock=immediate` + 30 s busy timeout + `SetMaxOpenConns(1)`, outbox pending warning in `aom channel read`, `generic.md.tmpl` non-coding profile, `base.md.tmpl` commit fallback + generic class examples
- E2E feedback (rounds 10–11 — codex background terminal root-cause): `aom agent add` name-runtime mismatch warning, `Idle (pane live)` indicator in `aom status`, `.aom/` added to `defaultGitignoreEntries` + `aom doctor` `.aom/ tracked` check, `builder.md.tmpl` + `reviewer.md.tmpl` + `frontend.md.tmpl` + `orchestrator.md.tmpl` Sandbox Constraints section; **provider-level fix**: `KillPaneAndDescendants()` in tmux manager (BFS via `pgrep -P`, SIGTERM→SIGKILL) replaces `KillPane` in session stop path; **3-layer auto-cleanup**: `aom session stop` kills all descendant processes, `aom task accept` auto-stops bound Idle sessions, `aom status` auto-stops Idle sessions whose `log.md` contains `task.completed`
- Per-Agent Workspace (Free-Roam A1–A8): `aom agent provision <name>` creates permanent git worktree at `.aom/agents/<name>/workspace/` on `agents/<name>` branch; `workspace_path` persists through DB Upsert (CASE WHEN); workspace agents skip per-task worktree creation; session spawn uses workspace path as CWD; `materializeAgentContext` writes identity files to workspace; merge check/prepare/continue uses `agents/<name>` branch with `--fixed-strings` for `[TASK-xxx]` grep; real E2E verified in WSL with native claude session auto-detection
- Same-runtime conflict guards: **G1** — `aom session spawn` warns when two agents share a runtime but neither has a workspace; **G2** — `aom doctor` `[WARN] workspace: <runtime>` lists agents missing workspaces with fix commands; **G3** — `aom project init` prints `aom agent provision` next-steps for each agent
- E2E feedback fixes (2026-05-22): **G1 upgraded to hard error** (returns `error` instead of warning; `--allow-collision` to bypass); **task.completed gate** (`runTaskVerifyChecks` helper extracted; `aom task accept` blocked if checks fail, `--force` to bypass; `autoStopCompletedSessions` skips if checks fail); **reviewer guard** (`aom review` blocked on empty branch, `--allow-empty-branch` to bypass; `reviewer.md.tmpl` updated with readiness check section); **repo layout** (`aom project layout` writes `.aom/shared/repo-layout.md` + pushes to active worktrees; `materializeAgentContext` injects layout at session spawn)
- Session resume fix: `loadSessionByIdentifier` now picks the **newest** session when matching by agent name (previously picked oldest dead session — workspace agents accumulate multiple sessions over time)
- `task.md` workspace-agent fix: `SyncParams.AgentWorkspacePath` added; `renderTaskMarkdown` uses 3-way logic — workspace agent gets **absolute** `Artifact Root` path + workspace note; traditional worktree gets relative path + CWD note; unprovisioned gets "not provisioned yet"
- Runtime test fix: 5 assertions in `internal/runtime/launch_test.go` updated to include `NiceExecPrefix` (`exec nice -n 10`) and `npm_config_cache` that had been added without corresponding test updates
- WSL2 bwrap root-cause fix: `codex_bypass_sandbox: true` in `policy.yaml` switches codex from `--sandbox danger-full-access` to `--dangerously-bypass-approvals-and-sandbox`, skipping bwrap entirely and preventing git CPU spin on WSL2; `GIT_OPTIONAL_LOCKS=0` + `GIT_TERMINAL_PROMPT=0` preamble env vars as partial mitigation; `aom doctor` adds WSL2 bypass check (`/proc/version` detection) and `codex: bg terminal timeout` check
- WSL2 auto-detect: `internal/provider/codex.go` now reads `/proc/version` automatically; bypass is applied on WSL2 without any `policy.yaml` change; `aom doctor` shows `[PASS] codex: wsl2 bypass WSL2 detected — applied automatically`; macOS/Windows unaffected
- WSL2 deny-command wrapper loop fix: smart wrappers in `buildCodexWrapperPreamble` replaced `${PATH#binDir:}` (only strips prefix) with `sed "s|binDir:||g;s|:binDir||g"` — removes the AOM policy dir from any PATH position, preventing infinite self-exec inside bwrap where codex-linux-sandbox prepends its own entries before the policy dir
- Single-quote `sh -lc` pane crash fix: `buildCodexWrapperPreamble` `passThroughLine` was using `sed 's|...|'` (single quotes) which terminated the outer `sh -lc '...'` wrapper, causing sh syntax error → immediate pane exit whenever `deny_commands` were configured (all default projects); fixed by using `\"s|...|g\"` (escaped double quotes); also fixed `printf` hex escapes (`\x7b`) → POSIX-compatible `\"` escapes; new invariant test in `TestBuilderBuildCodexWrapsDenyCommands`
- Phase 2 completion (2026-05-26): **F2** — `runTaskVerifyChecks` Check 1b adds `[TASK-xxx]` tagged-commit verification (workspace agents share one branch; missing tag = silent empty merge); **F4** — merged duplicate "Starting a session" protocols into single 6-step sequence; **F5** — commit guard in `executeTaskShow` extended to workspace agents; **F7/F8** — orchestrator profile updated with verify-gate note + `--force` warning; frontend profile adds `[TASK-xxx]` commit convention; **`aom task signal`** new command (`task.completed`, `handoff.prepared`, `checkpoint.created`, `step.completed`) replaces manual log.md writes; writes to task artifact log + mirrors to workspace log; E2E 2-agent test (builder→reviewer in WSL2, claude provider) passed 5/5 verify checks; `docs/AOM-MASTER-PLAN.md` + `docs/dev/e2e-2agent-test-plan.md` added
- Phase 4 Operator UX (2026-05-26): **`aom switch <agent-name>`** — jumps to agent's live tmux pane by name (no session ID needed), auto-logs `operator.intervention`; **`aom task verify --watch [--interval] [--timeout]`** — polls every 10s until all checks pass, prints iteration timestamp + check results on each poll; **`aom status --action-items`** — shows only APPROVAL / ACCEPT / SPAWN / BLOCKED items with exact commands to run; **`aom dashboard [--interval]`** — ANSI live terminal UI (clear+redraw loop) showing sessions table, action items, recent channel, Ctrl+C exits cleanly; `buildActionItems` helper extracted and shared between `--action-items` and dashboard
- Phase 5 Guided Autonomy (2026-05-26): **`aom task accept --auto [--interval 15s] [--timeout 30m]`** — polling loop calls `runTaskVerifyChecks` each interval; breaks and auto-accepts when all checks pass; timeout prints agent-specific escalation hints; **`aom session watch [--auto-spawn] [--interval] [--timeout] [--real|--mock]`** — new session subcommand; polls `buildActionItems` for SPAWN items; `--auto-spawn` calls `parseSpawnItemCommand` + `executeSessionSpawn` (non-fatal per spawn); **`aom run-pipeline <task-id> [--agent] [--timeout] [--real|--mock] [--skip-merge]`** — new top-level command in `internal/cli/pipeline_cmd.go`; 5 stages: spawn → wait(task.completed) → verify → accept → merge; `escalate(stage, hint)` prints remaining budget + resume commands on timeout; 24 new tests in `internal/cli/phase5_test.go`
- Role & Class management (2026-06-04): **three-zone profile architecture** — Zone A (`base.md.tmpl` embedded, read-only AOM Workflow Protocol), Zone B (class template in `.aom/templates/profiles/<class>.md.tmpl`, shared per class), Zone C (per-agent `## Custom Instructions` in `profile.md`); `internal/project/roles_classes.go` — `ListRoles`, `CreateRole`, `UpdateRole`, `DeleteRole`, `PreviewRoleProfile`, `ListClasses`, `GetClassTemplate`, `SetClassTemplate`, `DeleteClassTemplate`, `GetSystemTemplate`, `PreviewClassProfile`; `aom role list/show/create/update/delete/preview` and `aom class list/show/create/edit/override/delete/preview` and `aom system-template show`; 12 new REST API endpoints under `/api/v1/projects/{id}/roles`, `/api/v1/projects/{id}/classes`, and `/api/v1/system-template`; Web UI **Roles** view (`RolesView.tsx`) with 4-step `CreateRoleWizard`, `ClassEditorPanel` with live preview, source badges (builtin/custom/builtin-overridden), `SystemTemplateSection` collapsible viewer; **Agents** view updated to 3-tab per-agent panel (Custom Instructions | Role Template | System Template); built-in classes: builder, frontend, reviewer, orchestrator, generic; custom classes supported via `.aom/templates/profiles/`; class lookup: project override → embedded default → `default.md.tmpl` fallback
- Phase 3.3 cross-provider E2E fixes (2026-05-26): **P0 preamble cd** — codex preamble now prepends `cd "<WorktreePath>"` when WorktreePath set (threads LaunchSpec.WorktreePath → SessionSpec.WorktreePath → codex preamble); **Fix 1** — `hasTaskCompletedEvent` accepts both `task.completed` and `task.closed`; **Fix 2** — `runTaskVerifyChecks` Check 3 falls back to workspace `.agent/handoff.md` when task artifact still has template text; **Fix 3** — `executeTaskSignal` auto-calls `promoteWorkspaceHandoff` on `task.completed` to copy workspace handoff → task artifact; `builder.md.tmpl` Completion Signal section added; 3 new unit tests; cross-provider E2E v2 (codex-be + claude-fe) verified 5/5 checks and merged cleanly

---

## Key Documentation

Read these before reviewing or advising on any area:

| File | Covers |
|------|--------|
| `docs/AOM-planning.md` | Full vision, product goals, scope assumptions |
| `docs/AOM-milestones.md` | Milestone breakdown and sequencing |
| `docs/state-machine.md` | Complete state lifecycle for all entities |
| `docs/artifact-schemas.md` | Markdown artifact contracts and field schemas |
| `docs/cli-spec.md` | Full CLI command specifications |
| `docs/project-structure.md` | Package organization and dependency rules |
| `docs/engineering-guidelines.md` | Code style, patterns, and guardrails |
| `docs/project-config.md` | `.aom/` config file layout and schemas |
| `docs/dev/current-status.md` | Handoff document — current implementation progress |
| `docs/system-diagrams.md` | Mermaid diagrams — architecture, state machines, flows |
| `AGENTS.md` | Working guidelines for all implementation partners |
