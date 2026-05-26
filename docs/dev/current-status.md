# Current Status

## Purpose

This document is the current handoff point for the repository.

It should be enough for a developer or agent to:
- understand what is already implemented
- verify the current state quickly
- continue the next milestone without re-discovering context

## Current Milestone Status

### Milestone 0

Completed.

Foundation specs are in place:
- [AOM planning](AOM-planning.md)
- [Milestone plan](AOM-milestones.md)
- [State machine](state-machine.md)
- [Artifact schemas](artifact-schemas.md)
- [Project config](project-config.md)
- [CLI spec](cli-spec.md)
- [Project structure](project-structure.md)
- [Engineering guidelines](engineering-guidelines.md)

### Milestone 1

Completed.

Implemented:
- Go module bootstrap
- config loader and validation
- SQLite bootstrap and migrations
- `aom project init`
- `aom open`
- `aom status`

Main reference:
- [Milestone 1 plan](milestone-1-implementation-plan.md)

### Milestone 2

Completed in code, tests, and live local E2E on macOS.

Implemented:
- tmux manager skeleton
- tmux availability detection
- stable workspace naming
- session schema v2
- session repository and service
- workspace create or reuse on `aom open`
- `aom session spawn`
- `aom session list`
- `aom session show`
- `aom attach`
- `aom capture`
- session-aware `aom status`

Main reference:
- [Milestone 2 plan](milestone-2-implementation-plan.md)

### Milestone 3

Started.

Implemented in the first slice:
- task schema v3 additions
- step table
- task repository and service
- step repository
- `aom task create`
- `aom task show`
- `aom step list`
- task-aware `aom status` counts

Implemented in the second slice:
- `aom task update`
- `aom task close`
- `aom step update`
- task status transition validation
- step status transition validation
- step `Ready` owner validation

Implemented in the third slice:
- `aom plan`
- lightweight orchestrator recommendation service
- mode inference for `Direct`, `Bugfix`, `Requirements-first`, and `Design-first`
- proposed step generation without immediate task creation
- `aom plan --create` to persist accepted planning output into a task with seeded steps
- template-based project init bootstrap for baseline config files
- `aom project init --template` for preset starter templates
- `aom project init --template-dir` for external starter templates
- richer `aom status` task and step visibility with recommended next action hints

### Milestone 4

Started.

Implemented in the first slice:
- artifact generator in `internal/artifact`
- task creation seeds `task.md`, `state.md`, `index.md`, and `log.md`
- structured modes seed mode-specific artifacts such as `requirements.md`, `design.md`, and `tasks.md`
- task and step updates refresh task artifacts and append canonical log events
- pre-worktree canonical artifact root at `.aom/tasks/<task-id>/`
- `session spawn --task` refreshes task artifacts with active session context and appends `session.created`
- task-bound session spawn now records `session.created` and `session.ready` lifecycle events in `log.md`
- failed task-bound session spawn now persists session status as `Failed` and appends `session.failed`
- task-bound session lifecycle events are now emitted in creation order as the spawn flow advances
- failure handling is covered for both pane creation failure and pane annotation failure after launch
- `attach` on a task-bound session now refreshes task artifacts and appends `operator.intervention`
- planned worktree mappings now persist in `internal/worktree` and are seeded during `task create` and `plan --create`
- task views and artifacts now surface planned worktree status, branch, and path even before real worktree provisioning
- best-effort git worktree provisioning now runs on top of the persisted mapping; git-backed repos move planned worktrees to `Ready`
- task-bound session spawn now uses the provisioned worktree path when the mapping is `Ready`, with repo-root fallback retained for non-git or unprovisioned cases
- canonical task artifacts now move into `<worktree>/.agent/` when the mapped worktree is `Ready`, while non-git or unprovisioned tasks still use the repo-root fallback
- worktree reconciliation now detects stale git/path drift and marks persisted mappings as `NeedsRepair`
- `status` and `task show` now surface explicit repair hints and task-level next actions when a worktree mapping is stale
- task-bound session spawn now promotes healthy provisioned worktrees from `Ready` to `Active`
- canonical task artifacts now continue to use `<worktree>/.agent/` while worktrees are `Active`, not only when they are merely `Ready`
- session views now reconcile persisted tmux pane bindings against live tmux state and downgrade missing panes to `Detached`
- worktree summaries now fall back from `Active` to `Ready` when no live task-bound session remains after reconciliation
- `task create` and `plan --create` now fail fast on git repos without an initial commit instead of persisting partial task state
- `status` and `task show` now surface canonical artifact root and task log paths so operators do not need to guess between repo-root and worktree artifacts
- `session spawn --real` and `session replace --real` now launch `codex` or `claude` for supported runtime roles and reject unsupported runtimes before pane creation
- the SQLite bootstrap now applies a `busy_timeout` so single-operator CLI bursts are less likely to fail with immediate `SQLITE_BUSY`

### Milestone 5

Started.

Implemented in the current slice:
- dedicated task-to-worktree continuity is live for task-bound sessions
- `worktree repair` restores missing or unregistered safe worktrees and logs canonical repair events
- `session replace` preserves task and worktree continuity across session turnover
- task-bound session lifecycle now reconciles `Ready`, `Active`, `Detached`, `Stopped`, and `Archived` state more explicitly
- one-writer-per-worktree guardrails now block a second `dedicated-writer` session on the same task while still allowing read-only roles

### Milestone 6

Started.

Implemented in the first slice:
- `aom checkpoint`
- `aom handoff`
- `checkpoint.created` and `handoff.prepared` canonical log events
- `handoff.md` generation in the canonical task artifact root
- task artifact refresh now surfaces latest checkpoint and handoff presence in `index.md`
- handoff flow now marks the source session `WaitingHandoff`

Implemented in the second slice:
- `aom review` as a lightweight review workflow wrapper
- `review-notes.md` template creation and refresh in the canonical task artifact root
- unresolved review item counting from `review-notes.md`
- `index.md`, `task show`, and `status` now surface unresolved review state
- review-driven next-action hints now prefer resolving open review items before continuing implementation
- `review` now degrades cleanly on environments without tmux by preparing artifacts without requiring live session spawn
- `handoff` now transfers task ownership and active-step ownership to the target role or agent immediately while leaving actual execution start to the receiving session
- `review` now reuses an existing review step when possible and otherwise creates a new explicit `review` step with reviewer ownership and simple dependency seeding
- `review` now prefers reusing an existing live reviewer session for the same task and agent before spawning a new reviewer pane
- `review` now promotes task and review-step state conservatively: preparation without a live reviewer session moves them to `Ready`, while a live reviewer session moves them to `InProgress`
- `review-notes.md` template seeding is now non-destructive and preserves existing review findings on repeated `review` runs
- when unresolved review findings already exist while the review task is active, `review` now moves the task and review step to `NeedsAttention` automatically
- `handoff` to a role-only target now resets `InProgress` or `Blocked` task/step state back to `Ready` so work is no longer shown as actively owned by the source session
- unresolved review findings now derive a conservative preferred-owner hint from `review-notes.md`; when all open findings point to one owner role, the task preferred owner is reset to that role with no pinned agent
- review owner hints now surface directly in `status`, `task show`, and `index.md`; mixed unresolved owners are called out explicitly as an operator ambiguity
- review owner ambiguity now changes next-action guidance directly: one shared owner routes follow-up work to that role, while mixed owners force an explicit operator choice
- when a shared review owner hint matches exactly one enabled project agent, AOM now auto-picks that concrete agent for preferred follow-up ownership instead of leaving the hint role-only
- when review findings infer a single follow-up owner, AOM now also updates the latest non-review follow-up step owner hint to match that role or agent

Implemented in the third slice:
- `aom session send`
- `tmux.Manager.SendKeys` for literal prompt delivery into live panes
- task-bound prompt delivery now appends canonical `orchestrator.prompt` events to `log.md`

Implemented in the fourth slice:
- `claude` runtime support in `internal/runtime/launch.go` under `--real` mode
- `session spawn --real` now supports `reviewer-main` and other `claude` agents through the same launch validation path as `codex`

Implemented in the fifth slice:
- task-bound `session spawn` now seeds a non-destructive `handoff.md` template in the canonical artifact root
- seeded handoff templates now include current session, runtime, task, step, and a default next-action reminder for the assigned worker

### Post-Milestone-6 Fixes and Improvements

Implemented after live multi-agent E2E testing on macOS (2026-05-14/15) and WSL E2E (2026-05-15):

- `CreatePane` now uses `new-window` instead of `split-window` so each agent session gets its own full-size tmux window; previously all sessions shared one window and panes shrank until Codex TUI crashed
- `SendKeys` now inserts a 50ms pause between literal text delivery and Enter so TUI apps (codex, claude) finish buffering before submission
- macOS symlink path fix in `internal/worktree/service.go`: `filepath.EvalSymlinks` resolves `/tmp` → `/private/tmp` before worktree path comparison to prevent false `NeedsRepair` status
- `--dangerously-skip-permissions` is now passed to `claude` on `--real` spawn so Claude Code does not block unattended orchestration with interactive approval prompts
- stale `Detached` sessions no longer block `session spawn` for the same task; they are auto-transitioned to `Stopped` before the new session is created
- `project init` now adds `.agent/` to `.gitignore` so worktree-local artifacts are not committed to main and do not cause merge conflicts
- `project init --agents` now accepts inline agent definitions in `name:role:runtime` form (e.g. `frontend-main:builder:claude`) allowing custom agents not in the default template
- `aom help` output rewritten to be agent-readable: includes operator workflow sequence, grouped command reference, and key rules in under 60 lines
- provider-native session resume is now live for both `claude` (`claude --resume <uuid>`) and `codex` (`codex resume <session-id>`); `session spawn --task` automatically looks up the previous native session ID for the task+agent pair and resumes it when one exists
- `--fresh` flag added to `session spawn`: forces a clean context even when a previous native session ID exists; spawn output always shows the continuity decision (resume or fresh start) with a `--fresh` hint when resuming, so the orchestrator or operator can make an informed choice
- `session spawn --real claude` now auto-detects the native session UUID from `~/.claude/projects/<worktree-hash>/` after spawn, auto-accepts the bypass-permissions dialog, and registers the UUID via `SetVendorSessionID` so the next spawn resumes automatically without requiring a manual `set-agent-id` call
- `aom session set-agent-id <session-id> <uuid>` added as a manual fallback for registering native session IDs when auto-detection is not available
- default branch detection in `project init` now reads the current `git` HEAD branch instead of hardcoding `main`, so projects on `master` or any other branch are handled correctly
- `EnsureWorkspace` now names the base tmux window `aom` so it persists visibly when all agent windows are closed
- `tmux.Manager.RenameWindow` added; `session spawn` uses it to label each agent window with the agent name for easy operator identification
- CRLF normalization added to template rendering so config files generated on Windows use consistent LF line endings
- `aom session wait <session-id> --event <type> [--timeout 30m]` added: polls the task's `log.md` every 3 seconds until the named event type appears or the timeout expires; enables unattended AI orchestrator loops
- `aom task reanalyze <task-id>` added: re-syncs `index.md`/`state.md` from current system state, appends `reanalysis.completed` to `log.md`, and prints recommended next action; use after manual intervention or when the orchestrator needs to sync context
- `aom channel append "<message>" [--agent <name>]` and `aom channel read` added: shared `.aom/channel.md` artifact lets agents post and read team-level messages without routing through the orchestrator
- `aom broadcast "<message>" --sessions <id,id,...>` added: delivers the same prompt to multiple live sessions in one command; reports per-session delivery status
- `aom approve <session-id>` and `aom deny <session-id> [--reason <why>]` added (M8): unblock or reject a session in `WaitingApproval`; transitions to `Idle` on approve and `Blocked` on deny, both appending canonical `approval.approved` / `approval.denied` events
- `aom doctor` validates environment (tmux, config, writable `.aom/`, database, runtime binaries, active worktree paths); exits non-zero on failure
- `aom runtime list` and `aom runtime inspect <runtime>` show configured runtime availability and capabilities
- `aom session resume <session-id> --task <task-id>` added: rebinds an `Idle` or `WaitingHandoff` session to a new task without spawning a new process; sends `cd <worktree>` to the live pane, materializes the identity file in the new worktree, advances the new task's worktree to `Active`, and syncs both tasks' artifacts with canonical events
- `aom watch --task <task-id> [--event <type>] [--timeout 30m]` added (M12): with `--event` blocks until the named event type appears in the task's `log.md` then exits (same polling as `session wait` but task-centric); without `--event` runs in tail mode — streams every new non-empty log line as it appears until timeout
- `aom doctor` added: validates the local environment in a single pass — checks tmux binary availability, project config loading, `.aom/` write access, SQLite database presence, configured runtime binary availability (shows which agents use each runtime), and active/ready worktree path health; exits with a non-zero status when any check fails so it can be used in scripts; exits zero when all checks pass; warnings (e.g. missing sessions.db on a brand-new project) do not count as failures
- `aom runtime list` added: shows all runtimes referenced in the project `agents.yaml`, whether the binary is found in PATH, and which agents use each runtime
- `aom runtime inspect <runtime>` added: shows binary path, availability, launch modes, resume support (with concrete CLI invocation examples), and a table of all agents using that runtime with their role and enabled state
- `aom watch` (without `--task`) now watches all active tasks simultaneously: in tail mode prefixes each new log line with `[TASK-xxx]`; in `--event` mode exits as soon as any active task's log shows the target event and reports which task matched; active tasks are those in `InProgress`, `Blocked`, `NeedsAttention`, or `Ready` state
- `tailLogEvents` bug fixed: was using line-count to track position (off-by-one when files end with `\n`); now tracks by byte offset so no events are missed
- `internal/cli/log_wait.go` extracted into its own file: contains `waitForLogEvent`, `tailLogEvents`, `scanLogForEvent`, `tailMultiTaskLogEvents`, `waitForMultiTaskLogEvent`

### Milestone 9 — Project Governance (Skills, MCP, Policy)

