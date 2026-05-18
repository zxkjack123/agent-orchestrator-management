# AOM Refactoring Plan

Branch: `refactor`

## Why This Refactor

The codebase has grown past M17 and has two structural problems that will compound as gemini/kiro are added:

1. **`internal/cli/root.go` is 6,606 lines** — mixes CLI dispatch, business logic, orchestration, and formatting for 7+ domains. The architectural rule ("CLI handlers must be thin") is violated at scale.
2. **Provider logic is scattered** — 35+ hardcoded `switch runtime` statements across 5 files. Adding gemini or kiro today requires touching all 5 files in parallel.

---

## Part A — Provider/Runtime Architecture

**Goal:** One new file per runtime. Adding gemini/kiro = add one file, zero switch-statement changes elsewhere.

### New package: `internal/provider/`

```
internal/provider/
    provider.go    — Provider interface + Registry + fallbackProvider
    claude.go      — claudeProvider (full implementation)
    codex.go       — codexProvider (full implementation)
    gemini.go      — geminiProvider (stub — LaunchCommand returns error, CLI flags unconfirmed)
    kiro.go        — kiroProvider   (stub — same)
```

### Provider interface

```go
type Provider interface {
    Name() string
    IdentityFilename() string                                          // e.g. "CLAUDE.md"
    LaunchCommand(spec LaunchSpec, lookPath func(string)(string,error)) (string, error)
    ResumeInfo() ResumeInfo                                            // supported + example strings
    MCPConfigStyle() MCPStyle                                          // MarkdownAppend | JSONFile | None
    PolicyEnforcementLevel() PolicyEnforcement                        // RuntimeFlag | InstructionOnly
    NativeSessionDetection() *NativeSessionStrategy                   // nil = not supported
}

type Registry map[string]Provider
func DefaultRegistry() Registry   // pre-populated with all 4 providers
func (r Registry) Lookup(name string) Provider  // returns fallbackProvider if unknown
```

Supporting value types also in `provider.go`: `LaunchSpec`, `ResumeInfo`, `MCPStyle`, `PolicyEnforcement`, `NativeSessionStrategy`.

### Migration: 5 dispatch sites → registry lookups

| File | Change |
|------|--------|
| `internal/runtime/launch.go` | `Builder` gains `registry` field; `realRuntimeShellCommand` delegates to `registry.Lookup(runtime).LaunchCommand(...)`; `execRuntimeCommand` + its switch deleted; `buildDisallowedToolsFlag` moves to `provider/claude.go` |
| `internal/artifact/service.go` | `Service` gains `registry` field; `runtimeIdentityFilename` replaced by `registry.Lookup(...).IdentityFilename()`; `MaterializeMCPConfig` switches on `registry.Lookup(...).MCPConfigStyle()` |
| `internal/cli/runtime_cmd.go` | `runtimeResumeInfo` uses `r.registry.Lookup(rt).ResumeInfo()` |
| `internal/cli/root.go` | board-pointer switch → `registry.Lookup(...).IdentityFilename()`; `enforcePolicyDefaults` switch → `registry.Lookup(...).PolicyEnforcementLevel()`; native session detection block → `registry.Lookup(...).NativeSessionDetection()` |
| `internal/cli/vendor_session.go` | `claudeSessionForWorktree` + `claudeProjectsDirForPath` move into `provider/claude.go`; file deleted |

### Registry wiring
- `Runner` struct gains `registry provider.Registry`, set in `Execute()` to `provider.DefaultRegistry()`
- `artifact.Service` struct gains `registry provider.Registry`, set in `NewService()` to `provider.DefaultRegistry()`
- `runtime.Builder` gains `registry`, set in `NewBuilderWithLookPath()` to `provider.DefaultRegistry()`; new `NewBuilderWithRegistry()` constructor for tests

### Files changed in Part A
- **New:** `internal/provider/provider.go`, `claude.go`, `codex.go`, `gemini.go`, `kiro.go`
- **Modified:** `internal/runtime/launch.go`, `internal/artifact/service.go`, `internal/cli/runtime_cmd.go`, `internal/cli/root.go`
- **Deleted:** `internal/cli/vendor_session.go` (content moved to `provider/claude.go`)

---

## Part B — CLI `root.go` File Split

