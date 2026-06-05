# Changelog

All notable changes to AOM (Agent Orchestrator Management) are documented here.

## [Unreleased]

### Added
- Web UI (`aom serve`) — React-based dashboard for managing projects, sessions, tasks, agents, roles, and classes via browser

### Fixed
- Test suite: fixed hanging tests (`--real` spawn mode) by applying `stubRegistryFactory` to skip native session detection loops
- Demo scripts: replaced hardcoded user paths with `AOM_BIN` / `AOM_PROJECT_DIR` environment variables

---

## [1.0.5] — 2026-06-01

### Fixed
- Advance sender cursor on message send to eliminate watch race condition (#27)

---

## [1.0.4] — 2026-06-01

### Fixed
- Message watch cursor desync; default orchestrator-main agent in `project init` (#26)

---

## [1.0.3] — 2026-05-31

### Fixed
- Remove unsupported `folder` field from Homebrew brews config (#25)

---

## [1.0.2] — 2026-05-31

### Fixed
- Set `folder: Formula` in GoReleaser brews config so tap writes to `Formula/aom.rb` (#24)

---

## [1.0.1] — 2026-05-31

### Fixed
- Block `--real` session spawn when agent has no workspace and no task assigned (#23)

---

## [1.0.0] — 2026-05-31

### Added
- `aom session spawn --in-team` — spawn all agents in a project in one command (#22)
- `aom session stop --all` — stop all active sessions (#22)
- curl installer, Homebrew tap, and binary install instructions (#20)
- Clarified Windows requires WSL2 (tmux not available natively) (#21)

---

## [0.1.0] — 2026-05-30

Initial public release.

### Features
- Multi-agent orchestration via tmux + git worktrees
- Task/session lifecycle with SQLite state (`Draft → Done → Archived`)
- Plan-approval gate: `propose-plan` / `plan-approve` / `plan-reject`
- Lifecycle hooks: `on-task-done` (live), `on-blocked`, `on-needs-attention`, `on-plan-*` (`.sh.example`)
- Event bus: async/sync subscribers; exit 2 blocks operation
- Per-role verify checks: reviewer vs builder policy
- Auto-stop sessions when task Done; idempotent pane cleanup
- Merge commit strips `.agent/` artifacts automatically
- Support for Claude Code and Codex runtimes
- Cross-platform: Linux, macOS (arm64/amd64), Windows via WSL2
