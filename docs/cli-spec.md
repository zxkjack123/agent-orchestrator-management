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

- `aom project ...` (`init`, `resources`, `layout`, `share`)
- `aom open`
- `aom status`
- `aom plan`
- `aom agent ...` (`add`, `list`, `set-model`, `provision`, `profile`)
- `aom runtime ...` (`list`, `inspect`)
- `aom doctor`
- `aom task ...` (`create`, `show`, `update`, `close`, `list`, `accept`, `verify`, `signal`, `ready`, `cancel`, `link`, `unlink`, `record-result`, `request`, `list-requests`, `approve-request`, `reject-request`, `propose-plan`, `plan-approve`, `plan-reject`, `reanalyze`)
- `aom step ...` (`list`, `update`)
- `aom session ...` (`spawn`, `list`, `show`, `send`, `resume`, `rebind`, `recover`, `replace`, `stop`, `archive`, `set-agent-id`, `wait`, `watch`, `health`)
- `aom worktree ...` (`repair`, `read-file`, `prune`)
- `aom merge ...` (`check`, `prepare`, `commit`, `continue`, `abort`)
- `aom message ...` (`send`, `read`, `clear`, `watch`, `reply`)
- `aom channel ...` (`append`, `read`)
- `aom outbox ...` (`flush`, `list`)
- `aom team ...` (`view`, `status`, `brief`, `roster`)
- `aom orchestrate`
- `aom orchestrator ...` (`start`, `view`, `status`)
- `aom switch`
- `aom dashboard`
- `aom next`
- `aom run-pipeline`
- `aom pause-all` / `aom resume-all`
- `aom attach`
- `aom capture`
- `aom checkpoint`
- `aom handoff`
- `aom review` / `aom review close`
- `aom approve`
- `aom deny`
- `aom reanalyze`
- `aom broadcast`
- `aom watch`
- `aom events ...`
- `aom metrics`
- `aom policy ...` (`list`)
- `aom goal ...` (`set`, `show`, `complete`)
- `aom memory ...` (`append`, `show`, `clear`)
- `aom claim` / `aom claim release` / `aom claim list`
- `aom token-usage`
- `aom role ...` (`list`, `show`, `create`, `update`, `delete`, `preview`)
- `aom class ...` (`list`, `show`, `create`, `edit`, `override`, `delete`, `preview`)
- `aom system-template show`
- `aom serve`

## Agent Workspace Commands

### aom agent provision

#### Purpose

Creates a permanent git worktree for an agent at `<repo>/.aom/agents/<name>/workspace/`
on branch `agents/<name>`. The workspace persists across all tasks assigned to the agent.
After provisioning, session spawns for this agent use the workspace as the execution path
instead of a per-task worktree.

#### Example

```bash
aom agent provision backend-main
aom agent provision frontend-main
```

#### Inputs

- positional: `name` (required) — agent name as defined in `agents.yaml`

#### Behavior

- Validates that the agent exists in the project config
- Creates `<repo>/.aom/agents/<name>/workspace/` via `git worktree add -b agents/<name> <path>`
- If branch already exists, uses `git worktree add <path> agents/<name>` (no `-b`)
- If workspace directory already exists and is a valid worktree: prints "already provisioned" and exits 0 (idempotent)
- Writes `workspace_path` to agent DB record
- Materializes agent context (identity file, skills, MCP config) into the new workspace
- Prints the workspace path on success

#### Output

```
Agent:     backend-main
Workspace: /path/to/repo/.aom/agents/backend-main/workspace
Branch:    agents/backend-main
Status:    provisioned

Next: aom session spawn backend-main --real
```

---

## Free-Roam Messaging Commands

### aom message watch

#### Purpose

Stream new inbox messages for an agent as they arrive (reactive inbox).
Eliminates the need to poll `aom message read` repeatedly.

#### Example

```bash
aom message watch --agent backend-main
aom message watch --agent backend-main --timeout 2h
```

#### Inputs

- required flag: `--agent <name>`
- optional flag: `--timeout <duration>` (default: 30m)

#### Behavior

- Locates `.aom/mailbox/<agent>.md`
- Tracks current byte offset; polls every 2 seconds for new content beyond that offset
- Prints each new `### ...` entry as it appears, with a blank line separator
- Exits 0 on timeout (informational message printed)
- Exits 1 on error (file not found, permission error)
- Reuses the `tailLogEvents` byte-offset pattern from `internal/cli/log_wait.go`

#### Output (streaming)

```
[inbox] 2026-05-21T14:30:01Z | MSG-1748123456 | from: frontend-main
  Hey, is the auth endpoint ready?

[inbox] 2026-05-21T14:38:22Z | MSG-1748129012 | from: operator
  Please review frontend-main's latest commit
```

---

### aom message reply

#### Purpose

Reply to a specific message by ID. Automatically routes the reply to the original sender
without the agent needing to know who sent it or construct a `send` command manually.

#### Example

```bash
aom message reply MSG-1748123456789 "yes, JWT endpoint is ready at /api/auth/login"
```

#### Inputs

- positional: `<msg-id>` — message ID in the format `MSG-<unix-nano>`
- positional: `<message>` — reply text

#### Behavior

- Reads `.aom/mailbox/` to find the message with the given ID
- Extracts the `from:` field as the reply-to target agent
- Calls `appendMailboxMessage` targeting the sender's mailbox
- Prefixes the reply body with `[reply to MSG-xxx] ` for traceability
- Reads sender identity from `AOM_ACTOR` env var (falls back to `"operator"`)
- Prints confirmation: `Reply sent to <agent> (MSG-xxx)`

#### Output

