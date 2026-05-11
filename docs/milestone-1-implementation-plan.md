# Milestone 1 Implementation Plan

## Purpose

This document turns `Milestone 1: Local Project Control Skeleton` into an implementation-ready plan.

The goal of this milestone is to create the smallest useful AOM control plane that can:

- initialize a project
- load project config
- create or open the SQLite system-of-record
- register project and agent definitions
- show basic project state through CLI

This milestone should not attempt live terminal orchestration yet. It exists to build the durable local foundation that later tmux, session, worktree, and artifact flows depend on.

## Milestone Goal

Deliver a working Go CLI that supports:

- `aom project init`
- `aom open`
- `aom status`

and persists project state in SQLite using the configuration model defined in Milestone 0.

## Scope

### In scope

- Go module bootstrap
- repository folder structure for the CLI app
- config file generation and loading
- config validation
- SQLite bootstrap
- initial schema and migrations
- project registration in DB
- agent registration in DB from `agents.yaml`
- status output for project and agents

### Out of scope

- tmux session management
- runtime spawning
- worktree creation
- task creation
- step model persistence
- markdown artifact generation
- approval enforcement
- MCP runtime wiring
- provider adapters

This milestone is intentionally narrow.

## Working Assumptions

1. The implementation language is Go.
2. SQLite is the only database for MVP.
3. The CLI is local-only.
4. Project config lives under `.aom/`.
5. `aom open` should validate and register system state, but not yet launch terminal panes.

## Deliverables

By the end of Milestone 1, the repository should contain:

- Go module and entrypoint
- initial package structure
- config structs and loader
- SQLite connection bootstrap
- schema migration mechanism
- initial repositories/models for:
  - projects
  - agents
  - sessions
  - tasks
- CLI command implementations for:
  - `aom project init`
  - `aom open`
  - `aom status`

## Recommended Code Layout

The first implementation slice should use this structure:

```text
cmd/
  aom/
    main.go

internal/
  app/
  cli/
  config/
  db/
  project/
  agent/
  session/
  task/
```

### Package responsibilities

- `internal/app`
  - top-level app wiring and shared dependencies
- `internal/cli`
  - command construction and CLI handlers
- `internal/config`
  - YAML structs, loading, validation
- `internal/db`
  - SQLite bootstrap, migrations, shared DB helpers
- `internal/project`
  - project initialization, project lookup, project registration
- `internal/agent`
  - role/agent registration and lookup
- `internal/session`
  - session records only for now, no runtime management yet
- `internal/task`
  - task model placeholders and basic repository shape

This is intentionally minimal. Do not introduce runtime adapters or tmux packages yet.

## CLI Framework Recommendation

Use a simple Go CLI library with subcommand support.

Recommended:

- `cobra`

Why:

- easy command grouping
- stable flags/subcommands model
- fits the command surface already specified

Non-goal:

- do not overbuild CLI middleware yet

## Config Implementation Plan

### Files to support in this milestone

- `.aom/project.yaml`
- `.aom/agents.yaml`
- `.aom/resources.yaml`
- `.aom/policy.yaml`

### Required config operations

1. Generate baseline config during `aom project init`
2. Load config during `aom open`
3. Validate config during `aom open` and `aom status`

### Validation required now

#### project.yaml

- repo path exists
- default branch is present
- terminal runtime is `tmux`
- state dir is present

#### agents.yaml

- every agent references a known role
- runtime is in the supported planned runtime set
- role class is valid

#### resources.yaml

- role bindings point to existing skills and MCP servers
- skill paths are repo-local or project-controlled

#### policy.yaml

- approval scope is `per-session`
- yolo mode is `enabled` or `disabled`

## SQLite Schema v1

Milestone 1 should create the first durable system-of-record with these tables:

### projects

Fields:

- `id`
- `name`
- `repo_path`
- `default_branch`
- `created_at`

### agents

Fields:

- `id`
- `project_id`
- `name`
- `runtime`
- `role`
- `enabled`
- `created_at`

### tasks

Fields:

- `id`
- `project_id`
- `title`
- `mode`
- `status`
- `created_at`

This table may remain mostly unused in Milestone 1, but should exist now because `status` output and later milestone continuity depend on it.

### sessions

Fields:

- `id`
- `project_id`
- `agent_id`
- `task_id`
- `runtime`
- `status`
- `worktree_path`
- `tmux_session_name`
- `tmux_window`
- `tmux_pane`
- `vendor_session_id`
- `last_seen_at`
- `created_at`

