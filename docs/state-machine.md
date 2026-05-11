# AOM State Machine

## Purpose

This document defines the lifecycle model for AOM so task execution, session continuity, worktree ownership, approvals, and handoffs behave consistently across runtimes.

The state model covers:

- `Task`
- `Step`
- `Session`
- `Approval`
- `Handoff`
- `Worktree`

This document is scoped to Milestone 0. It defines the contracts that later implementation must follow.

## Core Principles

1. `Task` is the main operator-facing unit of work.
2. `Step` is the execution unit managed under a task.
3. `Session` is a replaceable runtime worker, not the primary source of truth.
4. `Worktree` is the durable execution boundary for active implementation work.
5. `Approval` is tracked per session.
6. `Handoff` is explicit and artifact-driven.
7. If state conflicts exist, prefer `AOM state + markdown artifacts` over vendor chat memory.
8. `NeedsAttention` is a real state gate, not a cosmetic label.

## State Ownership Model

AOM manages continuity through three durable anchors:

- `Task/Step state`
- `Worktree state`
- `Operational memory artifacts`

Live sessions are important, but replaceable.

This leads to four rules:

1. A task may outlive many sessions.
2. A session may be replaced without replacing the task.
3. A session may be replaced without replacing the worktree.
4. Work must continue from artifacts and worktree state, not from transcript memory alone.

## Task States

### States

- `Draft`
- `Planned`
- `Ready`
- `InProgress`
- `Blocked`
- `NeedsAttention`
- `Done`
- `Archived`

### Meaning

- `Draft`
  - Task exists, but no confirmed execution plan is ready yet.
- `Planned`
  - Task mode and step plan are defined.
- `Ready`
  - Task is ready for execution.
- `InProgress`
  - At least one active step is underway.
- `Blocked`
  - The task cannot continue because of a known dependency, resource, approval, or continuity issue.
- `NeedsAttention`
  - The task cannot safely continue without an operator decision.
- `Done`
  - The operator has explicitly closed the task.
- `Archived`
  - The task has been moved out of the active workflow.

### Allowed Transitions

- `Draft -> Planned`
- `Draft -> Archived`
- `Planned -> Ready`
- `Planned -> NeedsAttention`
- `Ready -> InProgress`
- `Ready -> Archived`
- `InProgress -> Blocked`
- `InProgress -> NeedsAttention`
- `InProgress -> Ready`
- `InProgress -> Done`
- `Blocked -> Ready`
- `Blocked -> NeedsAttention`
- `NeedsAttention -> Planned`
- `NeedsAttention -> Ready`
- `NeedsAttention -> InProgress`
- `NeedsAttention -> Done`
- `Done -> Archived`

### Rules

- `Done` requires explicit operator closure.
- A task must not move directly from `Draft` to `Done`.
- `Blocked` is used when the system knows what is preventing continuation.
- `NeedsAttention` is used when the operator must make a decision before work continues.

## Step States

### States

- `Proposed`
- `Confirmed`
- `Ready`
- `InProgress`
- `Blocked`
- `NeedsAttention`
- `Completed`
- `Skipped`
- `Canceled`

### Meaning

- `Proposed`
  - The orchestrator suggested the step, but the operator has not confirmed it yet.
- `Confirmed`
  - The step is accepted into the workflow.
- `Ready`
  - The step has enough assignment and dependency resolution to start.
- `InProgress`
  - An owner session is actively working on the step.
- `Blocked`
  - The step cannot proceed because of a known dependency, resource, approval, or session issue.
- `NeedsAttention`
  - The step requires an operator decision or re-analysis before continuing.
- `Completed`
  - The step has met its completion contract.
- `Skipped`
  - The step was intentionally not executed.
- `Canceled`
  - The step is no longer part of the current task plan.

### Allowed Transitions

- `Proposed -> Confirmed`
- `Proposed -> Skipped`
- `Proposed -> Canceled`
- `Confirmed -> Ready`
- `Confirmed -> Canceled`
- `Ready -> InProgress`
- `Ready -> Skipped`
- `Ready -> Canceled`
- `InProgress -> Blocked`
- `InProgress -> NeedsAttention`
- `InProgress -> Completed`
- `InProgress -> Ready`
- `Blocked -> Ready`
- `Blocked -> NeedsAttention`
- `NeedsAttention -> Ready`
- `NeedsAttention -> InProgress`
- `NeedsAttention -> Canceled`