```
Reply sent to frontend-main (MSG-1748123456789)
```

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
- model (configured slug or `(default)` when unset)
- profile path (or hint to run `aom open` if not seeded)

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
- check git identity (`user.name` + `user.email`) — FAIL with fix command if unset
- detect WSL2/NTFS mount and warn about worktree git limitations

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

## Role and Class Commands

### Purpose

Roles and classes define what an agent does and how it behaves within AOM.

- A **role** is a logical label assigned to an agent in `agents.yaml` (e.g., `backend`, `reviewer`, `task-manager`). Roles reference a class and carry workflow settings (worktree mode, checkpoint expectation, default session mode).
- A **class** is a profile template (`.md.tmpl` file) that defines an agent's responsibilities, work standards, and domain-specific guidance — Zone B of the three-zone profile system.
- The **system template** (`base.md.tmpl`) defines the AOM Workflow Protocol — Team Communication, Collaboration Routines, State Machine — which is automatically injected into every agent profile. It is embedded in the binary and is never editable.

**Three-zone profile architecture:**

| Zone | Source | Editable |
|------|--------|----------|
| A — AOM Workflow Protocol | `base.md.tmpl` (embedded in binary) | No — read-only, system-managed |
| B — Role Class Template | `.aom/templates/profiles/<class>.md.tmpl` | Yes — shared by all agents of that class |
| C — Agent Custom Instructions | `## Custom Instructions` in `profile.md` | Yes — per-agent override |

### aom role list

Lists all roles defined in `agents.yaml`.

```bash
aom role list
```

Output: table of role name, class, worktree mode, checkpoint expectation, agents using the role.

### aom role show

Displays the full configuration for a named role.

```bash
aom role show <name>
```

#### Inputs

- positional: `name` — role name

#### Output

Role config fields (class, worktree_mode, checkpoint_expectation, default_session_mode) plus the list of agents using it.

### aom role create

Creates a new role in `agents.yaml`.

```bash
aom role create <name> --class <class> [--worktree-mode isolated|workspace] \
    [--checkpoint-expectation required|optional|none] \
    [--default-session-mode real|mock]
```

#### Inputs

- positional: `name` — role name (must be unique)
- `--class` — class name to use as the profile template (required)
- `--worktree-mode` — `isolated` (per-task worktree) or `workspace` (permanent agent workspace); default `isolated`
- `--checkpoint-expectation` — `required`, `optional`, or `none`; default `optional`
- `--default-session-mode` — `real` or `mock`; default `real`

#### Behavior

- Validates that the specified class exists (built-in or custom)
- Appends the new role to `agents.yaml` under `roles:`
- No agents are assigned to the role automatically

### aom role update

Updates configuration for an existing role.

```bash
aom role update <name> [--class <class>] [--worktree-mode <mode>] \
    [--checkpoint-expectation <value>] [--default-session-mode <mode>]
```

Only the flags provided are updated; others remain unchanged.

### aom role delete

Removes a role from `agents.yaml`.

```bash
aom role delete <name>
```

Fails with a conflict error if any agent in `agents.yaml` is currently assigned to this role.

### aom role preview

Shows the fully composed agent profile that would result from this role, including Zone A (system template) and Zone B (class template).

```bash
aom role preview <name>
```

### aom class list

Lists all available classes.

```bash
aom class list
```

Output: table of class name and source (`builtin`, `custom`, or `builtin-overridden`).

- `builtin` — embedded in binary; protected; cannot be edited directly
- `custom` — defined in `.aom/templates/profiles/<name>.md.tmpl`
- `builtin-overridden` — built-in class with a project-level override file present

### aom class show

Displays the raw template content for a class.

```bash
aom class show <name>
```

For built-in classes, shows the embedded template. For overridden classes, shows the project-level override.

### aom class create

Creates a new custom class with a starter template.

```bash
aom class create <name>
```

Writes a starter `.md.tmpl` file to `.aom/templates/profiles/<name>.md.tmpl`. Does not overwrite an existing file.

### aom class edit

Opens the class template in `$VISUAL` or `$EDITOR` (fallback: `vi`).

```bash
aom class edit <name>
```

Only works for custom classes and built-in-overridden classes. Use `aom class override` first to make a built-in class editable.

### aom class override

Creates a project-level editable copy of a built-in class.

```bash
aom class override <name>
```

Copies the embedded built-in template to `.aom/templates/profiles/<name>.md.tmpl`, making it editable without modifying the binary default. The class source becomes `builtin-overridden`.

### aom class delete

Deletes a custom class or reverts a built-in override.

```bash
aom class delete <name>
```

- For a `custom` class: removes `.aom/templates/profiles/<name>.md.tmpl`
- For a `builtin-overridden` class: removes the project override, reverting to the embedded default
- For a `builtin` class with no override: returns an error ("nothing to delete")

Fails if any role in `agents.yaml` is currently using this class.

### aom class preview

Shows the full composed profile that would be generated using this class, combining Zone A (system template) + Zone B (this class template).

```bash
aom class preview <name>
```

### aom system-template show

Displays the read-only AOM Workflow Protocol (Zone A) embedded in the binary.

```bash
aom system-template show
```

This template is injected automatically into every agent profile and cannot be overridden. It defines how agents interact with the AOM state machine, communicate via the channel, and signal lifecycle events.

---

## Agent Management Commands

### aom agent add

#### Purpose

Register a new agent in the project.

#### Example

```bash
aom agent add backend-api --role developer --runtime claude --class builder
```

#### Inputs

- positional: `name` — agent name (alphanumeric + hyphen)
- `--role <name>` (required) — role defined in `agents.yaml`
- `--runtime <name>` (required) — one of: `claude`, `codex`, `gemini`, `kiro`
- `--class <class>` (optional) — profile class; defaults to role's class

#### Behavior

