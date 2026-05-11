# AOM Artifact Schemas

## Purpose

This document defines the markdown artifact contracts under `.agent/` so AOM can use them as the operational memory layer for continuity, handoff, recovery, and cross-provider session replacement.

The goals are:

- a new session can continue from artifacts without relying on transcript memory alone
- provider replacement remains possible with enough portable context
- orchestrator re-analysis is grounded in durable artifacts
- ownership and canonicality remain clear

## General Rules

### Artifact Scope

All task artifacts are task-local and live inside the task worktree:

```text
<worktree>/.agent/
```

### Ownership Model

#### AOM-owned canonical files

- `index.md`
- `log.md`

#### Agent-updated files under AOM protocol

- `task.md`
- `state.md`
- `handoff.md`
- `review-notes.md`
- `requirements.md`
- `design.md`
- `tasks.md`

### Format Rules

- Prefer human-readable markdown first.
- Use predictable headings and stable section names.
- Bullets, checklists, and simple tables are allowed.
- Avoid unstructured freeform documents with no recognizable sections.
- Important metadata should live in explicit sections, not only in prose.

### Canonicality Rules

- If `log.md` conflicts with transcript memory, prefer `log.md`.
- If `index.md` conflicts with a live session view, AOM must refresh `index.md`.
- If `state.md` is stale, continuity quality should be treated as degraded.
- Session resume must not rely on vendor chat history alone.

## Required Artifact Set

Always required for active tasks:

- `.agent/task.md`
- `.agent/state.md`
- `.agent/index.md`
- `.agent/log.md`

Mode-dependent or workflow-dependent:

- `.agent/handoff.md`
- `.agent/review-notes.md`
- `.agent/requirements.md`
- `.agent/design.md`
- `.agent/tasks.md`

## task.md

### Purpose

Defines what the task is, what scope it covers, and what success means.

### Ownership

- readable by agents and orchestrator
- initially seeded by AOM
- mostly stable after creation

### Required Fields

- `Task ID`
- `Title`
- `Task Mode`
- `Status`
- `Created By`
- `Assigned Role`
- `Assigned Agent` (optional)
- `Worktree`
- `Goal`
- `Scope`
- `Out of Scope`
- `Constraints`
- `Success Criteria`

### Suggested Structure

```md
# Task

## Identity
- Task ID: TASK-001
- Title: Fix login validation
- Task Mode: Direct
- Status: Ready
- Created By: operator
- Assigned Role: backend
- Assigned Agent: claude-backend
- Worktree: worktrees/TASK-001-login-validation

## Goal
Fix the login validation behavior for invalid email format.

## Scope
- Update request validation
- Preserve current API contract where possible

## Out of Scope
- No DB schema changes
- No frontend changes

## Constraints
- Stay within auth module
- Follow existing error response style

## Success Criteria
- Invalid email is rejected
- Existing valid login flow still passes
- Relevant tests pass
```

### Rules

- `task.md` should remain mostly stable, not become a scratchpad.
- If the task mode changes, AOM should update task metadata and record the change in `log.md`.
- If scope changes materially, the change should be recorded in `log.md`.

## state.md

### Purpose

Stores the current working memory for the active owner and must be sufficient for respawn, resume, and replacement.

### Ownership

- updated by the active agent under AOM protocol
- validated by AOM for presence and freshness
- should have one active owner at a time

### Required Fields

- `Status`
- `Current Owner`
- `Current Runtime`
- `Current Session`
- `Current Step`
- `Goal`
- `Completed Work`
- `Remaining Work`
- `Touched Files`
- `Constraints`
- `Open Questions`
- `Next Action`
- `Last Updated By`

### Suggested Structure

```md
# Current State

## Status
- Task Status: InProgress
- Step Status: InProgress

## Ownership
- Current Owner: backend
- Current Runtime: claude
- Current Session: SESS-001
- Current Step: STEP-002 implementation

## Goal
Fix login validation behavior without changing API shape.

## Completed Work
- Identified validation entry point
- Updated request parsing
- Added initial validation branch

## Remaining Work
- Align error payload with existing format
- Run focused tests
- Prepare checkpoint

## Touched Files
- internal/auth/handler.go
- internal/auth/validation.go

## Constraints
- Do not change DB schema
- Preserve response envelope format

## Open Questions
- Should invalid email return 400 or the current domain-specific error code?

## Next Action
Run auth handler tests, then adjust error format if needed.

## Last Updated By
- Agent: claude-backend
- Session: SESS-001
```

### Rules

- `state.md` should answer where the work stands right now.
- It must not replace `log.md` as the historical timeline.
- Replacement sessions should read this file first.
- When ownership changes, `state.md` should be refreshed before continuation.

## index.md

### Purpose

Acts as the task-local manifest and current control summary for the task.

### Ownership

- AOM-owned canonical file

### Required Fields

- `Task Identity`
- `Task Mode`
- `Task Status`
- `Active Step`
- `Assigned Role/Agent`
- `Active Session`
- `Worktree Status`
- `Artifact Inventory`
- `Latest Checkpoint`
- `Unresolved Review Count`
- `Pending Approvals`
- `Continuity Readiness`
- `Recommended Next Action`