Implemented in one slice (2026-05-15):
- `internal/config/config.go` now exposes `ResourcesForRole(roleName, runtimeName)` that returns `RoleResources{Skills, MCPServers}` filtered by runtime compatibility; `ResolvedSkill` and `ResolvedMCPServer` wrapper types carry the name alongside the config for downstream use
- `internal/project/service.go` `OpenResult` now includes `Resources config.ResourcesFile` and `Policy config.PolicyFile` so CLI handlers have full governance context without re-loading config
- `MaterializeSkillFiles` added to `internal/artifact/service.go`: copies role-bound skill markdown files from the repo root into the worktree root at spawn time; missing source files are silently skipped
- `MaterializeMCPConfig` added to `internal/artifact/service.go`: for `claude` runtimes appends a `## MCP Servers` section to `CLAUDE.md`; for `codex` runtimes writes `.codex/mcp.json`; other runtimes are no-ops
- `MaterializePolicyConstraints` added to `internal/artifact/service.go`: appends a `## Policy Constraints` section listing deny_commands to the runtime identity file (`CLAUDE.md` for claude, `AGENTS.md` for codex); no-op when deny list is empty
- `materializeAgentContext` helper in `internal/cli/root.go` consolidates identity file, skill files, MCP config, and policy constraints into a single call from both spawn sites; prevents divergence between spawn and rebind paths
- `enforcePolicyDefaults` warns on `yolo_mode=enabled` at spawn time; full runtime interception deferred to M10
- `aom project resources` added: prints all role bindings (skills, MCP servers) and policy summary loaded from the project config
- `aom status` now uses ANSI color for status fields (green=active/healthy, yellow=attention needed, red=failed, dim=archived/done) and bold section headers; colors are suppressed automatically when stdout is not a TTY or `NO_COLOR` is set
- `internal/cli/status_format.go` added: contains `isTTYWriter`, `colorize`, `colorStatus`, and `sectionLabel` helpers

### Milestones 13–17 — Workflow Intelligence Layer (2026-05-16)

All five milestones implemented and committed in sequence.

#### M13 — Task Graph & Priority
- `task_dependencies` junction table (schema-v5) — `ALTER TABLE tasks ADD COLUMN priority`
- BFS cycle detection before inserting a dependency edge
- `aom task link <task-id> --blocks <other>` / `aom task unlink`
- `--priority high|normal|low` on `task create` and `task update`
- `aom next` — ordered list of unblocked tasks by priority; shows "waiting on: TASK-xxx" for blocked tasks
- `index.md` now renders `Priority:` and `Blocked by:` fields
- New log events: `task.linked`, `task.unlinked`

#### M14 — Agent Self-Service & Team Briefing
- Agents write requests to `.aom/requests/<id>.md` via `aom task request`
- Operator reviews with `aom task list-requests`, `aom task approve-request`, `aom task reject-request`
- `aom team brief` generates `.aom/team-brief.md` — machine-readable full team state summary (tasks, requests, channel, agents)
- `session spawn --task` now prints team-brief.md path alongside task.md

#### M15 — Merge Coordination
- `internal/merge/` package: `CheckOverlaps` runs `git diff --name-only` on two branches, scores Green/Yellow/Red
- `aom merge check <task-id> [--against <other-task-id|branch>]` — dry-run overlap report
- `aom merge prepare <task-id> [--into <branch>]` — writes `merge-plan.md` into task artifact root, sets NeedsAttention

#### M16 — Communication & Feedback Upgrade
- P2P mailboxes at `.aom/mailbox/<agent>.md`; `aom message send/read/clear`
- Passive CI feedback: `aom task record-result <task-id> --passed|--failed [--summary ...]` appends test events and moves failed tasks to NeedsAttention
- `aom session health [--all]` — time since last checkpoint, warns if >2h; lists handoff presence
- `aom pause-all [--reason ...]` — transitions all Working sessions to WaitingApproval and broadcasts pause
- `aom resume-all` — bulk-approves all WaitingApproval sessions

#### M17 — Observability
- `aom worktree read-file <task-id> <path>` — read-only cross-worktree file access; path-traversal protection via `filepath.Clean` + prefix check; appends `worktree.read` audit event
- `aom metrics [--days N]` — team velocity report: tasks completed, avg duration, blocked events per agent, bottleneck hint; derived from `log.md` event timestamps and task DB records
- `internal/cli/metrics.go`: `BuildVelocityReport`, `PrintVelocityReport`, `parseBlockEvents`

### Post-M17 — Bug Fixes, UX Polish & Merge Completion (2026-05-16)

Implemented after E2E simulation revealed gaps:

#### Bug fixes
- `seedAgentProfiles` bug fixed: was only called during `project init`, not `project open`; agents added to `agents.yaml` after init never received profile files — fixed by calling `seedAgentProfiles` in `Open()` (idempotent, skips existing profiles); all agent identity files now populate correctly at spawn time
- `aom worktree repair` now returns a `(bool, *Record, error)` triple — `bool` indicates whether a repair actually occurred; CLI distinguishes `"Worktree repaired"` from `"Worktree already healthy, no repair needed"` so health-check runs are non-noisy
- `aom watch` timeout now exits 0 with an informational message instead of returning an error; `tailLogEvents` and `tailMultiTaskLogEvents` return `nil` on normal timeout; `waitForLogEvent` and `waitForMultiTaskLogEvent` still return errors (polled event detection uses error path as signal)
- `aom pause-all` shows `"No Working sessions found to pause"` with a note about mock/idle sessions when nothing is paused
- `aom handoff --to` error now lists all valid agent names and role names so the operator can fix the argument without checking `agents.yaml` manually
- `aom task approve-request` output includes `"Next: aom task show <task-id>"` to guide next step
- `aom merge prepare` skips integration step creation when task is already `Done` or `Archived`
- `aom task close` now auto-skips placeholder `integration` steps that are still `Proposed` or `Ready`, so merge-prep bookkeeping no longer blocks a clean close flow
- `aom merge commit` now auto-completes open `integration` steps after a successful merge so the task does not retain stale merge bookkeeping
- `aom message send` reads `AOM_ACTOR` env var for sender identity; falls back to `"operator"` when unset; enables AI orchestrator sessions to self-identify in mailbox messages

#### New commands
- `aom task list` — tabular display of all tasks with columns: TASK, STATUS, PRIORITY, ROLE, AGENT, TITLE; shows `[blocked by: TASK-xxx]` suffix for blocked tasks
- `aom task claim <task-id> [--agent <name>]` — self-assigns a task to an agent (reads `AOM_ACTOR` env when `--agent` is omitted); refreshes `project-board.md`
- `aom merge commit <task-id> [--into <branch>]` — executes `git merge --no-ff` of the task branch into the target; requires task status `Done`; requires current branch to match target; requires at least one commit ahead of target (errors with hint if branch is empty); appends `merge.committed` event to task log

#### Guards added to existing commands
- `aom merge commit` errors when source branch has no commits ahead of target — prevents silent `"Already up to date"` no-ops where agent work was never committed to git
- `aom task close` warns when the task worktree has uncommitted tracked changes or when the task branch has no commits ahead of the default branch — operator is informed before closing so agent work is not silently lost

#### New features
- `project-board.md` auto-refresh: `aom/project-board.md` is regenerated after every task mutation (`task create`, `task update`, `task close`, `task link`, `task unlink`, `task claim`, `plan --create`); non-fatal — board failure never blocks the main operation
- Session spawn now injects a `## Project Board` section into the agent's identity file (`CLAUDE.md` for claude, `AGENTS.md` for codex) with the absolute path to `project-board.md` and a `aom task list` hint
- Runtime-level policy enforcement: `deny_commands` in `policy.yaml` are now passed as `--disallowed-tools 'Bash(cmd*)'` flags when launching claude sessions; codex has no equivalent flag — identity file injection remains the maximum available enforcement for codex
- E2E smoke test script: `scripts/e2e-smoke.sh` — covers 43 checks across 12 sections using `--mock` mode; builds binary, creates temp git repo, exercises all major command groups, reports PASS/FAIL per check

### Six Additional Pre-Gemini/Kiro Features (2026-05-15)

Implemented after M9, before gemini/kiro runtime support:
- **Initial context delivery**: `session spawn --task` output now includes the `task.md` path and a reminder to read it before starting work; when `--real` mode, also prints the `aom session send` command hint for delivering the file reference to the agent
- **Policy enforcement via identity file**: `MaterializePolicyConstraints` appends deny_commands list to the runtime identity file at spawn time, making project policy visible to agents without manual instruction; called from `materializeAgentContext` alongside identity, skill, and MCP materialization
- **Pane rebind without re-spawn**: `aom session rebind <session-id>` reconnects a `Detached` session to a live tmux pane; if the original pane is still alive it un-detaches the session to `Idle`; if the pane is gone it creates a new placeholder pane in the project workspace using a fresh `LaunchModePlaceholder` command
- **Dynamic continuity readiness in index.md**: `- Continuity Readiness:` in `index.md` is now computed from task status, active session presence, worktree health, and unresolved review count; returns `High` (green path), `Medium` (partial), or `Low` (blockers present)
- **Orchestrator actor type via `AOM_ACTOR` env var**: `aom session send` now reads `AOM_ACTOR` from the environment; if set, uses its value as the `Actor` field in `orchestrator.prompt` log events instead of the hardcoded `"operator"` string; enables AI orchestrator sessions to self-identify in the task log
- **`aom review close <task-id>`**: closes the active review step (advances through `InProgress` → `Completed` to respect step state machine), transitions the task back to `InProgress`, records a `review.closed` event in `log.md`, and if the review notes contain an unambiguous owner hint applies it to the task's preferred agent

## Current CLI Surface

Implemented commands:
- `aom project init`
- `aom open`
- `aom plan`
- `aom status`
- `aom task create [--description <text>]`
- `aom task update`
- `aom task close`
- `aom task show`
- `aom task link` / `aom task unlink` (M13)
- `aom task request` / `aom task list-requests` / `aom task approve-request` / `aom task reject-request` (M14)
- `aom task record-result` (M16)
- `aom next [--format json]` (M13, enhanced 2026-05-18)
- `aom team brief` (M14)
- `aom merge check` / `aom merge prepare` (M15)
- `aom merge commit` (Post-M17)
- `aom merge continue` / `aom merge abort`
- `aom message send` / `aom message read` / `aom message clear` (M16)
- `aom pause-all` / `aom resume-all` (M16)
- `aom task list [--status <value>] [--format json]` (Post-M17, enhanced 2026-05-18)
- `aom task claim` (Post-M17)
- `aom metrics` (M17)
- `aom worktree repair`
- `aom worktree read-file` (M17)
- `aom step list`
- `aom step update`
- `aom session send`
- `aom session spawn`
- `aom session list [--active]`
- `aom session show`
- `aom session replace`
- `aom session stop`
- `aom session archive`
- `aom session health` (M16)
- `aom session resume [--task <id>]` (smart auto-recovery without `--task`; rebind-to-task with `--task`)
- `aom attach`
- `aom capture`
- `aom checkpoint`
- `aom handoff`
- `aom review`
- `aom review close`
- `aom session rebind`
- `aom project resources`
- `aom doctor [--fix]`
- `aom runtime list`
- `aom runtime inspect`
- `aom policy list [--task <task-id>]`
- `aom task verify <task-id>` (E2E feedback)
- `aom task verify <task-id> --watch [--interval <dur>] [--timeout <dur>]` (Phase 4)
- `aom task ready <task-id>` (E2E feedback)
- `aom task cancel <task-id>` (E2E feedback)
- `aom task signal <event-type> --task <id> [--summary <text>] [--step <id>]` (Phase 2)
- `aom worktree prune [--dry-run]` (E2E feedback)
- `aom project share <file>` (E2E feedback)
- `aom status --json` / `-j` (E2E feedback)
- `aom status --action-items` (Phase 4)
- `aom capture --summary` (E2E feedback)
- `aom switch <agent-name>` (Phase 4)
- `aom dashboard [--interval <dur>]` (Phase 4)