- Validates runtime against known providers
- Warns if agent name implies a different runtime than specified
- Warns if another agent already uses the same role and runtime
- Appends agent to `agents.yaml`
- Generates `profile.md` from class template

---

### aom agent set-model

#### Purpose

Update the LLM model for an agent without overwriting the rest of `agents.yaml`.

#### Example

```bash
aom agent set-model backend-main claude-opus-4
```

#### Inputs

- positional: `name` — agent name
- positional: `model` — model identifier

#### Behavior

- Validates model against the provider's known model list (warning only — allows future models)
- Updates only the `model:` field in `agents.yaml`
- Takes effect on the next session spawn

---

### aom agent profile show

#### Purpose

Print the full composed profile for an agent (Zone A + Zone B + Zone C).

#### Example

```bash
aom agent profile show backend-main
```

#### Inputs

- positional: `name` — agent name

---

### aom agent profile update

#### Purpose

Append content to the Responsibilities or Constraints sections of an agent's profile.

#### Example

```bash
aom agent profile update backend-main --responsibilities "REST endpoint security"
aom agent profile update backend-main --constraints "No direct DB writes from HTTP layer"
```

#### Inputs

- positional: `name` — agent name
- `--responsibilities <text>` — append to Responsibilities section
- `--constraints <text>` — append to Constraints section

---

### aom agent profile set-instructions

#### Purpose

Set Zone C custom instructions for an agent (per-agent override in `profile.md`).

#### Example

```bash
aom agent profile set-instructions backend-main "Always add unit tests for critical paths."
aom agent profile set-instructions backend-main --file custom.md
aom agent profile set-instructions backend-main --clear
```

#### Inputs

- positional: `name` — agent name
- positional: `<text>` — instruction text (inline, mutually exclusive with `--file`)
- `--file <path>` — load instructions from a Markdown file
- `--clear` — remove all custom instructions

#### Behavior

- Writes to the `## Custom Instructions` section in `profile.md`
- Injected at session spawn as Zone C

---

## Task Extended Commands

### aom task list

#### Purpose

List tasks in the project, with optional filtering and JSON output.

#### Example

```bash
aom task list
aom task list --active
aom task list --active --json
```

#### Inputs

- `--active` — show only tasks in Ready, InProgress, Blocked, or NeedsAttention states
- `--json` / `-j` — output as JSON array

#### Output

Table with columns: `ID`, `Title`, `Status`, `Mode`, `Priority`, `Agent`.

---

### aom task accept

#### Purpose

Accept a task after the assigned agent signals completion, advancing it to Done.

#### Example

```bash
aom task accept TASK-001
aom task accept TASK-001 --force
aom task accept TASK-001 --auto --timeout 120m
```

#### Inputs

- positional: `task-id`
- `--force` — skip verify checks and accept anyway
- `--auto` — poll until all checks pass, then auto-accept
- `--interval <duration>` — poll interval for `--auto` (default: 15s)
- `--timeout <duration>` — max wait for `--auto` (default: 30m)

#### Behavior

- Runs `task verify` checks before accepting; blocks if checks fail (bypass with `--force`)
- `--auto` polls every 15 s until all checks pass, then accepts
- Sets task status to Done; logs `task.accepted` event
- Auto-stops the bound session if it is Idle

---

### aom task verify

#### Purpose

Run completion checks on a task to confirm it is ready to accept.

#### Example

```bash
aom task verify TASK-001
aom task verify TASK-001 --watch
aom task verify TASK-001 --watch --interval 30s --timeout 60m
```

#### Inputs

- positional: `task-id`
- `--watch` — poll until all checks pass; prints iteration status on each poll
- `--interval <duration>` — poll frequency (default: 10s); requires `--watch`
- `--timeout <duration>` — max watch time (default: 30m); requires `--watch`

#### Checks performed

1. `task.completed` or `task.closed` event present in `.agent/log.md`
2. Tagged commit `[TASK-xxx]` on agent branch (workspace agents)
3. `handoff.md` filled with real content (not template placeholder)
4. All required steps completed (when steps are defined)

---

### aom task signal

#### Purpose

Manually write a lifecycle event to the task artifact log. Used when the agent must signal a state transition explicitly rather than letting AOM detect it.

#### Example

```bash
aom task signal TASK-001 task.completed
aom task signal TASK-001 step.completed
aom task signal TASK-001 checkpoint.created
aom task signal TASK-001 handoff.prepared
```

#### Inputs

- positional: `task-id`
- positional: `event` — one of: `task.completed`, `task.closed`, `handoff.prepared`, `checkpoint.created`, `step.completed`

#### Behavior

- Appends the event to `.agent/log.md` in the task artifact root
- For workspace agents: also mirrors to workspace `.agent/log.md`
- `task.completed` auto-promotes the workspace handoff to the task artifact

---

### aom task ready

#### Purpose

Advance a task from Draft or Planned to Ready, signalling it can be started.

#### Example

```bash
aom task ready TASK-001
```

#### Inputs

- positional: `task-id`

#### Behavior

- Validates task is in Draft or Planned state
- Sets status to Ready
- Auto-promotes dependent tasks to Ready if all their blockers are now Done

---

### aom task cancel

#### Purpose

Cancel a task and remove it from the active work queue.

#### Example

```bash
aom task cancel TASK-001
aom task cancel TASK-001 --reason "scope removed"
```

#### Inputs

- positional: `task-id`
- `--reason <text>` — optional cancellation reason (logged to artifact)

#### Behavior

- Sets task status to Archived with a cancelled annotation
- Stops any bound live session

---

### aom task link

#### Purpose

Declare a dependency: one task blocks on a prerequisite task.

#### Example

```bash
aom task link TASK-003 --depends-on TASK-001
```

#### Inputs

