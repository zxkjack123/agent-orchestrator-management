# AOM Milestone Plan

## Purpose

This document breaks the AOM planning document into implementation milestones that can be reviewed, accepted, and implemented incrementally.

The goal is to let us work in small, inspectable slices instead of trying to build the whole system at once.

## Planning Strategy

AOM should be delivered in layers:

1. Lock system language and data contracts
2. Prove the terminal team control loop
3. Add durable task memory and worktree continuity
4. Add workflow gating and review/handoff behavior
5. Add project governance for roles, skills, and MCP
6. Refine runtime adapters and operator ergonomics

Each milestone below is intended to be:

- independently reviewable
- useful on its own
- small enough to validate before moving on

## Milestone 0: Foundation Specs

### Goal

Turn the current planning document into implementation-grade specs so the first code milestone is constrained and consistent.

### Scope

- define task states
- define step states
- define session states
- define approval states
- define handoff states
- define markdown artifact schemas
- define project config file layout
- define CLI command surface for MVP

### Deliverables

- `docs/state-machine.md`
- `docs/artifact-schemas.md`
- `docs/project-config.md`
- `docs/cli-spec.md`

### Key decisions to lock

- exact lifecycle transitions for task, step, and session
- what fields are mandatory in `index.md`, `log.md`, `state.md`, and `handoff.md`
- how task mode upgrades are recorded
- what counts as `Needs attention`

### Acceptance criteria

- we can describe one complete task lifecycle without ambiguity
- we can describe one complete session lifecycle without ambiguity
- every core markdown artifact has a defined schema
- MVP CLI commands are named and scoped clearly enough to implement

### Risks addressed

- building the wrong workflow engine
- inventing incompatible file formats later
- confusion between orchestration state and agent state

## Milestone 1: Local Project Control Skeleton

### Goal

Create the minimal local AOM control plane that can open a project, load config, and track system state.

### Scope

- initialize project structure
- load and validate config
- create or open SQLite database
- register project and agent definitions
- expose basic `status` and `project open` behavior

### Deliverables

- project bootstrap command
- config loader
- SQLite bootstrap and migrations
- core data models for project, agent, task, session
- basic status output

### Suggested commands

- `aom project init`
- `aom open`
- `aom status`

### Acceptance criteria

- a project can be initialized locally
- config files can be loaded and validated
- DB opens cleanly and persists state
- project and agent records are visible through `status`

### Risks addressed

- no stable system-of-record
- repo layout drifting before workflow logic exists

## Milestone 2: Terminal Team MVP

### Goal

Prove the core user experience: one operator can see and manage multiple native agent terminals in one project.

### Scope

- create tmux session
- create tiled panes for configured agents
- create orchestrator summary/control pane
- spawn agent runtimes in panes
- attach to a chosen pane
- capture pane output

### Deliverables

- tmux session manager
- pane layout logic
- agent spawn logic
- attach and capture commands
- summary pane rendering strategy

### Suggested commands

- `aom open`
- `aom session spawn`
- `aom session attach`
- `aom capture`

### Acceptance criteria

- at least three active agents can be shown in one project
- the operator can inspect each active session
- the operator can attach to any session directly
- the system can capture visible session output

### Risks addressed

- losing the native terminal feeling
- overbuilding a dashboard before proving the real workflow

## Milestone 3: Task + Step Workflow Core

### Goal

Introduce the workflow model that the orchestrator manages: tasks, steps, planning modes, and explicit operator confirmation.

### Scope

- create tasks
- assign task mode
- create step lists
- represent simple dependencies
- mark step status
- mark task as `Needs attention`
- allow operator-driven task closure

### Deliverables

- task creation flow
- step model
- task mode model
- simple orchestrator recommendation model
- task and step status views

### Suggested commands

- `aom task create`
- `aom task show`
- `aom step list`
- `aom step update`

### Acceptance criteria

- a task can be created in `Direct` mode
- a task can be upgraded to another mode
- steps can represent sequential or simple parallel work
- a task can be flagged `Needs attention`
- the operator closes the task explicitly

### Risks addressed

- treating every task the same
- missing the operator confirmation model
- unclear orchestration state

## Milestone 4: Operational Memory Layer

### Goal

Make durable markdown artifacts the center of continuity so work survives resume, respawn, and intervention.

### Scope

- generate `.agent/task.md`
- generate `.agent/state.md`
- generate `.agent/index.md`
- generate `.agent/log.md`
- support mode-dependent artifacts
- append canonical events to `log.md`
- refresh `index.md` from current system state

### Deliverables

- artifact generator
- artifact refresh logic
- log append policy
- manifest/index refresh policy

### Artifacts in scope

- `task.md`
- `state.md`
- `index.md`
- `log.md`
- `handoff.md`
- `review-notes.md`
- `requirements.md`
- `design.md`
- `tasks.md`

### Acceptance criteria

- every active task has core markdown artifacts
- `index.md` reflects current task truth
- `log.md` records canonical AOM events
- a fresh session can start from artifact context alone

### Risks addressed

- overreliance on vendor session memory
- broken continuity after respawn or session loss

## Milestone 5: Git Worktree Continuity

### Goal

Bind task execution to isolated worktrees so specialist sessions have safe boundaries and resumable context.

### Scope

- create one worktree per task
- map task to branch and worktree path
- attach sessions to worktrees
- enforce one-writer-per-worktree in MVP
- keep task artifacts inside the worktree

### Deliverables

