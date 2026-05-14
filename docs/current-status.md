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
- `session spawn --real` and `session replace --real` now launch `codex` for supported runtime roles and reject unsupported runtimes before pane creation
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
- `aom worktree repair`
- `aom step list`
- `aom step update`
- `aom session spawn`
- `aom session list`
- `aom session show`
- `aom session replace`
- `aom session stop`
- `aom session archive`
- `aom attach`
- `aom capture`
- `aom checkpoint`
- `aom handoff`
- `aom review`

Current behavior notes:
- `open` ensures tmux workspace and fails clearly when tmux is unavailable
- `plan` gives a lightweight orchestrator recommendation by default, and `plan --create` persists it into a task with seeded steps
- `project init` renders baseline config from template assets instead of hardcoded agent structs
- `project init --template` lets a project pick a preset starter team from `templates/project-init/<name>`
- `project init --template-dir` lets a project supply its own starter config templates
- `status` shows project, terminal summary, agents, sessions, detailed task rows, step summaries, and task-level recommended next action hints
- `task create` defaults to `Direct` mode and creates one initial `Proposed` implementation step
- `task create` and `plan --create` now seed task-local continuity artifacts under `.aom/tasks/<task-id>/`
- `task update` and `step update` validate allowed state transitions, including `NeedsAttention`
- `session spawn --task` binds `task_id` into the session record and refreshes `state.md`, `index.md`, and `log.md`
- task-bound `session spawn` records both boot and ready lifecycle transitions in canonical task log events
- task-bound `session spawn` failure after durable record creation is logged canonically and leaves the session record in `Failed`
- task-bound `session spawn` writes `session.created` before launch, then `session.ready` or `session.failed` based on the observed result
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
- `worktree repair <task-id>` now recovers missing or pruned git-backed task worktrees, restores `.agent/` artifacts into the repaired path, and appends a canonical `worktree.repaired` event
- `worktree repair <task-id>` now also recreates an unregistered worktree path automatically when the existing path is safe to replace because it is empty or contains only `.agent/`
- unregistered worktree paths with non-artifact content now remain operator-repair cases; AOM surfaces a manual cleanup hint instead of deleting the path automatically
- `open`, `status`, `session list`, `session show`, and task views now reconcile tmux pane liveness and persist `Detached` when the pane binding is gone
- `session stop` now intentionally terminates a live tmux pane when present, marks the durable record `Stopped`, keeps the worktree intact, and records tmux cleanup warnings in canonical task log events when pane teardown fails
- `session archive` now transitions eligible inactive sessions to `Archived` while preserving audit history
- `session replace` now spawns a replacement session in the same task/worktree context, preserves continuity through task artifacts, records a canonical `session.replaced` event, and prints explicit operator action hints when the old session is intentionally left running
- `session replace` now auto-archives superseded sessions that have already reconciled to `Detached`, while still stopping replaceable idle sessions and leaving active `Working` sessions for explicit operator intervention
- `session spawn --mock` launches a mock runtime transcript for live local flow verification
- `session spawn --real` launches the `codex` CLI for supported runtime roles and fails before pane creation when the role runtime is unsupported or `codex` is unavailable
- `session replace --real` uses the same runtime validation and launch path as `session spawn --real`
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
- narrow real-runtime launch is verified for `codex` via `session spawn --real` on macOS
- broader multi-runtime provider-native E2E is still not complete beyond the current `codex` slice

Recommended path for live E2E:
- Linux or macOS should work best for continued live runtime validation
- Windows still needs a working WSL + tmux path if it is used again for live checks
- the next real-agent smoke work should be executed on a machine with verified `tmux`, `git`, and target runtime availability

## What Is Intentionally Not Done Yet

Still out of scope at the current handoff point:
- first-class real-runtime launch for runtimes beyond the current `codex` slice
- provider-native resume flows
- richer review and unresolved review-item handling under Milestone 6

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

- `aom session send <session-id> <message>` — inject prompt into pane via tmux
  send-keys (new CLI command + new tmux.Manager.SendKeys method)
- `aom session resume <session-id> --task <task-id>` — bind new task to existing
  WaitingHandoff session and deliver initial brief (new CLI command)
- `claude` runtime support in `internal/runtime/launch.go` under `--real` mode
  (mirrors the codex implementation)
- `handoff.md` template seeding in `internal/artifact/service.go` when a task-bound
  session is spawned (schema defined in docs/artifact-schemas.md, not yet implemented)
- Completion event convention: agents append `task.completed` or `handoff.prepared`
  to log.md when work is done; see docs/artifact-schemas.md Agent Completion Protocol
- Runtime identity file materialization: on `session spawn`, AOM should read
  `.aom/agents/<name>/profile.md` and write it into the task worktree as the
  appropriate runtime config file (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`) so each
  agent session has its own role identity; without this, two sessions of the same
  runtime are indistinguishable to the agent; see docs/project-config.md Runtime
  Identity Files section for the storage model and delivery design

### Gaps — Tier 2 (ergonomic orchestration)

- `aom session wait <session-id> --until <event-type> [--timeout 30m]` — poll
  log.md until a target event appears or timeout is reached
- Initial context delivery: task-bound session spawn should note the .agent/task.md
  path in the launch message so the agent knows its brief on startup

### Gaps — Tier 3 (robust orchestration)

- Continuity readiness scoring populated in index.md (field exists in schema,
  logic not yet in artifact/service.go)
- `aom task reanalyze <task-id>` — refresh index.md and recommend next action
  after manual intervention (planned in Milestone 7)
- Orchestrator actor type in log events: distinguish `"actor": "orchestrator-ai"`
  from `"actor": "operator"` for audit clarity
- Provider-native resume for pane recovery: try `claude --continue` or codex
  equivalent before falling back to artifact-backed session replacement (Milestone 10)

### Recommended implementation order

1. `claude` runtime in `--real` (small, mirrors codex)
2. `aom session send` command
3. `handoff.md` seeding + completion event convention
4. Verify minimal E2E loop: spawn claude → send brief → wait for handoff.prepared
   → read handoff.md → decide next step
5. `aom session resume` command
6. `aom session wait` command

## Immediate Next Step

Next milestone to continue:
- `Milestone 6: Handoff and Checkpoint Flow`

Recommended first implementation slice:
1. decide whether reused reviewer sessions should eventually receive an explicit prompt delivery flow once `session send` exists
2. decide whether mixed-owner review findings should also be reflected in a dedicated event type or artifact field instead of only output wording
3. decide whether follow-up step owner hint updates from review findings should ever create a new fix step instead of reusing the latest non-review step

Current next recommended slice:
1. decide whether checkpoint output should capture richer changed-file summaries from git state inside the task worktree
2. decide whether `handoff` should update task ownership directly or remain session-scoped until the receiving session starts
3. decide whether repair and replacement outcomes need richer canonical detail such as drift-kind or supersession policy reason in `log.md`

Planned validation-oriented slice after that:
1. extend the real-runtime validation path beyond `codex` only if a concrete next runtime is agreed
2. verify follow-up runtime slices on macOS or Linux before treating them as stable workflows
3. use findings from those smoke paths to inform later Milestone 6 and Milestone 10 work

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