- positional: `task-id` — the task that will be blocked
- `--depends-on <task-id>` (required) — the prerequisite task

#### Behavior

- Validates no circular dependency (BFS cycle detection)
- Records the edge in DB; the dependent task is demoted to Blocked if the prerequisite is not Done

---

### aom task unlink

#### Purpose

Remove a dependency between two tasks.

#### Example

```bash
aom task unlink TASK-003 --depends-on TASK-001
```

#### Inputs

- Same as `aom task link`

#### Behavior

- Removes the dependency edge
- If the prerequisite was the last blocker, promotes the dependent task to Ready

---

### aom task record-result

#### Purpose

Write the agent's final result summary to the task artifact.

#### Example

```bash
aom task record-result TASK-001 "Implemented GET /users endpoint with pagination."
```

#### Inputs

- positional: `task-id`
- positional: `<result-text>`

#### Behavior

- Appends result under `## Result` in `task.md`
- Logs `task.result-recorded` event

---

### aom task request

#### Purpose

Submit a request to the operator for clarification, an unblock, or approval.

#### Example

```bash
aom task request TASK-001 "Need clarification on auth spec before proceeding."
aom task request TASK-001 "DB schema finalized — please unblock." --type unblock
```

#### Inputs

- positional: `task-id`
- positional: `<message>`
- `--type <type>` — `clarification`, `unblock`, `approval`, or `custom` (default: `custom`)

#### Behavior

- Creates a request record in DB; logs `task.request-created` event
- Prints the request ID for use with `approve-request` / `reject-request`

---

### aom task list-requests

#### Purpose

List pending task requests awaiting operator action.

#### Example

```bash
aom task list-requests
aom task list-requests --task TASK-001
```

#### Inputs

- `--task <task-id>` — filter to a specific task

---

### aom task approve-request

#### Purpose

Operator approves a pending task request.

#### Example

```bash
aom task approve-request REQ-001 --note "Auth spec in docs/auth.md"
```

#### Inputs

- positional: `request-id`
- `--note <text>` — optional approval note sent back to the agent

---

### aom task reject-request

#### Purpose

Operator rejects a pending task request.

#### Example

```bash
aom task reject-request REQ-001 --reason "Out of scope for this milestone"
```

#### Inputs

- positional: `request-id`
- `--reason <text>` — optional rejection reason

---

### aom task propose-plan

#### Purpose

Submit a multi-step execution plan for operator approval before starting work.

#### Example

```bash
aom task propose-plan TASK-001 \
  --steps "Design schema,Implement endpoint,Write tests"
```

#### Inputs

- positional: `task-id`
- `--steps <text>` — comma-separated step descriptions

#### Behavior

- Creates step records in DB
- Sets task status to Planned, pending operator approval via `plan-approve`

---

### aom task plan-approve

#### Purpose

Operator approves the proposed plan; advances the task to Ready.

#### Example

```bash
aom task plan-approve TASK-001
```

---

### aom task plan-reject

#### Purpose

Operator rejects the proposed plan with feedback; agent must revise and re-propose.

#### Example

```bash
aom task plan-reject TASK-001 --feedback "Too many steps — simplify to 2."
```

#### Inputs

- positional: `task-id`
- `--feedback <text>` — rejection reason sent to agent

---

## Session Extended Commands

### aom session watch

#### Purpose

Stream task events from `log.md` as they arrive, optionally exiting when a specific event is detected.

#### Example

```bash
aom session watch --task TASK-001 --event task.completed --timeout 30m
aom session watch --auto-spawn --real --timeout 60m
```

#### Inputs

- `--task <task-id>` — watch a single task (omit to watch all active tasks)
- `--event <type>` — exit on detection of this event (e.g., `task.completed`)
- `--timeout <duration>` — max watch duration (default: 30m)
- `--auto-spawn` — automatically spawn sessions for SPAWN action items
- `--real` / `--mock` — required with `--auto-spawn`
- `--interval <duration>` — polling interval (default: 10s)

#### Behavior

- Polls every 10 s; prints new events as they are appended to `log.md`
- With `--event`: exits 0 when event found; exits 1 on timeout
- With `--auto-spawn`: parses action items each poll; calls `session spawn` for any SPAWN item

---

### aom session health

#### Purpose

Show checkpoint recency and handoff status across all active sessions.

#### Example

```bash
aom session health
```

#### Output

Table: session ID, agent, task, time since last checkpoint, handoff status, overall health. Warns when checkpoint age exceeds 2 hours or no handoff has been written.

---

## Merge Commands

### aom merge check

#### Purpose

Analyze merge readiness: conflicts, file overlap with other active tasks, and any blockers.

#### Example

```bash
aom merge check TASK-001
aom merge check TASK-001 --against main
```

#### Inputs

- positional: `task-id`
- `--against <branch>` — comparison target (default: project default branch)

#### Behavior

- Diffs the task branch against the target
- Reports changed files, overlap score with other active worktrees, and add/add conflict risk
- Suggests `aom merge prepare` when safe to proceed

---

### aom merge prepare

#### Purpose

Generate the merge plan document and create an integration step in the task.

#### Example

```bash
aom merge prepare TASK-001
aom merge prepare TASK-001 --into main
```

#### Inputs

- positional: `task-id`
- `--into <branch>` — target branch (default: project default branch)

#### Behavior

- Runs full merge check; writes results to `.aom/merge-plan.md`
- Creates an integration step in the task if it is still active
- Syncs plan to the task artifact directory

---

### aom merge commit

#### Purpose

Execute the git merge, auto-resolve safe conflicts, and finalize the commit.

#### Example

```bash
aom merge commit TASK-001
aom merge commit TASK-001 --into main --prefer-branch
```

#### Inputs