### Rules

- A step must have an assigned role or agent before entering `Ready`.
- A step should have only one active owner at a time.
- Parallel steps are allowed when dependencies permit them.
- `Completed` means the step-specific contract is satisfied, not just that the session stopped.

## Session States

### States

- `Created`
- `Booting`
- `Idle`
- `Working`
- `WaitingApproval`
- `WaitingHandoff`
- `Blocked`
- `Detached`
- `Failed`
- `Stopped`
- `Archived`

### Meaning

- `Created`
  - AOM created the session record, but the runtime has not started yet.
- `Booting`
  - AOM is starting the runtime, binding the worktree, and establishing terminal control.
- `Idle`
  - The runtime is available and not currently executing active work.
- `Working`
  - The runtime is actively executing work.
- `WaitingApproval`
  - The session is paused pending operator approval.
- `WaitingHandoff`
  - The session finished its current execution segment and is waiting for checkpoint or owner transfer.
- `Blocked`
  - The session is alive but cannot continue because of a known issue.
- `Detached`
  - The AOM session record still exists, but the live terminal or runtime binding is incomplete or missing.
- `Failed`
  - The session cannot be trusted to continue, or continuity cannot be safely recovered from it.
- `Stopped`
  - The session was intentionally stopped.
- `Archived`
  - The session has been removed from the active system.

### Allowed Transitions

- `Created -> Booting`
- `Created -> Archived`
- `Booting -> Idle`
- `Booting -> Detached`
- `Booting -> Failed`
- `Idle -> Working`
- `Idle -> Detached`
- `Idle -> Stopped`
- `Working -> Idle`
- `Working -> WaitingApproval`
- `Working -> WaitingHandoff`
- `Working -> Blocked`
- `Working -> Detached`
- `Working -> Failed`
- `WaitingApproval -> Working`
- `WaitingApproval -> Blocked`
- `WaitingApproval -> Detached`
- `WaitingApproval -> Failed`
- `WaitingHandoff -> Idle`
- `WaitingHandoff -> Detached`
- `WaitingHandoff -> Stopped`
- `Blocked -> Idle`
- `Blocked -> Detached`
- `Blocked -> Failed`
- `Detached -> Idle`
- `Detached -> Failed`
- `Detached -> Stopped`
- `Failed -> Archived`
- `Stopped -> Archived`

### Rules

- A session is not the primary system of record.
- Resume is allowed, but resumed sessions must be reconciled against artifacts and AOM state.
- `Detached` is used when the session identity still exists but the live tmux or runtime binding is incomplete.
- `Failed` is used when continuity is not trustworthy enough to keep using the same session.
- `Stopped` is for intentional shutdown, not accidental loss.

## Approval States

### States

- `NotRequired`
- `Pending`
- `Approved`
- `Denied`
- `Expired`
- `Bypassed`

### Meaning

- `NotRequired`
  - No approval is needed for the current action.
- `Pending`
  - The session is waiting for an operator decision.
- `Approved`
  - The operator approved the request.
- `Denied`
  - The operator denied the request.
- `Expired`
  - The approval context is no longer valid.
- `Bypassed`
  - Approval was intentionally bypassed under a valid exception such as session-scoped YOLO mode.

### Allowed Transitions

- `NotRequired -> Pending`
- `NotRequired -> Bypassed`
- `Pending -> Approved`
- `Pending -> Denied`
- `Pending -> Expired`
- `Approved -> Pending`
- `Denied -> Pending`

### Rules

- Approval is tracked per session.
- `Bypassed` must be explainable and attributable.
- Critical approval conflicts may force the related step or task into `NeedsAttention`.

## Handoff States

### States

- `NotNeeded`
- `Preparing`
- `Ready`
- `Accepted`
- `Rejected`
- `Superseded`

### Meaning

- `NotNeeded`
  - No owner transfer is needed yet.
