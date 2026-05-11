# AOM CLI Spec

## Purpose

This document defines the MVP CLI surface for AOM so the operator can manage projects, tasks, sessions, worktrees, handoffs, approvals, and continuity from a local-first command interface.

The CLI must support the agreed interaction model:

- one main orchestrator control surface
- multiple native specialist sessions
- explicit operator control over workflow, replacement, recovery, and closure

This document defines the Milestone 0 command contract, not the final long-term CLI.

## CLI Design Principles

1. Operator-first
2. Project-scoped by default
3. Explicit over magical
4. Native-session friendly
5. Minimal MVP surface

## Command Groups

The CLI is grouped by intent:

- `aom project ...`
- `aom open`
- `aom status`
- `aom plan`
- `aom agent ...`
- `aom runtime ...`
- `aom doctor`
- `aom task ...`
- `aom step ...`
- `aom session ...`
- `aom attach`
- `aom capture`
- `aom checkpoint`
- `aom handoff`
- `aom review`
- `aom approve`
- `aom deny`
- `aom reanalyze`
- `aom events ...`

Not every command must be fully implemented in the first coding slice, but the intent and naming should be locked now.

## Project Commands

## aom project init

### Purpose

Creates `.aom/` project structure and baseline config.

### Example

```bash
aom project init my-app --repo /repos/my-app
```

### Inputs

- positional: `name`
- required flag: `--repo`
- optional flag: `--default-branch`
- optional flag: `--session-prefix`

### Behavior

- create `.aom/`
- create baseline:
  - `project.yaml`
  - `agents.yaml`
  - `resources.yaml`
  - `policy.yaml`
- initialize or open `sessions.db`
- register project in the database

### Output

Should show:

- project name
- repo path
- config files created
- database initialized or reused

## aom open

### Purpose

Re-enter the project control plane and reconcile current project state.

### Example

```bash
aom open
aom open my-app
```

### Inputs

- optional positional: project name
- optional flag: `--attach`
- optional flag: `--no-attach`

### Behavior

- load project config
- load database
- reconcile tasks, steps, sessions, and worktrees
- reconcile tmux bindings
- create tmux layout if needed
- show or attach control surface

### Output

Should summarize:

- active tasks
- active sessions
- worktree repair or recovery needs
- pending approvals
- recommended next actions

### Side Effects

- may create tmux session
- may refresh `index.md`
- may mark sessions `Detached` or `Failed`
- may produce recovery recommendations

## aom status

### Purpose

Shows current project state without attaching to the session UI.

### Example

```bash
aom status
aom status --tasks
aom status --sessions
```

### Inputs

- optional flags:
  - `--tasks`
  - `--steps`
  - `--sessions`
  - `--approvals`
  - `--worktrees`
  - `--agents`
  - `--runtime`

### Output

Should show:

- project
- active tasks
- active steps
- active sessions
- runtime health summary
- pending approvals
- needs-attention items

## Planning Commands

## aom plan

### Purpose

Requests orchestrator planning for a new piece of work without immediately spawning execution.

### Example

```bash
aom plan "fix login validation"
aom plan "add registration endpoint" --mode requirements-first
```

### Inputs

- positional: work description
- optional flag: `--mode`
- optional flag: `--role`
- optional flag: `--agent`

### Behavior

- evaluate task intent
- recommend task mode
- recommend steps
- recommend role or agent assignment

### Output

Should show:

- recommended task mode
- proposed steps
- proposed owner roles or agents
- recommended next action

## Agent and Runtime Commands

## aom agent list

### Purpose

Lists project-defined agents and their readiness.

### Example

```bash
aom agent list
```

### Output

Should show:

- agent name
- role
- runtime
- enabled state
- resource bindings summary
- readiness or health hints

## aom runtime inspect

### Purpose

Shows runtime capabilities and environment readiness.

### Example

```bash
aom runtime inspect claude
aom runtime inspect codex
```

### Output

Should show:

- runtime version if detectable
- resume support
- headless support
- hooks support
- structured output support
- MCP support
- known limitations

## aom doctor

### Purpose

Validates project readiness and catches configuration or environment drift.

### Example

```bash
aom doctor
```

### Behavior

- validate config files
- validate runtime binaries
- validate worktree mappings
- validate resource bindings
- surface policy enforcement gaps

### Output

Should show:

- passed checks
- failed checks
- warnings
- recommended fixes

## Task Commands

## aom task create

### Purpose

Creates a task and provisions its continuity boundary immediately.

### Example

```bash
aom task create "fix login validation"
aom task create "add registration endpoint" --mode requirements-first
```

### Inputs

- positional: task title
- optional flag: `--mode`
- optional flag: `--role`
- optional flag: `--agent`

### Behavior

