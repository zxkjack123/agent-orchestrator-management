# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Role in This Repository

You are operating as an **implementation partner**. You may read, edit, and create
files as needed to implement tasks. Follow the engineering guidelines in
`docs/engineering-guidelines.md` and `AGENTS.md` before making changes. Always
read the relevant documentation before advising or implementing any area.

---

## Commands

```bash
# Build
go build -o aom cmd/aom/main.go

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
| Worktree | Planned → Provisioning → Ready → Active → NeedsRepair/Archived |

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

**Immediate next work** (see `docs/dev/current-status.md` for full detail):

*Deferred:*
- gemini and kiro runtime launch (`internal/provider/gemini.go`, `internal/provider/kiro.go`) — blocked on confirmed CLI flags

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
