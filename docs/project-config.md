# AOM Project Config

## Purpose

This document defines the project-level configuration layout for AOM so projects can be reopened consistently, specialist agent teams remain reproducible, and roles, runtimes, worktrees, resources, and policies are governed at the project level.

This config model is intentionally scoped to Milestone 0 and Milestone 1. It defines the minimum explicit structure needed before implementation begins.

## Config Layout

The recommended repository layout is:

```text
.aom/
  project.yaml
  agents.yaml
  resources.yaml
  policy.yaml
  sessions.db
  logs/
  templates/
```

### File Responsibilities

- `project.yaml`
  - project identity and global runtime defaults
- `agents.yaml`
  - role profiles and project agent definitions
- `resources.yaml`
  - skills, MCP servers, and role bindings
- `policy.yaml`
  - deny, approval, and owner-exception rules

## Design Principles

1. Project config is authoritative.
2. Owner-managed governance is the default.
3. AOM should use a role-first model, not loose CLI aliases.
4. Config should remain minimal and explicit for the MVP.

## project.yaml

### Purpose

Defines project identity and AOM runtime defaults.

### Required Fields

- `name`
- `repo`
- `default_branch`
- `runtime.terminal`
- `runtime.session_prefix`
- `context.state_dir`
- `context.checkpoint_required`

### Suggested Structure

```yaml
name: my-app
repo: /repos/my-app
default_branch: main

runtime:
  terminal: tmux
  session_prefix: myapp

context:
  state_dir: .agent
  checkpoint_required: true
```

### Field Rules

- `name`
  - project identifier used by AOM
- `repo`
  - canonical repository path
- `default_branch`
  - base branch for task worktrees
- `runtime.terminal`
  - terminal driver; MVP uses `tmux`
- `runtime.session_prefix`
  - prefix used for tmux session naming
- `context.state_dir`
  - artifact directory used for operational memory
- `context.checkpoint_required`
  - whether checkpoints are expected before handoff or task completion

### Locked MVP Decisions

- `runtime.terminal` is `tmux`
- `context.state_dir` is `.agent`

## agents.yaml

### Purpose

Defines the specialist team for the project using a two-layer model:

- `roles`
- `agents`

This lets AOM separate role behavior from concrete runtime workers.

### Suggested Structure

```yaml
roles:
  orchestrator:
    class: orchestrator
    worktree_mode: read-only
    checkpoint_expectation: required
    default_session_mode: interactive

  backend:
    class: builder
    worktree_mode: dedicated-writer
    checkpoint_expectation: required
    default_session_mode: interactive

  reviewer:
    class: reviewer
    worktree_mode: read-only
    checkpoint_expectation: required
    default_session_mode: interactive

  qa:
    class: qa
    worktree_mode: read-only
    checkpoint_expectation: optional
    default_session_mode: interactive

agents:
  orchestrator-main:
    runtime: claude
    role: orchestrator
    enabled: true

  backend-claude:
    runtime: claude
    role: backend
    enabled: true

  backend-codex:
    runtime: codex
    role: backend
    enabled: true

  reviewer-main:
    runtime: claude
    role: reviewer
    enabled: true

  qa-main:
    runtime: kiro
    role: qa
    enabled: true
```

### Required Role Fields

- `class`
- `worktree_mode`
- `checkpoint_expectation`
- `default_session_mode`

### Required Agent Fields

- `runtime`
- `role`
- `enabled`

### Role Field Rules

#### class

Recommended values for MVP:

- `orchestrator`
- `builder`
- `reviewer`
- `qa`
- `architect`
- `docs`
- `release`

#### worktree_mode

Recommended values:

- `dedicated-writer`
- `read-only`

#### checkpoint_expectation

Recommended values:

- `required`
- `optional`

#### default_session_mode

Recommended values:

- `interactive`
- `headless`

MVP should favor `interactive` as the default.

### Agent Field Rules

#### runtime

Planned runtime values:

- `codex`
- `claude`
- `kiro`
- `gemini`

First implementation wave should prioritize:

- `codex`
- `claude`
- `kiro`

#### enabled

Determines whether the agent is available for use in the project.

## resources.yaml

### Purpose

Defines project-governed skills, MCP servers, and role bindings.

