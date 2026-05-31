# AOM — Agent Orchestrator Management

AOM is a project-level control plane for managing multiple CLI-based AI agents (Claude Code, Codex, Kiro) as a coordinated team. A single operator runs `aom` to dispatch tasks, manage agent sessions, and maintain durable state across sessions and git worktrees.

## Overview

Modern AI coding agents are powerful individually, but coordinating several of them on the same project — tracking who is working on what, handing off context, avoiding conflicts — quickly becomes manual overhead. AOM solves this at the project layer without replacing the native agent terminals.

AOM manages:
- **State continuity** — durable task and session artifacts survive restarts and respawns
- **Session lifecycle** — spawn, recover, stop, and hand off agent sessions from one place
- **Git worktree isolation** — each task gets its own branch and worktree; no cross-task conflicts
- **Operator workflow** — explicit approval gates, handoff coordination, merge pipeline
- **Agent communication** — broadcast briefs, send targeted messages, read shared channels

## Interaction Models

### Option A — Operator as Orchestrator

Run `aom` commands directly to dispatch tasks, monitor workers, and manage handoffs. Best for structured pipelines, sequential handoffs, and strict git isolation per task.

### Option B — AI Orchestrator

Delegate an AI session (Claude Code, role: orchestrator) to manage workers, read artifacts, send prompts, and report back to you. Best for long-running parallel work with minimal operator intervention.

### Option C — Free-Roam Workspace

Each agent has a permanent workspace. Operator walks freely between agent terminals. AOM acts as the communication backbone. Best for exploratory work and direct feedback loops.

## Requirements

