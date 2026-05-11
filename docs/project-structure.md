# AOM Project Structure

## Purpose

This document defines the intended codebase structure for AOM before implementation begins.

The goal is to keep the repository:

- easy to read
- easy to extend
- easy to test
- easy to maintain
- resistant to early architectural drift

This structure is intentionally conservative. It favors explicit package boundaries over clever reuse.

## Official Go Basis

This structure follows official Go guidance first, then adds a small number of AOM-specific project rules.

Primary references:

- [Organizing a Go module](https://go.dev/doc/modules/layout)
- [How to Write Go Code](https://go.dev/doc/code)
- [Effective Go](https://go.dev/doc/effective_go)
- [Package names](https://go.dev/blog/package-names)

Interpretation for AOM:

- `cmd/` is a convention, not a language requirement, but it is a good fit for a mixed repository containing one command and supporting internal packages.
- `internal/` is the preferred place for supporting packages that should not be imported by external modules.
- package boundaries should follow responsibility, not artificial layering for its own sake.

## Design Principles

1. Keep top-level responsibilities obvious.
2. Keep domain logic out of CLI wiring.
3. Keep persistence logic out of domain rules.
4. Avoid generic frameworks and abstraction-first design.
5. Prefer small, explicit packages with clear ownership.

## Top-Level Layout

The intended repository layout is:

```text
cmd/
  aom/
    main.go

internal/
  app/
  cli/
  config/
  db/
  project/
  agent/
  task/
  step/
  session/
  worktree/
  artifact/
  audit/
  runtime/
  tmux/

docs/
```

Not every package needs to be implemented immediately. This layout defines the long-term shape so early code lands in the right place.

This is a repository convention for AOM, not a claim that Go requires this exact folder tree.

## Package Responsibilities

## cmd/aom

Owns:

- process entrypoint
- root command bootstrapping
- top-level wiring into `internal/cli`

Rules:

- keep `main.go` thin
- no domain logic here
- no direct DB or config logic here beyond app bootstrap

## internal/app

Owns:

- application wiring
- dependency construction
- shared application context

Examples:

- config loader setup
- DB bootstrap wiring
- repository construction
- command dependency assembly

Rules:

- this is orchestration glue, not business logic
- do not let domain rules accumulate here

## internal/cli

Owns:

- command definitions
- flags
- argument validation
- command-to-use-case mapping
- human-readable output formatting

Rules:

- CLI handlers should be thin
- parse input, call use-case logic, print result
- do not put state transition logic here
- do not put SQL here

## internal/config

Owns:

- config structs
- YAML loading
- validation
- config defaults

Files likely to model:

- `project.yaml`
- `agents.yaml`
- `resources.yaml`
- `policy.yaml`

Rules:

- config package should not know about Cobra or tmux
- validation should stay close to config models
- keep config parsing separate from DB sync

## internal/db

Owns:

- SQLite connection setup
- migrations
- transaction helpers
- low-level DB shared helpers

Rules:

- keep SQL bootstrapping and migration logic here
- avoid mixing domain-specific query rules into app bootstrap

## internal/project

Owns:

- project initialization
- project lookup
- project registration and sync
- project filesystem layout logic

Rules:

- this package owns project-level lifecycle behavior
- it may depend on `config` and `db`
- it should not depend on runtime-specific packages

## internal/agent

Owns:

- role and agent models
- agent registration
- agent lookup
- role-to-agent resolution rules

Rules:

- resource binding decisions may later interact with `runtime`, but keep basic agent records simple

## internal/task

Owns:

- task models
- task persistence
- task lifecycle rules

Rules:

- task status transitions belong here or in a tightly related domain layer
- do not leak task lifecycle logic into CLI handlers

## internal/step

Owns:

- step models
- step persistence
- step lifecycle rules

Rules:

- keep step behavior separate from task behavior once implemented
- do not merge task and step state logic into one generic blob

## internal/session

Owns:

- session records
- session state transitions
- recovery and replacement metadata

Rules:

- session domain logic belongs here
- live runtime control may call into this package, but this package should still own the session model

## internal/worktree

Owns:

- task-to-worktree mapping
- worktree creation
- worktree lookup
- worktree continuity and repair logic

Rules:

- AOM owns worktrees; this package will encode that rule

## internal/artifact

Owns:

- `.agent/*` generation
- schema-aware artifact updates
- artifact refresh logic
- continuity packet assembly

Rules:

- markdown operational memory behavior belongs here
- keep artifact schema logic separate from CLI and session logic

## internal/audit

Owns:

- event recording
- audit append helpers
- log integration

Rules:

- event and history recording should not be spread across packages ad hoc

## internal/runtime

Owns:

- runtime capability models
- runtime adapter contracts
- runtime-neutral session launch/resume specs

Rules:

- this package should remain runtime-neutral
- runtime-specific integrations should hang off this layer later

## internal/tmux

Owns:

- tmux-specific commands
- pane/session/window binding logic
- capture/attach helpers

Rules:

- keep tmux concerns isolated here
- the rest of the system should depend on a session/runtime abstraction, not on raw tmux commands directly

## Dependency Direction

Preferred dependency direction:

```text
cmd/aom
  -> internal/cli
  -> internal/app

internal/cli
  -> internal/app
  -> internal/project
  -> internal/agent
  -> internal/task
  -> internal/session

internal/project|agent|task|step|session|worktree|artifact|audit
  -> internal/config
  -> internal/db

internal/runtime
  -> internal/session

internal/tmux
  -> internal/session
```

Important rule:

- lower-level packages must not import `internal/cli`
- domain packages must not depend on Cobra
- DB helpers must not depend on command-layer types

This dependency direction is a project rule for AOM. It is designed to preserve idiomatic Go package clarity while keeping AOM-specific concerns from collapsing into the CLI layer.

## File Organization Rules

### Prefer small files by responsibility

Examples:

- `types.go` for core structs only when it stays small
- `loader.go` for config loading
- `validate.go` for config validation
- `repo.go` for repository interface or repository implementation entrypoint
- `migrations.go` for DB migration bootstrap

Do not create giant multi-purpose files early.

This follows the spirit of Go's package-oriented code organization: keep related code together, but do not let one file or package become a dumping ground.

### Prefer explicit package-local models

Avoid one giant `models` package.

Instead:

- project structs live with project logic
- session structs live with session logic
- task structs live with task logic

This keeps code ownership readable.

## What Not To Do

Avoid these patterns in this repository:

- generic service container frameworks
- giant `util` packages
- one package that owns every domain type
- hidden state transitions inside CLI commands
- runtime-specific logic inside config parsing
- tmux calls scattered across unrelated packages
- “helpers” files that become dumping grounds

These are AOM-specific guardrails layered on top of official Go guidance about package naming, package cohesion, and keeping code easy to navigate.

## Implementation Sequence Guidance

When implementing new milestones, prefer this order:

1. add domain structs and contracts
2. add persistence/bootstrap code
3. add use-case logic
4. add CLI wiring
5. add tests or verification for that slice

This keeps the codebase aligned with the domain instead of letting command code become the architecture.

## Milestone 1 Starter Subset

For Milestone 1, only these packages need to exist immediately:

```text
cmd/aom
internal/app
internal/cli
internal/config
internal/db
internal/project
internal/agent
internal/task
internal/session
```

Everything else can wait until the milestone that truly needs it.
