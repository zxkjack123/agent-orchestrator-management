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

# Test all packages
go test ./...

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

**Immediate next work** (see `docs/current-status.md` for full detail and implementation order):

*Cross-platform fixes (partially done — remaining):*
- `project.yaml.tmpl` → `repo: .` (absolute Windows path breaks Linux binary)
- gitignore binary artifacts + README WSL setup section
- `aom task ready <task-id>` command (Planned→Ready in one shot)
- auto merge check before `aom merge commit`

*Windows/WSL2 E2E feedback — new issues (all unimplemented):*
- NTFS `mkdir` false-positive in `project init` (`internal/project/service.go:112`)
- NTFS `index.lock`: agent profile NTFS fallback instruction + `aom doctor` NTFS warning
- Hook UX: `project init` generate live `on-task-done.sh`; `aom doctor` warn if `.example` not activated
- Model validation soft-warn before session spawn (`internal/provider/`)
- `CLAUDE.md` add/add merge conflict: auto-resolve with "ours" in `executeMergeCommit`
- `aom session send --file -` stdin pipe support
- `aom task cancel <task-id>` for orphan Draft/Planned/Ready tasks
- `--prefer-branch` flag for `aom merge commit`

*Deferred:*
- gemini and kiro runtime launch (`internal/provider/gemini.go`, `internal/provider/kiro.go`) — blocked on confirmed CLI flags

**Recent additions** (see `docs/current-status.md` for full detail):
- M13: `aom task link/unlink`, cross-task dependency graph with BFS cycle detection, `--priority` flag, `aom next`
- M14: `aom task request/list-requests/approve-request/reject-request`, `aom team brief` (`.aom/team-brief.md`)
- M15: `aom merge check/prepare`, `internal/merge` package, `merge-plan.md` artifact
- M16: `aom message send/read/clear`, `aom task record-result`, `aom session health`, `aom pause-all/resume-all`
- M17: `aom worktree read-file` (cross-worktree read with path-traversal guard), `aom metrics` (velocity report from log events)
- Post-M17: `aom merge commit` (executes git merge with guards), `aom task list`, `aom task claim`, `project-board.md` auto-refresh, runtime-level policy enforcement (`--disallowed-tools` for claude), `seedAgentProfiles` bug fix in `Open()`, E2E smoke test script

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
| `docs/current-status.md` | Handoff document — current implementation progress |
| `docs/system-diagrams.md` | Mermaid diagrams — architecture, state machines, flows |
| `AGENTS.md` | Working guidelines for all implementation partners |