### Suggested Structure

```md
# Task Index

## Identity
- Task ID: TASK-001
- Title: Fix login validation
- Mode: Direct
- Status: InProgress

## Active Control
- Active Step: STEP-002 implementation
- Assigned Role: backend
- Assigned Agent: claude-backend
- Active Session: SESS-001
- Runtime: claude
- Worktree Status: Active
- Continuity Readiness: Good

## Artifacts
- task.md: present
- state.md: present
- log.md: present
- handoff.md: absent
- review-notes.md: absent
- requirements.md: n/a
- design.md: n/a
- tasks.md: n/a

## Checkpoint
- Latest Checkpoint: CHK-003
- Last Checkpoint At: 2026-05-11T10:30:00+07:00

## Attention
- Unresolved Review Items: 0
- Pending Approvals: 0
- Session Recovery Status: Live

## Recommended Next Action
Run focused tests and prepare checkpoint.
```

### Rules

- AOM refreshes this file after meaningful orchestration events.
- Agents may read it but should not own it.
- Manual intervention should cause AOM to refresh this file.
- This file is the primary entry point for re-analysis.

## log.md

### Purpose

Serves as the append-only canonical task timeline.

### Ownership

- AOM-owned canonical file

### Required Entry Fields

Each event should contain at least:

- timestamp
- event id
- event type
- actor
- related task, step, or session
- summary
- outcome or state effect

### Suggested Structure

```md
# Task Log

## Events

### 2026-05-11T09:00:00+07:00 | EVT-001 | task.created
- Actor: orchestrator
- Task: TASK-001
- Summary: Task created in Direct mode
- State Effect: Task Draft

### 2026-05-11T09:10:00+07:00 | EVT-002 | worktree.provisioned
- Actor: aom
- Task: TASK-001
- Worktree: worktrees/TASK-001-login-validation
- Summary: Worktree created and base artifacts seeded
- State Effect: Worktree Ready

### 2026-05-11T09:30:00+07:00 | EVT-003 | operator.intervention
- Actor: operator
- Session: SESS-001
- Summary: Operator entered specialist session and redirected scope to preserve API error format
- State Effect: Re-analysis required
```

### Recommended Event Types

- `task.created`
- `task.mode_changed`
- `task.closed`
- `step.proposed`
- `step.confirmed`
- `step.completed`
- `session.created`
- `session.detached`
- `session.failed`
- `session.replaced`
- `worktree.provisioned`
- `worktree.needs_repair`
- `approval.pending`
- `approval.approved`
- `approval.denied`
- `checkpoint.created`
- `handoff.prepared`
- `handoff.accepted`
- `operator.intervention`
- `reanalysis.completed`

### Rules

- `log.md` is append-only.
- AOM writes the canonical timeline.
- Corrections should be recorded as new events instead of rewriting history.
- Replacement, recovery, and manual intervention must always be logged.

## handoff.md

### Purpose

Acts as the explicit transfer packet between owners.

### Ownership

- updated by agents under AOM protocol
- AOM may seed the template and validate completeness

### Required Fields

- `From Role/Agent/Session`
- `From Runtime`
- `To Role`
- `Suggested Runtime` (optional)
- `Task`
- `Step`
- `Replacement Reason` or `Handoff Reason`
- `What Was Completed`
- `What Remains`
- `Touched Files`
- `Constraints`
- `Warnings`
- `Exact Next Action`
- `Do Not Redo`

### Suggested Structure

```md
# Handoff

## Transfer
- From Role: backend
- From Agent: claude-backend
- From Session: SESS-001
- From Runtime: claude
- To Role: reviewer
- Suggested Runtime: claude
- Task: TASK-001
- Step: STEP-003 review
- Reason: Implementation checkpoint complete, ready for review

## Completed
- Updated login validation logic
- Preserved response envelope
- Added focused handler tests

## Remaining
- Review diff for response consistency
- Check for scope creep

## Touched Files
- internal/auth/handler.go
- internal/auth/validation.go
- internal/auth/handler_test.go

## Constraints
- No DB schema changes
- No frontend changes

## Warnings
- Existing validation helper is shared with registration flow

## Exact Next Action
Review the current diff and record findings in review-notes.md.

## Do Not Redo
- Do not re-implement validation logic from scratch
- Do not broaden scope into registration flow
```

### Rules

- A handoff should exist whenever ownership changes.
- Cross-provider replacement should still use this file shape.
- If handoff is rejected, the event should be logged and reflected in task or step state.

## review-notes.md

### Purpose

Stores structured review findings so a builder or fixer can act on them precisely.

### Ownership

- usually written by a reviewer agent
- validated by AOM for structure and unresolved counts

### Required Fields Per Item

- `Item ID`
- `Severity`
- `Path`
- `Issue`
- `Expected Fix`
- `Status`
- `Owner`

### Suggested Structure

