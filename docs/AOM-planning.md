# AOM Planning Document

## Vision

AOM is a project-level control plane for specialist CLI agents. It is designed for a single operator who wants to work with multiple native agent terminals as a coordinated team without manually copying context between them.

The orchestrator does not replace the agents. It manages:

- state continuity
- session lifecycle
- git worktree isolation
- project-scoped resources
- workflow handoff

The system should still feel like normal terminal-based agent work:

- one main chat with the orchestrator for planning and control
- multiple spawned native agent sessions for execution
- live visibility into every active agent
- durable markdown artifacts so work can resume, respawn, or hand off cleanly

Seamlessness means continuity of state across:

- resumed sessions
- newly spawned replacement sessions
- git worktree boundaries
- manual operator intervention
- specialist-to-specialist handoff

## Core Product Goal

AOM should let one operator manage three or more agents in one project without manually copying context between them. It should support:

- one-off small fixes
- feature-sized workflows
- sequential steps
- parallel steps
- resumable specialist sessions
- inspectable terminal-native execution

The orchestrator is a dispatcher and state gate. It is not the primary implementation agent and not the default code reviewer.

## Product Identity

AOM is not a new all-in-one AI assistant. It is a project operating layer around native CLI agents such as:

- Codex
- Claude Code
- Kiro CLI

Its job is to make specialist agents behave like a well-managed team inside one project.

## Operating Principles

- The system is local-first.
- The system is single-operator first.
- Native agent terminals remain visible and usable.
- The operator can always intervene directly.
- Durable project artifacts matter more than transient chat history.
- Tasks are active-workflow oriented, not backlog-heavy in the MVP.
- Task closure is explicit by the operator.

## Interaction Model

### Main surfaces

AOM should provide three primary surfaces:

1. Main orchestrator chat
2. Specialist agent session views
3. Summary/control surface

### Main orchestrator chat

The main orchestrator chat is used to:

- accept new work
- propose task mode
- propose step breakdown
- propose specialist assignments
- manage session continuity
- route handoffs
- request approvals
- propose recovery actions
- re-analyze active work after manual intervention

The main orchestrator chat should focus on active work, not long-term backlog management in the MVP.

### Specialist session views

Specialist sessions are the real execution surfaces. The operator should be able to:

- inspect any active specialist session
- attach to it immediately
- see what the agent is doing
- type directly into it
- treat it like a native terminal session

### Summary/control surface

The summary/control surface should remain simple and interactive. It should show:

- agent name
- role
- task and active step
- runtime
- session status
- last activity
- approvals pending
- needs attention state
- next recommended action

The operator should also be able to type short control actions from this surface.

### Recommended MVP live layout

- tiled agent panes
- one orchestrator summary/control pane

The summary pane should not be read-only.

## Role Model

### Orchestrator

The orchestrator is a dispatcher plus state gate.

Responsibilities:

- accept new work
- recommend task mode
- propose steps and assignments
- manage worktree and session continuity
- route handoffs between specialist agents
- detect missing state artifacts
- detect approval and recovery attention points
- re-analyze active work after manual intervention
- recommend next actions for operator confirmation

The orchestrator should not:

- act as the default quality judge of code
- silently rewrite task intent
- close tasks automatically
- take away direct control from the operator

### Specialist agents

Specialist agents are project-defined workers such as:

- backend
- reviewer
- qa
- architect
- docs
- release

They should feel like native CLI sessions, but be governed through:

- project-defined role profiles
- worktree policy
- allowed skills
- allowed MCP servers
- task context packs
- checkpoint expectations
- handoff expectations

## Task and Workflow Model

### Task shape

AOM must support both:

- small direct tasks
- larger workflow tasks

Examples:

- fix one validation issue
- update one endpoint
- run one review
- implement a feature with multiple specialist steps

The core workflow model is:

- Task + Steps

This is intended to support:

- single-step work
- sequential work
- parallel work through clear dependencies

### Step model

Each step should include at least:

- step ID
- step type
- title
- assigned role or agent
- dependencies
- status
- notes
- handoff condition

Suggested step types:

- implementation
- review
- qa
- research
- handoff
- coordination

### Task closure