For Milestone 1, many of these fields may remain empty, but the schema should already reflect the continuity model.

### migrations

Fields:

- `id`
- `applied_at`

Used to track schema bootstrap versioning.

## Data Model Rules for Milestone 1

1. Project identity must be persisted in DB after `aom project init` or `aom open`.
2. Agents defined in config must be reflected in DB.
3. Re-running `aom open` should not duplicate project or agent records.
4. DB bootstrap must be idempotent.

## Command Plan

## 1. aom project init

### Behavior for Milestone 1

- verify target repo path
- create `.aom/`
- generate baseline config files
- create SQLite DB
- apply migrations
- insert or upsert project record
- optionally seed empty/default agents file content

### Verification

- `.aom/` exists
- config files exist
- DB file exists
- `aom status` can read the new project

## 2. aom open

### Behavior for Milestone 1

- resolve project from current directory or provided name
- load config files
- validate config
- open DB
- apply migrations if needed
- upsert project record
- sync agents from config into DB
- print summary of current project state

### Verification

- command succeeds repeatedly
- project is visible in DB-backed status
- agent definitions appear in status output

### Important limitation

In Milestone 1, `aom open` does not yet create tmux sessions or session panes. It only prepares durable state.

## 3. aom status

### Behavior for Milestone 1

- load current project
- read project and agent rows from DB
- show project summary
- show enabled agents
- show basic counts for tasks and sessions

### Verification

- after `project init`, `status` shows project metadata
- after `open`, `status` shows synced agents from config
- output remains human-readable and operator-scannable

## Baseline Config Generation

`aom project init` should generate conservative starter files.

### project.yaml

- use provided project name and repo path
- set `default_branch` to `main` unless explicitly overridden
- set terminal runtime to `tmux`
- set state dir to `.agent`

### agents.yaml

Create a small starter team, not a full kitchen sink.

Recommended baseline:

- `orchestrator-main` using `claude`
- `backend-main` using `codex`
- `reviewer-main` using `claude`

This is enough to prove config loading and role registration without overcommitting product defaults.

### resources.yaml

Start empty but valid:

- no skills required yet
- no MCP servers required yet
- empty role bindings allowed

### policy.yaml

Generate the conservative baseline from the config spec:

- default deny commands
- default approval-required actions
- per-session approval scope
- YOLO disabled

## Status Output Shape

Milestone 1 `aom status` should stay simple and human-readable.

Recommended sections:

### Project

- name
- repo path
- default branch
- database path

### Agents

- name
- role
- runtime
- enabled

### Counts

- tasks
- sessions

### Warnings

- invalid config
- missing repo
- missing runtime binary checks may be deferred to `doctor`

## Non-Goals and Guardrails

### Non-goals

- do not add task planning logic yet
- do not generate `.agent/*` artifacts yet
- do not create worktrees yet
- do not attempt tmux orchestration yet
- do not implement runtime adapters yet

### Guardrails

1. Keep package boundaries small and explicit.
2. Avoid speculative abstractions.
3. Prefer plain structs and repositories over generic frameworks.
4. Do not hide side effects in `open`.
5. Make DB and config operations idempotent.

## Suggested Implementation Order

### Slice 1

- Go module
- `cmd/aom/main.go`
- Cobra root command

### Slice 2

- config structs
- YAML loader
- validation helpers

### Slice 3

- SQLite bootstrap
- migrations table
- schema v1

### Slice 4

- project repository
- agent repository
- project init command

### Slice 5

- open command
- config-to-DB sync
- status command

## Verification Plan

Milestone 1 should be verified with focused manual checks:

1. Run `aom project init <name> --repo <path>`
   - verify `.aom/` files exist
   - verify DB exists
2. Run `aom open`
   - verify config loads
   - verify DB opens
   - verify agents are synced
3. Run `aom status`
   - verify project and agent summary appears
4. Re-run `aom open`
   - verify no duplicate project/agent records

If easy enough, add narrow unit tests for:

- config loading
- config validation
- migration bootstrap
- project upsert
- agent sync

## Acceptance Criteria

Milestone 1 is complete when:

- a project can be initialized locally
- baseline config files are created correctly
- SQLite opens and applies schema idempotently
- project state is persisted
- agent definitions are loaded from config and registered
- `aom open` succeeds repeatedly without duplicate state
- `aom status` shows useful project and agent state

## Risks Addressed

This milestone reduces the following risks:

- no durable system-of-record
- config drift before workflow logic exists
- unclear project identity
- weak base for later session/worktree continuity