Current behavior notes:
- `open` ensures tmux workspace and fails clearly when tmux is unavailable
- `plan` gives a lightweight orchestrator recommendation by default, and `plan --create` persists it into a task with seeded steps
- `project init` renders baseline config from template assets instead of hardcoded agent structs
- `project init --template` lets a project pick a preset starter team from `templates/project-init/<name>`
- `project init --template-dir` lets a project supply its own starter config templates
- `project init --agents` can filter the starter team to selected template agents
- `project init` now prompts for agent selection on interactive terminals when `--agents` is omitted, while non-interactive runs keep the full template agent set
- `project init --agents` and the interactive prompt both accept inline agent definitions in `name:role:runtime` form; unknown inline roles get a minimal dedicated-writer builder role config automatically
- `status` shows project, terminal summary, agents, sessions, detailed task rows, step summaries, and task-level recommended next action hints
- `task create` defaults to `Direct` mode and creates one initial `Proposed` implementation step
- `task create` and `plan --create` now seed task-local continuity artifacts under `.aom/tasks/<task-id>/`
- `task update` and `step update` validate allowed state transitions, including `NeedsAttention`
- `session spawn --task` binds `task_id` into the session record and refreshes `state.md`, `index.md`, and `log.md`
- task-bound `session spawn` records both boot and ready lifecycle transitions in canonical task log events
- task-bound `session spawn` failure after durable record creation is logged canonically and leaves the session record in `Failed`
- task-bound `session spawn` writes `session.created` before launch, then `session.ready` or `session.failed` based on the observed result
- task-bound `session spawn` now also seeds `handoff.md` once so the active worker can fill it in before signaling `handoff.prepared`
- `session send` delivers a literal prompt plus Enter into a live tmux pane and prints the session, pane, and message summary
- task-bound `session send` appends canonical `orchestrator.prompt` events to `log.md`
- `attach` records manual operator intervention for task-bound sessions and marks the task log with `Re-analysis required`
- `task create` and `plan --create` now persist a `Planned` task-to-worktree mapping with deterministic branch and path naming under `.aom/worktrees/`
- `task show`, `status`, `task.md`, and `index.md` now expose the planned worktree mapping
- when the project repo is a valid git worktree host, `task create` and `plan --create` now provision the linked worktree immediately and mark the mapping `Ready`
- `session spawn --task` now launches from the mapped worktree path when available and records the chosen path in the durable session record
- worktree-backed tasks now write `task.md`, `state.md`, `index.md`, `log.md`, and mode-dependent artifacts inside the real task worktree under `.agent/`
- `status` and `task show` now reconcile persisted worktree mappings against both the filesystem and `git worktree list --porcelain`
- stale worktree mappings now surface as `NeedsRepair` with explicit operator repair hints instead of silently looking healthy
- stale worktree hints now distinguish `MissingPath`, `UnregisteredArtifactOnlyPath`, and `UnregisteredDirtyPath` so operator next actions are more specific in `status` and `task show`
- task-bound session launch now moves healthy worktrees into `Active` so operator-visible state distinguishes idle worktrees from live ones
- `aom doctor` passes after `project init`; shows `[PASS]`/`[WARN]`/`[FAIL]` per check with a summary line; exits non-zero only when at least one `[FAIL]` is present
- `aom runtime list` shows all runtimes in `agents.yaml` with binary availability; errors when no project is found
- `aom runtime inspect <runtime>` shows binary path, availability, resume CLI examples, and agent table
- `worktree repair <task-id>` now recovers missing or pruned git-backed task worktrees, restores `.agent/` artifacts into the repaired path, and appends a canonical `worktree.repaired` event
- `worktree repair <task-id>` now also recreates an unregistered worktree path automatically when the existing path is safe to replace because it is empty or contains only `.agent/`
- unregistered worktree paths with non-artifact content now remain operator-repair cases; AOM surfaces a manual cleanup hint instead of deleting the path automatically
- `open`, `status`, `session list`, `session show`, and task views now reconcile tmux pane liveness and persist `Detached` when the pane binding is gone
- `session stop` now intentionally terminates a live tmux pane when present, marks the durable record `Stopped`, keeps the worktree intact, and records tmux cleanup warnings in canonical task log events when pane teardown fails
- `session archive` now transitions eligible inactive sessions to `Archived` while preserving audit history
- `session replace` now spawns a replacement session in the same task/worktree context, preserves continuity through task artifacts, records a canonical `session.replaced` event, and prints explicit operator action hints when the old session is intentionally left running
- `session replace` now auto-archives superseded sessions that have already reconciled to `Detached`, while still stopping replaceable idle sessions and leaving active `Working` sessions for explicit operator intervention
- `session spawn --mock` launches a mock runtime transcript for live local flow verification
- `session spawn --real` launches the `codex` or `claude` CLI for supported runtime roles and fails before pane creation when the role runtime is unsupported or the required CLI is unavailable
- `session spawn --real` automatically resumes the previous native session for the same task+agent pair when one is registered; use `--fresh` to force a clean context instead
- `session spawn --real claude` auto-detects and registers the native session UUID after spawn; the next spawn resumes that session without any manual step
- `session spawn --fresh` forces a fresh agent context regardless of previously registered native session IDs
- `session replace --real` uses the same runtime validation and launch path as `session spawn --real`
- `session set-agent-id <session-id> <native-id>` registers a native session ID manually as a fallback when auto-detection is unavailable
- `task create` and `plan --create` fail before persisting task state when the repo is git-backed but still has an unborn default branch
- SQLite connections now apply a `busy_timeout` to reduce transient `SQLITE_BUSY` failures during short command bursts
- `task show` now prints canonical `Artifact root` and `Task log` paths
- `status` now prints canonical `artifacts=... | log=...` lines for each task
- task-bound `session spawn` now blocks a second `dedicated-writer` from occupying the same task worktree while still allowing read-only roles such as reviewers
- `checkpoint` appends a canonical checkpoint event, refreshes task artifacts, and reports the latest checkpoint summary
- `handoff` writes `handoff.md`, records `handoff.prepared`, refreshes task artifacts, and moves the current session to `WaitingHandoff`
- `handoff` now also updates task preferred owner and active-step owner to the target role or agent immediately
- `review <task-id>` prepares `review-notes.md`, records `review.prepared`, and spawns the reviewer session when tmux is available
- `review <task-id>` now binds the reviewer session to an explicit review step when tmux is available
- `review <task-id>` now reuses an existing `Idle` or `WaitingHandoff` reviewer session for the same task when possible instead of spawning a duplicate pane
- `review <task-id>` still prepares review context on Windows or other tmux-unavailable environments and prints a follow-up action instead of failing after artifact setup
- `review <task-id>` now promotes `Planned -> Ready` during review preparation and promotes `Ready -> InProgress` once a reviewer session is live
- `review <task-id>` now preserves existing `review-notes.md` content and, when unresolved findings are present during active review, pushes task and review step state to `NeedsAttention`
- `handoff <session-id> --to <role>` now clears the specific agent owner and returns active task/step state from `InProgress` or `Blocked` to `Ready`
- repeated `review <task-id>` runs now use open `review-notes.md` owners to steer preferred follow-up ownership back to the fixing role when the findings all agree on one owner
- `status`, `task show`, and `index.md` now distinguish a single shared review owner hint from mixed-owner ambiguity and adjust next-action guidance accordingly
- mixed-owner review findings now surface as a first-class operator ambiguity instead of looking like a normal single-owner follow-up
- single-owner review hints now surface as concrete agent-level guidance when the hinted role has exactly one enabled agent available
- single-owner review hints now flow into both task-level preferred ownership and the latest non-review follow-up step owner hint
- `index.md`, `task show`, and `status` now count unresolved review items from structured `review-notes.md`
- when unresolved review items are open, task next-action guidance now prefers addressing review findings before continuing implementation
- `attach` and `capture` operate through the tmux manager abstraction

### E2E Feedback Round 1–3 Improvements (2026-05-20)

Implemented after live WSL multi-agent test session (login-app project):

#### Operator UX
- `--help` / `-h` flag now handled in all top-level command dispatchers (`agent`, `session`, `project`, `worktree`, `merge`, `message`, `channel`, `step`) before the subcommand switch; prevents misleading "unknown command" errors
- `wrapProjectNotFound` helper wraps the `"no AOM project found"` error with an actionable hint: `Run aom project init <name> --repo . to initialise a project first, then aom open`; applied in `executeOpen` and `executeStatus`
- `aom agent add --class <class>` already existed; fixed false warning message that referenced a non-existent `aom agent disable` command
- `aom capture --summary` flag added: filters raw pane output to structured lines only (pipe-delimited AOM log events, `##` headers, `key=value` pairs, error/warning markers); reduces noise when capturing long agent sessions

#### Session detection stability
- `detectUniqueVendorSessionID` outer loop now sleeps 1 second when `DetectFn` returns empty to prevent tight spinning
- claude session file timestamp filter widened from `spawnedAt` to `spawnedAt - 2s` to tolerate filesystem timestamp rounding

#### Session management UX
- `aom session list` now shows `readiness=` label per session: `done-pending-review` / `needs-operator` / `awaiting-peer` / `in-progress` / `no-progress`; derived from log.md, outbox.md, and channel.md without extra DB queries
- Detached sessions in `aom session list` and `aom status` now show a `next=` hint with the exact recovery command (`resume`, `recover`, or `archive` depending on available context)
- `aom session replace` default `launchMode` corrected from `LaunchModeReal` to `LaunchModePlaceholder` so `--mock` flag works without conflicting with the default; fixes three integration test failures
- Session spawn now auto-appends a channel announcement: `aom: spawned <agent> (session <id>) for task <id> — waiting for operator prompt`
- `aom session spawn` warns when model ends with `-mini` and the task has more than one step

#### SQLite
- WAL journal mode and `NORMAL` synchronous pragma applied in `configureConnection`; eliminates most `SQLITE_BUSY` errors from parallel operator commands

#### Agent profiles
- `base.md.tmpl` rewritten with a mandatory 3-step protocol: update `state.md` directly → `aom step update --status completed` → `aom channel append`; concrete template showing which state.md sections to fill
- `orchestrator.md.tmpl` expanded to full working protocol: how to check worker progress, when/how to accept tasks, handling blocked workers, dispatching commands

#### Task lifecycle guards
- `aom task show` commit guard: if `task.completed` appears in the task log but the worktree branch has no commits ahead of the default branch, a `⚠ task.completed logged but no commits found` warning is printed with a hint to run `aom task verify`
- `aom task ready` now refreshes `project-board.md` after transitioning to Ready (already done in close/accept/cancel)
- `aom task verify <task-id>` added: 4-point checklist — commits on branch, `state.md` updated, `handoff.md` filled, `task.completed` in log; prints `[ok]` / `[FAIL]` per check with notes; also runs any invariant checks
- `aom task cancel` already refreshed project board; confirmed consistent with other task state changes

#### New commands
- `aom worktree prune [--dry-run]`: removes Archived worktrees from git (`git worktree remove --force`) and runs `git worktree prune`; `--dry-run` previews without changes; safe to run after `task cancel` or `task close`

### E2E Feedback Round 4–5 Improvements (2026-05-20)

Implemented after second real WSL test session analysis (AOM_FEEDBACK.md):

#### Session readiness
- `sessionReadiness(repoPath, session.Record) string` computes a human-readable readiness label per session beyond raw status: `needs-operator` (WaitingApproval / WaitingHandoff / Blocked), `done-pending-review` (task.completed in log.md), `awaiting-peer` (outbox.md present), `in-progress` (agent posted to channel), `no-progress` (Idle with no evidence of activity)
- Readiness label surfaces in `aom session list` (after each session line) and `aom status` Sessions section
- JSON output includes `readiness` field per session

#### Machine-readable output
- `aom status --json` / `aom status -j` outputs structured JSON: `project` (name, repo, defaultBranch, dbPath), `agents`, `sessions` (with readiness), `tasks` (with steps), `counts`

#### Collaboration enforcement
- `aom step update <id> --status completed` now warns when the assigned agent has not posted any channel message: `Warning: no channel activity from "<agent>" found — consider posting a summary via aom channel append before marking complete`; warning only (does not block)

#### Task invariants
- `aom task create --invariant required-path=<prefix> --invariant forbidden-path=<glob>` stores constraints in `.aom/<stateDir>/<taskID>/invariants.txt`, one per line
- `aom task show` displays invariants when present
- `aom task verify` checks invariants: `required-path=<prefix>` fails if no committed file has that prefix; `forbidden-path=<prefix>` fails if any committed file matches

#### Shared brief distribution
- `aom project share <file>` copies a file to `.aom/shared/<filename>` (operator-owned) and pushes it to `.agent/shared/<filename>` in every active (Ready/Active) worktree; agents read from `.agent/shared/` without needing to know another task's worktree path; output lists every destination written

## Current Packages

### Working packages

- [cmd/aom/main.go](../cmd/aom/main.go)
- [internal/app/app.go](../internal/app/app.go)
- [internal/app/sessions.go](../internal/app/sessions.go)
- [internal/cli/root.go](../internal/cli/root.go)
- [internal/config/config.go](../internal/config/config.go)
- [internal/db/db.go](../internal/db/db.go)
- [internal/project/service.go](../internal/project/service.go)
- [internal/project/repository.go](../internal/project/repository.go)
- [internal/artifact/service.go](../internal/artifact/service.go)
- [internal/project/templates/project-init/agents.yaml.tmpl](../internal/project/templates/project-init/agents.yaml.tmpl)
- [templates/project-init/default/agents.yaml.tmpl](../templates/project-init/default/agents.yaml.tmpl)
- [templates/project-init/minimal/agents.yaml.tmpl](../templates/project-init/minimal/agents.yaml.tmpl)
- [internal/plan/service.go](../internal/plan/service.go)
- [internal/agent/repository.go](../internal/agent/repository.go)
- [internal/session/repository.go](../internal/session/repository.go)
- [internal/session/service.go](../internal/session/service.go)
- [internal/step/repository.go](../internal/step/repository.go)
- [internal/task/repository.go](../internal/task/repository.go)
- [internal/task/service.go](../internal/task/service.go)
- [internal/tmux/manager.go](../internal/tmux/manager.go)
- [internal/worktree/service.go](../internal/worktree/service.go)
- [internal/runtime/launch.go](../internal/runtime/launch.go)
- [internal/cli/vendor_session.go](../internal/cli/vendor_session.go)
- [internal/cli/log_wait.go](../internal/cli/log_wait.go)
- [internal/cli/channel.go](../internal/cli/channel.go)
- [internal/cli/doctor.go](../internal/cli/doctor.go)
- [internal/cli/runtime_cmd.go](../internal/cli/runtime_cmd.go)
- [internal/cli/message.go](../internal/cli/message.go) (M16 — mailboxes, session health)
- [internal/cli/metrics.go](../internal/cli/metrics.go) (M17 — velocity report)
- [internal/cli/request.go](../internal/cli/request.go) (M14 — task requests)
- [internal/merge/service.go](../internal/merge/service.go) (M15 — git overlap detection)

### Tests

- [internal/config/config_test.go](../internal/config/config_test.go)
- [internal/db/db_test.go](../internal/db/db_test.go)
- [internal/project/repository_test.go](../internal/project/repository_test.go)
- [internal/project/service_test.go](../internal/project/service_test.go)
- [internal/artifact/service_test.go](../internal/artifact/service_test.go)
- [internal/plan/service_test.go](../internal/plan/service_test.go)
- [internal/agent/repository_test.go](../internal/agent/repository_test.go)
- [internal/session/repository_test.go](../internal/session/repository_test.go)
- [internal/session/service_test.go](../internal/session/service_test.go)
- [internal/step/repository_test.go](../internal/step/repository_test.go)
- [internal/task/repository_test.go](../internal/task/repository_test.go)
- [internal/task/service_test.go](../internal/task/service_test.go)
- [internal/tmux/manager_test.go](../internal/tmux/manager_test.go)
- [internal/cli/root_test.go](../internal/cli/root_test.go)
- [internal/worktree/service_test.go](../internal/worktree/service_test.go)
- [internal/runtime/launch_test.go](../internal/runtime/launch_test.go)
- [internal/cli/log_wait_test.go](../internal/cli/log_wait_test.go)

## Verified State

