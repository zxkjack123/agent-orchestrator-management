# E2E Checklist: Milestone 1 Slice 4

## Purpose

This checklist verifies the first real end-to-end behavior in AOM:

- `aom project init`

At this stage, the goal is not full workflow execution. The goal is to confirm that AOM can initialize a local project foundation correctly.

## Scope

This checklist verifies:

- CLI command execution
- `.aom/` project structure creation
- baseline config generation
- SQLite DB creation
- safe rerun behavior

This checklist does not yet verify:

- `aom open`
- `aom status`
- tmux behavior
- task creation
- artifact generation

## Prerequisites

- Windows environment with Go installed at:
  - `C:\Program Files\Go\bin\go.exe`
- run from the repository root:
  - `C:\Users\lattapon.kea\Desktop\Agents-Orchestfator-Management`

## Recommended Command

Run with local cache paths to avoid user-profile permission issues:

```powershell
$env:GOTOOLCHAIN='local'
$env:GOCACHE="$PWD\.cache\gocache"
$env:GOMODCACHE="$PWD\.cache\gomodcache"
$env:GOTELEMETRY='off'
$env:GOTELEMETRYDIR="$PWD\.cache\gotelemetry"
& 'C:\Program Files\Go\bin\go.exe' run .\cmd\aom project init my-app --repo .
```

## Expected Result

The command should print a summary like:

- `Project initialized`
- project name
- repo path
- `.aom` path
- `sessions.db` path

## Files That Must Exist After Success

These files should exist:

```text
.aom/project.yaml
.aom/agents.yaml
.aom/resources.yaml
.aom/policy.yaml
.aom/sessions.db
```

## What To Inspect

### 1. project.yaml

Check:

- `name` matches the command input
- `repo` points to the repo path
- `default_branch` is `main`
- `runtime.terminal` is `tmux`
- `context.state_dir` is `.agent`

### 2. agents.yaml

Check:

- baseline roles exist
- baseline agents exist
- at least:
  - `orchestrator-main`
  - `backend-main`
  - `reviewer-main`

### 3. resources.yaml

Check:

- file exists
- file is valid YAML
- empty resource sets are allowed

### 4. policy.yaml

Check:

- deny commands exist
- approval-required actions exist
- approval scope is `per-session`
- YOLO mode is `disabled`

### 5. sessions.db

Check:

- file exists
- DB was created successfully

## Rerun Check

Run the same command again:

```powershell
$env:GOTOOLCHAIN='local'
$env:GOCACHE="$PWD\.cache\gocache"
$env:GOMODCACHE="$PWD\.cache\gomodcache"
$env:GOTELEMETRY='off'
$env:GOTELEMETRYDIR="$PWD\.cache\gotelemetry"
& 'C:\Program Files\Go\bin\go.exe' run .\cmd\aom project init my-app --repo .
```

Expected:

- command does not crash
- config files still exist
- DB still exists
- rerun remains safe for the current implementation slice

## Failure Signals

Treat these as bugs for the current slice:

- `.aom/` is not created
- any required config file is missing
- `sessions.db` is missing
- command prints success but files are incomplete
- rerun crashes
- generated YAML is malformed

## Current Known Limitation

This slice initializes the local project foundation only.

It does not yet prove:

- DB contents through `aom status`
- config-to-DB sync through `aom open`
- session/worktree runtime behavior

Those are part of the next slice.
