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

Completed in code and tests.

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

## Current CLI Surface

Implemented commands:
- `aom project init`
- `aom open`
- `aom status`
- `aom session spawn`
- `aom session list`
- `aom session show`
- `aom attach`
- `aom capture`

Current behavior notes:
- `open` ensures tmux workspace and fails clearly when tmux is unavailable
- `status` shows project, terminal summary, agents, sessions, and counts
- `session spawn` uses a placeholder shell command, not a real provider CLI yet
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
- [internal/agent/repository.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\agent\repository.go)
- [internal/session/repository.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\repository.go)
- [internal/session/service.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\service.go)
- [internal/tmux/manager.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\tmux\manager.go)

### Tests

- [internal/config/config_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\config\config_test.go)
- [internal/db/db_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\db\db_test.go)
- [internal/project/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\project\repository_test.go)
- [internal/project/service_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\project\service_test.go)
- [internal/agent/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\agent\repository_test.go)
- [internal/session/repository_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\repository_test.go)
- [internal/session/service_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\session\service_test.go)
- [internal/tmux/manager_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\tmux\manager_test.go)
- [internal/cli/root_test.go](C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management\internal\cli\root_test.go)

## Verified State

Last verified state before this handoff:
- `go test ./...` passes

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

### tmux and Windows

Important current limitation:
- live tmux E2E has not been run successfully in this Windows environment
- `tmux` was not available in the Windows shell path
- `wsl.exe` returned an access-denied error in the current execution context

What this means:
- code and tests for tmux logic pass
- live tmux behavior still needs E2E validation on a machine or shell context with working tmux

Recommended path for live E2E:
- Linux or macOS
- or Windows with working WSL access and tmux installed inside WSL

## What Is Intentionally Not Done Yet

Still out of scope at the current handoff point:
- real provider runtime launch for Codex, Claude, or Kiro
- task and step workflow engine
- markdown artifact write flows
- worktree-aware session spawn
- handoff and checkpoint logic
- provider-native resume and replacement flows

## Immediate Next Step

Next milestone to start:
- `Milestone 3: Task + Step Workflow Core`

Recommended first implementation slice:
1. task repository and schema
2. step repository and schema
3. `aom task create`
4. `aom step list`
5. basic task and step status transitions

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
