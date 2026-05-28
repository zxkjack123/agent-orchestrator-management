# Contributing to AOM

Thank you for your interest in contributing. This document covers how to build, test, and submit changes.

## Prerequisites

- Go 1.24+
- tmux
- git

## Build

```bash
go build -o aom cmd/aom/main.go
```

## Test

```bash
# All packages (integration tests run real git ops — allow extra time)
go test -timeout 20m ./...

# Single package
go test ./internal/<package>/...
```

## Before You Open a PR

1. **Read [`AGENTS.md`](AGENTS.md)** — working principles, implementation guardrails, and what makes a good change in this repo.
2. **Read [`docs/engineering-guidelines.md`](docs/engineering-guidelines.md)** — code style, package boundaries, and design rules.
3. **Check [`docs/project-structure.md`](docs/project-structure.md)** — where new code should live.

## Key Rules

- CLI handlers must be thin — parse input, call domain service, print result. No business logic.
- Domain logic must not import Cobra or any CLI type.
- Touch only what your task requires. No opportunistic refactoring.
- No speculative abstractions — implement the minimum for the current change.
- State transitions belong in domain packages, not in CLI handlers.

## Submitting Changes

- Keep PRs small and focused on one change.
- Include a clear description of what changed and why.
- Make sure `go build ./...` and `go test ./...` pass before submitting.

## Questions

Open an issue for feature requests, bugs, or design questions.
