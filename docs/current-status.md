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
- `aom task create`
- `aom task update`
- `aom task close`
- `aom task show`
- `aom task link` / `aom task unlink` (M13)
- `aom task request` / `aom task list-requests` / `aom task approve-request` / `aom task reject-request` (M14)
- `aom task record-result` (M16)
- `aom next` (M13)
- `aom team brief` (M14)
- `aom merge check` / `aom merge prepare` (M15)
- `aom merge commit` (Post-M17)
- `aom message send` / `aom message read` / `aom message clear` (M16)
- `aom pause-all` / `aom resume-all` (M16)
- `aom task list` (Post-M17)
- `aom task claim` (Post-M17)
- `aom metrics` (M17)
- `aom worktree repair`
- `aom worktree read-file` (M17)
- `aom step list`
- `aom step update`
- `aom session send`
- `aom session spawn`
- `aom session list`
- `aom session show`
- `aom session replace`
- `aom session stop`
- `aom session archive`
- `aom session health` (M16)
- `aom attach`
- `aom capture`
- `aom checkpoint`
- `aom handoff`
- `aom review`
- `aom review close`
- `aom session rebind`
- `aom project resources`
- `aom doctor`
- `aom runtime list`
- `aom runtime inspect`

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

Last verified state before this handoff:
- `go test ./...` passes
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
  go test ./...
```

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

## Immediate Next Step

Milestones 0–17 are complete. Remaining work before a production-ready release:

1. **gemini/kiro runtime support** — fill in `LaunchCommand` in `internal/provider/gemini.go` and `internal/provider/kiro.go`; blocked on confirmed CLI flags; no other files need to change (provider registry handles the rest)
2. **Runtime-level policy enforcement** — ✓ DONE: claude gets `--disallowed-tools` at the process level; `enforcePolicyDefaults` now prints per-runtime enforcement level at spawn time (runtime-enforced for claude, instruction-only for codex/others); no further work unless a codex enforcement flag becomes available
3. **Live E2E for M13–M17** — ✓ DONE: smoke test expanded to 51 checks across 12 sections; all M13–M17 commands now covered (task unlink, task claim, task reject-request, merge prepare, session health, pause-all, resume-all)

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
