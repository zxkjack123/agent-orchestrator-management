# AOM Engineering Guidelines

## Purpose

This document defines coding style, design guardrails, and implementation patterns for AOM.

The goal is to keep the code:

- readable
- boring in the good way
- easy to modify
- easy to review
- aligned with the project plan

## Official Go Basis

These guidelines are based on official Go documentation and idiomatic Go conventions first. AOM-specific rules are intentionally marked as project decisions rather than language rules.

Primary references:

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Go Doc Comments](https://go.dev/doc/comment)
- [Go Wiki: Errors](https://go.dev/wiki/Errors)
- [Package names](https://go.dev/blog/package-names)

## Core Principles

1. Prefer clarity over cleverness.
2. Prefer explicit code over generic frameworks.
3. Prefer local reasoning over hidden magic.
4. Prefer small, composable pieces over giant abstractions.
5. Prefer stable domain boundaries over convenience shortcuts.

## Code Style

### Naming

- Use straightforward names.
- Prefer domain names over vague technical names.
- Name structs after what they represent, not what they might become.
- Name functions after what they do, not how they do it.
- Follow Go initialism rules such as `ID`, `URL`, and `HTTP`.

Good examples:

- `LoadProjectConfig`
- `ValidateAgentsConfig`
- `OpenDatabase`
- `SyncProjectAgents`
- `CreateProjectLayout`

Avoid:

- `HandleStuff`
- `ProcessData`
- `ManagerUtil`
- `BaseService`

This follows official Go guidance on package names, mixed caps, and initialisms.

### Function Size

- Prefer short functions with one clear job.
- If a function mixes parsing, validation, persistence, and output, split it.
- CLI handlers should stay especially small.

### File Size

- Prefer small focused files.
- If a file starts owning multiple responsibilities, split it.

### Comments

- Write comments only when intent is not obvious from code.
- Prefer comments that explain rules or constraints.
- Do not narrate trivial code.
- Exported names should have doc comments.
- Doc comments should be complete sentences.

This follows official guidance in `go/doc/comment` and `CodeReviewComments`.

## Error Handling

### Rules

- Return explicit errors.
- Wrap errors with useful context.
- Do not swallow errors.
- Do not invent broad recovery behavior without a product rule.
- Error strings should start lower-case and avoid trailing punctuation unless there is a strong reason otherwise.

### Guidance

Good:

- `"load project config: %w"`
- `"open sqlite database: %w"`
- `"validate agents config: missing role %q"`

Avoid:

- `"something went wrong"`
- silent fallback behavior that changes meaning

This follows official Go guidance on error handling and error strings.

## Logging and Output

### CLI output

- Keep CLI output human-readable first.
- Output should be concise but structured enough to scan.
- Surface identity, state, and next action clearly.

### Internal logging

- Do not add logging everywhere by default.
- Add logs where lifecycle visibility matters:
  - DB bootstrap
  - config loading
  - project registration
  - session recovery

Avoid noisy debug-style logs as the default path.

## Design Patterns To Prefer

## 1. Thin CLI, explicit domain calls

Pattern:

- CLI parses input
- CLI calls domain/use-case function
- domain returns result
- CLI prints result

Do not put domain rules in Cobra handlers.

## 2. Explicit repositories

Use repositories where persistence exists.

Examples:

- `ProjectRepository`
- `AgentRepository`
- `TaskRepository`
- `SessionRepository`

Rules:

- repositories should be explicit, not generic CRUD frameworks
- repository methods should use domain language

This is a project rule. Go does not require repository patterns; AOM uses them only where persistence boundaries are real and useful.

## 3. Domain-owned transitions

Task, step, session, and worktree state transitions should live near the owning domain logic.

Do not:

- mutate states ad hoc from every command handler

## 4. Simple constructors over dependency magic

Prefer plain constructors like:

- `NewProjectService(...)`
- `NewAgentRepository(...)`

Do not introduce container frameworks.

This is an AOM maintainability decision, not a Go language rule.

## Design Patterns To Avoid

- god services
- giant interface hierarchies before multiple implementations exist
- generic repository frameworks
- catch-all `common` or `helpers` packages
- inheritance-like base types
- speculative plugin or middleware systems

These are AOM project guardrails added to keep the codebase aligned with milestone-driven development.

## Interface Guidelines

Only introduce interfaces when there is a real boundary.

Good candidates:

- DB-backed repositories
- runtime adapter contracts
- tmux/session driver boundaries

Avoid interfaces for:

- simple structs with one implementation
- packages that are not yet substitutable

Rule of thumb:

- start concrete
- introduce interface when the second real implementation or boundary appears

This follows official Go advice that interfaces generally belong on the consumer side and should not be introduced prematurely.

## Data Modeling Guidelines

### Keep structs explicit

- prefer typed structs over `map[string]any`
- prefer named fields over loose containers

### Keep DB schema and domain model aligned

- do not let database convenience define the domain model
- do not let CLI flags define the domain model either

### Keep config model separate from DB model

- YAML config structs are not automatically DB structs
- conversion between config and persisted state should be explicit

This separation is a project rule for clarity; Go itself does not impose it.

## Package Boundary Rules

### CLI layer

- may depend on app/domain packages
- must not own domain rules

### Domain packages

- may depend on config and DB
- must not depend on Cobra

### DB package

- should not know about CLI concerns

### Runtime/tmux packages

- should remain isolated
- should not become the center of the app architecture

## Testing and Verification

### For early milestones

Prefer focused verification:

- config loading tests
- config validation tests
- DB migration tests
- repository sync tests

### Rule

- test the smallest meaningful behavior
- do not introduce broad integration harnesses too early

## Formatting and Consistency

- Use standard Go formatting.
- Run `gofmt`.
- Follow idiomatic Go naming and package style.
- Keep exported surface small unless needed across packages.
- Prefer unexported helpers until cross-package use is required.

`gofmt` is non-negotiable unless there is a compelling generated-code exception.

## Change Discipline

When implementing:

1. identify the milestone
2. identify the exact slice
3. change only the packages needed for that slice
4. verify the slice
5. stop

Do not “prepare the future” by writing extra layers not required yet.

## Repository-Specific Guardrails

For AOM specifically:

- do not let session runtime behavior become the source of truth
- do not let tmux concerns leak into unrelated packages
- do not let markdown artifact logic spread ad hoc
- do not skip explicit state transitions
- do not bake one provider too deeply into the core architecture

## Definition of Clean Code in This Repo

Code is clean in AOM when it is:

- easy to locate
- easy to trace
- explicit about state changes
- narrow in scope
- aligned with milestone boundaries
- safe to extend without rewriting unrelated parts