Last verified state before this handoff (2026-05-26, after Phase 3.3 cross-provider E2E v2):
- `go build ./...` passes — all Phase 5 commands + P0 codex workspace-cd fix compile cleanly
- `go test ./...` passes — all unit tests including `TestHasTaskCompletedEventAcceptsTaskClosed`, `TestPromoteWorkspaceHandoffCopiesWhenArtifactHasTemplate`, `TestPromoteWorkspaceHandoffSkipsWhenArtifactAlreadyGood`, `TestBuilderBuildCodexPrependsWorkspaceCd`
- **Phase 3.3 cross-provider E2E v2** (WSL2, codex-be + claude-fe, 2026-05-26):
  - Both agents used permanent per-agent workspaces (free-roam mode)
  - codex-be: `server.py` + `test_server.py` committed to `agents/codex-be` with `[TASK-xxx]` prefix ✅
  - claude-fe: `index.html` committed to `agents/claude-fe` with `[TASK-xxx]` prefix ✅
  - claude-fe: **5/5 verify checks** autonomously — used `aom task signal task.completed` correctly
  - codex-be: **5/5 verify checks** after operator sends `aom task signal task.completed` — work was complete but codex used `step.completed`+`checkpoint.created` instead of the final task signal
  - Both tasks accepted (`aom task accept`) and merged to main (`aom merge commit --prefer-branch`) cleanly
  - Fixes confirmed working: handoff.md path mismatch (promoteWorkspaceHandoff), task.closed accepted (hasTaskCompletedEvent), state.md update (profile enforcement)
  - Remaining compliance gap: codex runtime sometimes uses `step.completed`/`checkpoint.created` instead of `task.completed` as final signal — operator can unblock by running `aom task signal task.completed` manually
- WSL2 E2E (earlier): builder → reviewer full pipeline passed 5/5 verify checks; `aom task accept` without `--force`; `aom merge commit` to main (see `E2E-REPORT-WSL2-CLAUDE.md`)
- (Preserved below: earlier flow records from milestones 2–4 and worktree repair/session/artifact coverage)
- `go test ./...` passes (earlier milestone baseline)
- live local Milestone 2 flow passes on macOS:
  - `aom project init aom --repo .`
  - `aom open`
  - `aom status`
  - `aom session spawn backend-main`
  - `aom session list`
  - `aom session show <session-id>`
  - `aom capture <session-id>`
- live first-slice Milestone 3 flow passes on macOS:
  - `aom task create "Implement milestone 3 slice" --role backend --agent backend-main`
  - `aom task show <task-id>`
  - `aom step list <task-id>`
  - `aom status`
- live second-slice Milestone 3 flow passes on macOS:
  - `aom task update <task-id> --mode bugfix --status ready`
  - `aom step update <step-id> --status confirmed`
  - `aom step update <step-id> --status ready`
  - `aom task update <task-id> --status in-progress`
  - `aom task close <task-id>`
- live third-slice Milestone 3 flow passes on macOS:
  - `aom plan "fix login bug"`
  - `aom plan "fix checkout bug" --create`
  - `aom project init template-check --repo /private/tmp/aom-template-init-check --template-dir ./internal/project/templates/project-init`
  - `aom project init minimal-check --repo /private/tmp/aom-template-minimal-check --template minimal`
  - `aom status`
  - `aom session spawn reviewer-main --mock`
  - `aom capture SESS-1778508537180275000`
- live first-slice Milestone 4 flow passes on macOS:
  - `aom task create "Seed artifact layer" --role backend --agent backend-main`
  - inspect `.aom/tasks/TASK-1778509207359142000/{task,state,index,log}.md`
  - `aom plan "capture auth requirements" --mode requirements-first --create`
  - inspect `.aom/tasks/TASK-1778509234475738000/{task,state,index,log,requirements,tasks}.md`
  - `aom session spawn backend-main --task TASK-1778509474319106000 --mock`
  - inspect `.aom/tasks/TASK-1778509474319106000/{index,log}.md`
- current repo verification on macOS:
  - `go test ./...`
  - focused unborn-branch preflight coverage:
    - `task create` and `plan --create` reject git repos without an initial commit before task persistence
  - focused worktree repair coverage in `internal/worktree` and `internal/cli`
  - live local `worktree repair` smoke flow on macOS:
    - `aom task create "Repair flow live check" --role backend --agent backend-main`
    - remove the provisioned `.aom/worktrees/<task>/` path manually
    - `aom status`
    - `aom worktree repair <task-id>`
    - `aom task show <task-id>`
  - focused drift classification and safe repair coverage:
    - missing worktree paths surface a recreate-specific repair hint and next action
    - unregistered artifact-only worktree paths now auto-repair when the path is empty or contains only `.agent/`
    - unregistered worktree paths with non-artifact content now surface a manual cleanup requirement instead of auto-repair
  - focused session/worktree reconciliation coverage:
    - missing tmux pane drives `Idle -> Detached`
    - detached task-bound session removes the `Active` worktree claim and returns the mapping to `Ready`
  - focused explicit shutdown/archive coverage:
    - `session stop` moves a live task-bound session to `Stopped` and returns the worktree to `Ready`
    - `session stop` still persists `Stopped` and appends the cleanup warning to `log.md` when tmux pane teardown fails
    - `session archive` moves a stopped session to `Archived`
  - focused replacement coverage:
    - `session replace <old> --agent <new-agent> --reason ...` creates a new session in the same task/worktree, keeps the worktree `Active`, and stops the superseded session when possible
    - if the old session has already reconciled to `Detached`, replacement now archives it automatically after the replacement session is created
    - when the old session is still `Working`, replacement output now leaves it running intentionally and prints a concrete `aom session stop <old-session-id>` hint for the operator
  - focused real-runtime launch coverage:
    - `session spawn backend-main --task <task-id> --step <step-id> --real` launches `codex` in a real tmux pane on macOS
    - `session spawn reviewer-main --real` launches `claude` in a real tmux pane on supported environments
    - unsupported runtime roles fail before pane creation
    - `session replace <session-id> --agent backend-main --real` reuses the same runtime validation path
  - focused canonical artifact-path reporting coverage:
    - `task show <task-id>` prints canonical `Artifact root` and `Task log` paths
    - `status` prints `artifacts=... | log=...` under each task
  - focused one-writer-per-worktree coverage:
    - a second `dedicated-writer` on the same task is rejected while read-only roles can still attach to the task context
- focused checkpoint and handoff coverage:
  - `checkpoint <session-id>` appends `checkpoint.created`, refreshes `index.md`, and prints checkpoint metadata
  - `handoff <session-id> --to <role-or-agent>` writes `handoff.md`, appends `handoff.prepared`, and moves the source session to `WaitingHandoff`
- focused review wrapper and unresolved review coverage:
  - `review <task-id>` writes `review-notes.md` and returns a tmux-unavailable follow-up hint without failing on Windows-style environments
  - `status` and `task show` surface unresolved review item counts and review-driven next actions from `review-notes.md`

Suggested verification commands on a new machine:

```bash
env \
  GOTOOLCHAIN=local \
  GOCACHE=$PWD/.cache/gocache \
  GOMODCACHE=$PWD/.cache/gomodcache \
  GOTELEMETRY=off \
  GOTELEMETRYDIR=$PWD/.cache/gotelemetry \
  go test -timeout 20m ./...
```

Note: `internal/cli` integration tests run real git operations and take ~10 minutes on Windows. Use `-timeout 20m` on Windows; macOS/Linux is typically under 5 minutes.

## Environment Notes

### Go

This repo has been verified with:
- Go 1.24.x

The repository currently uses:
- `gopkg.in/yaml.v3`
- `modernc.org/sqlite`

### tmux and Live E2E

Current state:
- live tmux E2E is verified on macOS with working `tmux`
- the earlier Windows environment did not successfully run live tmux E2E
- the earlier Windows execution context did not have a working `tmux` path or usable `wsl.exe`
- the current Windows handoff environment is still not suitable for meaningful live tmux plus real-agent E2E validation

What this means:
- code and tests for tmux logic pass
- live local tmux behavior is verified on macOS
- narrow real-runtime launch is verified in code and tests for `codex` and `claude`; live macOS E2E is still only recorded for the earlier `codex` path
- broader multi-runtime provider-native E2E is still not complete beyond the current narrow `codex` plus `claude` launch slice

Recommended path for live E2E:
- Linux or macOS should work best for continued live runtime validation
- Windows still needs a working WSL + tmux path if it is used again for live checks
- the next real-agent smoke work should be executed on a machine with verified `tmux`, `git`, and target runtime availability

### Agent UX Improvements — Post-M17 (2026-05-18)

Implemented based on feedback from live login-app multi-agent pipeline:

#### CLI UX Fixes
- `aom task link` now accepts `--depends-on` as an alias for `--blocked-by` — matches intuitive flag name agents expect
- `aom session wait` error message now lists all valid event types: `task.completed`, `handoff.prepared`, `checkpoint.created`, `step.completed`, `task.unblocked`
- `aom task list` now accepts `--status <value>` (case-insensitive filter) and `--format json` (machine-readable output with `blocked_by` array); args were previously silently ignored
- `aom next` now accepts `--format json` — returns `{ unblocked: [...], blocked: [...] }` for scripted pipeline automation

#### Behavior Fixes
- Codex trust-dir dialog response now retried 3 times at 4-second intervals instead of once after a fixed 3-second sleep — prevents brief swallow when the codex prompt renders slowly
- `aom task close` now auto-emits `task.unblocked` events to all dependent tasks whose blockers are fully resolved; also appends a channel message so the orchestrator can detect unblocking via `aom session wait --event task.unblocked` without polling

#### Agent Onboarding — Spawn-time Context
- **Team Roster** injected into agent identity file (`CLAUDE.md`/`AGENTS.md`) at spawn time: table of all project agents with roles, runtimes, and current task assignments; includes ready-to-use `aom message send <name>` commands so agents know who to contact
- **Pipeline Position** added to `task.md` artifact: lists which tasks block the current task and which tasks it unblocks when done, with agent assignments and current status — agent understands its place in the workflow without asking
- **AOM Workflow Guide** expanded in `profile.md` template: replaces the 3-bullet "Working Protocol" with a full guide covering session start sequence, mid-task checkpointing, completion signaling (valid log events with descriptions), `session wait` usage, and `aom next` for discovering what to work on

### Cross-Platform UX & Merge Conflict Fixes (2026-05-18)

From a second round of live feedback (Windows 11 + WSL, login-app pipeline).

#### Status

| Priority | Issue | Status |
|---|---|---|
| Critical | `project.yaml` stores absolute Windows path — Linux binary fails to read it | **DONE** — `repo: .` in all templates |
| Critical | `aom` binary committed to repo is Windows PE — WSL operators must build manually | **NOT DONE** |
| High | `aom merge commit` conflict: raw git error, no guidance | **DONE** — `aom merge continue` + `aom merge abort` added |
| Medium | Planned→Ready requires 3 commands | **DONE** — `aom task ready` added |
| Low | `aom session send` shell-escaping for long messages | **DONE** — `--file <path>` flag added |
| Low | `aom merge commit` does not auto-run merge check first | **DONE** — `executeMergeCheck` called at top of `executeMergeCommit` (`merge_cmd.go:486`) |

#### What was verified as already working (no fix needed)

- **Walk-up project detection**: `findProjectRoot()` in `internal/config/config.go:164` already walks up from CWD to find `.aom/project.yaml` — agents running inside worktrees can call AOM commands without setting a project root manually. The reported error was from a Windows-path validation failure, not from missing walk-up logic.
- **Channel content at spawn**: already injected via `materializeAgentContext()` from session before
- **Team roster at spawn**: already injected via `buildTeamRosterNote()`

#### Remaining work

1. `.gitignore` + README WSL section (docs only)

---

### Windows/WSL2 Multi-Agent E2E Feedback — Planned Fixes (2026-05-18)

From a full live run of the login-app pipeline on Windows 11 + WSL2 (3 agents: orchestrator-main claude-haiku, backend-main×2 codex gpt-5.5, reviewer-main claude-haiku). All 4 tasks completed successfully, but significant friction was identified. All P0–P3 issues are now resolved.

#### Hook System Analysis

There are two hook layers in AOM. This explains why "agents didn't use hooks":

**Layer 1 — AOM Shell Hooks** (`.aom/hooks/*.sh`, fired by `runHook()` in `internal/cli/hooks.go`):
- Fires on `on-session-spawn`, `on-task-ready`, `on-task-done` events
- Only runs if the `.sh` file exists (not `.sh.example`)
- In the live test, only `on-task-done.sh.example` was present — never renamed → hooks never fired
- Fix: `project init` should generate a working `on-task-done.sh` by default, not `.example`; `aom doctor` should warn when `.example` files exist without a live `.sh` counterpart

**Layer 2 — Agent-Initiated AOM Calls** (agents call `aom` CLI directly):
- `profile.md` template instructs agents to call `aom task checkpoint`, `aom worktree commit`, etc.
- On NTFS mount, `git commit` fails with `index.lock: Read-only file system`, breaking the automation loop
- Agents did the work correctly but couldn't signal completion via git

#### New Issues Found

| Priority | Issue | Status |
|---|---|---|
| **P0** | `project init` fails with `mkdir .aom: file exists` on NTFS mount | **DONE** — stat-check fallback in `project/service.go` |
| **P0** | Agents can't `git commit` — `index.lock: Read-only file system` on NTFS | **DONE** — `aom doctor` NTFS mount warning + NTFS hint in `base.md.tmpl` |
| **P0** | AOM hooks never fire — `.sh.example` not renamed | **DONE** — `ensureHooksDir` in `config_files.go` generates live `on-task-done.sh`; `aom doctor` warns on unactivated `.sh.example` |
| **P1** | Model spawn fails silently — error appears after spawn | **DONE** — `KnownModels()` soft-warn in `session_spawn_helpers.go` |
| **P2** | `CLAUDE.md` written by Claude agents → add/add merge conflict | **DONE** — `resolveAgentArtifactConflicts` in `merge_cmd.go` auto-resolves with "ours" |
| **P2** | `--file /dev/stdin` permission denied in WSL2 | **DONE** — `--file -` reads from `os.Stdin` |
| **P3** | No `task cancel` — orphan Draft/Planned tasks accumulate | **DONE** — `aom task cancel` added |
| **P3** | Skeleton files from T1 cause add/add conflict when T2/T3 merge | **DONE** — `--prefer-branch` flag for `aom merge commit` |