- `Preparing`
  - The current owner is preparing handoff artifacts.
- `Ready`
  - The handoff packet is ready for the next owner.
- `Accepted`
  - The next owner accepted the handoff and can continue.
- `Rejected`
  - The handoff packet is insufficient or incorrect.
- `Superseded`
  - A newer handoff replaces the old one.

### Allowed Transitions

- `NotNeeded -> Preparing`
- `Preparing -> Ready`
- `Ready -> Accepted`
- `Ready -> Rejected`
- `Ready -> Superseded`
- `Rejected -> Preparing`
- `Accepted -> Superseded`

### Rules

- Handoff should be created whenever owner responsibility changes.
- Handoff must be based on artifacts and current worktree state, not transcript memory alone.
- If handoff is rejected, the related step or task should normally move to `NeedsAttention`.

## Worktree States

### States

- `Planned`
- `Provisioning`
- `Ready`
- `Active`
- `NeedsRepair`
- `Archived`

### Meaning

- `Planned`
  - The task exists, but AOM has not created the worktree yet.
- `Provisioning`
  - AOM is creating or preparing the worktree and its task artifacts.
- `Ready`
  - The worktree exists and is ready for a session to bind to it.
- `Active`
  - At least one active step or session is using the worktree.
- `NeedsRepair`
  - The worktree mapping, path, branch, or required artifacts are incomplete or unreliable.
- `Archived`
  - The worktree is no longer part of active execution.

### Allowed Transitions

- `Planned -> Provisioning`
- `Planned -> Archived`
- `Provisioning -> Ready`
- `Provisioning -> NeedsRepair`
- `Ready -> Active`
- `Ready -> NeedsRepair`
- `Active -> Ready`
- `Active -> NeedsRepair`
- `NeedsRepair -> Ready`
- `NeedsRepair -> Archived`
- `Ready -> Archived`

### Rules

- AOM owns the worktree lifecycle.
- AOM must persist the task-to-worktree mapping as system state.
- A session may be replaced without replacing the worktree.
- The system must not rely on a live session to discover the current worktree.
- If worktree continuity is not trustworthy, the task must not silently continue.

## NeedsAttention Triggers

### Task enters `NeedsAttention` when

- a required step fails
- a critical approval requires operator intervention
- unresolved review or handoff issues block safe continuation
- session continuity is not trustworthy
- manual intervention invalidates the previous plan
- task mode should change but has not been confirmed
- worktree continuity is not trustworthy

### Step enters `NeedsAttention` when

- the output is insufficient to close the step
- a handoff is rejected
- review notes are incomplete or contradictory
- manual intervention invalidates the previous plan
- the session or worktree can no longer be trusted to continue safely

## Session Identity and Continuity Model

AOM should treat session continuity as a durable system concern, not a tmux detail.

Each execution context has three layers:

### 1. AOM Session Record

This is the canonical session identity stored by AOM.

Examples:

- `aom_session_id`
- `task_id`
- `step_id`
- `agent_id`
- `runtime`
- `worktree_path`
- `vendor_session_id`
- `tmux_session_name`
- `tmux_window`
- `tmux_pane`
- `status`
- `last_seen_at`
- `recovery_hint`

### 2. Live Terminal Binding

This is the current tmux and runtime attachment state.

Examples:

- tmux session name
- pane id
- attach target
- live runtime binding

This layer is ephemeral and may disappear after reboot, tmux exit, or runtime failure.

### 3. Runtime Continuity Hint

This is the stored evidence AOM uses to decide whether to resume or replace a session.

Examples:

- latest checkpoint
- latest artifact sync
- resume capability
- context freshness
- last handoff status

## Session Reconciliation After Restart

AOM must treat restart, reboot, or tmux loss as normal operational events.

On `aom open`, AOM should reconcile persisted sessions by checking:

- whether the worktree still exists
- whether required artifacts still exist
- whether the tmux target still exists
- whether the runtime can be resumed
- whether continuity is trustworthy enough to reuse the session

The result should drive a recovery recommendation such as:

- reuse existing live session
- recreate tmux binding
- resume runtime in the same worktree
- spawn replacement session from artifacts
- archive the old session and continue with a new one

