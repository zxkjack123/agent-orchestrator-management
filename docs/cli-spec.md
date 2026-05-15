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

- `aom project ...` (`init`, `resources`)
- `aom open`
- `aom status`
- `aom plan`
- `aom agent ...`
- `aom runtime ...` (`list`, `inspect`)
- `aom doctor`
- `aom task ...` (`create`, `show`, `update`, `close`)
- `aom step ...` (`list`, `update`)
- `aom session ...` (`spawn`, `list`, `show`, `send`, `resume`, `rebind`, `recover`, `replace`, `stop`, `archive`, `set-agent-id`, `wait`)
- `aom worktree repair`
- `aom attach`
- `aom capture`
- `aom checkpoint`
- `aom handoff`
- `aom review` / `aom review close`
- `aom approve`
- `aom deny`
- `aom reanalyze`
- `aom broadcast`
- `aom channel ...` (`append`, `read`)
- `aom watch`
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
- optional flag: `--template`
- optional flag: `--template-dir`
- optional flag: `--agents`

### Behavior

- create `.aom/`
- create baseline:
  - `project.yaml`
  - `agents.yaml`
  - `resources.yaml`
  - `policy.yaml`
- when `--template` is provided, load the named preset from `templates/project-init/<name>`
- when `--template-dir` is provided, render those files from the provided template directory instead of the built-in starter templates
- `--template` and `--template-dir` cannot be used together
- when `--agents` is provided, keep only the selected starter agents
- `--agents` accepts both template agent names such as `backend-main` and inline agent definitions such as `frontend-main:builder:claude`
- inline agent definitions reuse an existing role config when the named role already exists in the selected template
- inline agent definitions create a minimal role config when the named role is missing:
  - `class: builder`
  - `worktree_mode: dedicated-writer`
  - `checkpoint_expectation: required`
  - `default_session_mode: interactive`
- inline agent names must be alphanumeric plus hyphen
- inline runtimes are case-insensitive and must be one of `claude`, `codex`, `gemini`, or `kiro`
- on interactive terminals, when `--agents` is omitted, prompt for comma-separated agent selections and accept the same `name:role:runtime` inline syntax
- on non-interactive runs, when `--agents` is omitted, keep the full template agent set
- initialize or open `sessions.db`
- register project in the database

### Output

Should show:

- project name
- repo path
- config files created
- database initialized or reused

## aom project resources

### Purpose

Shows the project's governance configuration: role-to-resource bindings, skills, MCP servers, and policy summary.

### Example

```bash
aom project resources
```

### Behavior

- load `resources.yaml` and `policy.yaml` from the current project
- for each role binding, list bound skills and MCP servers with runtime compatibility
- show policy: `deny_commands`, `require_approval`, `session_defaults`, and `owner_exceptions`

### Output

- role bindings table (role → skills, MCP servers)
- policy summary (deny list, approval list, yolo_mode, owner exceptions)

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
aom plan "fix login validation" --create
```

### Inputs

- positional: work description
- optional flag: `--mode`
- optional flag: `--role`
- optional flag: `--agent`
- optional flag: `--create`

### Behavior

- evaluate task intent
- recommend task mode
- recommend steps
- recommend role or agent assignment
- when `--create` is provided, persist the accepted plan as a new task and seed its planned steps

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

Current implementation note:
- until the worktree milestone is implemented, canonical task artifacts are seeded under `.aom/tasks/<task-id>/`

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
aom session spawn reviewer-main --mock
```

### Inputs

- positional: agent name
- optional flag: `--task`
- optional flag: `--step`
- optional flag: `--attach`
- optional flag: `--headless`
- optional flag: `--mock` — launch a mock transcript shell for local flow verification
- optional flag: `--real` — launch the actual runtime CLI (codex, claude) in a live tmux pane
- optional flag: `--fresh` — force a clean context even when a previous native session exists for this task

### Behavior

