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
- [AOM planning](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\AOM-planning.md)
- [Milestone plan](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\AOM-milestones.md)
- [State machine](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\state-machine.md)
- [Artifact schemas](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\artifact-schemas.md)
- [Project config](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\project-config.md)
- [CLI spec](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\cli-spec.md)
- [Project structure](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\project-structure.md)
- [Engineering guidelines](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\engineering-guidelines.md)

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
- [Milestone 1 plan](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\milestone-1-implementation-plan.md)

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
- [Milestone 2 plan](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\milestone-2-implementation-plan.md)

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
- `session spawn` otherwise uses a placeholder shell command, not a real provider CLI yet
- `attach` and `capture` operate through the tmux manager abstraction

## Current Packages

### Working packages

- [cmd/aom/main.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\cmd\aom\main.go)
- [internal/app/app.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\app\app.go)
- [internal/app/sessions.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\app\sessions.go)
- [internal/cli/root.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\cli\root.go)
- [internal/config/config.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\config\config.go)
- [internal/db/db.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\db\db.go)
- [internal/project/service.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\project\service.go)
- [internal/project/repository.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\project\repository.go)
- [internal/artifact/service.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\artifact\service.go)
- [internal/project/templates/project-init/agents.yaml.tmpl](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\project\templates\project-init\agents.yaml.tmpl)
- [templates/project-init/default/agents.yaml.tmpl](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\templates\project-init\default\agents.yaml.tmpl)
- [templates/project-init/minimal/agents.yaml.tmpl](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\templates\project-init\minimal\agents.yaml.tmpl)
- [internal/plan/service.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\plan\service.go)
- [internal/agent/repository.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\agent\repository.go)
- [internal/session/repository.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\repository.go)
- [internal/session/service.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\service.go)
- [internal/step/repository.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\step\repository.go)
- [internal/task/repository.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\task\repository.go)
- [internal/task/service.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\task\service.go)
- [internal/tmux/manager.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\tmux\manager.go)

### Tests

- [internal/config/config_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\config\config_test.go)
- [internal/db/db_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\db\db_test.go)
- [internal/project/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\project\repository_test.go)
- [internal/project/service_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\project\service_test.go)
- [internal/artifact/service_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\artifact\service_test.go)
- [internal/plan/service_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\plan\service_test.go)
- [internal/agent/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\agent\repository_test.go)
- [internal/session/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\repository_test.go)
- [internal/session/service_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\service_test.go)
- [internal/step/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\step\repository_test.go)
- [internal/task/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\task\repository_test.go)
- [internal/task/service_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\task\service_test.go)
- [internal/tmux/manager_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\tmux\manager_test.go)
- [internal/cli/root_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\cli\root_test.go)

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

Suggested verification commands on a new machine:

```powershell
$env:GOTOOLCHAIN='local'
$env:GOCACHE="$PWD\.cache\gocache"
$env:GOMODCACHE="$PWD\.cache\gomodcache"
$env:GOTELEMETRY='off'
$env:GOTELEMETRYDIR="$PWD\.cache\gotelemetry"
& 'C:\Program Files\Go\bin\go.exe' test ./...
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
- provider runtime launch is still placeholder-only and not yet provider-native E2E

Recommended path for live E2E:
- Linux or macOS should work best for continued live runtime validation
- Windows still needs a working WSL + tmux path if it is used again for live checks
- the next real-agent smoke work should be executed on a machine with verified `tmux`, `git`, and target runtime availability

## What Is Intentionally Not Done Yet

Still out of scope at the current handoff point:
- real provider runtime launch for Codex, Claude, or Kiro
- handoff and checkpoint logic
- provider-native resume and replacement flows

## Immediate Next Step

Next milestone to continue:
- `Milestone 4: Operational Memory Layer`

Recommended first implementation slice:
1. append richer canonical log events around session lifecycle
2. move artifact root from repo fallback to task worktree when Milestone 5 begins
3. start task-to-worktree mapping so task-bound sessions launch in isolated paths

Current next recommended slice:
1. decide whether repair and replacement outcomes should append richer canonical detail such as classified drift kind or supersession policy reason in `log.md`
2. decide whether additional replacement end states beyond `Detached -> Archived` are worth automating, or whether other cases should remain explicitly operator-driven
3. decide whether dirty unregistered worktree paths need a dedicated inspect/report command instead of relying on `status`, `task show`, and repair hints alone

Planned validation-oriented slice after that:
1. use [docs/experimental-agent-e2e-plan.md](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\experimental-agent-e2e-plan.md) to add an opt-in experimental real-agent launch path for one runtime
2. verify that slice on macOS or Linux before treating it as a stable workflow
3. use findings from that smoke path to inform Milestone 5 and later Milestone 10 work

## Suggested First Checks On Another Machine

1. Clone the repo and open the root directory.
2. Read:
   - [AGENTS.md](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\AGENTS.md)
   - [docs/project-structure.md](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\project-structure.md)
   - [docs/engineering-guidelines.md](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\docs\engineering-guidelines.md)
   - this file
3. Run `go test ./...`
4. If tmux is available, manually test:
   - `aom project init`
   - `aom open`
   - `aom session spawn backend-main`
   - `aom session list`
   - `aom capture <session-id>`
5. If tmux is not available, continue with Milestone 3 and keep tmux E2E deferred.