- positional: `task-id`
- `--into <branch>` — target branch (default: project default branch)
- `--prefer-branch` — auto-resolve add/add conflicts by keeping the task-branch version

#### Behavior

- Requires task status Done
- Auto-resolves `.agent/`, `AGENTS.md`, `CLAUDE.md` conflicts (AOM identity files)
- Strips `.agent/` and `.aom/` artifacts from the merge commit
- If conflicts remain: pauses with instructions for `aom merge continue` or `aom merge abort`

---

### aom merge continue

#### Purpose

Complete a merge paused by conflicts after the operator has resolved them.

#### Example

```bash
aom merge continue TASK-001
```

#### Inputs

- positional: `task-id`

#### Behavior

- Verifies a merge is in progress (`MERGE_HEAD` present)
- Operator must have resolved conflicts and staged files with `git add`
- Runs `git merge --continue` and finalizes the commit

---

### aom merge abort

#### Purpose

Abort a merge in progress and restore the working tree.

#### Example

```bash
aom merge abort TASK-001
```

#### Inputs

- positional: `task-id`

#### Behavior

- Requires a merge to be in progress
- Runs `git merge --abort`

---

## Messaging Commands

### aom message send

#### Purpose

Send a direct message to an agent's inbox.

#### Example

```bash
aom message send backend-main "please review the auth endpoint spec"
aom message send frontend-main "design docs at docs/ui-spec.md" --from orchestrator-main
```

#### Inputs

- positional: `agent-name`
- positional: `<message>`
- `--from <sender>` — override sender identity (default: `AOM_AGENT_NAME` → `AOM_ACTOR` → `operator`)

#### Behavior

- In agent sandbox mode: stages to `.aom/outbox/<agent>.md`; operator flushes with `aom outbox flush`
- Otherwise: writes directly to `.aom/mailbox/<agent>.md`
- Sends a tmux notification to the agent's live pane if one is active

---

### aom message read

#### Purpose

Print messages from an agent's inbox since the last read.

#### Example

```bash
aom message read backend-main
```

#### Inputs

- positional: `agent-name`

#### Behavior

- Reads `.aom/mailbox/<agent>.md` from the current cursor offset
- Advances the cursor after reading
- Prints "no messages" if mailbox is empty or fully read

---

### aom message clear

#### Purpose

Archive the mailbox contents and reset it to empty.

#### Example

```bash
aom message clear backend-main
```

#### Inputs

- positional: `agent-name`

#### Behavior

- Moves existing messages to `.aom/mailbox/<agent>.archive.md`
- Resets active mailbox to empty; resets cursor

---

## Team Commands

### aom team view

#### Purpose

Join all active agent panes into the shared team tmux window and attach for real-time monitoring.

#### Example

```bash
aom team view
aom team view --layout tiled
```

#### Inputs

- `--layout <mode>` — `tiled`, `even-horizontal`, `even-vertical` (default: `tiled`)

#### Behavior

- Creates the team tmux window if it does not exist
- Prunes stale panes; adds missing live panes
- Applies the layout; attaches the operator

---

### aom team status

#### Purpose

Show how each agent session is currently arranged (team window, dedicated, or solo).

#### Example

```bash
aom team status
```

#### Output

Table: agent, session state, pane placement. Includes quick-action commands.

---

### aom team brief

#### Purpose

Generate a shared team briefing: active tasks, pending requests, recent channel messages, agent roster.

#### Example

```bash
aom team brief
```

#### Behavior

- Collects active tasks and dependencies
- Appends the last 5 channel messages
- Lists all agents with session status
- Writes briefing to `.aom/shared/team-brief.md` and pushes to active worktrees

---

### aom team roster

#### Purpose

Refresh the worktree-local team roster snapshot used by agents to orient themselves.

#### Example

```bash
aom team roster --agent backend-main
```

#### Inputs

- `--agent <name>` — agent whose worktree receives the update (falls back to `AOM_ACTOR` env var)

#### Behavior

- Generates a snapshot: all agents, sessions, task dependency graph
- Writes to `.agent/team-roster.md` in the agent's worktree

---

## Orchestration Commands

### aom orchestrate

#### Purpose

Spawn all enabled agents into a shared team tmux grid simultaneously.

#### Example

```bash
aom orchestrate --real
aom orchestrate --real --layout even-horizontal
```

#### Inputs

- `--real` / `--mock` (required)
- `--layout <mode>` — `tiled`, `even-horizontal`, `even-vertical` (default: `tiled`)
- `--allow-collision` — bypass the single-writer guard

#### Behavior

- Iterates all enabled agents; skips those already live in the team window
- Auto-provisions workspaces for agents that lack one
- Applies layout; attaches the operator to the team window

---

### aom orchestrator start

#### Purpose

Spawn the designated orchestrator agent and hand it the current project goal.

#### Example

```bash
aom orchestrator start --real
aom orchestrator start --goal "Deploy v2 API" --real
```

#### Inputs

- `--goal "<text>"` — write goal before spawning (optional; uses existing goal if omitted)
- `--real` / `--mock` — default: `--real`
- `--no-grid` — spawn in a dedicated window instead of the shared team grid

#### Behavior

- Finds the first enabled agent whose role class is `orchestrator`
- Writes `.aom/goal.json` if `--goal` is provided
- Spawns the agent with full profile and team context

---

### aom orchestrator view

#### Purpose

Attach to the team grid showing the orchestrator and workers side-by-side.

#### Example

```bash
aom orchestrator view
aom orchestrator view --layout even-horizontal
```

#### Inputs

- `--layout <mode>` — (default: `tiled`)

---

### aom orchestrator status

#### Purpose

Show the orchestrator goal, sessions, and recent channel activity.

#### Example

```bash
aom orchestrator status
```

#### Output

