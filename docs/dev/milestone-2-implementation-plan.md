# Milestone 2 Implementation Plan

## Purpose

Milestone 2 exists to prove the `Terminal Team MVP` in the simplest defensible form.

At the end of this milestone, AOM should be able to:
- prepare a tmux-backed live terminal workspace for one project
- create and track session records for visible specialist sessions
- spawn at least a few live panes that the operator can inspect directly
- attach to a chosen pane
- capture visible pane output

This milestone is about proving the operator experience, not full workflow orchestration.

## Goal

Prove that one operator can manage multiple native terminal sessions in one AOM project without losing the native terminal feel.

## What This Milestone Should Deliver

By the end of Milestone 2:
- `aom open` can create or refresh a tmux control surface
- `aom session spawn` can create a live tmux pane and register a session
- `aom attach` can attach the operator to a chosen pane
- `aom capture` can capture visible pane output
- `aom status` can show live session summaries from the DB plus tmux bindings

## What This Milestone Should Not Do

Do not overreach into later milestones.

Out of scope for Milestone 2:
- task and step workflow orchestration
- markdown artifact generation
- worktree orchestration
- provider-specific runtime adapters for Codex, Claude, or Kiro
- automated handoff
- approvals flow
- recovery automation beyond simple visibility and basic recommendations

For this milestone, a shell-based placeholder runtime is enough to prove the terminal model.

## Key Design Decisions

### 1. Keep tmux isolated

All direct tmux behavior should live under `internal/tmux`.

Other packages should not build raw tmux command strings themselves.

### 2. Keep system session identity above tmux

tmux is the live terminal surface, not the canonical session source of truth.

AOM should:
- create DB-backed session records
- store tmux binding metadata on those records
- treat tmux loss as a session continuity concern, not as the whole identity

### 3. Use a placeholder runtime first

The first live-session implementation should not try to launch real provider CLIs yet.

Instead, use a shell command placeholder that makes the pane visibly alive and identifiable, for example:
- a shell prompt opened in the project repo
- an echo banner that prints agent name, role, runtime, and session id

This keeps Milestone 2 focused on terminal orchestration itself.

### 4. Build testing in from the first slice

Every slice should add the smallest useful verification possible:
- package tests for state and parsing
- command-level tests where possible
- one focused E2E check once live tmux behavior exists

## Planned Scope

### In scope

- tmux availability detection
- tmux session naming and binding rules
- DB session records for live terminal sessions
- session spawn/list/show
- tmux layout creation for operator control
- attach and capture flows
- status output that includes live session bindings

### Out of scope

- provider-native resume
- worktree-aware spawning
- task-linked session continuity packets
- orchestrator summary pane with rich live workflow intelligence

The summary surface in this milestone can remain lightweight.

## Proposed Package Additions

### `internal/tmux`

Owns:
- tmux availability checks
- tmux session creation and discovery
- pane creation
- attach helpers
- pane capture helpers

### `internal/session`

Owns:
- session repository
- session service
- tmux binding metadata on session records
- live session summaries

### `internal/cli`

Will grow command handlers for:
- `session spawn`
- `session list`
- `session show`
- `attach`
- `capture`

### `internal/app`

Will wire:
- project service
- session service
- tmux manager

## Data Model Changes

Milestone 1 already created the `sessions` table, but it is still too thin for live terminal control.

Milestone 2 should extend it with at least:
- `project_id`
- `agent_name`
- `role_name`
- `runtime`
- `status`
- `repo_path`
- `tmux_session_name`
- `tmux_window`
- `tmux_pane`
- `last_seen_at`
- `created_at`
- `updated_at`

If a migration is needed, add a new migration instead of rewriting the existing schema in place.

## Command Scope for Milestone 2

### `aom open`

Milestone 2 behavior:
- load project
- verify tmux availability
- create or reuse a tmux workspace for the project
- print control-surface summary

Do not yet force complex pane creation for every known agent automatically unless that behavior is explicitly defined in the slice.

### `aom session spawn`

Milestone 2 behavior:
- resolve agent from config
- create session record
- create or reuse project tmux session
- create pane
- launch placeholder shell command in pane
- persist tmux binding metadata
- optionally print attach target

### `aom session list`

Milestone 2 behavior:
- list DB-backed sessions
- show tmux binding fields
- show status and last seen

### `aom session show`

Milestone 2 behavior:
- show one session record with tmux binding detail