- resolve agent
- resolve task and optional step
- ensure worktree exists
- create session record
- start runtime in the correct worktree
- when `--mock` is provided, launch a mock transcript shell instead of a provider runtime for local flow verification
- when `--real` is provided, launch the agent's configured CLI runtime (must be in PATH)
- bind tmux pane
- inject initial context envelope
- when `--task` is provided, refresh task continuity artifacts with the active session and append a canonical `session.created` event
- when `--task` is provided and a previous native session ID exists for the same task and agent, the spawn automatically resumes that session (via `--resume` for claude, `codex resume` for codex)
- when `--fresh` is provided, the previous native session ID is ignored and the agent starts a clean context
- for `--real claude` spawns, AOM auto-detects the native session UUID from `~/.claude/projects/` and registers it so the next spawn can resume automatically
- the continuity decision (resume vs. fresh) is always surfaced in spawn output so the operator or orchestrator can verify the chosen path

### Output

Should show:

- session id
- runtime
- task and step
- worktree path
- attach target
- continuity mode: resuming `<session-id>` or fresh start, with a `--fresh` hint when resuming an existing session

## aom session set-agent-id

### Purpose

Manually registers the native CLI session ID for an AOM session so the next spawn
can resume that conversation context. This is a fallback for cases where auto-detection
did not succeed.

### Example

```bash
aom session set-agent-id SESS-001 abc-123-def-456
```

### Inputs

- positional: AOM session id
- positional: native vendor session id (UUID for claude, session identifier for codex)

### Behavior

- update `vendor_session_id` on the session record
- next `session spawn` for the same task and agent will resume this session

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

## aom session send

### Purpose

Sends a prompt or message into a live session pane via tmux send-keys. Used by
the AI orchestrator to deliver task briefs and instructions to sub-agent sessions
without requiring manual terminal interaction.

### Example

```bash
aom session send SESS-001 "read .agent/task.md and begin work"
aom session send SESS-001 "your next task is ready — read .agent/task.md"
```

### Behavior

- load session record and verify pane binding is live
- send the message text to the tmux pane via send-keys
- append `orchestrator.prompt` event to task log if session is task-bound; actor defaults to `"operator"` but is overridden by the `AOM_ACTOR` environment variable (e.g. `AOM_ACTOR=orchestrator-ai`) to support AI-driven orchestrator loops
- fail clearly if pane is not live

### Output

Should show:

- session id
- message delivered
- pane target

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

## aom session rebind

### Purpose

Reconnects a `Detached` session to a live tmux pane without spawning a new runtime process.

### Example

```bash
aom session rebind SESS-001
```

### Behavior

- reject if session status is not `Detached`
- if the existing pane is still alive (verified via `PaneExists`): mark session `Idle` without touching the pane
- if the pane is gone: create a new placeholder pane in the project workspace using `LaunchModePlaceholder`, update `TmuxWindow`/`TmuxPane`/`TmuxSessionName` on the session record, mark `Idle`

### Output

- session id
- agent name
- pane id
- new status (`Idle`)
- whether the pane was reused or recreated

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
- old session outcome
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

## aom review close

### Purpose

Closes the active review step and returns the task to `InProgress` when the operator or orchestrator has accepted the review findings.

### Example

```bash
aom review close TASK-001
```

### Behavior

- find the active review step (status `InProgress`, `Ready`, or `NeedsAttention`)
- advance through `InProgress → Completed` (respecting step state machine)
- transition task status to `InProgress`
- if `review-notes.md` contains an unambiguous owner hint, apply it to the task's `preferred_agent`
- append `review.closed` event to `log.md`
- refresh task artifacts

### Output

- task id
- review step id
- new task status
- preferred agent (if applied from review hint)

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

## aom runtime list

### Purpose

Lists all runtimes configured in the project `agents.yaml` and reports binary
availability in the current PATH environment.

### Example

```bash
aom runtime list
```

### Output

Should show per runtime:

- runtime name
- binary availability and path
- which agents use that runtime

## aom session wait

### Purpose

Polls a task's `log.md` until a specific event type appears, then exits.
Enables unattended orchestrator loops to block on agent completion signals.

### Example

```bash
aom session wait SESS-001 --event task.completed
aom session wait SESS-001 --event handoff.prepared --timeout 1h
```

### Inputs

- positional: session id
- required flag: `--event` — event type to wait for (e.g. `task.completed`, `handoff.prepared`)
- optional flag: `--timeout` — maximum wait duration (default 30m)

### Behavior

- resolve the task bound to the session
- poll the task's `log.md` every 3 seconds
- exit zero when a `### ... | <event-type>` heading line is detected
- exit non-zero with a timeout error when the deadline is reached