- Current goal (text, status, set date)
- All orchestrator-class sessions and their states
- Last 10 channel messages

---

## Operator UX Commands

### aom switch

#### Purpose

Jump directly into an agent's live tmux pane by name, logging an `operator.intervention` event.

#### Example

```bash
aom switch backend-main
```

#### Inputs

- positional: `agent-name`

#### Behavior

- Finds the most recently created live session for the agent
- Requires an active tmux pane
- Logs `operator.intervention` to the task artifact log

---

### aom dashboard

#### Purpose

Display a live-refreshing ANSI terminal dashboard: sessions, action items, recent channel messages. Press Ctrl+C to exit.

#### Example

```bash
aom dashboard
aom dashboard --interval 10s
```

#### Inputs

- `--interval <duration>` — refresh interval (default: 5s)

#### Output

- Sessions table: agent, status, task, pane health
- Action items: APPROVAL / ACCEPT / SPAWN items with exact commands
- Last 6 channel messages

---

### aom next

#### Purpose

List unblocked tasks ready to start and blocked tasks waiting on dependencies.

#### Example

```bash
aom next
aom next --format json
```

#### Inputs

- `--format json` — output as JSON

#### Output

Two sections: **Ready** (can be started now) and **Blocked** (waiting on prerequisite tasks), each with ID, title, priority, preferred role/agent, and blocker list.

---

### aom pause-all

#### Purpose

Pause all currently Working sessions by transitioning them to WaitingApproval.

#### Example

```bash
aom pause-all
aom pause-all --reason "deployment freeze"
```

#### Inputs

- `--reason <text>` — optional reason appended to the pause notification

#### Behavior

- Transitions all Working sessions to WaitingApproval
- Sends a notification to each session's tmux pane
- Logs `approval.pause` to task artifacts

---

### aom resume-all

#### Purpose

Resume all paused sessions by transitioning them from WaitingApproval back to Idle.

#### Example

```bash
aom resume-all
```

#### Behavior

- Transitions all WaitingApproval sessions to Idle
- Logs `approval.resume` event

---

## Project Extended Commands

### aom project layout

#### Purpose

Generate a `repo-layout.md` snapshot from the git tree and push it to all active agent worktrees.

#### Example

```bash
aom project layout
```

#### Behavior

- Extracts top-level structure via `git ls-tree`
- Writes to `.aom/shared/repo-layout.md`
- Copies to `.agent/shared/` in every active worktree

---

### aom project share

#### Purpose

Copy a file to the shared directory and push it to all active agent worktrees.

#### Example

```bash
aom project share docs/api-spec.md
```

#### Inputs

- positional: `<file-path>`

#### Behavior

- Copies file to `.aom/shared/<filename>`
- Pushes to `.agent/shared/<filename>` in every active worktree

---

## Worktree Extended Commands

### aom worktree read-file

#### Purpose

Read a file from a specific task's worktree without switching into it.

#### Example

```bash
aom worktree read-file TASK-001 src/handler.go
```

#### Inputs

- positional: `task-id`
- positional: `relative-path` — relative to the worktree root

#### Behavior

- Path-traversal guard: rejects paths that escape the worktree root
- Requires worktree status Ready or Active
- Logs a `worktree.read` audit event

---

### aom worktree prune

#### Purpose

Remove archived and orphaned worktrees from git and the filesystem.

#### Example

```bash
aom worktree prune
aom worktree prune --dry-run
```

#### Inputs

- `--dry-run` — list worktrees that would be removed without modifying anything

#### Behavior

- Finds worktrees in Archived status or missing a DB record
- Runs `git worktree remove --force` for each
- Runs `git worktree prune` to clean up stale git references

---

## Pipeline Command

### aom run-pipeline

#### Purpose

Run the full task lifecycle in sequence: spawn → wait(task.completed) → verify → accept → [merge].

#### Example

```bash
aom run-pipeline TASK-001 --agent backend-main --real
aom run-pipeline TASK-001 --real --timeout 120m --skip-merge
```

#### Inputs

- positional: `task-id`
- `--agent <name>` — override the task's preferred agent
- `--timeout <duration>` — total time budget for all stages (default: 60m)
- `--real` / `--mock` (required)
- `--skip-merge` — stop after accept; do not run merge

#### Behavior

Five sequential stages with shared timeout tracking:
1. **spawn** — run `session spawn`
2. **wait** — poll `log.md` for `task.completed` event
3. **verify** — run `task verify` checks
4. **accept** — run `task accept`
5. **merge** — run `merge commit` (skipped with `--skip-merge`)

On timeout: prints per-stage escalation hints and the exact command to resume.

---

## Metrics and Observability

### aom metrics

#### Purpose

Show team velocity metrics: completed tasks, average duration, blocked events, and bottleneck analysis.

#### Example

```bash
aom metrics
aom metrics --days 14
aom metrics --task TASK-001
```

#### Inputs

- `--days <number>` — look-back window in days (default: 7)
- `--task <task-id>` — filter to a single task

#### Output

- Tasks completed in window with duration
- Per-agent completion counts
- Tasks blocked for more than 1 hour
- Suggested bottleneck agent

---

### aom policy list

#### Purpose

Display the project's policy: blocked commands, approval-gated commands, and per-task enforcement.

#### Example

```bash
aom policy list
aom policy list --task TASK-001
```

#### Inputs

- `--task <task-id>` — include the task's assigned agent and enforcement level

#### Output

- `deny_commands` list
- `require_approval` list
- `yolo_mode` and approval scope defaults
- With `--task`: agent assignment and active enforcement level

---

## Outbox Commands

### aom outbox flush

#### Purpose

Publish all pending outbox messages from agent worktrees to the shared channel or agent mailboxes.

#### Example

```bash
aom outbox flush
```

