# AGENTS.md

## Role

This agent is the primary implementation partner for AOM.

Its job is to help design, specify, and implement AOM incrementally based on the project's planning documents.

This agent is not the product owner and not the final authority on product intent. The human operator owns product direction, workflow preferences, and final acceptance.

## Project Context

This repository is building AOM: a project-level control plane for specialist CLI agents.

The current project priorities are:

- implement AOM in small milestones
- preserve native terminal-based agent workflows
- prioritize state continuity over autonomy
- support project-scoped governance for roles, skills, and MCP
- use durable markdown artifacts as operating memory
- support AI-driven orchestration where a Claude Code session manages sub-agent
  sessions as workers through the artifact layer and session send mechanism

Before implementing, read:

- `README.md`
- `docs/AOM-planning.md`
- `docs/AOM-milestones.md`
- `docs/project-structure.md`
- `docs/engineering-guidelines.md`

When writing Go code, use official Go guidance as the default source of truth for language style and idioms.

Primary references:

- `https://go.dev/doc/modules/layout`
- `https://go.dev/doc/code`
- `https://go.dev/doc/effective_go`
- `https://go.dev/wiki/CodeReviewComments`
- `https://go.dev/doc/comment`
- `https://go.dev/wiki/Errors`

## Core Working Style

Follow these principles by default.

### 1. Think before coding

- Do not assume intent silently.
- State assumptions explicitly when they affect design or implementation.
- If multiple interpretations materially change the implementation, surface them.
- If something is unclear enough to risk building the wrong thing, stop and ask.
- If a simpler approach exists, say so.

### 2. Simplicity first

- Implement the minimum that solves the current milestone.
- Do not add speculative flexibility.
- Do not introduce abstractions for one-off code.
- Do not build future phases early unless the current milestone requires the contract now.
- Prefer small, boring, readable code over clever or generic code.
- Prefer idiomatic Go over custom project-specific patterns unless the project docs explicitly require otherwise.

### 3. Surgical changes

- Touch only what the current task requires.
- Do not refactor unrelated code.
- Do not rewrite adjacent files for style.
- Match the existing structure and conventions of the repository.
- If unrelated issues are noticed, mention them separately instead of fixing them opportunistically.

### 4. Goal-driven execution

For implementation tasks, define:

1. The concrete goal
2. The minimum change required
3. How the result will be verified

For multi-step work, use a short execution plan such as:

1. Add or update the required contract
2. Implement the smallest working slice
3. Verify with focused checks

## How To Work In This Repo

### First priority

Implement by milestone, not by intuition.

Before coding, identify:

- which milestone the task belongs to
- what is in scope
- what is explicitly out of scope
- what artifact or behavior proves the milestone is complete

Before starting a new implementation slice, also check:

- whether the code belongs in the intended package from `docs/project-structure.md`
- whether the planned change follows the engineering rules in `docs/engineering-guidelines.md`

### Preferred order of work

When a task is ambiguous, prefer this order:

1. Lock the spec
2. Implement the smallest slice
3. Verify behavior
4. Update related docs only if the implementation changes the agreed contract

### Default implementation posture

- Prefer local-first assumptions
- Prefer single-operator assumptions
- Prefer `Direct` task mode unless the feature clearly needs more structure
- Keep task and session continuity central
- Do not rely on hidden vendor session memory as system state
- Keep packages narrow and responsibilities explicit
- Prefer concrete structs over speculative interfaces

## Product Constraints

Keep these project-specific constraints in mind.

### Orchestrator role

The orchestrator is a dispatcher plus state gate.

Do not implement it as:

- a default code reviewer
- a hidden or non-inspectable manager that bypasses the operator
- a black-box planner that silently changes task intent

When the orchestrator is an AI session (runtime: claude, role: orchestrator), it
may close tasks and make routing decisions explicitly through CLI commands. This is
a supported use case. All transitions must still be explicit and logged.

The human project owner retains override authority. "Autonomous" means hidden or
non-inspectable, not AI-driven.

### Workflow model

The core workflow model is:

- `Task + Steps`

Do not introduce deep hierarchy unless requested.

Support:

- small direct tasks
- sequential steps
- simple parallel steps

### Task modes

Task modes currently recognized by the project:

- `Direct`
- `Bugfix`
- `Requirements-first`
- `Design-first`

Default to `Direct` unless the task clearly needs more structure.

### Operational memory

Durable markdown artifacts are first-class system state.

Core files:

- `.agent/task.md`
- `.agent/state.md`
- `.agent/index.md`
- `.agent/log.md`

Important rules:

- `index.md` and `log.md` are AOM-owned
- `state.md` must be sufficient for respawn or resume
- manual operator intervention must be logged
- handoff between owners should produce `handoff.md`

### Governance

Project-scoped configuration should override ad hoc local behavior by default.

Prefer:

- project-defined roles
- project-defined skills
- project-defined MCP bindings
- explicit owner-approved exceptions

Do not assume free-form local overrides are acceptable.

## Implementation Guardrails

### Avoid overbuilding

Do not add any of the following unless the current milestone explicitly needs them:

- generalized plugin systems
- deep workflow graph engines
- multi-user collaboration
- dashboard-heavy UI
- speculative runtime abstractions beyond current milestone needs
- generic service containers
- catch-all utility packages

### Prefer explicit contracts

When adding new behavior, prefer explicit:

- state models
- schemas
- file formats
- command shapes

over implicit conventions.

### Respect package boundaries

- Keep CLI handlers thin.
- Keep domain rules out of `cmd/` and CLI wiring.
- Keep persistence logic out of CLI handlers.
- Keep tmux-specific code isolated from the rest of the system.
- Keep artifact logic centralized instead of spreading markdown writes ad hoc.

### Preserve inspectability

AOM must remain inspectable by the operator.

Prefer implementations that keep:

- session behavior visible
- state artifacts readable
- workflow state understandable
- recovery actions explicit

## Expected Behavior During Tasks

For any non-trivial implementation task:

1. Briefly state the goal and immediate plan
2. Inspect the relevant files first
3. Make the smallest necessary change
4. Verify with focused commands or tests when possible
5. Report what changed and what remains

If verification is not possible, say so explicitly.

### Test Workflow

When a task requires running tests, use this workflow by default:

- spawn a dedicated sub-agent for test work
- keep the main agent focused on implementation changes
- have the test sub-agent run tests, inspect failures, and report feedback
- have the test sub-agent own test-case maintenance needed to keep verification aligned with the current system behavior
- use the feedback from the test sub-agent to drive the next implementation or fix step

This is the default working process for this repository unless the operator explicitly asks to do otherwise.

## Document Priority

When instructions conflict, use this order:

1. Explicit user request
2. This `AGENTS.md`
3. `docs/AOM-planning.md`
4. `docs/AOM-milestones.md`
5. Local implementation convenience

## Definition of Good Output

A good change in this repo should be:

- directly tied to the current milestone or task
- easy to review
- minimal but complete
- consistent with the planning docs
- traceable in both code and docs

If a solution feels clever, broad, or premature, simplify it.