- create task record
- choose default mode `Direct` if none is provided
- provision task worktree immediately
- create initial task artifacts:
  - `task.md`
  - `state.md`
  - `index.md`
  - `log.md`
- create initial step proposals if planning requires them

### Output

Should show:

- task id
- selected mode
- initial status
- worktree path
- recommended next step

## aom task show

### Purpose

Shows detailed task state.

### Example

```bash
aom task show TASK-001
```

### Output

Should show:

- task metadata
- mode
- status
- steps
- active owner or session
- worktree state
- continuity readiness
- recommended next action

## aom task update

### Purpose

Updates task metadata such as mode, status intent, or preferred owner.

### Example

```bash
aom task update TASK-001 --mode bugfix
aom task update TASK-001 --status needs-attention
```

### Inputs

- positional: task id
- optional flags:
  - `--mode`
  - `--status`
  - `--role`
  - `--agent`

### Behavior

- update task metadata
- log the change
- trigger re-analysis when needed

## aom task close

### Purpose

Closes a task explicitly.

### Example

```bash
aom task close TASK-001
```

### Behavior

- mark task `Done`
- log close event
- leave archiving as a separate action unless explicitly requested

## Step Commands

## aom step list

### Purpose

Lists steps for a task.

### Example

```bash
aom step list TASK-001
```

### Output

Should show:

- step id
- type
- title
- owner role
- status
- dependencies

## aom step update

### Purpose

Updates step status or ownership.

### Example

```bash
aom step update STEP-002 --status ready
aom step update STEP-003 --agent reviewer-main
```

### Inputs

- positional: step id
- optional flags:
  - `--status`
  - `--role`
  - `--agent`

### Behavior

- validate state transition
- update step metadata
- log the change

## Session Commands

## aom session spawn

### Purpose

Starts a specialist session for a task or step.

### Example

```bash
aom session spawn backend-claude --task TASK-001
aom session spawn reviewer-main --task TASK-001 --step STEP-003
```

### Inputs

- positional: agent name
- required flag: `--task`
- optional flag: `--step`
- optional flag: `--attach`
- optional flag: `--headless`

### Behavior

- resolve agent
- resolve task and optional step
- ensure worktree exists
- create session record
- start runtime in the correct worktree
- bind tmux pane
- inject initial context envelope

### Output

Should show:

- session id
- runtime
- task and step
- worktree path
- attach target
- continuity source

## aom session list

### Purpose

Lists sessions with continuity and recovery hints.

### Example

```bash
aom session list
aom session list --task TASK-001
```

### Output

Should show:

- session id
- agent
- runtime
- task and step
- worktree
- status
- last seen
- recovery recommendation if needed

## aom session show

### Purpose

Shows detailed session metadata.

### Example

```bash
aom session show SESS-001
```

### Output

Should show:

- session metadata
- runtime
- tmux binding
- worktree binding
- approval state
- continuity state
- latest checkpoint summary

## aom session resume

### Purpose

Attempts to resume an existing session using the best valid continuity path.

### Example

```bash
aom session resume SESS-001
```

### Behavior

- inspect session record
- inspect worktree
- inspect runtime resume availability
- attempt to restore or recreate live binding
- reconcile against current artifacts

### Output

Should show:

- session id
- resume result
- reconciliation status
- next recommended action

## aom session recover

### Purpose

Runs recovery assessment for a detached, stale, or failed session and recommends the safest continuation path.

### Example

```bash
aom session recover SESS-001
```

### Behavior

- inspect task, step, worktree, and artifact continuity
- inspect tmux availability
- inspect runtime availability
- recommend:
  - resume
  - rebind
  - replace
  - archive

### Output

Should show:

- session id
- recovery assessment
- continuity quality
- recommended action

## aom session replace

### Purpose

Replaces a session while preserving task continuity.

### Example

```bash
aom session replace SESS-001 --agent backend-codex
```

### Inputs

- positional: session id
- required flag: `--agent`
- optional flag: `--reason`

### Behavior

- evaluate continuity readiness
- prepare continuity packet
- create replacement session in the same worktree
- log replacement event
- transition the old session to `Detached`, `Failed`, `Stopped`, or `Archived` as appropriate

### Output

Should show:

- old session id
- new session id
- replacement reason
- continuity quality
- next recommended action

## aom session stop

### Purpose

Stops a session intentionally.

### Example

```bash
aom session stop SESS-001
```

### Behavior

- stop or detach runtime intentionally
- mark session `Stopped`
- keep worktree intact
- log the event

## aom session archive

### Purpose

Archives a session out of the active system.

### Example

```bash
aom session archive SESS-001
```

### Behavior

- mark session `Archived`
- preserve audit history

## Attach and Capture Commands

## aom attach

### Purpose

Attaches to a live specialist session.

### Example