#### Behavior

- Scans `.aom/worktrees/*/outbox.md` and `.aom/agents/*/workspace/.aom/outbox.md`
- Publishes channel messages to `.aom/channel.md`
- Publishes mailbox messages to `.aom/mailbox/<agent>.md`
- Sends tmux notifications to live recipient sessions
- Empties each outbox after publishing

---

### aom outbox list

#### Purpose

Show pending outbox messages without publishing them.

#### Example

```bash
aom outbox list
```

#### Output

Table grouped by worktree: destination (channel or `mailbox:<agent>`), message preview.

---

## Goal Commands

### aom goal set

#### Purpose

Set the project goal for the orchestrator agent.

#### Example

```bash
aom goal set "Implement a REST API with full CRUD for users and products"
```

#### Inputs

- positional: `<goal-text>`

#### Behavior

- Writes to `.aom/goal.json`; overwrites any existing goal

---

### aom goal show

#### Purpose

Print the current project goal and its status.

#### Example

```bash
aom goal show
```

#### Output

Goal text, status (`Active` / `Complete`), and the date it was set.

---

### aom goal complete

#### Purpose

Mark the current project goal as complete.

#### Example

```bash
aom goal complete
```

---

## Memory Commands

### aom memory append

#### Purpose

Append a timestamped note to `project-memory.md`. Memory entries are injected into every agent session at spawn.

#### Example

```bash
aom memory append "Frontend uses Tailwind CSS 3.x, not Bootstrap"
```

#### Inputs

- positional: `<note>`

#### Behavior

- Creates `project-memory.md` with a header on first use
- Appends `[YYYY-MM-DD] <actor>: <note>`
- Actor resolved from `AOM_ACTOR` env var (fallback: `operator`)

---

### aom memory show

#### Purpose

Print all entries in `project-memory.md`.

#### Example

```bash
aom memory show
```

---

### aom memory clear

#### Purpose

Erase all entries from `project-memory.md`.

#### Example

```bash
aom memory clear --confirm
```

#### Inputs

- `--confirm` — required safety flag

#### Behavior

- Resets `project-memory.md` to the header only

---

## Claim Commands

File claims prevent multiple agents from unknowingly editing the same paths simultaneously.

### aom claim

#### Purpose

Claim one or more file paths for an agent, warning if overlap with an existing claim is detected.

#### Example

```bash
aom claim src/core.py src/config.py --agent backend-main --task TASK-001
```

#### Inputs

- positional: `<paths...>` — one or more relative file paths
- `--agent <name>` — claiming agent (default: resolved from `AOM_ACTOR`)
- `--task <id>` — optional task context

#### Behavior

- Saves claim to `.aom/claims/<agent>.json`
- Warns if overlap detected with another agent's existing claim

---

### aom claim release

#### Purpose

Release all file claims for an agent.

#### Example

```bash
aom claim release --agent backend-main
```

#### Inputs

- `--agent <name>` — agent whose claims to release

---

### aom claim list

#### Purpose

List all active file claims across all agents.

#### Example

```bash
aom claim list
```

#### Output

Table: agent, claimed paths, task context, claim timestamp.

---

## Miscellaneous Commands

### aom token-usage

#### Purpose

Display provider-specific instructions for checking token usage.

#### Example

```bash
aom token-usage
```

#### Output

- **Claude**: claude.ai dashboard → Usage tab
- **Codex**: platform.openai.com → Usage

Automatic token tracking is not yet implemented.

---

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

### Roles and classes

- `aom role list`
- `aom role show`
- `aom role create`
- `aom role update`
- `aom role delete`
- `aom role preview`
- `aom class list`
- `aom class show`
- `aom class create`
- `aom class edit`
- `aom class override`
- `aom class delete`
- `aom class preview`
- `aom system-template show`

### Task and step

- `aom task create`
- `aom task show`
- `aom task list`
- `aom task update`
- `aom task close`
- `aom task accept`
- `aom task verify`
- `aom task signal`
- `aom task ready`
- `aom task cancel`
- `aom task link` / `aom task unlink`
- `aom task record-result`
- `aom task request` / `aom task list-requests` / `aom task approve-request` / `aom task reject-request`
- `aom task propose-plan` / `aom task plan-approve` / `aom task plan-reject`
- `aom task reanalyze`
- `aom step list`
- `aom step update`
- `aom worktree repair`

### Session

- `aom session spawn`
- `aom session send`
- `aom session wait`
- `aom session watch`
- `aom session list`
- `aom session show`
- `aom session health`
- `aom session set-agent-id`
- `aom session resume`
- `aom session replace`
- `aom session recover`
- `aom session rebind`
- `aom session stop`
- `aom session archive`

### Merge workflow

- `aom merge check`
- `aom merge prepare`
- `aom merge commit`
- `aom merge continue`
- `aom merge abort`

### Monitoring and orchestration

- `aom status`
- `aom dashboard`
- `aom next`
- `aom switch`
- `aom watch`
- `aom metrics`
- `aom team view` / `aom team status` / `aom team brief` / `aom team roster`
- `aom orchestrate`
- `aom orchestrator start` / `aom orchestrator view` / `aom orchestrator status`
- `aom run-pipeline`
- `aom broadcast`
- `aom channel append` / `aom channel read`
- `aom message send` / `aom message read` / `aom message clear` / `aom message watch` / `aom message reply`
- `aom outbox flush` / `aom outbox list`
- `aom policy list`
- `aom pause-all` / `aom resume-all`
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

### Project tools

- `aom project layout`
- `aom project share`
- `aom worktree read-file`
- `aom worktree prune`
- `aom goal set` / `aom goal show` / `aom goal complete`
- `aom memory append` / `aom memory show` / `aom memory clear`
- `aom claim` / `aom claim release` / `aom claim list`
- `aom token-usage`

