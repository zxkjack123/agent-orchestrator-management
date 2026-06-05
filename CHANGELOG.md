# Changelog

All notable changes to AOM (Agent Orchestrator Management) are documented here.

## [Unreleased]

---

## [1.0.7] — 2026-06-05

### Fixed
- `aom serve stop` / `aom serve restart` now work even when the pid file is missing — falls back to `lsof` to find the process on port 7777

### Changed
- Homebrew formula renamed from `aom` to `aom-agents` to avoid conflict with the AV1 video codec (`homebrew/core`). The installed binary is still named `aom`.
  ```bash
  brew install lattapon-aek/tap/aom-agents   # install
  brew upgrade aom-agents                    # upgrade
  ```

---

## [1.0.6] — 2026-06-05

### Added
- Web UI (`aom serve`) — React-based dashboard for managing projects, sessions, tasks, agents, roles, and classes via browser
- Role & Class system — three-zone agent profiles, 7 built-in classes (`orchestrator`, `builder`, `frontend`, `reviewer`, `researcher`, `generic`, `default`), full CRUD via CLI and API
- Class descriptions — every built-in class has a short description shown in `aom class list`, web UI, and Add Agent modal
- `researcher` class — for research, analysis, and investigation tasks (does not write production code)
- `generic` class — universal worker for any task type: coding, writing, analysis, slide decks, documents
- `aom team brief --push` — generate brief and broadcast to team channel + agent worktrees in one command
- Agent templates: all agents now read `.aom/team-brief.md` at session start for shared project context
- Agent templates: step 5 in "Completing work" — agents run `aom message watch` after finishing a task so they stay reactive

### Fixed
- Test suite: fixed hanging tests (`--real` spawn mode) by applying `stubRegistryFactory`
- Add Agent modal role dropdown: was hardcoded; now loads from API so custom roles appear
- Class template View modal: was not scrollable; fixed flex layout
- Demo scripts: replaced hardcoded user paths with `AOM_BIN` / `AOM_PROJECT_DIR` env vars
- Path traversal (CWE-022): `fs.go` filesystem browser and `registry.go` now use `filepath.EvalSymlinks` + home-directory boundary guard

### Changed
- `docs/cli-spec.md` expanded from 42 to 122 command sections — every registered command is now documented
- `CHANGELOG.md` created

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