### Output

Should show:

- session id and task id
- log path being polled
- matched event line when found

## aom watch

### Purpose

Streams new log events for monitoring. With `--task` monitors one task; without
`--task` monitors all active tasks simultaneously (InProgress, Blocked,
NeedsAttention, Ready). With `--event` blocks until the target event appears;
without `--event` runs in continuous tail mode until timeout.

### Example

```bash
aom watch --task TASK-001
aom watch --task TASK-001 --event task.completed
aom watch --event handoff.prepared --timeout 1h
aom watch
```

### Inputs

- optional flag: `--task` — restrict watch to one task; omit to watch all active tasks
- optional flag: `--event` — event type to wait for; omit for continuous tail mode
- optional flag: `--timeout` — maximum run duration (default 30m)

### Behavior

- when `--task` is provided, resolve and watch only that task's `log.md`
- when `--task` is omitted, enumerate all tasks in active states and watch all of them
- in event mode: exit zero when the event is found; print which task matched when watching multiple tasks
- in tail mode: print new non-empty log lines as they appear; prefix with `[TASK-xxx]` when watching multiple tasks

### Output

Should show:

- list of watched tasks and log paths
- timeout and event type (if applicable)
- streamed log lines (prefixed with task id in multi-task mode)
- matched task and event line when event mode exits

## aom broadcast

### Purpose

Delivers the same prompt to multiple live sessions in a single command.

### Example

```bash
aom broadcast "standup: what is your current status?" --sessions SESS-001,SESS-002
```

### Inputs

- positional: message to deliver
- required flag: `--sessions` — comma-separated list of session ids

### Behavior

- for each session id, call the same delivery path as `session send`
- append canonical `orchestrator.prompt` events to each task's `log.md`
- continue to remaining sessions even if one delivery fails
- report per-session delivery status

### Output

Should show per session:

- session id
- delivery status (success or error reason)

## aom channel

### Purpose

Shared team-level communication channel. Agents post completion notes, blockers,
or questions; the orchestrator reads and reacts. Backed by `.aom/channel.md`.

### Example

```bash
aom channel append "backend-main: task TASK-001 complete, handoff.md ready"
aom channel append "need clarification on auth spec" --agent reviewer-main
aom channel read
```

### Inputs (append)

- positional: message text
- optional flag: `--agent` — agent or actor name (default: `operator`)

### Inputs (read)

No arguments.

### Behavior

- `append`: prepend a timestamped entry to `.aom/channel.md`; create the file if absent
- `read`: print the current contents of `.aom/channel.md`

### Output

- `append`: confirms appended message with timestamp and actor
- `read`: raw channel content

## aom worktree repair

### Purpose

Recovers a missing, unregistered, or pruned git worktree for a task. Restores
`.agent/` artifacts and updates the worktree mapping from `NeedsRepair` to `Ready`.

### Example

```bash
aom worktree repair TASK-001
```

### Inputs

- positional: task id

### Behavior

- check the current worktree mapping state for the task
- if the path is missing, recreate the git worktree from the persisted branch name
- if the path is unregistered but safe (empty or `.agent/` only), adopt and repair it
- restore `.agent/` artifacts from the canonical task artifact root
- update worktree status to `Ready`
- append a canonical `worktree.repaired` event to the task log

### Output

Should show:

- worktree path before and after repair
- repair action taken
- new worktree status

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
- `aom runtime list`
- `aom runtime inspect`

### Task and step

- `aom task create`
- `aom task show`
- `aom task update`
- `aom task close`
- `aom task reanalyze`
- `aom step list`
- `aom step update`
- `aom worktree repair`

### Session

- `aom session spawn`
- `aom session send`
- `aom session wait`
- `aom session list`
- `aom session show`
- `aom session set-agent-id`
- `aom session resume`
- `aom session replace`
- `aom session stop`
- `aom session archive`

### Monitoring and orchestration

- `aom watch`
- `aom broadcast`
- `aom channel append`
- `aom channel read`
- `aom approve`
- `aom deny`

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
9. Task close is an explicit operator action. "Operator" may be a human or an AI
   orchestrator session — both drive explicit CLI commands, never hidden mutations.