### `aom attach`

Milestone 2 behavior:
- attach to target pane when binding exists
- fail clearly when the pane is missing

### `aom capture`

Milestone 2 behavior:
- capture visible pane output from tmux
- print it to stdout

## Implementation Slices

## Slice 1: tmux manager skeleton

### Goal

Prove that AOM can inspect tmux availability and compute stable project tmux names.

### Scope

- add `internal/tmux`
- implement:
  - binary detection
  - project session naming
  - helper for safe tmux target formatting

### Deliverables

- tmux manager struct
- availability method
- naming helpers
- focused tests

### Verification

- unit tests for naming
- command-level check that missing tmux reports a clear error

## Slice 2: session repository and schema v2

### Goal

Make live session state durable in the DB before any real pane orchestration happens.

### Scope

- extend `sessions` schema
- implement `internal/session/repository.go`
- support create/list/get/update operations

### Deliverables

- migration for session schema
- repository tests

### Verification

- `go test ./internal/session ./internal/db`
- migration upgrade test from a fresh Milestone 1 DB

## Slice 3: tmux project workspace creation

### Goal

Allow `aom open` to create or reuse a tmux workspace for the project.

### Scope

- create tmux session if missing
- detect if session already exists
- record or display the chosen tmux session name

### Deliverables

- tmux create/find methods
- `aom open` integration

### Verification

- package tests for command construction where possible
- focused E2E check:
  - run `aom open`
  - confirm tmux session exists

## Slice 4: session spawn with placeholder runtime

### Goal

Spawn a visible live pane for a configured agent and persist the binding.

### Scope

- `aom session spawn`
- create pane in project tmux session
- run placeholder shell command in the repo directory
- persist session record

### Deliverables

- session service
- CLI spawn command
- session listing output

### Verification

- repo tests for session service
- focused E2E check:
  - spawn three sessions
  - confirm three panes exist
  - confirm `session list` shows them

## Slice 5: attach and capture

### Goal

Make the live panes inspectable in the way the operator actually needs.

### Scope

- `aom attach`
- `aom capture`
- `aom session show`

### Deliverables

- attach handler
- pane capture helper
- session detail output

### Verification

- package tests where possible
- E2E:
  - capture a pane after spawn
  - verify output contains session banner

## Slice 6: status integration and lightweight control summary

### Goal

Surface live session state through `aom status` and `aom open`.

### Scope

- show session count
- show active session rows
- show tmux bindings
- show stale/missing pane hints when detectable

### Deliverables

- status output update
- open output update

### Verification

- CLI tests
- E2E:
  - open project
  - spawn sessions
  - run status
  - confirm live session summary is shown

## Recommended Order

1. Slice 1: tmux manager skeleton
2. Slice 2: session repository and schema v2
3. Slice 3: tmux project workspace creation
4. Slice 4: session spawn with placeholder runtime
5. Slice 5: attach and capture
6. Slice 6: status integration and lightweight control summary

This order keeps the durable session model ahead of the live pane UX, which is safer.

## Testing Strategy

### Unit tests

Use package tests for:
- tmux naming rules
- session repository
- migration behavior
- CLI argument handling

### Integration tests

Add command-level tests for:
- `session spawn`
- `session list`
- `session show`

Where tmux is not available in the test environment, keep tests focused on explicit error behavior.

### E2E checks

Milestone 2 should have a new E2E checklist, similar to the Milestone 1 checklists, that covers:
- project open
- tmux workspace creation
- spawn three sessions
- capture one pane
- list visible sessions

## Acceptance Criteria

Milestone 2 is complete when:
- `aom open` can create or reuse a project tmux session
- `aom session spawn` can create at least three visible live panes
- `aom session list` shows durable session records for those panes
- `aom attach` can target a live pane
- `aom capture` can return visible pane output
- the operator can inspect live sessions without leaving the native terminal model

## Risks to Watch

### 1. Letting tmux leak everywhere

Do not let CLI or project packages embed ad hoc tmux command logic.

### 2. Trying to launch real provider CLIs too early

Milestone 2 should prove terminal orchestration first.

### 3. Overbuilding the summary pane

A lightweight summary output is enough for this milestone.

### 4. Treating tmux as the whole session identity

The DB-backed session record must stay primary.

## Recommended Next Step After This Plan

Start with Slice 1 and Slice 2 together only if the code stays simple.

If the tmux manager and session repository start to tangle, do them separately.