#### What Worked Well (no fix needed)

- `aom doctor` — clear environment diagnosis from the start
- `aom worktree commit` — essential workaround for NTFS, works correctly
- `aom session spawn` output — model, worktree path, native session ID all visible
- `aom merge check` — Green/Yellow/Red score is useful and accurate
- `aom task link --blocked-by` — dependency chain worked correctly
- `aom task accept` / `aom capture` / session resume — all functioned as designed
- Policy enforcement (`--disallowed-tools`) — enforced correctly for both claude and codex

#### All P0–P3 Issues Resolved

All items above are implemented. No remaining Windows/WSL2 work other than the binary distribution note (operators on WSL2 must build the binary from source — `go build -o aom cmd/aom/main.go`).

---

### Per-Agent Workspace (Free-Roam) — Full Implementation (2026-05-21)

Complete implementation of the per-agent workspace model described in `docs/free-roam-workspace.md`.
Verified end-to-end in WSL with real `--real` claude sessions.

#### Track A — Per-Agent Workspace (all steps done)

| Step | Status | Notes |
|------|--------|-------|
| A1 — DB schema: `workspace_path` column | ✅ Done | `internal/db/db.go` migration v9 |
| A2 — `agent.Record.WorkspacePath` + Upsert | ✅ Done | `internal/agent/repository.go` + CASE WHEN preservation |
| A3 — `ProvisionAgentWorkspace` | ✅ Done | `internal/worktree/service.go` |
| A4 — `aom agent provision <name>` | ✅ Done | `internal/cli/agent_cmd.go` |
| A5 — session spawn uses workspace CWD | ✅ Done | `internal/cli/session_cmd.go` — checks `WorkspacePath` before task guard |
| A6 — artifact root routes to workspace | ✅ Done | `internal/artifact/service.go` + `SyncParams.AgentWorkspacePath` |
| A7 — task create skips per-task worktree | ✅ Done | `internal/cli/task_cmd.go` — `agentHasWorkspace` guard |
| A8 — merge uses `agents/<name>` branch | ✅ Done | `internal/cli/merge_cmd.go` — `resolveSourceBranch` + `--fixed-strings` |

#### Bug Fixes (found during E2E testing)

| Bug | Fix | File |
|-----|-----|------|
| `workspace_path` reset to `""` by `agentRepo.Sync()` on every project Open | CASE WHEN SQL preserves existing value when incoming path is empty | `internal/agent/repository.go` |
| `task create` still provisioned per-task worktree for workspace agents | Skip `ensurePlannedWorktree` when `agentHasWorkspace` | `internal/cli/task_cmd.go` |
| `merge check/prepare` crashed for workspace agents (`Worktree == nil`) | Use `resolveSourceBranch` in all merge paths | `internal/cli/merge_cmd.go` |
| `session spawn` without `--task` wrote identity files to repo root, not workspace | Check `WorkspacePath` before task guard in `executeResolvedSessionSpawn` | `internal/cli/session_cmd.go` |
| `aom session resume <agent>` picked oldest dead session, not newest | `loadSessionByIdentifier` now keeps last match when scanning by agent name | `internal/cli/helpers.go` |
| `task.md` showed relative `Artifact Root` — wrong path from workspace CWD | 3-case `renderTaskMarkdown`: workspace → absolute path + workspace note; worktree → relative + CWD note | `internal/artifact/service.go` |
| 5 runtime test assertions stale after `NiceExecPrefix` + `npm_config_cache` added | Updated all 5 expected strings | `internal/runtime/launch_test.go` |

#### Same-Runtime Conflict Guards (G1/G2/G3)

Multiple agents using the same runtime (e.g. two claude agents) without workspaces
would overwrite each other's `CLAUDE.md`/`AGENTS.md` in the repo root.
Three guards prevent this:

- **G1** (`session_cmd.go`): `aom session spawn` warns when the agent has no workspace and another enabled agent with the same runtime also lacks a workspace — prints fix command.
- **G2** (`doctor.go`): `aom doctor` adds a `workspace: <runtime>` check — `[WARN]` lists agents missing workspaces with exact `aom agent provision` commands; `[PASS]` when all same-runtime agents have workspaces.
- **G3** (`project_cmd.go`): `aom project init` prints a "Next: provision a dedicated workspace" block with `aom agent provision <name>` for every agent created.

#### E2E Verification (WSL, 2026-05-21)

All features verified in `/tmp/aom-g1g2g3-test` (WSL Ubuntu, 3 agents: frontend-main/claude, reviewer-main/claude, backend-main/codex):

- G3: init shows provision hints ✅
- G2: doctor WARN → partial WARN → PASS as agents are provisioned ✅
- G1: spawn warns when both claude agents lack workspace; no warning after one is provisioned ✅
- Bug 1 (workspace_path persistence): `aom agent list` shows workspace after re-open ✅
- Bug 2 (no worktree): `.aom/worktrees/` absent for workspace-agent tasks ✅
- Bug 4 (workspace CWD): `CLAUDE.md` written to workspace, not repo root; session shows correct `Worktree path` ✅
- Real `--real` spawn: native session ID `b2731ec2-...` auto-detected; conversation file created at `~/.claude/projects/-tmp-aom-g1g2g3-test--aom-agents-frontend-main-workspace/` ✅
- `claude -r` from workspace finds the session ✅
- `task.md` for workspace agent shows absolute Artifact Root ✅
- Both models (workspace + traditional worktree) coexist in same project ✅

## What Is Intentionally Not Done Yet

Still out of scope at the current handoff point:
- gemini/kiro full runtime launch: `internal/provider/gemini.go` and `internal/provider/kiro.go` are stubbed — `LaunchCommand` returns an error; fill in when CLI flags are confirmed
- provider-native resume for `gemini` and `kiro` (claude and codex resume flows are live)
- runtime command interception for policy enforcement (M10): `deny_commands` are enforced via `--disallowed-tools` for claude (runtime-level); codex has no equivalent flag — identity file injection is the maximum available enforcement for codex
- M17 Gemini/Kiro: deferred — no confirmed CLI flags available for testing

## AI Orchestrator Path