This file is the core of project-scoped resource governance.

### Suggested Structure

```yaml
skills:
  api-patterns:
    path: .aom/skills/api-patterns.md
    shared: true
    runtimes: [codex, claude, kiro]

  security-review:
    path: .aom/skills/security-review.md
    shared: true
    runtimes: [claude, codex]

mcp_servers:
  repo-index:
    type: stdio
    command: uvx
    args: ["repo-index-server"]
    shared: true
    runtimes: [codex, claude, kiro]

  docs-server:
    type: http
    url: http://localhost:8123/mcp
    shared: false
    runtimes: [codex, claude]

role_bindings:
  backend:
    skills: [api-patterns]
    mcp_servers: [repo-index, docs-server]

  reviewer:
    skills: [security-review]
    mcp_servers: [repo-index]
```

### Required Skill Fields

- `path`
- `shared`
- `runtimes`

### Required MCP Fields

For `stdio` MCP:

- `type`
- `command`
- `args`
- `shared`
- `runtimes`

For `http` MCP:

- `type`
- `url`
- `shared`
- `runtimes`

### Required Role Binding Fields

- `skills`
- `mcp_servers`

### Rules

- Project resources must be declared before use.
- Roles may only use resources bound to them.
- Undeclared local resources should be disallowed by default.
- Runtime compatibility should be validated against the declared runtime list.

## policy.yaml

### Purpose

Defines project policy for denied actions, approval-required actions, session-scoped defaults, and owner exceptions.

### Suggested Structure

```yaml
policy:
  deny_commands:
    - "rm -rf"
    - "git push --force"
    - "curl * | sh"
    - "npm publish"
    - "terraform apply"

  require_approval:
    - "delete file"
    - "database migration"
    - "deploy"
    - "read secrets"
    - "network access"

  session_defaults:
    approval_scope: per-session
    yolo_mode: disabled

  owner_exceptions:
    enabled: true
    log_required: true
```

### Required Fields

- `deny_commands`
- `require_approval`
- `session_defaults.approval_scope`
- `session_defaults.yolo_mode`
- `owner_exceptions.enabled`
- `owner_exceptions.log_required`

### Rules

- Approvals are scoped per session.
- YOLO mode is scoped per session.
- Owner exceptions must be logged.
- Deny rules should win wherever runtime enforcement supports them.
- If a runtime cannot enforce a rule directly, AOM should surface the enforcement gap.

## Validation Rules

AOM should validate config at project initialization and project open time.

### project.yaml

- `repo` must exist
- `default_branch` must be present
- `runtime.terminal` must be `tmux` for the MVP

### agents.yaml

- every agent must reference an existing role
- runtime must be in the allowed runtime set
- role class must be in the known class set
- `dedicated-writer` should normally be restricted to non-reviewer implementation roles

### resources.yaml

- role bindings must reference existing skills and MCP servers
- resource runtime compatibility must overlap with the target agent runtime
- skill paths should remain inside the repository or a project-controlled path

### policy.yaml

- `approval_scope` must be `per-session`
- `yolo_mode` must be `enabled` or `disabled`

## Default Behavior Rules

### Task Creation

- new tasks default to `Direct` mode
- AOM provisions worktrees, not agents

### Agent Selection

- orchestrator recommends an agent based on role and runtime availability
- operator confirms before session spawn

### Resource Usage

- only project-declared and role-bound resources are allowed by default

### Session Behavior

- `interactive` is the default session mode
- replacement sessions inherit the same role contract
- provider switching is allowed only if role and resource compatibility still hold

## Intentionally Out of Scope for This Config Round

The following are intentionally deferred:

- nested config inheritance
- user-global override systems
- OS-specific environment matrices
- secret vault design
- multi-owner permission models
- plugin marketplace design
- advanced resource version pinning

## Locked MVP Decisions

For Milestone 0 and Milestone 1, the following decisions are locked:

1. Project config lives under `.aom/`
2. Project config is authoritative
3. Roles and agents are separate concepts
4. Resources are modeled as `skills`, `mcp_servers`, and `role_bindings`
5. Approvals and YOLO mode are `per-session`
6. Owner exceptions must be explicit and logged
7. Worktree policy lives at the role level
8. The terminal runtime for the MVP is `tmux`