```bash
aom attach SESS-001
aom attach backend-claude
```

### Behavior

- resolve session or agent
- attach to the live tmux pane when available
- if detached, show a recovery-oriented result instead of failing ambiguously

## aom capture

### Purpose

Captures visible session output.

### Example

```bash
aom capture SESS-001
aom capture backend-claude
```

### Output

- latest visible pane output
- optional persisted log path

## Checkpoint and Handoff Commands

## aom checkpoint

### Purpose

Creates a checkpoint for the current work state.

### Example

```bash
aom checkpoint SESS-001
```

### Behavior

- capture session and worktree state
- update `state.md`
- refresh `index.md`
- append `log.md`
- create checkpoint record

### Output

Should show:

- checkpoint id
- task and step
- owner
- changed files summary
- next action summary

## aom handoff

### Purpose

Prepares a handoff to a new owner.

### Example

```bash
aom handoff SESS-001 --to reviewer
aom handoff SESS-001 --to backend-codex
```

### Inputs

- positional: session id
- required flag: `--to`
- optional flag: `--reason`

### Behavior

- create or refresh `handoff.md`
- update task and step state
- prepare continuity context for the next owner
- log handoff event

### Output

Should show:

- handoff target
- handoff readiness
- recommended next spawn or attach action

## aom review

### Purpose

Provides a lightweight review wrapper for the MVP workflow.

### Example

```bash
aom review TASK-001
aom review TASK-001 --agent reviewer-main
```

### Behavior

- resolve task and worktree
- spawn or reuse reviewer session
- prepare review context
- create or refresh `review-notes.md`
- update unresolved review state in `index.md`
- suggest the next action after review

### Note

For MVP, this is a workflow wrapper, not a full review subsystem.

## Approval Commands

## aom approve

### Purpose

Approves the current pending session-scoped request.

### Example

```bash
aom approve SESS-001
```

### Behavior

- resolve pending approval for the session
- log approval event
- let the session continue

## aom deny

### Purpose

Denies the current pending session-scoped request.

### Example

```bash
aom deny SESS-001
```

### Behavior

- deny the current approval request
- log denial event
- leave the session blocked or route it into attention flow as needed

## Re-analysis and Event Commands

## aom reanalyze

### Purpose

Re-evaluates task state after manual intervention, session loss, mode change, replacement, or recovery.

### Example

```bash
aom reanalyze TASK-001
```

### Behavior

- inspect current database state
- inspect worktree and artifacts
- inspect session continuity signals
- refresh `index.md`
- produce recommended next actions

### Output

Should show:

- recommended next step
- recommended session action
- recommended role or provider if replacement is needed
- continuity concerns

## aom events tail

### Purpose

Shows recent orchestration events for debugging, review, and recovery visibility.

### Example

```bash
aom events tail
aom events tail --task TASK-001
```

### Output

Should show:

- recent event id
- event type
- actor
- related task, step, or session
- timestamp
- short summary

## Recommended MVP Command Set

The minimum recommended implementation set is:

### Project and control

- `aom project init`
- `aom open`
- `aom status`
- `aom plan`
- `aom doctor`

### Agent and runtime

- `aom agent list`
- `aom runtime inspect`

### Task and step

- `aom task create`
- `aom task show`
- `aom task update`
- `aom task close`
- `aom step list`
- `aom step update`

### Session

- `aom session spawn`
- `aom session list`
- `aom session show`
- `aom session resume`
- `aom session recover`
- `aom session replace`
- `aom session stop`

### Execution support

- `aom attach`
- `aom capture`
- `aom checkpoint`
- `aom handoff`
- `aom review`
- `aom reanalyze`
- `aom events tail`

Approval commands may be implemented shortly after if the first coding slice does not yet enforce approvals end to end.

## Output Style Rules

For the MVP:

- default output should be human-readable
- outputs should remain structured enough to scan quickly
- command results should surface:
  - object identity
  - current status
  - continuity or recovery information
  - recommended next action

## Command Guardrails

1. Commands that change workflow state must log events.
2. Commands that change owner or session must refresh relevant artifacts.
3. Commands that affect continuity must validate worktree state first.
4. Session replacement must not destroy the current worktree.
5. Task close must remain explicit.
6. Re-analysis must not silently mutate workflow intent without visible recommendation.

## Locked Decisions for Milestone 0

1. The CLI surface is grouped by intent.
2. `aom open` is the primary re-entry command.
3. `aom status` is the summary command.
4. `aom session replace` is first-class.
5. `aom session resume` and `aom session recover` are first-class.
6. `aom reanalyze` is first-class.
7. `aom review` exists in MVP as a workflow wrapper.
8. Worktree provisioning happens during task creation and session flow, not through agent self-management.
9. Task close is an explicit operator action.