The system has been validated as a viable foundation for AI-driven orchestration,
where a Claude Code session acts as the orchestrator and manages sub-agent sessions
(codex, claude, kiro) as workers. The artifact layer (.agent/*.md) serves as the
structured interface between orchestrator and sub-agents. The orchestrator reads
only artifact summaries, not raw terminal output.

### Core insight

Each sub-agent maintains focused context only for its own task in its own worktree.
The orchestrator context stays small because it reads handoff.md and state.md
summaries rather than execution details.

### Session reuse model

After a sub-agent signals completion, the orchestrator may resume the same session
(same tmux pane, same agent conversation) for the next task via `aom session send`
instead of spawning a new session. This preserves native conversation context across
tasks. The state machine already supports this: Working → WaitingHandoff → Idle →
Working. If the pane has died, artifact-backed continuity kicks in via session
replace.

### Gaps — Tier 1 (minimal viable AI orchestration)

- Completion event convention: agents append `task.completed` or `handoff.prepared`
  to log.md when work is done; see docs/artifact-schemas.md Agent Completion Protocol — convention is defined, live E2E verified with codex and claude
- Runtime identity file materialization: DONE — `project init` seeds `.aom/agents/<name>/profile.md`
  for every enabled agent; `session spawn` and `session resume` call `artifact.MaterializeIdentityFile`
  to copy the profile into the worktree as `CLAUDE.md` (claude), `AGENTS.md` (codex), or `GEMINI.md`
  (gemini); kiro uses a directory format (`.kiro/rules/`) and is intentionally deferred

### Gaps — Tier 2 (ergonomic orchestration)

- Initial context delivery: task-bound session spawn should note the .agent/task.md
  path in the launch message so the agent knows its brief on startup

### Gaps — Tier 3 (robust orchestration)

- Continuity readiness scoring populated in index.md (field exists in schema,
  logic not yet in artifact/service.go)
- Orchestrator actor type in log events: distinguish `"actor": "orchestrator-ai"`
  from `"actor": "operator"` for audit clarity
- Provider-native resume for pane recovery: resume on spawn is now live for claude and codex; auto-rebind of a detached pane to an existing running session (without a full re-spawn) is still out of scope (Milestone 10)

### Recommended implementation order

1. gemini/kiro runtime support — fill in `LaunchCommand` in `internal/provider/gemini.go` and `internal/provider/kiro.go` when CLI flags are confirmed; no other files need to change
2. M9 governance — MCP resource bindings, role skill injection at `session spawn`, policy enforcement

## E2E Feedback Improvements (2026-05-18)

Implemented from real-world usage feedback (`AOM_FEEDBACK.md`):

### Codex runtime fixes
- **Smart deny-command wrapper** (`internal/provider/codex.go`): multi-word deny entries (e.g. `git push`) now generate a wrapper that intercepts only the specific sub-command and passes everything else through to the real binary — no longer blocks all `git` operations
- **PYTHONDONTWRITEBYTECODE=1** injected into codex session preamble to suppress `__pycache__` noise in worktrees

### tmux capture improvements
- **Auto-flush outbox** on `aom capture`: if a project is open, staged outbox messages are flushed before reading pane output
- **`aom capture --follow` / `--diff`**: `--follow` polls the pane at `--interval` (default 2s) until Ctrl-C; `--diff` prints only new lines since the last capture (no repeated output)

### UX improvements
- **`aom status --active`**: filters to InProgress/Blocked/NeedsAttention/Ready tasks and non-archived sessions only
- **`aom status --graph`**: prints an ASCII dependency graph with status symbols (✓/⟳/○/!)
- **`aom doctor --global`**: checks only tmux + runtime binaries; skips project-local checks — useful before `aom project init`
- **`aom task create --step-type <type>`**: sets the initial step type (defaults to existing behaviour)
- **Branch name truncation**: worktree branch names are capped at 80 characters to avoid filesystem limits

### Agent profile system refactor
- Profile content moved from hardcoded Go strings to embedded `.md.tmpl` template files (`internal/project/templates/project-init/profiles/`)
- 3-level template lookup at profile seed time: explicit `templateDir` → `.aom/templates/profiles/` → embedded default
- Built-in role classes with full profiles: `builder`, `frontend`, `reviewer`, `orchestrator`; unknown classes fall back to `default.md.tmpl`
- Custom classes: drop a `<class>.md.tmpl` file into `.aom/templates/profiles/` — no code change required

### Default agent template
- Removed `orchestrator-main` from default init template (operator is the orchestrator)
- Added `frontend-main` (runtime: claude, role: frontend)

### Team building
- **`aom agent add --class <class>`**: sets the role profile class; if the role already exists, `--class` updates its class; if the role is new, the class is used instead of defaulting to `builder`
- **Team Building section** in `base.md.tmpl`: every agent profile explains how to add new agents via `aom agent add`, assign tasks, and spawn sessions — agents can build their own team without operator involvement
- **`--help` operator workflow**: top of `aom --help` now shows Option A (operator as orchestrator) and Option B (delegate to an orchestrator agent) so the operator knows both patterns from day one

### Pending (Windows/WSL2 cross-platform)
- `project.yaml.tmpl` → `repo: .` (absolute Windows path breaks Linux binary)
- ~~NTFS `mkdir` false-positive in `project init` (`internal/project/service.go:112`)~~ **DONE** (stat-check fallback already in place)
- ~~NTFS `index.lock`: agent profile NTFS fallback instruction + `aom doctor` NTFS warning~~ **DONE** — `doctor.go` NTFS mount check + `base.md.tmpl` NTFS hint
- ~~`CLAUDE.md` add/add merge conflict: auto-resolve with "ours" in `executeMergeCommit`~~ **DONE** — `resolveAgentArtifactConflicts` in `merge_cmd.go`
- ~~`--prefer-branch` flag for `aom merge commit`~~ **DONE** — `merge_cmd.go`

### E2E Feedback — Second Round Fixes (2026-05-19)

Implemented from a full analysis of `AOM-FEEDBACK.md` after the first live multi-agent run.

#### `aom project init` — auto git init

- `ensureGitReady(repoPath)` added to `internal/project/service.go`: runs after `writeConfigFiles` + `seedAgentProfiles`
  - If not a git repo: runs `git init -b main`
  - If no HEAD commit: runs `git commit --allow-empty -m "initial commit [aom init]"` with hardcoded identity (`-c user.email=aom@aom -c user.name=AOM`)
  - All AOM-triggered git commands run via `aomGit()` helper which sets `credential.helper=`, `commit.gpgsign=false`, and `GIT_TERMINAL_PROMPT=0` — prevents credential-helper hangs on Windows
- `InitResult` struct gains `GitInitialized bool` and `GitInitialCommit bool`
- `executeProjectInit` prints git status lines when git actions were taken
- **Effect**: `aom project init` on an empty directory now produces a git-ready repo; `task create` immediately provisions real worktrees instead of staying `Planned`

#### `aom doctor` — new checks

- **git initial commit**: warns when the repo has no HEAD commit (worktree provisioning will fail)
- **codex update dialog**: warns when `~/.codex/version.json` has no `dismissed_version` set (codex spawn may hang on the update prompt)

#### Codex provider — dismissed_version guard

- `internal/provider/codex.go` preamble now writes `~/.codex/version.json` with `{"dismissed_version":"9999.0.0"}` if the file doesn't exist — suppresses the Codex update dialog at session spawn time

#### `aom worktree commit` — clearer errors

- No-worktree error now suggests `git commit` directly + `aom checkpoint <session-id>`
- Non-ready worktree error includes current status + same fallback suggestion

#### `aom task create --description`

- `--description <text>` flag added; stored in new `description` column (schema-v8 migration: `ALTER TABLE tasks ADD COLUMN description TEXT NOT NULL DEFAULT ''`)
- Description printed in `task create` output and available in `task show`

#### Windows git stability fixes

- `aomGit()` helper in `internal/project/service.go`: wraps all `ensureGitReady` git commands with `credential.helper=`, `commit.gpgsign=false`, and `GIT_TERMINAL_PROMPT=0` env var — prevents hangs from credential managers and GPG signing on Windows
- `worktree.NewService.runGit` updated with same env vars (`credential.helper=`, `GIT_TERMINAL_PROMPT=0`)
- `changedFilesSummary` in `internal/cli/worktree_cmd.go` now uses `exec.CommandContext` with a 5-second timeout — prevents git status hang from blocking handoff
- Test command updated to `-timeout 20m` in `CLAUDE.md` — cli integration tests run real git ops on Windows and need extra time

### E2E Feedback — Third Round Fixes (2026-05-19)

Implemented from analysis of `aom-feedback.md` after the login-demo full-pipeline run (backend-main/codex + frontend-main/claude + reviewer-main/claude).

#### `task.md` artifact — worktree path annotation

- `internal/artifact/service.go`: `Worktree:` field now appends `(informational — already your CWD; create files using relative paths only)`
- **Problem solved**: agents reading the absolute `/mnt/c/...` path from `task.md` were also creating files in the main workspace alongside the worktree, causing untracked files that blocked `aom merge commit`

#### `aom doctor` — git identity check

- New check block after "git: initial commit" inside the `git` availability guard
- Runs `git config --get user.email` and `git config --get user.name`; FAILs if either is empty
- Output: `[PASS] git: identity    Your Name <you@example.com>` or `[FAIL]` with exact fix command
- **Problem solved**: `aom worktree commit` was failing with "Author identity unknown" in WSL with no pre-flight warning

#### `aom agent list` — model column

- `internal/cli/agent_cmd.go`: added `MODEL` column between `ENABLED` and `PROFILE`
- Displays configured model slug (e.g. `gpt-5.3-codex`) or `(default)` when none is set
- **Problem solved**: operators could not tell which model each agent was using without reading `agents.yaml` directly

#### `agents.yaml` templates — commented model examples

- All 3 template files (`internal/project/templates/project-init/agents.yaml.tmpl`, `templates/project-init/default/agents.yaml.tmpl`, `templates/project-init/minimal/agents.yaml.tmpl`) now include commented `# model:` lines with valid aliases for each runtime
- **Problem solved**: `model:` field was undiscoverable from the generated config file

### Stability & Observability Round (2026-05-19)

Three spec-defined gaps implemented:

#### `aom session recover`

- New command: `aom session recover <session-id|agent-name>`
- Diagnoses a stopped or failed session: pane liveness, runtime binary availability, task artifact continuity, native session ID
- Outputs a continuity quality rating and a recommended recovery action:
  - Pane alive → `aom session rebind`
  - Native session ID → `aom session replace --real`
  - Task-bound → `aom session spawn --task --real`
  - No task, no session → `aom session archive`
- Wired in `executeSession` switch (`internal/cli/root.go`); implemented in `internal/cli/session_cmd.go`

#### `aom events tail`

- New top-level command: `aom events tail [--task <id>] [--timeout <duration>]`
- Streams new log.md events for a task to stdout as they appear, polling every 2s (reuses `tailLogEvents` from `internal/cli/log_wait.go`)
- Auto-detects task from `AOM_ACTOR` env var when `--task` is omitted — matches how agents call AOM from inside a session
- Default timeout: 30 minutes; override with `--timeout 2h`
- Implemented in `internal/cli/observability_cmd.go`

#### Codex commit reminder at spawn

- `executeResolvedSessionSpawn` in `internal/cli/session_cmd.go`: for real-mode codex sessions with a bound task, auto-sends a tmux key sequence instructing the agent to `git add -A && git commit` and append `task.completed` before finishing
- Sent after the startup dialog loop to avoid interfering with the "1" response keys
- Eliminates the need to manually remind codex agents to commit via `aom session send`

### Session UX Polish (2026-05-19)

Two targeted improvements to session operator UX:

#### `aom session list --active`

- `executeSessionList` in `internal/cli/session_cmd.go` now accepts `--active` flag
- Filters output to sessions in active statuses: `Booting`, `Idle`, `Working`, `WaitingApproval`, `WaitingHandoff`, `Blocked`
- `isActiveSessionStatus` helper extracted to `session_spawn_helpers.go` (reused by `resumeSessionNative`)
- Help text updated: `aom session list [--active]`

#### `aom session resume` smart auto-recovery

- `executeSessionResume` in `internal/cli/session_cmd.go` now supports two modes:
  - **With `--task`**: original rebind-to-new-task behaviour (`executeSessionResumeToTask`)
  - **Without `--task`**: new smart auto-recovery (`executeSessionAutoResume`)
- Auto-recovery decision tree (4 paths in priority order):
  1. Tmux pane still alive → un-detach (fastest, full context intact)
  2. `VendorSessionID` exists → `resumeSessionNative` creates new tmux pane resuming the native agent session (`claude --resume` / `codex resume`)
  3. Task-bound but no native session → prints `aom session spawn --task --real` hint
  4. No recovery path → prints `aom session archive` hint
- `resumeSessionNative` in `session_cmd.go`: creates a new pane using the stored `VendorSessionID`, updates session record, renames window, emits `session.resumed` log event

### E2E Feedback — Fourth Round Fixes (2026-05-19)

Fixes derived from live login-demo workflow with backend-main (Codex), frontend-main (Claude Haiku), and reviewer-main (Claude Haiku).

#### Codex commit loop root cause fix

- `session_cmd.go` commit reminder now instructs codex to run `git add -A && git commit` **synchronously in the foreground** (not in a background terminal)
- Fallback changed from NTFS-only to **any failure**: "If that fails for ANY reason, use: `aom worktree commit <task-id>`"
- Explicit prohibition: "Do NOT use timeout wrappers, perl alarms, or retry loops"
- `base.md.tmpl` AGENTS.md template updated to match — expands NTFS-only fallback to all failure types
- `artifact/service.go` task.md Success Criteria updated to reference `aom worktree commit` fallback

#### DB permissions fix (P0)

- `internal/db/db.go`: pre-creates `sessions.db` with `0o664` permissions before `sql.Open`
- Previously created by the SQLite driver with default `0o644`, blocking Codex sandbox writes (`attempt to write a readonly database`)

#### `aom doctor` improvements

- Added **`aom in PATH`** check (warn if binary not findable by agents)
- Improved **database** check: now tests write access with `os.OpenFile(O_WRONLY)` instead of just `os.Stat`; prints `chmod 664` fix hint when not writable

#### `aom agent set-model <name> <model>`

- New command that safely updates only the `model:` field in `agents.yaml` without touching other required fields (`role`, `runtime`, `enabled`)
- Prevents accidental full-overwrite of `agents.yaml` that previously caused `unknown role ""` errors
- Validates model slug against provider's known list and warns with ChatGPT-vs-API distinction for codex
- `internal/project/agent_profiles.go`: new `SetAgentModel(aomPath, agentName, model)` function

#### Codex model slug warning clarity

- `internal/provider/codex.go` `ModelHint()`: explicitly notes that `gpt-4.x` series (gpt-4o, gpt-4.1, gpt-4.1-mini) requires an **OpenAI API account**, not a ChatGPT account

#### Builder profile: sandbox network constraint

- `profiles/builder.md.tmpl`: added **Sandbox Constraints** section
- Instructs agents: if npm/pip/go install fails with a network error, write a stub, note "requires npm install post-merge" in `state.md`, and continue — do not retry in a loop
- Also instructs agents to always `cd` to the correct subdirectory before running package manager commands

### E2E Feedback — Fifth Round Fixes (2026-05-19)

Fixes derived from live login-demo workflow (AOM-FEEDBACK.md — second full E2E run with backend-main/Codex, frontend-main/Claude Haiku, reviewer-main/Claude Haiku).

#### P1 — `.agent/` directory permission fix

- `internal/artifact/service.go`: new `ensureDir(path)` helper that calls `os.MkdirAll` then `os.Chmod(0755)` — forces execute bits regardless of process umask
- All `os.MkdirAll(dir, 0o755)` call sites in the artifact service replaced with `ensureDir(dir)` (covers `.agent/`, `.codex/`, `.aom/team-brief.md`, `.aom/project-board.md`)
- **Root cause**: `os.MkdirAll` respects the caller's umask; a strict umask (e.g. `0113`) strips the execute bit from newly created directories, leaving `.agent/` at `drw-rw-r--` (0664) — which prevents `ls`, `cd`, or file writes inside it
- **Effect**: `.agent/` is always created with `drwxr-xr-x` (0755); agents can write task artifacts without manual `chmod` workarounds

#### P3 — `aom watch` no longer returns immediately when no active tasks

- `executeWatchAllTasks` in `internal/cli/observability_cmd.go`: instead of printing "No active tasks to watch." and returning, now calls `waitForActiveTasks(result, timeout)` which polls every 5 seconds until at least one active task appears or the timeout elapses
- New `waitForActiveTasks` helper polls task + worktree service in a loop; starts streaming as soon as active tasks are found
- **Fix**: `aom watch --timeout 8m` launched before tasks are started now blocks and begins streaming when tasks enter InProgress/Blocked/NeedsAttention/Ready

#### P5 — `aom policy list [--task <task-id>]`

- New command in `internal/cli/policy_cmd.go`; wired in `internal/cli/root.go` (`case "policy"`)
- Without `--task`: prints project-level `deny_commands` (BLOCK list), `require_approval` (GATE list), yolo mode, and approval scope
- With `--task <id>`: additionally shows the assigned agent, its runtime, and the enforcement level (`--disallowed-tools` flag, PATH wrapper scripts, or instruction-only) so agents can see exactly what is blocked without reading `policy.yaml`

#### P6 — `aom session stop` idempotent

- `internal/session/service.go` `Stop()`: returns no-op (`&record, nil`) when session status is already `"Stopped"` instead of returning an error
- **Fix**: scripts that run `stop` + `accept` in sequence no longer need error handling for the double-stop case

#### P7 — Dependent tasks auto-transition to Ready when all blockers Done

- `internal/task/service.go` `Update()`: after a task transitions to `Done`, calls new `promoteUnblockedDependents(taskID)` method
- `promoteUnblockedDependents` iterates all tasks that depend on the completed task via `UnblocksIDs`; for each `Planned` dependent whose every blocker is `Done` or `Archived`, transitions it to `Ready`
- Errors are swallowed so a Done transition never fails due to a dependent-promotion failure
- **Fix**: review tasks (and any task with all blockers done) automatically become `Ready` without requiring a manual `aom task ready` call

#### `aom doctor --fix`

- `internal/cli/doctor.go`: new `--fix` flag triggers `executeDoctorFix()` instead of the diagnostic run
- Fixes: `sessions.db` → `chmod 664`; all `.agent/` directories in `.aom/worktrees/` → `chmod 755`; all `.agent/*.md` files → `chmod 664`
- Reports `FIXED <path> → <mode>` per item and `Fixed: N  Failed: M` summary

#### `aom broadcast --file <path>`

- `internal/cli/tmux_cmd.go` `executeBroadcast`: added `--file <path>` flag that reads message content from a file instead of an inline string argument
- Mutually exclusive with inline message; enables sending detailed Markdown briefs (e.g. `aom broadcast --file .aom/team-brief.md --sessions ...`)

#### macOS test fix

- `internal/cli/root_test.go` `TestExecuteSessionSpawnWithTaskRefreshesArtifacts`: `filepath.EvalSymlinks(repoRoot)` applied at test start so path comparisons work correctly on macOS where `/var` is a symlink to `/private/var`

## E2E Feedback Rounds 8–11 — Codex Background Terminal Cleanup (2026-05-20)

### Root cause identified

Codex uses a **parallel background terminal model**: each exploratory/build command runs in a separate `/bin/zsh` subprocess. When codex hits a usage limit or gets interrupted, these child processes become orphaned. When `.aom/sessions.db` was tracked in git, every `git add -A` staged a 69 KB binary → git slow → codex spawned more background terminals trying to retry → fan activity.

Claude Code does not have this problem: it uses a sequential tool-use loop (one bash subprocess, commands run serially).

### Fixes applied

#### `.aom/` gitignore + doctor check (round 10)
- `config_files.go` `defaultGitignoreEntries`: added `.aom/` and `.agent/` as first entries so `aom project init` and `aom open` always write them before any agent commit
- `doctor.go`: new `git: .aom/ tracked` check — warns with fix command if `sessions.db` or `channel.md` is committed
- `builder.md.tmpl`, `reviewer.md.tmpl`: added commit guard warnings

#### Agent name-runtime mismatch warning (round 10)
- `agent_cmd.go` `executeAgentAdd`: checks if agent name contains a known runtime hint (codex, claude, gemini, kiro) that differs from `--runtime`; prints warning before creating agent
- `base.md.tmpl` Team Building section: IMPORTANT note that agent name is a label only

#### `Idle (pane live)` indicator (round 10)
- `helpers.go` `printProjectSummary`: for Idle sessions with pane alive, appends `(pane live)` in yellow and a line `attached=yes — process still running; run: aom session stop <id>`

#### Sandbox Constraints in all profiles (rounds 9–11)
- `builder.md.tmpl`, `frontend.md.tmpl`, `reviewer.md.tmpl`, `orchestrator.md.tmpl`: each has a Sandbox Constraints section with the essential codex constraints (commit fallback, hang recovery)
- `base.md.tmpl` Completing work step 2: one-line CODEX reminder not to use background terminal feature (shared, read by all agents)
- Profiles trimmed: removed duplicate 10-line blocks; shared rule lives in base once

#### `generic.md.tmpl` non-coding profile (round 9)
- New file `internal/project/templates/project-init/profiles/generic.md.tmpl`
- Responsibilities, Work Standards, Output & Handoff — no npm/git/sandbox assumptions
- Used by `researcher`, `analyst`, `writer` classes via `aom agent add --class generic`

#### SQLite concurrent write fix (round 9)
- `internal/db/db.go`: `_txlock=immediate` in DSN forces `BEGIN IMMEDIATE` so `busy_timeout` fires at transaction start; `defaultBusyTimeoutMS = 30000`; `SetMaxOpenConns(1)` eliminates in-process contention

#### `aom channel read` outbox warning (round 9)
- `channel_cmd.go` `executeChannelRead`: calls `countPendingOutboxMessages()`; if n > 0 prints `⚠  N outbox message(s) pending — run: aom outbox flush` before channel content

#### `KillPaneAndDescendants` — provider-level infrastructure fix (round 11)
- `internal/tmux/manager.go`: new `PanePID(paneID)` reads `#{pane_pid}` from tmux; new `paneDescendants(rootPID)` BFS via `pgrep -P` (macOS + Linux); new `KillPaneAndDescendants(paneID)` SIGTERM all → 2 s → SIGKILL survivors → `kill-pane`
- `internal/cli/session_cmd.go` `stopSessionRecord`: replaced `KillPane` with `KillPaneAndDescendants` in stop path — codex background terminals, caffeinate, policy wrappers all killed when operator stops a session

#### 3-layer auto-cleanup chain (round 11)
- **Layer 1 — `aom status`**: `executeStatus` calls `autoStopCompletedSessions()` after loading sessions; for each Idle session with pane alive + `task.completed` in `.agent/log.md` → auto-stop + print `ℹ  auto-stopped <id> (<agent>)`. No operator action required.
- **Layer 2 — `aom task accept`**: `autoStopIdleSessionsForTask()` called at end of `executeTaskAccept`; stops any Idle/WaitingHandoff session bound to the accepted task
- **Layer 3 — `aom session stop`**: always uses `KillPaneAndDescendants`

#### Profile files changed
| File | Change |
|------|--------|
| `profiles/base.md.tmpl` | commit fallback `--local`, CODEX no-background-terminal line, generic class examples |
| `profiles/builder.md.tmpl` | Sandbox Constraints trimmed to 6 lines; `.aom/` gitignore warning |
| `profiles/frontend.md.tmpl` | Sandbox Constraints + `.aom/` gitignore warning (new section) |
| `profiles/reviewer.md.tmpl` | Commit Rules section + Sandbox Constraints (new section) |
| `profiles/orchestrator.md.tmpl` | Sandbox Constraints (new section) |
| `profiles/generic.md.tmpl` | New file — non-coding use cases |

## WSL2 Bwrap Root-Cause Fix — E2E Feedback (2026-05-22)

### Root cause identified

Codex bundles its own **bwrap (bubblewrap)** binary and routes **ALL** subprocesses through
`codex-linux-sandbox → bwrap` — even with `--sandbox danger-full-access`. On WSL2, the bwrap
overlay FS causes git VFS operations to spin at 60–100% CPU in a tight retry loop for optional
lock files (e.g. `commit-graph.lock`, `FETCH_LOCK`). `GIT_OPTIONAL_LOCKS=0` partially mitigates
this but cannot reach git processes inside bwrap's mount namespace.

### Fixes applied

#### Step 1 — Environment-level mitigation (`GIT_OPTIONAL_LOCKS=0`, `GIT_TERMINAL_PROMPT=0`)

- `internal/provider/codex.go` preamble now sets `GIT_OPTIONAL_LOCKS=0` and `GIT_TERMINAL_PROMPT=0`
- Reduces git lock spinning for processes outside the bwrap namespace; prevents git from blocking on credential prompts
- Not a complete fix — git inside bwrap's mount namespace does not inherit these env vars
- `aom doctor` adds a **`codex: bg terminal timeout`** check: warns when `~/.codex/config.toml` is missing
  `background_terminal_max_timeout = 60000`; global safety net — kills stuck background terminals within 60 s
  (codex default is 1 hour = 3 600 000 ms); must be placed at the **top level** of `config.toml`, not under `[agents]`

#### Step 2 — Root fix: `codex_bypass_sandbox: true` policy option + WSL2 auto-detect

- **New policy field**: `CodexBypassSandbox bool \`yaml:"codex_bypass_sandbox"\`` added to `PolicyConfig` in `internal/config/config.go`
- Propagation chain: `PolicyConfig.CodexBypassSandbox → SessionSpec.BypassSandbox → LaunchSpec.BypassSandbox → codex provider`
- Files changed: `internal/config/config.go`, `internal/runtime/launch.go`, `internal/provider/provider.go`,
  `internal/provider/codex.go`, `internal/cli/session_cmd.go` (3 `SessionSpec` construction sites)
- When enabled, replaces `--sandbox danger-full-access -a never` with `--dangerously-bypass-approvals-and-sandbox` — skips bwrap entirely
- Fresh start: `exec nice -n 19 codex --dangerously-bypass-approvals-and-sandbox <flags>`
- Resume: `exec nice -n 19 codex resume <id> --dangerously-bypass-approvals-and-sandbox <flags>`
- Safe on WSL2: AOM is the external control boundary; deny_commands are enforced via wrapper scripts
- **WSL2 auto-detect (2026-05-22)**: bypass is now applied automatically on WSL2 — no `policy.yaml` entry needed.
  `internal/provider/codex.go` reads `/proc/version` and checks for "microsoft" or "wsl" (case-insensitive);
  `spec.BypassSandbox` (from policy) is ORed with the auto-detect result. macOS/Windows/Linux-native are unaffected
  (`/proc/version` doesn't exist → `os.ReadFile` fails → `bypassSandbox` stays false).
- `aom doctor` **`codex: wsl2 bypass`** check: shows `[PASS]` with auto-detect message on WSL2
  (with or without the explicit policy field); shows nothing on non-WSL2 platforms.

#### Step 3 — Deny-command wrapper infinite loop on bwrap PATH

- **Root cause**: `buildCodexWrapperPreamble` in `internal/provider/codex.go` generated smart wrappers that used
  `exec env "PATH=${PATH#binDir:}" git "$@"` to strip the AOM policy dir before exec-ing real git.
  Inside bwrap, `codex-linux-sandbox` prepends `.codex/tmp/arg0/...` and `vendor/codex-path` entries **before**
  the AOM policy dir — `${PATH#binDir:}` only strips when PATH *starts* with `binDir:`, so it never matched
  → wrapper exec'd itself recursively at 100% CPU until the session was killed.
- **Fix**: `passThroughLine` now uses sed with **double-quoted** sed expression:
  ```sh
  exec env "PATH=$(echo "$PATH" | sed "s|/tmp/aom-policy-SESS/bin:||g;s|:/tmp/aom-policy-SESS/bin||g")" git "$@"
  ```
  Two patterns cover all positions: `binDir:` (start/middle) and `:binDir` (end of PATH).
- The fix applies to all smart wrappers regardless of whether `codex_bypass_sandbox` is set, so deny_commands work
  correctly even when bwrap is active.

#### Step 4 — Single-quote bug: `sh -lc` pane closes immediately when deny_commands active (2026-05-22)

- **Root cause**: `passThroughLine` in `buildCodexWrapperPreamble` used `sed 's|binDir:||g'` with **single quotes**.
  The entire preamble is assembled by `assembleLoginShellCommand` into `sh -lc '...'` (single-quoted outer wrapper).
  Any `'` inside the inner content prematurely terminates the outer string → `sh` gets a syntax error → exits
  immediately → tmux pane closes → AOM reports "pane closed immediately after spawn".
  **Symptom**: `aom session spawn` always fails with the "closed immediately" error when the project
  `policy.yaml` has any `deny_commands` (the default templates include `rm -rf` and `git push --force`).
- **Fix**: changed sed expression from `'s|...|g'` to `\"s|...|g\"` (escaped double quotes). The shell strips
  the quotes before passing the argument to sed; the result is identical — sed receives `s|...|g` either way.
  No single quotes anywhere in any preamble statement is now the enforced invariant.
- **Also fixed**: version.json `printf` format changed from `\x7b\x22...\x7d` (hex escapes) to
  `{\"dismissed_version\":\"9999.0.0\"}` — hex escapes are not supported by POSIX `dash` (the default
  `/bin/sh` on Ubuntu/Debian/WSL2); `\"` is the correct POSIX approach.
- **New invariant test**: `TestBuilderBuildCodexWrapsDenyCommands` now verifies that no single quotes
  appear in the inner content of the assembled `sh -lc '...'` command.

#### New test coverage

- `TestBuilderBuildCodexBypassSandbox` added to `internal/runtime/launch_test.go`
  - Verifies fresh start with `BypassSandbox: true` contains `--dangerously-bypass-approvals-and-sandbox` and does not contain `--sandbox`
  - Verifies resume with `BypassSandbox: true` produces `codex resume <id> --dangerously-bypass-approvals-and-sandbox`
  - Verifies default (no bypass) still produces `--sandbox danger-full-access`
- `TestBuilderBuildCodexWrapsDenyCommands` enhanced with single-quote invariant check
- `TestBuilderBuildResumesCodexSession` updated: expects POSIX-compatible printf format, WSL2-conditional sandbox flag
- All 13 launch tests pass

#### Step 5 — E2E feedback fixes (2026-05-22)

Four issues from `/tmp/aom-test-005/feedback.md` addressed:

**Fix #2 — workspace collision hard error** (`internal/cli/session_cmd.go`):
- G1 guard changed from `fmt.Fprintf(Warning: ...)` to `return fmt.Errorf(...)` — spawn is now
  **blocked** when two agents share the same runtime but neither has a dedicated workspace.
- New flag `--allow-collision` added to `sessionSpawnParams` and `executeSessionSpawn` to bypass.
- Tests updated: `--allow-collision` added to tests that spawn reviewer/frontend alongside each other.

**Fix #3 — task.completed gate** (`internal/cli/task_cmd.go`, `project_cmd.go`):
- `type verifyCheck struct` and `runTaskVerifyChecks(result, view) []verifyCheck` extracted from
  `executeTaskVerify` as a reusable helper that returns all completion checks (commits, state.md,
  handoff.md, task.completed in log, invariants).
- `executeTaskAccept` now runs `runTaskVerifyChecks` and blocks acceptance if any check fails.
  New `--force` flag bypasses the gate.
- `autoStopCompletedSessions` now runs `runTaskVerifyChecks` and skips auto-stop if any check fails.
  Prevents killing sessions where `task.completed` was written prematurely (agent wrote the event
  but forgot to commit or fill handoff.md).

**Fix #4 — reviewer premature finalization guard** (`internal/cli/review_cmd.go`, reviewer.md.tmpl):
- `executeReview` checks git log before spawning reviewer: if the task branch has no commits ahead
  of default branch, spawn is **blocked** with an explanatory error.
- New `--allow-empty-branch` flag to bypass (e.g. documentation-only tasks).
- `reviewer.md.tmpl` updated with "Implementation Readiness Check" section and matching step in
  Review Process.
- Tests updated: `--allow-empty-branch` added to review tests that spawn on empty branches.

**Fix #5 — repo layout coordination** (`internal/cli/project_cmd.go`, `root.go`, `session_spawn_helpers.go`):
- `aom project layout` command added: reads `git ls-tree --name-only HEAD` to get top-level structure,
  writes `.aom/shared/repo-layout.md`, and copies to `.agent/shared/repo-layout.md` in every active
  worktree.
- `materializeAgentContext` injects `.aom/shared/repo-layout.md` into new sessions at spawn time,
  so agents always have the repo layout without manual `aom project layout` calls.

---

## E2E Feedback — WSL2 Claude-Only Run (2026-05-26)

Full E2E test run on WSL2 Ubuntu with 3 claude-haiku agents (all-claude, all-workspace mode).
See `E2E-REPORT-WSL2-CLAUDE.md` for full detail. Three bugs found and fixed:

### Fix 1 — `aom task verify` workspace artifact routing (`internal/cli/task_cmd.go`)

- `runTaskVerifyChecks` now looks up the assigned agent's `WorkspacePath` via
  `findAgentByName(result.Agents, view.Task.PreferredAgent)`.
- **state.md check**: prefers `workspace/.agent/state.md` over task artifact `tasks/<id>/state.md`
  when the agent has a workspace — this is the live copy the agent writes.
- **task.completed check**: ORs `workspace/.agent/log.md` with the task artifact log so the
  agent's own completion signal is recognised regardless of which log it wrote to.
- **commits check**: when `view.Worktree == nil` but workspace exists, checks
  `agents/<name>` branch instead of skipping the check entirely.
- **invariant checks**: similarly uses `agents/<name>` branch for workspace agents.

### Fix 2 — Completion checklist injected into `task.md` for workspace agents (`internal/artifact/service.go`)

- `renderTaskMarkdown` now builds a `## When Done — Run These Commands` section for workspace
  agents (when `AgentWorkspacePath != ""`).
- Section is pre-filled with the exact step ID of the first active step, the agent name, and
  the task ID so the agent can copy-paste without looking anything up.
- When all steps are already terminal the step update command is omitted; the channel append
  command is always included.
- Traditional (non-workspace) agents are unaffected — no section injected.

### Fix 3 — `handoff.md` sentinel check (`internal/cli/task_cmd.go`)

- `runTaskVerifyChecks` check 3 now detects four template placeholder strings
  (`"Fill this in when the work is ready for transfer"`, `"Fill in what was completed"`,
  `"Fill in what still needs to happen next"`, `"Record touched files before signaling"`)
  and returns FAIL with `"handoff.md still contains template placeholder text"` when found.
- Previously the check only tested file existence + length > 80 bytes, which accepted
  unmodified templates.

### Tests added
- `internal/artifact/service_test.go`: `TestRenderTaskMarkdownWorkspaceAgentHasCompletionSection`,
  `TestRenderTaskMarkdownTraditionalAgentHasNoCompletionSection`,
  `TestRenderTaskMarkdownWorkspaceAgentAllStepsDone`
- `internal/cli/task_verify_test.go`: `TestHasTaskCompletedEventDetectsWorkspaceLog`,
  `TestHandoffSentinelRejection`, `TestHandoffSentinelPassesOnRealContent`

### Verified in WSL
`aom task verify` on the E2E demo-app task now shows:
```
[ok]  commits on branch      (agents/backend-main)
[ok]  state.md updated       (workspace/.agent/state.md)
[FAIL] handoff.md filled     (template text — correct, agent didn't fill it)
[ok]  task.completed in log  (workspace/.agent/log.md)
```

---

## Phase 2 — Reliable Multi-Agent Handoff ✅ (Complete 2026-05-26)

All Phase 2 items resolved. Builder → reviewer pipeline passes without silent failure and without `--force`.

### `aom task signal` — New Command

- CLI: `aom task signal <event-type> --task <id> [--summary <text>] [--step <id>]`
- Valid events: `task.completed`, `handoff.prepared`, `checkpoint.created`, `step.completed`
- Actor defaults to `AOM_ACTOR` env var (set at session spawn) or `"agent"`
- Writes a structured event entry to the canonical task artifact log via `syncTaskArtifacts`
- Best-effort mirrors to `workspace/.agent/log.md` via `appendSignalToWorkspaceLog` (silent no-op when file absent)
- Replaces manual log.md writes — agents call one command; AOM owns the schema
- `base.md.tmpl` Constraints: "Use `aom task signal` — do NOT write to .agent/log.md directly"
- `base.md.tmpl` AOM Workflow step 3 updated with exact signal commands for each event type
- `task.md` completion checklist for workspace agents pre-fills `aom task signal task.completed` command

### Profile and Verification Fixes

| Fix | File | Change |
|-----|------|--------|
| F1 — log.md schema contradiction | `profiles/base.md.tmpl` | Constraints section no longer contradicts Workflow step 3; agents use `aom task signal` only |
| F2 — verify missing `[TASK-xxx]` check | `internal/cli/task_cmd.go` | `runTaskVerifyChecks` Check 1b: ≥1 commit tagged `[TASK-xxx]` for workspace agents |
| F3 — hardcoded "Out of Scope" | `internal/artifact/service.go` | `renderTaskMarkdown` now shows project-neutral scope boundary |
| F4 — duplicate starting protocol | `profiles/base.md.tmpl` | Merged into single 6-step "Starting a session"; removed duplicate from Collaboration Routines |
| F5 — commit guard skips workspace agents | `internal/cli/task_cmd.go` | `aom task show` commit guard extended: workspace log + tagged commits on `agents/<name>` |
| F6 — verify syntactic not semantic | `internal/cli/task_cmd.go` | state.md check prefers `workspace/.agent/state.md`; task.completed check reads workspace log |
| F7 — orchestrator profile verify-gate | `profiles/orchestrator.md.tmpl` | "When a worker finishes" adds `aom task verify` step; NOTE on gate refusal; `--force` as last resort |
| F8 — frontend commit convention | `profiles/frontend.md.tmpl` | `[TASK-xxx]` commit convention note in Work Standards; explains `aom merge commit` dependency |

### E2E Result

```
builder: task done → aom task signal task.completed → verify 5/5 checks PASS → accept (no --force)
reviewer: spawned → review-report.md written → aom task signal task.completed → accept → aom merge commit
```

All Phase 2 milestones verified in WSL2 with real claude-haiku sessions.  
See `docs/dev/e2e-2agent-test-plan.md` for the full test plan and results.

---

## Phase 4 — Operator UX: Navigation & Observability ✅ (Complete 2026-05-26)

All four Phase 4 items implemented and wired in `internal/cli/root.go`.

### `aom task verify --watch`

- `executeTaskVerify` parses `--watch`, `--interval <dur>` (default 10s), `--timeout <dur>` (default 30m)
- Loop: run all verify checks → exit `allOK` → sleep interval → repeat until timeout
- Output prefix: `#N  HH:MM:SS` per iteration; `All checks passed after N poll(s) — run: aom task accept <id>` on success
- Implemented in `internal/cli/task_cmd.go`

### `aom status --action-items`

- New `--action-items`/`--actions` flag in `executeStatus`
- Calls `buildActionItems(result, sessions, taskViews)` — shared helper also used by `aom dashboard`
- Priority 1 (red):    WaitingApproval sessions → `[APPROVAL]` + `aom approve <id>`
- Priority 2 (yellow): `task.completed` in log but status ≠ Done → `[ACCEPT]` + `aom task accept <id>`
- Priority 2 (yellow): Ready tasks with no active session → `[SPAWN]` + `aom session spawn <agent>`
- Priority 3 (dim):    Blocked tasks → `[BLOCKED]`
- Implemented in `internal/cli/project_cmd.go`; `actionItem` struct + `buildActionItems` + `printActionItems` helpers

### `aom switch <agent-name>`

- Top-level command; takes agent name (not session ID)
- Scans sessions newest-first for a live tmux pane belonging to the named agent
- On success: attaches pane + logs `operator.intervention` event to the task log
- On failure: lists all known sessions for agent + spawn hint
- Implemented in `internal/cli/tmux_cmd.go`

### `aom dashboard [--interval <dur>]`

- New file: `internal/cli/dashboard_cmd.go` (181 lines)
- Default interval 5s; `--interval 10s` to override; Ctrl+C exits cleanly via `signal.NotifyContext`
- ANSI clear-screen (`\033[?25l\033[H\033[2J`) + cursor hide/restore per frame
- Three sections per frame: **Sessions** (agent | status | task | pane live/dead), **Action Items** (from `buildActionItems`), **Recent Channel** (last 6 non-empty/non-heading lines from `channel.md`)

### Tests

- `TestAppendSignalToWorkspaceLog` (3 sub-cases) in `internal/cli/task_verify_test.go`
- `TestTaskSignalValidation` (4 sub-cases) in `internal/cli/task_verify_test.go`

---

## Phase 5 — Guided Autonomy ✅ (Complete 2026-05-26)

All four Phase 5 items implemented. Operator can now hand off monitoring to the CLI and only
intervene when the system escalates.

### `aom task accept --auto`

- New flags on `aom task accept`: `--auto`, `--interval <dur>` (default 15s), `--timeout <dur>` (default 30m)
- When `--auto` is set: enters a polling loop; calls `runTaskVerifyChecks` each iteration
- Prints `#N  HH:MM:SS` per iteration with `[ok]`/`[FAIL]` for each check
- Breaks and proceeds to accept when all checks pass
- On timeout: prints agent-specific escalation hints (`aom capture <agent> --diff`, `aom session recover`) and returns error
- Implemented in `internal/cli/task_cmd.go` (indexed loop flag parse + auto-poll block before accept body)

### `aom session watch [--auto-spawn]`

- New session subcommand: `aom session watch [--auto-spawn] [--interval 15s] [--timeout 60m] [--real|--mock]`
- Guard: `--auto-spawn` requires `--mock` or `--real` (returns error if neither set)
- Loop: loads all sessions + task views → calls `buildActionItems` → filters `SPAWN` items
- Without `--auto-spawn`: prints pending SPAWN items each interval (informational watch)
- With `--auto-spawn`: calls `parseSpawnItemCommand` on each SPAWN command string → `executeSessionSpawn`; spawn errors are non-fatal (logged + continue)
- On timeout: returns `nil` (clean exit; no error — watch expiring is expected)
- Implemented in `internal/cli/session_cmd.go`; `parseSpawnItemCommand` helper guards on `parts[i-1] == "session"` to avoid false matches

### `aom run-pipeline <task-id>`

- New top-level command: `aom run-pipeline <task-id> [--agent <name>] [--timeout 60m] [--real|--mock] [--skip-merge]`
- Requires `--mock` or `--real`; resolves agent from `task.PreferredAgent` or `--agent` override
- Five stages with stage headers (`━━━ Stage N: Name ━━━`):
  - **Stage 1 — Spawn**: `executeSessionSpawn([agentName, "--task", taskID, launchFlag])`
  - **Stage 2 — Wait for task.completed**: polls `runTaskVerifyChecks` for `"task.completed in log"` check; `goto stage3` when found
  - **Stage 3 — Verify**: polls `runTaskVerifyChecks` until all checks pass; prints `[ok]/[FAIL]` each iteration
  - **Stage 4 — Accept**: `executeTaskAccept([taskID])`
  - **Stage 5 — Merge**: `executeMergeCommit([taskID])`; if merge fails after accept, prints resume hints and returns wrapped error; `--skip-merge` skips stage 5
- Implemented in `internal/cli/pipeline_cmd.go` (new file, 235 lines)
- `escalate(stage, diagHint)` helper prints remaining budget + resume commands on timeout

### Timeout + Escalation

Every polling command prints remaining budget and agent-specific resume hints on timeout:

| Command | Stage-specific hint |
|---------|---------------------|
| `aom task accept --auto` | `aom capture <agent> --diff` + `aom session recover <id>` |
| `aom run-pipeline` (wait stage) | `aom capture <agent> --diff` + `aom session recover <id>` |
| `aom run-pipeline` (verify stage) | `aom task verify <task-id>` + `aom capture <agent> --diff` |

### Tests

24 new test cases in `internal/cli/phase5_test.go`:

- `TestTaskAcceptAutoFlagValidation` — 7 sub-cases: missing task id, bad --interval, bad --timeout, --interval missing value, --timeout missing value, unknown flag, two positional args
- `TestSessionWatchFlagValidation` — 6 sub-cases: --auto-spawn without mode, bad --interval, bad --timeout, --interval missing value, unknown flag, --mock + --real conflict
- `TestRunPipelineFlagValidation` — 6 sub-cases: no args, no launch mode, bad --timeout, --agent missing value, unknown flag, --mock + --real conflict
- `TestParseSpawnItemCommand` — 4 sub-cases: with task, without task (reviewer), "session" guard prevents false match

All 24 tests pass. `go build ./...` clean.

---

## Immediate Next Step

Phases 1–5 complete. Milestones 0–17, all E2E feedback rounds, cross-platform polish,
Free-Roam Workspace (Tracks A–C), and Guided Autonomy are fully resolved. The system is at
Level 5 of the readiness criteria defined in `docs/AOM-MASTER-PLAN.md`.

### Completed — no further action needed

| Work | Status |
|------|--------|
| Milestones 0–17 | ✅ Complete |
| E2E feedback rounds 1–11 | ✅ Complete |
| Cross-platform (Windows/WSL2/macOS) polish | ✅ Complete |
| Per-Agent Workspace / Free-Roam (Track A A1–A8) | ✅ Complete — see Per-Agent Workspace section |
| Free-Roam Messaging (Track B B1–B5) | ✅ Complete |
| Phase 2 — Reliable Multi-Agent Handoff (F1–F8) | ✅ Complete — see Phase 2 section |
| Phase 4 — Operator UX (switch / verify --watch / --action-items / dashboard) | ✅ Complete — see Phase 4 section |
| Phase 5 — Guided Autonomy (accept --auto / session watch / run-pipeline) | ✅ Complete — see Phase 5 section |

### Phase 3.3 — Cross-Provider E2E 🔲 (Ready to start)

- `claude` backend + `codex` frontend + `claude` reviewer in the same AOM pipeline
- Both providers are confirmed working individually (claude ✅ codex ✅)
- No blockers — can be run in WSL2 with existing toolchain

### Deferred (unchanged)

1. **gemini/kiro runtime support** — fill in `LaunchCommand` in `internal/provider/gemini.go` and `internal/provider/kiro.go`; blocked on confirmed CLI flags

## Refactoring (branch: refactor)

Completed on 2026-05-16 on the `refactor` branch. See `docs/refactoring-plan.md` for full detail.

### Part A — Provider/Runtime Architecture
- New `internal/provider/` package with `Provider` interface + `Registry`
- `claude.go`, `codex.go` (full); `gemini.go`, `kiro.go` (stubs)
- `internal/cli/vendor_session.go` deleted — logic moved to `provider/claude.go`
- All 5 provider `switch` dispatch sites replaced with `registry.Lookup()` calls
- **Adding a new runtime = one new file in `internal/provider/` only**

### Part B — CLI `root.go` Split
- `internal/cli/root.go` reduced from **6,606 → 357 lines**
- 16 new focused files in `package cli` (no sub-packages, no visibility changes):
  `task_cmd.go`, `session_cmd.go`, `session_spawn_helpers.go`, `review_cmd.go`,
  `handoff_cmd.go`, `merge_cmd.go`, `project_cmd.go`, `worktree_cmd.go`,
  `step_cmd.go`, `observability_cmd.go`, `approval_cmd.go`, `tmux_cmd.go`,
  `helpers.go`, `message_cmd.go`, `channel_cmd.go`, `metrics_cmd.go`
- Verified by `golang-refactor-tester`: 51/51 smoke checks, `go test ./...` 100% green

### M9 — Project Governance (complete)

Implemented in one slice:
- `ResourcesForRole(roleName, runtime)` in `internal/config/config.go` — resolves role-bound skills and MCP servers filtered by runtime compatibility
- `OpenResult` now carries `Resources config.ResourcesFile` and `Policy config.PolicyFile` — available to all downstream CLI handlers
- `MaterializeSkillFiles` in `internal/artifact/service.go` — copies role-bound skill `.md` files into the worktree root at spawn time; missing source files are silently skipped
- `MaterializeMCPConfig` in `internal/artifact/service.go` — for `claude`: appends `## MCP Servers` section to `CLAUDE.md`; for `codex`: writes `.codex/mcp.json`; other runtimes are no-ops pending M10
- `materializeAgentContext` private helper in `internal/cli/root.go` — consolidates identity + skill + MCP materialization; called at both spawn sites (`executeResolvedSessionSpawn` and session resume rebind) so they cannot diverge
- `enforcePolicyDefaults` in `internal/cli/root.go` — warns operator when `yolo_mode=enabled`; full deny_commands interception deferred to M10 (requires runtime adapter layer)
- `aom project resources` command — shows role bindings, skills, MCP servers, and policy summary so operators can verify governance is configured before spawning agents

## System Diagrams

Visual reference for architecture, state machines, and key flows:
- `docs/system-diagrams.md`

Diagrams included:
1. System Architecture
2. Package Dependency Direction
3. Three-Layer Truth Model
4. Task State Machine
5. Session State Machine
6. Worktree State Machine
7. Step State Machine
8. Session Spawn Flow (sequence)
9. AI Orchestrator Loop (sequence)
10. Artifact Lifecycle
11. Operator Definition (Human vs AI)
12. Runtime Identity File Delivery
13. Multi-Session Agent Model

## Suggested First Checks On Another Machine

1. Clone the repo and open the root directory.
2. Read:
   - [AGENTS.md](../AGENTS.md)
   - [docs/project-structure.md](project-structure.md)
   - [docs/engineering-guidelines.md](engineering-guidelines.md)
   - this file
3. Run `go test ./...`
4. If tmux is available, manually test:
   - `aom project init`
   - `aom open`
   - `aom plan "smoke test" --create`
   - `aom session spawn backend-main --real`
   - `aom session list`
   - `aom capture <session-id>`
5. If tmux is not available, continue with non-live verification and keep runtime E2E deferred.