AOM does not auto-close tasks. The operator closes a task explicitly.

If required work is incomplete or failed, the task state should become:

- Needs attention

### Step creation policy

Default behavior:

- orchestrator proposes steps
- operator confirms
- steps can be edited later
- task mode can be upgraded later

## Task Planning Modes

AOM should support adaptive planning modes inspired by Kiro’s structured spec flow, without forcing one rigid workflow for all tasks.

### Modes

- Direct
- Bugfix
- Requirements-first
- Design-first

### Mode intent

Direct:

- small, well-scoped work
- minimal planning overhead

Bugfix:

- current behavior
- expected behavior
- root cause
- fix plan

Requirements-first:

- requirements -> design -> tasks

Design-first:

- design or constraints -> requirements -> tasks

### Default planning behavior

Recommended default:

- new tasks start as Direct
- orchestrator may propose upgrade
- operator confirms upgrade

This keeps small work fast while still allowing large work to become structured.

### Review and QA policy

Review and QA should be template-driven, not hardcoded system-wide.

That means:

- each project or task template can define whether review is required
- each project or task template can define whether QA is required
- orchestrator enforces template expectations as a state gate
- the system should not assume one universal workflow for all teams

### Task templates

Task templates should be project-defined, not fixed in the system.

Possible examples:

- small-fix
- feature-standard
- risky-change
- qa-pass
- research-spike

Each template may define:

- default planning mode
- required steps
- optional steps
- review requirement
- QA requirement
- role suggestions

## Operational Memory Model

AOM should use persistent markdown as its operating memory layer.

Three layers of truth:

1. Raw inputs
2. Operational memory
3. Live runtime

### Raw inputs

Raw inputs include:

- repo state
- issue descriptions
- operator requests
- external docs

### Operational memory

Operational memory is stored in `.agent/*.md` artifacts and should be treated as durable task context.

### Live runtime

Live runtime includes:

- active CLI sessions
- active panes
- terminal output

Chat or session history alone must never be the only source of truth.

## Markdown Artifact Set

### Always present

- `.agent/task.md`
- `.agent/state.md`
- `.agent/index.md`
- `.agent/log.md`

### Mode or workflow dependent

- `.agent/handoff.md`
- `.agent/review-notes.md`
- `.agent/requirements.md`
- `.agent/design.md`
- `.agent/tasks.md`

## Artifact Ownership Model

### AOM-owned canonical files

- `index.md`
- `log.md`

These should be written by AOM, not primarily by the agent.

### Agent-updated files with AOM protocol expectations

- `state.md`
- `handoff.md`
- `review-notes.md`
- `requirements.md`
- `design.md`
- `tasks.md`

## Artifact Roles

### `index.md`

`index.md` should be a task-local manifest and current control summary, not just a file list.

It should contain:

- task ID
- task title
- task mode
- current status
- active step
- assigned agents and sessions
- required artifacts and their status
- latest checkpoint
- unresolved review item count
- pending approvals
- next recommended action
- references to related artifacts

### `log.md`

`log.md` should be an append-only canonical task timeline.

AOM should append events here, including:

- task creation
- step creation
- session spawn
- session resume
- session failure
- checkpoint creation
- review state transitions
- approval events
- manual operator intervention
- recovery recommendation

### `state.md`

`state.md` is the current working memory for the active owner.

Rules:

- it should have one active owner at a time
- it must be enough for respawn or resume
- it should not depend on hidden vendor memory

### `handoff.md`

`handoff.md` is the explicit transfer packet between owners.

Rules:

- create it whenever ownership changes between agents
- it can be brief, but should always exist on ownership change

### `review-notes.md`

`review-notes.md` should be structured.

Each item should include at least:

- item ID
- severity
- file or path
- issue
- expected fix
- status
- owner

### `requirements.md`, `design.md`, `tasks.md`

These are structured planning artifacts used only when the selected task mode requires them.

`tasks.md` should include at least:

- step ID
- owner role
- dependency
- status
- notes

## Re-analysis After Manual Intervention

The operator may always enter a specialist session directly.

When the operator does this:

