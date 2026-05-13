# Experimental Agent E2E Plan

## Purpose

This document defines the next narrow slice for connecting a real agent runtime to AOM before formal runtime adapters are implemented.

The goal is to improve end-to-end workflow testing without prematurely claiming Milestone 10 runtime support.

## Current Context

The repository already has:
- tmux-backed session orchestration
- task and step workflow primitives
- task-local artifact continuity
- planned and provisioned worktree continuity
- session replacement, stop, archive, and repair flows

What it does not have yet:
- real provider-native runtime launch
- provider-native resume
- provider-specific adapter contracts

At the current handoff point, `session spawn` is still placeholder-based or mock-based.

## Why This Slice Now

The current control plane is mature enough that placeholder sessions are now the main gap for realistic E2E validation.

A narrow experimental launch path would let us validate:
- live agent session startup in a real pane
- task-bound continuity with a real agent process
- attach and capture behavior during actual agent execution
- replacement and recovery behavior with a non-placeholder runtime

This gives better product feedback before formal adapter work begins.

## Milestone Fit

This slice is still pre-Milestone-10 work.

It should be treated as:
- a Milestone 4 and 5 support slice for better system validation
- not a completed runtime adapter
- not a new cross-runtime abstraction milestone

## Scope

Implement only a narrow opt-in experimental launch path for one runtime first.

Recommended first runtime:
- `codex`

Recommended command surface:
- `aom session spawn <agent> --real`
- `aom session replace <session-id> --agent <agent> --real`

Recommended behavior:
- `--mock` remains available
- default launch remains placeholder-based
- `--real` is explicit and opt-in
- only supported runtimes succeed
- unsupported runtimes fail clearly

Recommended initial semantics:
- start the runtime in the target repo or task worktree
- keep the pane fully interactive
- do not attempt provider-native resume yet
- do not inject deep structured prompts yet

## Explicit Non-Goals

Do not do any of the following in this slice:
- no full runtime adapter framework
- no provider-native resume protocol
- no structured output parsing
- no multi-provider support in the first implementation pass
- no hidden background orchestration beyond the current tmux model
- no contract claims that Codex, Claude, or Kiro are formally integrated

## Proposed Implementation Slice

### 1. Lock a narrow launch contract

Add a minimal runtime launch builder that decides between:
- placeholder
- mock
- experimental real

Keep it explicit and concrete.

Recommended package:
- `internal/runtime`

This package should only own:
- runtime-specific launch command construction
- launch mode validation for the current slice

It should not yet own:
- adapter registries
- provider capability negotiation
- resume logic

### 2. Support one runtime first

Start with `codex` only.

Reason:
- it is already a project runtime in default templates
- it is the most direct path for validating builder-style task execution
- it avoids broadening the slice into adapter design too early

### 3. Keep launch opt-in

Do not change the default behavior of `session spawn`.

Default should remain:
- placeholder launch for normal flow

Opt-in path:
- `--real` launches the real runtime

This preserves current repo stability while allowing stronger E2E checks where the environment supports it.

### 4. Cover failure behavior clearly

If `--real` is requested for an unsupported runtime:
- fail clearly before pane creation

If pane creation or pane annotation fails:
- preserve the current session failure behavior
- append canonical task log events exactly as current task-bound spawn does

### 5. Keep artifacts and worktree behavior unchanged

The experimental launch path should reuse the existing:
- task binding
- artifact refresh
- worktree resolution
- session replacement
- repair behavior

This slice should only change how the pane process is launched.

## Suggested CLI Behavior

### `aom session spawn backend-main --real`

Expected effect:
- create the durable session record
- resolve task worktree if one is bound
- create the tmux pane
- launch the real runtime interactively in the pane
- keep current artifact and lifecycle logging behavior

### `aom session spawn reviewer-main --real`

Expected effect in the first slice:
- fail clearly if `reviewer-main` uses an unsupported runtime such as `claude`

### `aom session replace <old> --agent backend-main --real`

Expected effect:
- replacement session uses the same task and worktree continuity path
- launch mode is real for the replacement session
- old-session handling remains governed by current replace logic

## Verification Plan

## Local code verification

On any machine:
- add focused unit tests around launch-mode selection
- add CLI tests for `--real` flag parsing and unsupported-runtime errors
- run `go test ./...`

## Live E2E verification

Prefer macOS or Linux with:
- working `tmux`
- working `git`
- accessible `codex` CLI in PATH

Recommended smoke flow:

1. `aom project init`
2. `aom open`
3. `aom plan "real runtime smoke test" --create`
4. `aom session spawn backend-main --task <task-id> --real`
5. `aom capture <session-id>`
6. `aom attach <session-id>`
7. verify task artifacts under the task path or worktree `.agent/`
8. `aom session replace <session-id> --agent backend-main --reason "continuity smoke" --real`
9. `aom session stop <session-id>` where appropriate
10. `aom session archive <session-id>` where appropriate

Additional repair flow:

1. create a task in a git-backed repo
2. force a worktree drift scenario
3. run `aom status`
4. run `aom worktree repair <task-id>`
5. spawn a real session again with `--real`

## Environment Constraints

This slice should not be considered fully verified from the current Windows machine.

Current Windows limitation:
- live tmux E2E has not been working reliably in this environment
- `wsl.exe` and `tmux` path assumptions were not previously usable here

Recommended verification host:
- macOS
- Linux
- or Windows with known-good WSL plus tmux and the target runtime installed

## Expected Outcome

After this slice:
- AOM still remains milestone-aligned
- placeholder mode still exists as the safe default
- one real runtime can be used for stronger E2E testing
- task, artifact, worktree, and recovery flows can be tested against a real agent process

After this slice, what is still intentionally not done:
- formal multi-runtime support
- provider-native resume
- structured runtime telemetry
- official Milestone 10 adapter completion

## Recommended Follow-Up

If the experimental slice proves useful:

1. collect real E2E findings from Codex launch behavior
2. close any continuity gaps found in Milestone 4 and 5
3. use those findings to shape Milestone 10 adapter contracts

If the experimental slice proves unstable:

1. keep placeholder and mock flows as the official verification path
2. record launch constraints in `current-status`
3. defer real runtime integration until Milestone 10