- worktree creation and lookup
- branch naming strategy
- task-to-worktree mapping
- worktree-aware session spawning

### Acceptance criteria

- a task can create and reuse a dedicated worktree
- a specialist session launches in the right worktree
- a second writer session is prevented or flagged
- artifacts remain colocated with the worktree task context

### Risks addressed

- context bleeding across tasks
- multiple sessions writing the same scope accidentally

## Milestone 6: Handoff and Checkpoint Flow

### Goal

Support specialist-to-specialist workflow without manual context copy-paste.

### Scope

- checkpoint command
- handoff artifact generation
- structured review notes
- owner change tracking
- unresolved review item tracking

### Deliverables

- `checkpoint` flow
- `handoff.md` generation
- `review-notes.md` template and parser
- owner transition recording

### Suggested commands

- `aom checkpoint`
- `aom handoff`
- `aom review`

### Acceptance criteria

- a builder can hand off to reviewer or QA without manual context copy
- `handoff.md` is created when ownership changes
- review items are structured and reusable
- `index.md` reflects unresolved review state

### Risks addressed

- manual copy-paste handoff
- missing fix expectations after review

## Milestone 7: Manual Intervention and Re-analysis

### Goal

Support the real workflow where the operator enters sessions directly, then returns to the orchestrator without corrupting the model.

### Scope

- record manual intervention event
- refresh `index.md` after intervention
- re-analyze task on return to orchestrator
- propose updated next steps

### Deliverables

- intervention detection or explicit operator command
- re-analysis flow
- next-action recommendation update

### Acceptance criteria

- operator intervention is treated as a supported path
- intervention is recorded in `log.md`
- orchestrator does not blindly continue stale assumptions
- updated guidance can be shown after returning to orchestrator

### Risks addressed

- hidden state drift after manual operator edits
- broken orchestrator trust

## Milestone 8: Session Approval and Recovery Control

### Goal

Add session-scoped approval handling and operator-approved recovery plans.

### Scope

- per-session approval context
- per-session YOLO mode flag
- pending approval tracking
- recovery recommendations for failed or stale sessions

### Deliverables

- approval model
- approval queue view
- recovery recommendation flow
- session failure handling

### Acceptance criteria

- approvals are tracked per session
- YOLO mode is scoped per session
- failed sessions surface recovery recommendations
- operator can decide how to continue

### Risks addressed

- approvals leaking across unrelated work
- silent or unsafe recovery behavior

## Milestone 9: Project Governance for Roles, Skills, and MCP

### Goal

Make project-defined agent teams reproducible through project-scoped role and resource control.

### Scope

- role profiles
- project resource registry
- approved skills
- approved MCP servers
- owner-approved exception model
- role-to-resource bindings

### Deliverables

- project config for agents and resources
- role binding rules
- local exception representation
- validation logic

### Acceptance criteria

- project-defined agents can declare allowed skills and MCP
- default resource usage is project-scoped and strict
- exceptions are explicit and owner-controlled
- specialist agents become reproducible across runs

### Risks addressed

- local machine drift
- uncontrolled tool access
- non-reproducible agent behavior

## Milestone 10: Runtime Adapters and Native Integration

### Goal

Refine runtime-specific integration for Codex, Claude Code, and Kiro while preserving the AOM orchestration model.

### Scope

- runtime capability model
- native instruction rendering
- start and resume commands per runtime
- structured output handling where available
- runtime-specific config mapping

### Deliverables

- Codex adapter
- Claude adapter
- Kiro adapter
- capability registry

### Acceptance criteria

- Codex works as a first-class runtime
- Claude works as a first-class runtime
- Kiro is integrated at the agreed support tier
- AOM artifact and workflow model stays consistent across runtimes

### Risks addressed

- baking in one runtime too deeply
- overpromising unsupported runtime behavior

## Milestone 11: Operator UX Refinement

### Goal

Polish the control experience after the workflow and continuity model are stable.

### Scope

- better summary pane content
- cleaner command ergonomics
- improved status visibility
- streamlined active-workflow focus

### Deliverables

- refined `status` output
- refined orchestrator summary view
- improved operator commands

### Acceptance criteria

- the operator can understand active work at a glance
- the orchestrator surface is useful without becoming a heavy dashboard
- control feels faster than the equivalent manual multi-terminal workflow

### Risks addressed

- a functional but clumsy operator experience

## Recommended Order

The recommended working order is:

1. Milestone 0
2. Milestone 1
3. Milestone 2
4. Milestone 3
5. Milestone 4
6. Milestone 5
7. Milestone 6
8. Milestone 7
9. Milestone 8
10. Milestone 9
11. Milestone 10
12. Milestone 11

## First Three Milestones To Do Next

If we want to work together incrementally, the best next slices are:

### Next slice A

Milestone 0: Foundation Specs

Why first:

- it locks the contracts before code exists
- it reduces ambiguity in task/session/memory behavior
- it gives us concrete review points

### Next slice B

Milestone 1: Local Project Control Skeleton

Why second:

- it creates the repo and runtime foundation
- it gives us the first persistent system state

### Next slice C

Milestone 2: Terminal Team MVP

Why third:

- it proves the core UX goal early
- it validates that the project is worth continuing before deeper workflow work

## Review Method

For each milestone we should do the same loop:

1. define scope and non-goals
2. lock data contracts and CLI behavior
3. implement only that slice
4. test the slice in isolation
5. review what changed in product behavior
6. adjust the next milestone if needed

This keeps AOM aligned with the real operator workflow instead of drifting into an overbuilt system.