```md
# Review Notes

## Summary
- Review Step: STEP-003
- Reviewer: claude-reviewer
- Session: SESS-004
- Status: Needs fixes

## Items

### RVW-001
- Severity: high
- Path: internal/auth/handler.go
- Issue: Invalid email path returns inconsistent error payload compared with existing auth errors
- Expected Fix: Use the existing response envelope helper
- Status: open
- Owner: backend

### RVW-002
- Severity: medium
- Path: internal/auth/handler_test.go
- Issue: Missing test for malformed email with whitespace
- Expected Fix: Add a focused negative test
- Status: open
- Owner: backend
```

### Rules

- Review item IDs should remain stable.
- Fix sessions should refer to review item IDs directly.
- AOM uses this file to count unresolved review items.
- Avoid long freeform prose that cannot be reused operationally.

## requirements.md

### Purpose

Used when the selected task mode requires structured requirements.

### Ownership

- typically orchestrator-planned, then updated by the active planning or implementation owner

### Required Fields

- `Problem`
- `Users/Actors`
- `Requirements`
- `Acceptance Criteria`
- `Non-Goals`

### Suggested Structure

```md
# Requirements

## Problem
Users receive unclear validation feedback during login.

## Users / Actors
- API client
- backend auth service

## Requirements
1. System must reject malformed email input before auth processing.
2. System must return the existing auth error envelope.
3. System must preserve current valid login behavior.

## Acceptance Criteria
- malformed emails return expected error payload
- valid login flow is unchanged

## Non-Goals
- registration flow changes
- schema changes
```

## design.md

### Purpose

Used when the selected task mode requires a technical approach or design contract.

### Ownership

- usually updated by implementation, architect, or planning-oriented agents

### Required Fields

- `Context`
- `Current Behavior`
- `Proposed Change`
- `Files/Modules Affected`
- `Risks`
- `Validation Plan`

### Suggested Structure

```md
# Design

## Context
Login validation currently mixes parsing and domain error shaping.

## Current Behavior
Malformed email reaches deeper auth logic and returns inconsistent payload.

## Proposed Change
Add early validation in handler layer and route malformed email through existing error envelope helper.

## Files / Modules Affected
- internal/auth/handler.go
- internal/auth/validation.go
- internal/auth/handler_test.go

## Risks
- shared validation helper may affect registration flow

## Validation Plan
- run focused auth handler tests
- inspect diff for envelope consistency
```

## tasks.md

### Purpose

Acts as the structured step plan for planning-heavy task modes.

### Ownership

- usually orchestrator-planned
- may be updated as the workflow evolves

### Required Fields Per Step

- `Step ID`
- `Type`
- `Title`
- `Owner Role`
- `Dependencies`
- `Status`
- `Notes`

### Suggested Structure

```md
# Tasks

## Steps

### STEP-001
- Type: requirements
- Title: Define expected login validation behavior
- Owner Role: orchestrator
- Dependencies: none
- Status: completed
- Notes: Direct mode upgraded to Requirements-first

### STEP-002
- Type: implementation
- Title: Update handler validation logic
- Owner Role: backend
- Dependencies: STEP-001
- Status: in_progress
- Notes: Preserve existing response envelope

### STEP-003
- Type: review
- Title: Review validation diff
- Owner Role: reviewer
- Dependencies: STEP-002
- Status: ready
- Notes: Focus on response consistency and scope
```

### Rules

- Step status should align with the step state machine.
- Dependencies should remain explicit and simple.
- This file is the source of planned execution steps for structured modes.

## Freshness and Continuity Rules

AOM should assess artifact continuity quality using at least:

- presence of required files
- presence of required sections
- recency of updates
- owner and session consistency
- consistency with database, worktree, and session state

### Suggested Continuity Readiness Levels

- `Good`
  - artifacts are complete and current
- `Usable`
  - continuity is good enough for replacement, with minor gaps
- `Weak`
  - continuity is possible, but requires operator review
- `Insufficient`
  - continuity is not safe enough; task or step should remain in `NeedsAttention`

These levels should usually be surfaced through `index.md`.

## Artifact Lifecycle Rules

### On Task Creation

Create:

- `task.md`
- `state.md`
- `index.md`
- `log.md`

### On Structured Mode Upgrade

Create additional planning artifacts as needed:

- `requirements.md`
- `design.md`
- `tasks.md`

### On Owner Change

Create or refresh:

- `handoff.md`
- `state.md`
- `index.md`
- `log.md`

### On Review

Create or refresh:

- `review-notes.md`
- `index.md`
- `log.md`

### On Manual Intervention

Refresh:

- `index.md`
- `log.md`
- optionally `state.md` after re-analysis

### On Session Replacement

Create or refresh:

- `handoff.md` or equivalent replacement packet
- `state.md`
- `index.md`
- `log.md`

## Locked Decisions for Milestone 0

The following decisions are intentionally locked for the first implementation slice:

- `task.md` is mostly stable after creation
- `state.md` uses fixed headings with flexible content under those headings
- `review-notes.md` uses stable bullet-per-item structure instead of a rigid table
- `log.md` stays as a timeline only; `index.md` carries current summary state
