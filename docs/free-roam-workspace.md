# Free-Roam Multi-Agent Workspace

## Vision

The Free-Roam workspace model allows the operator to walk freely between any agent terminal,
give and receive context directly, and have agents relay information to each other — all without
requiring a designated orchestrator or a structured handoff protocol.

AOM serves as the communication backbone. Agents can message each other, broadcast to the team,
and receive feedback through standard AOM CLI commands. The operator is a peer in this network,
not a gatekeeper.

```
          [Operator]  ← walks freely between terminals
         /     |     \
        /      |      \
  AgentA    AgentB    AgentC
      \        |        /
       \       |       /
            [AOM]           ← message bus + state + worktree registry
```

---

## Interaction Patterns

### Pattern 1 — Operator ↔ Any Agent (direct conversation)

Operator attaches to any agent's tmux session and has a direct conversation.
No orchestrator involved. Agent acts on feedback immediately.

```bash
aom attach backend-main     # jump into backend's terminal
# ... have a conversation, give feedback, ask questions ...
# Ctrl+B D  to detach and go talk to someone else
aom attach frontend-main    # jump into frontend's terminal
```

### Pattern 2 — Operator asks Agent A to contact Agent B

```
Operator → [Agent A terminal]: "ถาม B ว่า auth endpoint พร้อมยัง"
Agent A:
  1. aom agent list              # find B's name
  2. aom message send agent-b "auth endpoint พร้อมยัง?"
  3. aom message read agent-a    # poll or watch for reply
  4. relay answer back to operator
```

### Pattern 3 — Operator asks Agent C to tell the whole team

```
Operator → [Agent C terminal]: "บอกทีมว่า scope เปลี่ยนแล้ว ไม่ต้องทำ OAuth"
Agent C:
  aom broadcast "Scope change from operator via agent-c: OAuth removed from scope — skip /auth/oauth endpoints"
  aom channel append "agent-c: relayed operator scope change — OAuth endpoints removed"
```

---

## Foundation: Per-Agent Workspace

### Why Per-Agent Workspace Is Needed

The critical enabler of Free-Roam is that agents must NOT need to move between git worktrees
when taking on new tasks. In the current per-task-worktree model:

```
Agent (claude TUI) running in worktree-A
         │
New task arrives → new worktree-B provisioned
         │
aom session resume --task TASK-B
  → AOM: SendKeys(pane, "cd /path/to/worktree-B")
  → Claude TUI receives "cd /path" as a chat message
  → Claude's process CWD never changes
  → DB says agent is in worktree-B, process is still in worktree-A
         │
         ▼
     State diverges ❌
```

With Per-Agent Workspace:

```
Agent spawns once → workspace/   ← permanent home, never leaves
         │
aom task claim TASK-001
  → artifacts created in workspace/.agent/tasks/TASK-001/
  → current-task.md updated
  → no cd, no worktree switch
         │
aom task claim TASK-002
  → artifacts in workspace/.agent/tasks/TASK-002/
  → agent's CWD: still workspace/   ✅
         │
     State always in sync ✅
```

### Filesystem Layout

```
<repo>/                                        ← orchestrator domain (no task worktrees)
  .aom/
    agents/
      backend-main/
        workspace/                             ← permanent git worktree (branch: agents/backend-main)
          .agent/
            current-task.md                   ← active task pointer (updated by aom task claim)
            tasks/
              TASK-001/
                task.md, state.md, log.md      ← task artifacts
              TASK-003/
                task.md, state.md, log.md
          src/                                 ← implementation code (changes per commit)
          CLAUDE.md / AGENTS.md               ← identity file (materialized at spawn)
      frontend-main/
        workspace/
      reviewer-main/
        workspace/
    channel.md
    mailbox/
      backend-main.md
      frontend-main.md
    sessions.db
    project.yaml
```

### Walk-up Detection — Works Transparently

```
<repo>/.aom/agents/backend-main/workspace/
  ↑ walk up
<repo>/.aom/agents/backend-main/
  ↑
<repo>/.aom/agents/
  ↑
<repo>/.aom/
  ↑
<repo>/   ← ✅ .aom/project.yaml found
```