## aom serve

### Purpose

Starts the AOM web UI server. Serves a React-based dashboard embedded in the binary for monitoring and managing projects, sessions, tasks, and agents from a browser.

### Command

```
aom serve [--port <port>] [--host <host>]   # start server
aom serve stop                               # stop running server
aom serve restart [--port <port>] [--host <host>]  # stop then restart
```

### Flags

- `--port` — HTTP port to listen on (default: `7777`)
- `--host` — bind address (default: `localhost`)

### Behaviour

- Serves the embedded React frontend from `web/dist/` (built at compile time via `embed.FS`)
- Exposes the full REST API at `/api/v1/`
- Exposes three WebSocket endpoints: terminal streaming, event log, and per-agent mailbox
- No authentication — intended for local use only
- Cleans up stale aom-ws-* tmux sessions on startup
- Writes PID and bind address to `~/.aom/serve.pid`; `stop` and `restart` read this file to locate the process

### Example

```
aom serve
# → AOM Web Server listening on http://localhost:7777

aom serve stop
# → Server stopped (pid 12345)

aom serve restart --port 8080
# → AOM Web Server listening on http://localhost:8080
```

### REST API surface

All endpoints are prefixed `/api/v1/`.

**Projects**
```
GET    /projects                              list registered projects
POST   /projects                              add project  { path }
DELETE /projects/{id}                         remove project
POST   /projects/init                         init new project  { name, path, repo, agents[] }
```

**Agents**
```
GET    /projects/{id}/agents                  list agents
POST   /projects/{id}/agents                  add agent
PUT    /projects/{id}/agents/{name}           update agent (model, enabled)
DELETE /projects/{id}/agents/{name}           remove agent
POST   /projects/{id}/agents/{name}/provision provision workspace
```

**Sessions**
```
GET    /projects/{id}/sessions                list sessions
POST   /projects/{id}/sessions                spawn session  { agent, task_id, mode, persistent }
GET    /projects/{id}/sessions/{sid}          get session
DELETE /projects/{id}/sessions/{sid}          stop session
POST   /projects/{id}/sessions/{sid}/send     send message  { message, from }
POST   /projects/{id}/sessions/{sid}/resume
POST   /projects/{id}/sessions/{sid}/approve
POST   /projects/{id}/sessions/{sid}/deny
POST   /projects/{id}/sessions/{sid}/recover
POST   /projects/{id}/sessions/{sid}/archive
```

**Tasks**
```
GET    /projects/{id}/tasks                   list tasks
POST   /projects/{id}/tasks                   create task
GET    /projects/{id}/tasks/{tid}             get task
POST   /projects/{id}/tasks/{tid}/signal      send signal  { type, summary }
POST   /projects/{id}/tasks/{tid}/accept      accept task  { force }
POST   /projects/{id}/tasks/{tid}/close
POST   /projects/{id}/tasks/{tid}/cancel
GET    /projects/{id}/tasks/{tid}/artifact    read task.md / handoff.md / state.md
```

**Status & Communication**
```
GET    /projects/{id}/status                  dashboard summary (agents, counts, project_path)
GET    /projects/{id}/channel                 channel history
POST   /projects/{id}/channel                 post to channel
GET    /projects/{id}/mailbox/{agent}         agent mailbox history
POST   /projects/{id}/broadcast              broadcast to all sessions
POST   /projects/{id}/pause-all
POST   /projects/{id}/resume-all
```

**Roles & Classes**
```
GET    /projects/{id}/roles                   list roles
POST   /projects/{id}/roles                   create role  { name, class, worktree_mode, ... }
GET    /projects/{id}/roles/{name}            get role
PUT    /projects/{id}/roles/{name}            update role
DELETE /projects/{id}/roles/{name}            delete role (409 if agents using it)
GET    /projects/{id}/roles/{name}/preview    rendered profile preview (Zone A + Zone B)
GET    /projects/{id}/classes                 list classes with source (builtin/custom/overridden)
GET    /projects/{id}/classes/{name}          get class template content
PUT    /projects/{id}/classes/{name}          set/override class template  { content }
DELETE /projects/{id}/classes/{name}          delete custom class or revert builtin override
GET    /projects/{id}/classes/{name}/preview  rendered profile preview for this class
GET    /system-template                       AOM Workflow Protocol (Zone A, read-only)
```

**Extras**
```
GET    /projects/{id}/requests                list agent task requests
POST   /projects/{id}/requests/{rid}/approve
POST   /projects/{id}/requests/{rid}/reject
GET    /projects/{id}/metrics                 velocity report
POST   /projects/{id}/doctor                  run health checks
GET    /projects/{id}/team-brief              read .aom/team-brief.md
PUT    /projects/{id}/team-brief              write .aom/team-brief.md
POST   /projects/{id}/merge/check             { task_id }
POST   /projects/{id}/merge/prepare           { task_id }
POST   /projects/{id}/merge/commit            { task_id }
```

**Filesystem**
```
GET    /fs/browse                             browse directory  ?path=<dir>
POST   /fs/mkdir                              create directory  { path }
```

**Terminal**
```
GET    /terminal/{pane}/history               last N lines of pane output
```

### WebSocket API

```
WS /ws/terminal/{pane}
  Client → Server: raw keystroke bytes
  Server → Client: ANSI terminal output stream

WS /ws/events/{project}
  Server → Client: { type, timestamp, level, message }

WS /ws/mailbox/{project}/{agent}
  Server → Client: { type: "message", from, text, timestamp }
```

### Build Note

The frontend must be built before the Go binary is compiled — the React bundle is embedded at Go compile time:

```bash
cd web && npm run build
go build -o aom cmd/aom/main.go
```

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