- AOM should record the intervention in `log.md`
- AOM should refresh `index.md`
- when the operator returns to the orchestrator, the orchestrator should re-analyze the active task
- the previous workflow assumption should not be blindly reused

This should be treated as a normal supported workflow, not an error path.

## Session and Continuity Model

State continuity is a first-class product goal.

AOM must preserve continuity across:

- session resume
- replacement spawn
- worktree reuse
- handoff between specialist agents
- manual intervention
- recovery after failure

### Continuity rules

- work must not depend solely on vendor chat or session memory
- every active task must have durable markdown state
- a fresh session must be able to continue from artifacts
- a resumed session must still be reconciled against current artifacts
- the orchestrator should recommend recovery plans instead of silently making recovery decisions

## Session Approval Model

Approvals are scoped per session.

Default policy:

- each session has its own approval context
- YOLO mode is also scoped per session
- project governance remains strict by default
- project owners may define explicit exceptions

## Project Governance Model

AOM manages agents at the project level, not as loose terminal aliases.

Each project-defined agent should specify:

- runtime
- role
- worktree mode
- allowed skills
- allowed MCP servers
- context rules
- session behavior
- checkpoint expectations

### Resource policy

Project resources should live mostly in the repo and be governed by a strict registry.

Defaults:

- skills are project-managed
- MCP servers are project-managed
- only approved resources are available by default
- local overrides are not open-ended
- owner-approved exceptions are allowed explicitly

This ensures specialist agents are reproducible and project-shaped.

## Runtime Strategy

Primary runtimes under consideration:

- Codex
- Claude Code
- Kiro CLI

### Integration posture

Codex and Claude Code should be treated as first-wave first-class integrations.

Kiro should remain in scope, especially because its structured planning concepts are valuable, but integration depth may differ depending on CLI observability and runtime controls.

## Kiro-Inspired Concepts To Reuse

Kiro’s planning model is especially useful for AOM.

Important concepts to adapt:

- structured planning modes
- requirements, design, and task artifacts
- lightweight quick planning for small tasks
- the ability to scale from direct work to structured workflows

AOM should adapt these ideas as project-level task planning modes rather than hardcoding one universal lifecycle.

## Karpathy-Inspired Markdown Concepts To Reuse

The Karpathy markdown management concept is useful for AOM’s memory model.

Important ideas to adopt:

- treat markdown artifacts as durable operational memory
- separate raw inputs from processed operating memory
- keep append-only history where useful
- maintain a current manifest plus a timeline

In AOM terms:

- `index.md` is the current manifest
- `log.md` is the timeline
- `state.md` is the current active working memory
- planning and review files are structured task artifacts

## UI and Surface Model

AOM should remain simple and chat-first.

The intended management surface is:

- a main orchestrator chat
- a set of active specialist sessions
- a visible interactive summary/control surface

This should map directly to the operator workflow:

- talk to the orchestrator to manage the team
- inspect specialist sessions directly
- intervene when necessary
- return to orchestrator for re-analysis and next-step guidance

AOM should not become a dashboard-first system in the MVP.

## Recommended MVP Scope

The MVP should prove three things:

1. Terminal team UX
2. State continuity across sessions, worktrees, and handoffs
3. Project-governed roles and resources

The MVP should be able to support:

- one operator
- at least three active agents
- one project
- no manual context copy-paste between agents

## Open Questions For Next Planning Slice

- exact state machine for tasks, steps, sessions, approvals, and handoffs
- exact schema for `index.md`, `log.md`, `state.md`, and `handoff.md`
- how project templates define review and QA requirements
- how task-mode upgrades are represented and logged
- how project-managed skills and MCP map into Codex, Claude Code, and Kiro native config
- how much structured telemetry is available from each runtime for event normalization
- whether AOM should maintain project-level memory beyond active workflows in later phases

## Assumptions and Defaults

- MVP is local-first
- MVP is single-operator first
- orchestrator is a dispatcher plus state gate
- orchestrator is not the default implementation worker
- tasks are active-workflow oriented
- Direct is the default task mode
- task modes may be upgraded later
- Task + Steps is the primary workflow model
- operator intervention is always allowed
- operator intervention must be logged
- task closure is explicit by the operator
- live visibility into all active agent work is non-negotiable