- **tmux** — session management
- **git** — worktree isolation
- **Go 1.24+** — to build from source
- At least one supported AI agent runtime: [Claude Code](https://claude.ai/code), [Codex CLI](https://github.com/openai/codex), or Kiro CLI

> **macOS tip**: [iTerm2](https://iterm2.com) with native tmux integration (`tmux -CC`) gives each agent its own pane in a single native window — the most ergonomic way to watch the team grid. See [docs/iterm2-tmux-setup.md](docs/iterm2-tmux-setup.md).

## Installation

### Build from source

```bash
git clone https://github.com/lattapon-aek/agent-orchestrator-management.git
cd agent-orchestrator-management
go build -o aom cmd/aom/main.go
sudo mv aom /usr/local/bin/
```

### Verify installation

```bash
aom version
```

## Quick Start

### Single Agent — try it in 4 steps

```bash
cd your-project
aom project init "my-project"
aom agent add builder --role builder --class builder --runtime claude
aom agent provision builder   # creates a permanent workspace
aom session spawn builder --real
# → Claude Code opens with full AOM context; start talking to it
```

### AI Orchestrator — multi-agent team

Spawn a team and let an orchestrator agent handle task creation, assignment, and coordination for you.

Mix runtimes freely — Claude Code and Codex can work side-by-side on the same project.

```bash
cd your-project
aom project init "my-project"

# Register agents — mix runtimes as you prefer
aom agent add orchestrator --role orchestrator --class orchestrator --runtime claude
aom agent add backend      --role builder      --class builder      --runtime codex    # OpenAI Codex
aom agent add frontend     --role builder      --class frontend     --runtime claude   # Claude Code
aom agent add reviewer     --role reviewer     --class reviewer     --runtime claude

# Provision permanent workspaces
aom agent provision orchestrator
aom agent provision backend
aom agent provision frontend
aom agent provision reviewer

# Spawn the whole team in a tiled tmux window
aom orchestrate --real
# → Tell the orchestrator what you want to build — it assigns tasks and coordinates across runtimes
```

After spawning, monitor from another terminal:

```bash
aom status            # project-wide summary
aom dashboard         # live ANSI dashboard (Ctrl+C to exit)
aom channel read      # read the shared team channel
```

## Key Commands

### Project & Agents

```bash
aom project init          # Initialize AOM in current repo
aom agent add             # Register an agent with runtime + class
aom agent list            # List all configured agents
aom agent provision <name>  # Create permanent workspace (free-roam mode)
aom doctor                # Check system health and config
```

### Tasks & Steps

```bash
aom task create <description>   # Create a new task
aom task show <id>              # Show task details, steps, and artifacts
aom task list                   # List all tasks
aom task signal <id> <event>    # Send lifecycle signal (task.completed, etc.)
aom task verify <id>            # Run completion checks
aom task accept <id>            # Accept a completed task for merge
aom next                        # Show highest-priority ready task
```

### Sessions

```bash
aom session spawn --agent <name> --task <id>   # Spawn an agent session
aom session list                               # List all sessions
aom session resume <name>                      # Resume a session by agent name
aom session recover <id>                       # Diagnose and recover a failed session
aom session stop <name>                        # Stop a session cleanly
aom switch <agent-name>                        # Jump to agent's live tmux pane
aom attach <name>                              # Attach to a session directly
```

### Workflow

```bash
aom checkpoint                        # Save current session checkpoint
aom handoff                           # Prepare handoff for the next agent
aom review                            # Open a review session
aom approve / aom deny                # Approve or deny a pending action
aom merge check / prepare / commit    # Merge coordination pipeline
aom run-pipeline <task-id>            # Full automated pipeline: spawn → verify → accept → merge
```

### Team Grid

```bash
aom orchestrate [--layout tiled] [--real|--mock]  # --task is optional — spawn team without assigning a task first
aom team view                                      # Attach to the team window without respawning
```

### Monitoring & Communication

```bash
aom status                            # Project-wide status summary
aom status --action-items             # Show only items requiring operator action
aom dashboard                         # Live ANSI terminal dashboard (Ctrl+C to exit)
aom events tail                       # Stream live log events
aom broadcast "<message>"             # Push message to all live agent sessions + channel log
aom message send <agent> "<message>"  # Send a direct message (DM) — recipient notified instantly
aom message watch <agent> --timeout 5m  # Wait for a reply (exits when message arrives)
aom message reply <msg-id> "<reply>"    # Reply to a specific message by ID
aom channel append "<message>"          # Post to shared team channel log
aom channel read                      # Read the shared team channel log
aom metrics                           # Velocity report from task/step events
```

## Architecture

```
cmd/aom → internal/cli → internal/app → internal/{project,agent,task,step,session,worktree,artifact,plan}
                                       → internal/{config,db,tmux}
```

AOM has three layers of truth (in order of authority):

1. **`.agent/*.md` artifacts** — durable Markdown files (task.md, state.md, log.md, handoff.md) — primary source of truth
2. **SQLite DB** (`.aom/sessions.db`) — structured state for queries and transitions
3. **Live tmux sessions** — ephemeral, always replaceable

State machines are defined for Task, Step, Session, and Worktree. See [docs/state-machine.md](docs/state-machine.md).

## Configuration

AOM is configured through `.aom/project.yaml` in your project root, created automatically by `aom project init`. Agent profiles, runtime policies, and deny-command lists are managed in `.aom/`.

See [docs/project-config.md](docs/project-config.md) for the full configuration reference.

## Supported Agent Runtimes

| Runtime | Status | Notes |
|---------|--------|-------|
| Claude Code (`claude`) | Stable | Full support — workspace isolation, policy enforcement |
| Codex CLI (`codex`) | Stable | Full support — WSL2-compatible bwrap bypass, deny-command wrappers |
| Kiro CLI (`kiro`) | Planned | Pending confirmed CLI flags |
| Gemini CLI (`gemini`) | Planned | Pending confirmed CLI flags |

## Documentation

| File | Description |
|------|-------------|
| [docs/AOM-planning.md](docs/AOM-planning.md) | Product vision and operating principles |
| [docs/AOM-milestones.md](docs/AOM-milestones.md) | Milestone breakdown and status |
| [docs/state-machine.md](docs/state-machine.md) | Complete state lifecycle for all entities |
| [docs/artifact-schemas.md](docs/artifact-schemas.md) | Markdown artifact contracts and field schemas |
| [docs/cli-spec.md](docs/cli-spec.md) | Full CLI command specifications |
| [docs/engineering-guidelines.md](docs/engineering-guidelines.md) | Code style and design guardrails |
| [docs/project-config.md](docs/project-config.md) | `.aom/` config file layout and schemas |
| [docs/iterm2-tmux-setup.md](docs/iterm2-tmux-setup.md) | iTerm2 + tmux integration guide (recommended for macOS) |

## Contributing

Contributions are welcome. Please read [AGENTS.md](AGENTS.md) for working guidelines before submitting a pull request.

## License

[MIT](LICENSE)