Every `aom` command (message, channel, task, session, broadcast) works from any workspace
directory. No manual project root configuration needed.

### Git Strategy

Each agent workspace is a long-lived git worktree on a dedicated branch:

```
main                      ← production / default branch (orchestrator base)
agents/backend-main       ← backend agent's permanent working branch
agents/frontend-main      ← frontend agent's permanent working branch
agents/reviewer-main      ← reviewer agent's permanent working branch
```

Commit convention (task-tagged messages):
```
[TASK-001] implement JWT authentication endpoint
[TASK-001] add refresh token support
[TASK-003] fix session expiry edge case
```

Merge workflow:
```bash
aom merge commit TASK-001   # identifies [TASK-001] commits in agent branch → merges to main
```

### Comparison: Per-Task vs Per-Agent

| Aspect                   | Per-Task Worktree (current) | Per-Agent Workspace (new)  |
|--------------------------|-----------------------------|---------------------------|
| cd problem               | ❌ TUI CWD doesn't change   | ✅ No cd needed ever       |
| Session state sync       | ❌ DB and process can diverge | ✅ Always in sync         |
| Agent context            | ❌ Fresh per task            | ✅ Accumulates across tasks |
| Free-Roam UX             | ❌ Agent location changes   | ✅ Always same location    |
| Git task isolation       | ✅ One branch per task       | ⚠ One branch per agent    |
| Parallel tasks per agent | ✅ Multiple worktrees        | ❌ Sequential only         |
| Merge granularity        | ✅ Per task                  | ⚠ Per agent batch         |
| Orchestrator at root     | ⚠ Needs --task worktree    | ✅ Lives at repo root      |
| Backward compatibility   | —                            | ✅ Opt-in per agent        |

---

## Communication Features

### What Works Today (No Changes Needed)

```bash
aom message send <agent> "<msg>"     # direct message to agent
aom message read [<agent>]           # read inbox
aom broadcast "<msg>"                # send to all active sessions
aom channel append "<msg>"           # post to shared team channel
aom channel read                     # read team channel
aom agent list                       # team roster
aom worktree read-file <task> <path> # read another task's file
```

### Gap 1 — Reactive Inbox: `aom message watch` (missing)

Agents must currently poll `aom message read` manually. There is no live notification.

**New command:**
```bash
aom message watch --agent <name> [--timeout <dur>]
```

Behavior:
- Tails `.aom/mailbox/<agent>.md` for new `### ` entries
- Streams each new message to stdout as it arrives
- Reuses `tailLogEvents` byte-offset tracking from `internal/cli/log_wait.go`
- Default timeout: 30 minutes
- Exit code 0 on timeout, 1 on error

Usage patterns:
```bash
# Agent checks for messages at session start and between tasks
aom message watch --agent backend-main --timeout 5m

# Operator watches for replies while talking to another agent
aom message watch --agent operator --timeout 10m
```

### Gap 2 — Reply Threading: `aom message reply` (missing)

No built-in way to reply to a specific message. Agent must manually compose a send command.

**New command:**
```bash
aom message reply <msg-id> "<text>"
```

Behavior:
- Reads `.aom/mailbox/<self>.md` to find entry with matching `MSG-xxx` ID
- Extracts the `from:` field as reply target
- Calls `appendMailboxMessage` targeting the sender
- Prefixes message body: `[reply to MSG-xxx] <text>`

Usage:
```bash
# Agent B replying to Agent A's question
aom message reply MSG-1748123456789 "yes, JWT endpoint is ready at /api/auth/login"
# → automatically routes to agent-a's mailbox
```

---

## Implementation Roadmap

### Track A — Per-Agent Workspace (architectural foundation)