**Goal:** Reduce `root.go` from 6,606 lines to ~250 lines (Runner struct + dispatch routers only). All new files stay in `package cli` — no sub-packages, no visibility changes needed.

### Target file layout

| New file | Responsibility | Est. lines |
|----------|---------------|-----------|
| `root.go` (residual) | Runner struct, Execute, 10 dispatch routers, printHelp | ~250 |
| `task_cmd.go` | 15 task subcommand handlers + param types | ~900 |
| `session_cmd.go` | 12 session handlers + spawn/replace internals | ~1,200 |
| `session_spawn_helpers.go` | Spawn-specific helpers (ensurePlannedWorktree, enforceWriterWorktreeBoundary, resolveTaskExecutionPath, materializeAgentContext, enforcePolicyDefaults, setLaunchMode) | ~220 |
| `review_cmd.go` | executeReview, executeReviewClose + 13 review workflow helpers | ~480 |
| `handoff_cmd.go` | executeCheckpoint, executeHandoff + 11 handoff workflow helpers | ~380 |
| `merge_cmd.go` | executeMergeCheck, executeMergePrepare, executeMergeCommit | ~300 |
| `project_cmd.go` | executeProjectInit, executeProjectResources, executeOpen, executeStatus, executePlan | ~400 |
| `worktree_cmd.go` | executeWorktreeRepair, executeWorktreeReadFile + 2 helpers | ~180 |
| `step_cmd.go` | executeStepList, executeStepUpdate + 2 helpers | ~160 |
| `observability_cmd.go` | executeWatch variants, executeNext, executeMetrics, executeTeamBrief | ~430 |
| `approval_cmd.go` | executeApprove, executeDeny, executePauseAll, executeResumeAll | ~250 |
| `tmux_cmd.go` | executeAttach, executeCapture, executeBroadcast | ~140 |
| `helpers.go` | Cross-domain: taskView type, all load*/sync*/recommend* helpers, printProjectSummary | ~850 |

### Order of execution (run `go test ./internal/cli/...` after each file)

1. `step_cmd.go` — fewest cross-dependencies, safest first
2. `worktree_cmd.go`
3. `approval_cmd.go`
4. `tmux_cmd.go`
5. `merge_cmd.go`
6. **`helpers.go`** — highest-risk move (38 shared helpers); run full `go test ./...` after this
7. `session_spawn_helpers.go`
8. `session_cmd.go`
9. `task_cmd.go`
10. `project_cmd.go`
11. `observability_cmd.go`
12. `handoff_cmd.go`
13. `review_cmd.go` — most complex cross-references, last

### Business logic push-downs (done as part of the split)
- `transferHandoffOwnership` → new method on `internal/task.Service`
- `prepareReviewState` / `activateReviewState` → new methods on `internal/task.Service`
- `worktree.Service.CreateAndProvision()` — consolidates `ensurePlannedWorktree` two-step call

### Files changed in Part B
- **Modified:** `internal/cli/root.go` (shrinks to ~250 lines)
- **New (14 files):** all files listed in table above
- **Modified (business logic push):** `internal/task/service.go`, `internal/worktree/service.go`

---

## Agent coordination

| Agent | Scope | Order |
|-------|-------|-------|
| `golang-solid-refactor` | Executes Part A first (provider package), then Part B (CLI split) | Sequential — A then B |
| `golang-refactor-tester` | Runs after Part A completes and after Part B completes | Triggered by orchestrator after each part |

The two parts are done sequentially (not in parallel) because Part B touches `root.go` which Part A also modifies.

---

## Verification Checklist

After each part:

```bash
go build ./...                # must be clean
go test ./...                 # must be 100% green
bash scripts/e2e-smoke.sh     # must pass all 51 checks
```

### Key invariants that must hold after refactoring
- All 51 smoke test checks pass
- `aom session spawn --real claude` still passes `--disallowed-tools` flags
- `aom runtime inspect claude` still shows resume info
- `aom project resources` still shows MCP and policy info
- Adding a new runtime = create one file in `internal/provider/`, register in `DefaultRegistry()`

---

## How to Add a New Runtime After This (e.g., Gemini Full Implementation)

1. Edit `internal/provider/gemini.go` only
2. Fill in `LaunchCommand`, `MCPConfigStyle`, `ResumeInfo`, `NativeSessionDetection`
3. No other file changes needed