Live tmux bindings are ephemeral. AOM session records are durable.

## Tmux and Live Binding Loss

If AOM cannot find the expected tmux session or pane:

1. Keep the AOM session record.
2. Verify the worktree still exists.
3. Verify whether vendor runtime continuity is still possible.
4. Mark the session `Detached` if continuity may still be recoverable.
5. Mark the session `Failed` if continuity is not trustworthy.
6. Mark the related step or task `NeedsAttention` when operator choice is required.

The orchestrator should recommend recovery actions rather than silently choosing one.

## Session Replacement Continuity

Session replacement is a normal flow in AOM.

Replacement may happen because of:

- session loss
- provider limit reached
- runtime failure
- operator reassignment
- orchestrator recommendation
- provider switch for continuity

Replacement types include:

- same runtime replacement
- same role with different provider
- different role reassignment through handoff

### Continuity Packet

When replacing a session, AOM should prepare a continuity packet that is sufficient for a new session to continue work.

The packet should include:

- task identity
- step identity
- task mode
- worktree path
- allowed scope and constraints
- latest `state.md`
- latest `handoff.md` if present
- unresolved `review-notes.md`
- planning artifacts if relevant
- current diff summary
- changed files summary
- latest checkpoint summary
- replacement reason

### Replacement Rules

- A session may be replaced without replacing the task.
- A session may be replaced without replacing the worktree.
- A replacement session must start from AOM artifacts and worktree state, not transcript memory alone.
- Cross-provider replacement is allowed when the role contract remains valid.
- If continuity is insufficient, the task or step must remain `NeedsAttention`.

## Manual Intervention Re-analysis

Manual operator intervention is allowed and expected.

When the operator intervenes inside a live session:

1. AOM must record the intervention in `log.md`.
2. AOM should refresh `index.md`.
3. The previous workflow assumption should no longer be treated as fully current.
4. When the operator returns to the orchestrator, AOM should re-analyze the task before recommending next steps.

Manual intervention should not automatically fail a session, but it should trigger re-analysis.

## Minimal Lifecycle Examples

### Small Direct Task

- Task: `Draft -> Planned -> Ready -> InProgress -> Done`
- Step: `Proposed -> Confirmed -> Ready -> InProgress -> Completed`
- Worktree: `Planned -> Provisioning -> Ready -> Active`
- Session: `Created -> Booting -> Idle -> Working -> WaitingHandoff -> Idle`

### Review Finds Issues

- Task: `InProgress -> NeedsAttention -> Ready -> InProgress`
- Review step: `InProgress -> Completed`
- Fix step: `Ready -> InProgress -> Completed`
- Handoff: `Preparing -> Ready -> Accepted`

### Tmux Binding Disappears

- Session: `Working -> Detached`
- Step: `InProgress -> NeedsAttention`
- Task: `InProgress -> NeedsAttention`
- Operator chooses:
  - restore binding
  - resume runtime
  - spawn replacement session from artifacts

If recovery succeeds:

- Session: `Detached -> Idle -> Working`

If recovery fails:

- Session: `Detached -> Failed`

### Provider Limit Forces Session Replacement

- Original session: `Working -> Needs replacement trigger`
- Session state: `Working -> Detached` or `Working -> Failed`
- Step: `InProgress -> NeedsAttention`
- Task: `InProgress -> NeedsAttention`
- Orchestrator prepares continuity packet
- Operator confirms replacement
- New session: `Created -> Booting -> Idle -> Working`
- Worktree remains `Active`

## Simplification Rules for Milestone 0

To keep the first implementation constrained:

- use `NeedsAttention` only as a task and step state, not as a special session or handoff state
- keep recovery recommendations outside the core state machine
- keep worktree state smaller than task and session state
- keep replacement logic artifact-driven rather than vendor-specific in the core contract

## Acceptance Intent

This state model is complete enough for Milestone 0 if:

- one complete task lifecycle can be described without ambiguity
- one complete session replacement flow can be described without ambiguity
- worktree continuity remains valid even when tmux or runtime continuity is lost
- the operator remains the final decision point for closure, recovery, replacement, and workflow change