| Step | File(s) | What Changes |
|------|---------|-------------|
| A1 | `internal/db/db.go` | Schema migration v9: `ALTER TABLE agents ADD COLUMN workspace_path TEXT NOT NULL DEFAULT ''` |
| A2 | `internal/agent/repository.go` | Add `WorkspacePath string` field; update Upsert + scan |
| A3 | `internal/worktree/service.go` | Add `ProvisionAgentWorkspace(repoPath, agentName) (string, error)` — creates `<repo>/.aom/agents/<name>/workspace/` as git worktree on branch `agents/<name>`; idempotent |
| A4 | `internal/cli/agent_cmd.go` | Add `aom agent provision <name>` subcommand; wire to `ProvisionAgentWorkspace`; store path via `SetAgentWorkspacePath` |
| A5 | `internal/cli/session_spawn_helpers.go` | `resolveTaskExecutionPath`: check `agentRecord.WorkspacePath != ""` first; return workspace path immediately if set |
| A6 | `internal/artifact/service.go` | `TaskArtifactRoot(worktreePath, workspacePath, taskID)` — returns `<workspace>/.agent/tasks/<taskID>` when workspace set, else legacy `.agent/` |
| A7 | `internal/cli/task_cmd.go` | `aom task claim`: when agent has workspace, skip `ensurePlannedWorktree`; write `current-task.md` to workspace |
| A8 | `internal/cli/merge_cmd.go` | `aom merge commit`: `resolveSourceBranch` helper — for workspace agents uses `agents/<name>` branch and verifies `[TASK-xxx]` tagged commits exist before merge; `aom merge continue` updated consistently |

### Track B — Free-Roam Messaging

| Step | File(s) | What Changes |
|------|---------|-------------|
| B1 | `internal/cli/message_cmd.go` | Add `watch` subcommand using `tailLogEvents`-style byte tracking on mailbox file |
| B2 | `internal/cli/message_cmd.go` | Add `reply` subcommand; parse MSG-id from mailbox; route to `from:` sender |
| B3 | `internal/cli/root.go` (message dispatch) | Wire `watch` and `reply` into `executeMessage` switch |
| B4 | `profiles/base.md.tmpl` | Add "When user asks to contact a teammate" and "When user asks to tell the team" workflow |
| B5 | `profiles/orchestrator.md.tmpl` | Add "Relaying operator feedback" and "Acting as peer relay" sections |

### Suggested Implementation Order

```
A1 ─→ A2 ─→ A3 ─→ A4   (workspace provisioning — can test in isolation)
                    │
                    ▼
             A5 ─→ A6 ─→ A7   (session + artifact integration)
                              │
                              ▼
                         A8          (merge workflow)

B1 ─→ B2 ─→ B3   (can start in parallel after A3 is done)
                │
                ▼
           B4 ─→ B5   (profile updates — do last, after commands are spec'd)
```

---

## Document Update Checklist

| Document | Status | What to Add |
|----------|--------|-------------|
| `docs/free-roam-workspace.md` | ✅ **This file** | Concept + implementation plan |
| `docs/AOM-planning.md` | ✅ Done | Added "Option C — Free-Roam" to Interaction Models section |
| `docs/cli-spec.md` | ✅ Done | Added `aom message watch`, `aom message reply`, `aom agent provision` |
| `profiles/base.md.tmpl` | ✅ Done | Free-Roam communication workflow (Track B4) |
| `profiles/orchestrator.md.tmpl` | ✅ Done | Peer relay protocol (Track B5) |
| `docs/dev/current-status.md` | ✅ Done | Track A + Track B status tables with per-step completion |

---

## Key Design Decisions

### Backward Compatibility

Agents without `workspace_path` continue using per-task worktrees exactly as before.
`aom agent provision` is opt-in per agent. No forced migration of existing projects.

### Git Isolation Trade-off

Per-Agent Workspace trades strict per-task git isolation for session stability and Free-Roam UX.
This is the right trade-off when team collaboration and continuous context are the priority.
If strict isolation is needed for a specific agent, leave that agent without a workspace (legacy mode).

### One-Writer Guardrail Scope Change

Current: `one dedicated-writer per task worktree`
New:     `one dedicated-writer per agent workspace`

`enforceWriterWorktreeBoundary` logic stays the same; scope widens from task to agent.
An agent cannot have two active writer sessions in the same workspace simultaneously.

### Orchestrator Placement

Orchestrators should be spawned without `--task` so they live at the repo root, not in a task
worktree. This is already supported (`aom session spawn orchestrator-main` with no `--task`).
No change needed — just document and enforce in orchestrator profile.
