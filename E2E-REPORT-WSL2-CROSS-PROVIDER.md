# AOM E2E Test Report — WSL2 Cross-Provider Pipeline (codex + claude)

**Date:** 2026-05-26  
**Environment:** Windows 11 + WSL2 Ubuntu  
**Agents:** backend-main (codex / gpt-5.4-mini), frontend-main (claude / default)  
**Project:** e2e-xprovider (Python server + HTML page)  
**Binary:** Cross-compiled Windows Go → Linux amd64  
**Test mode:** `--real` (live agent sessions)

---

## Test Setup

| Config | Value |
|--------|-------|
| backend-main | codex runtime, gpt-5.4-mini (ChatGPT account, no API key needed) |
| frontend-main | claude runtime, default model (claude.ai Team subscription) |
| reviewer-main | claude runtime (provisioned, not used in this run) |
| Workspace mode | Per-agent workspace (Free-Roam) |
| Policy | 5 deny commands via PATH wrappers (codex) / --disallowed-tools (claude) |

---

## Results Summary

| Check | Result |
|-------|--------|
| `aom doctor` — 15 checks | ✅ 15/15 PASS (0 warnings after provision) |
| WSL2 bwrap bypass (codex) | ✅ Auto-detected from /proc/version |
| claude auth (claude.ai Team) | ✅ Logged in — no API key required |
| codex auth (ChatGPT account) | ✅ Works without OPENAI_API_KEY env var |
| Session spawn backend-main (codex) | ✅ SESS-1779802245478527841, native ID auto-detected |
| Session spawn frontend-main (claude) | ✅ SESS-1779802277902140856, native ID auto-detected |
| Parallel execution (no collision) | ✅ Both sessions ran simultaneously without conflict |
| frontend-main code quality | ✅ Full responsive index.html with fetch, error handling, accessible UI |
| backend-main code quality | ✅ Python server with tests, correct [TASK-xxx] commit tag |
| frontend verify gate | ⚠️ 4/5 pass — handoff.md template (see Issue 1) |
| frontend merge | ✅ Merged to main via `aom merge commit --prefer-branch` |
| backend workspace isolation | ❌ Codex exited workspace CWD (see Issue 2) |
| backend verify gate | ❌ 2/5 pass — commits on wrong branch (see Issue 2) |
| All work lands on main | ✅ Both server.py + index.html on main |

---

## What Worked Well ✅

### 1. doctor 15/15 PASS — both providers detected

```
[PASS] runtime: claude        /usr/bin/claude  (used by: reviewer-main, frontend-main)
[PASS] runtime: codex         /usr/bin/codex   (used by: backend-main)
[PASS] codex: wsl2 bypass     WSL2 detected — bwrap bypass applied automatically
[PASS] workspace: claude      all 2 claude agents have dedicated workspaces
```

### 2. codex works without OPENAI_API_KEY

codex 0.133.0 with model `gpt-5.4-mini` authenticates via ChatGPT account (stored in
`~/.codex/`) — no `OPENAI_API_KEY` env var required. Confirmed in doctor check.

### 3. Both sessions spawn and auto-detect native session IDs

```
backend-main (codex):   Native session ID: 019e647b-1fea-79d1-a79e-9c1aca4335d8
frontend-main (claude): Native session ID: 7e435cf8-7488-4668-b790-2e76f7d9f9dc
```

### 4. Parallel execution — no workspace collision

Both agents ran concurrently in the same project without file conflicts.
`aom doctor` G1/G2/G3 workspace guards prevented runtime collision.

### 5. frontend-main (claude) — excellent output

Claude produced a full-featured, production-quality `index.html`:
- Responsive CSS with design tokens (CSS custom properties)
- Keyboard-accessible button with ARIA attributes
- Loading spinner during fetch
- Error handling with network error messages
- JSON pretty-print in result display
- Correct `[TASK-1779802066447598337-1]` commit prefix

**Time to commit: ~65 seconds**

### 6. Channel communication across providers

Both agents posted to `channel.md`:
```
MSG | operator → frontend-main: STARTING — Building index.html page...
MSG | operator → frontend-main: task done — index.html built with fetch to GET /hello...
MSG | operator → backend-main: step done — GET /hello server and tests implemented...
MSG | operator → backend-main: task done — implemented GET /hello JSON server with tests...
```

### 7. `aom merge commit --prefer-branch` handles add/add conflict cleanly

`.gitignore` was added by both agents independently. The `--prefer-branch` flag
auto-resolved the conflict by keeping the source branch version. Merge succeeded.

### 8. Final state on main

```
git log --oneline main:
9b14e75 Merge agents/frontend-main into main
00a13b8 [TASK-1779802066388483885-1] implement simple /hello JSON server with tests
933e9b4 [TASK-1779802066447598337-1] add index.html with GET /hello fetch and JSON display
1f2f159 init

Files on main: server.py, test_server.py, index.html, .gitignore, README.md
```

---

## Issues Found ⚠️

### Issue 1 (Medium): claude doesn't fill handoff.md without explicit instruction

**Observed:** verify check `handoff.md filled` failed because `handoff.md` still contained
template placeholder text: "Fill this in when the work is ready for transfer".

**Root cause:** The agent profile (`base.md.tmpl`) workflow describes signaling `task.completed`
but doesn't explicitly tell the agent to update `handoff.md` before signaling.

**Impact:** `aom task accept` blocked — required `--force` to bypass.

**Fix direction:**
1. Add explicit step in `base.md.tmpl` AOM Workflow: "4b. Update `.agent/handoff.md` with what was done and what's next before signaling"
2. Or make `task.md` completion checklist include `handoff.md` update

---

### Issue 2 (High): codex ignores workspace CWD — works from project root

**Observed:** codex was spawned with CWD = `/tmp/e2e-xprovider/.aom/agents/backend-main/workspace/`
(correct workspace path), but wrote `server.py` and `test_server.py` to `/tmp/e2e-xprovider/`
(project root) and committed to `main` instead of `agents/backend-main`.

**Evidence:**
```
# codex capture output:
Added /tmp/e2e-xprovider/server.py ...   ← project root, not workspace
Committed in the foreground as 00a13b8   ← on main branch

# git log --all:
00a13b8  [TASK-xxx] implement simple /hello JSON server   ← on main (not agents/backend-main)
```

**Root cause:** codex appears to navigate to the git repo root automatically, ignoring the
workspace-as-CWD convention. The `AGENTS.md` identity file instructs it to stay in the
workspace directory, but codex's file-path reasoning led it to the project root.

**Impact:**
- Verify checks 1a + 1b fail: `commits on branch` and `[TASK-xxx] tagged commit` not found on `agents/backend-main`
- `aom merge commit` would find nothing to merge (workspace branch empty)
- Required `--force` to accept and the code landed on main directly (not via merge pipeline)

**Codex did correctly:**
- Used `[TASK-xxx]` commit prefix ✅
- Called `aom step update --status completed` ✅  
- Called `aom task close` (non-standard but functional) ✅
- Wrote good Python code with tests ✅

**Fix direction:**
1. `AGENTS.md` for codex workspace agents should include explicit: "Your workspace is `<path>`. ALL files must be created here. Do NOT create files in parent directories. Check your CWD with `pwd` before writing files."
2. Investigate if codex has a config to disable automatic repo-root navigation.
3. Consider a preflight check: verify CWD = workspace path in the session preamble.

---

## Workspace Isolation Assessment

| Provider | CWD respected | Committed to correct branch | [TASK-xxx] prefix | aom task signal |
|----------|--------------|----------------------------|-------------------|-----------------|
| claude (frontend-main) | ✅ Yes | ✅ agents/frontend-main | ✅ Yes | ✅ task.completed |
| codex (backend-main) | ❌ Navigated to root | ❌ main (wrong) | ✅ Yes | ❌ used aom task close |

---

## Phase 3.3 Checklist Results

```
Setup
[✅] aom project init succeeded
[✅] aom doctor 15/15 PASS after provision
[✅] provision created workspaces for all 3 agents
[✅] git worktree list shows 3 workspace worktrees

backend-main (codex) workflow
[✅] session spawn --real succeeded, native ID detected
[✅] codex wrote server.py + test_server.py (quality: good)
[✅] commit has [TASK-xxx] prefix
[❌] commit on wrong branch (main instead of agents/backend-main)
[❌] .agent/log.md not created in workspace
[✅] channel.md shows backend-main messages

frontend-main (claude) workflow
[✅] session spawn --real succeeded, native ID detected
[✅] claude wrote index.html (quality: excellent, accessible, responsive)
[✅] commit has [TASK-xxx] prefix on agents/frontend-main
[✅] state.md updated
[❌] handoff.md not updated from template
[✅] task.completed in log (workspace + task artifact)
[✅] channel.md shows frontend-main messages

No collision
[✅] agents/backend-main and agents/frontend-main are isolated worktrees
[✅] aom doctor G1/G2/G3 guards all PASS
[✅] both sessions ran in parallel without conflict

Verify and accept
[⚠️] aom task verify frontend → 4/5 pass (handoff.md template — accepted with --force)
[❌] aom task verify backend → 2/5 pass (wrong branch — accepted with --force)

Merge
[✅] aom merge check frontend → Green, 0 overlapping files
[✅] aom merge commit frontend --prefer-branch → Merged
[N/A] backend code already on main (codex committed directly)
[✅] git log main shows both [TASK-xxx] commits
[✅] server.py + index.html both present on main
```

---

## Key Finding: Cross-Provider Compatibility

**The AOM control plane handles multiple providers correctly:**
- Session lifecycle, DB state, and artifact paths work identically for claude and codex
- Channel, workspace provisioning, and doctor checks work for both
- The collision guards (G1/G2/G3) correctly handle mixed-runtime teams

**The gap is provider-specific behavior:**
- `claude` follows workspace CWD and profile instructions reliably
- `codex` navigates to git repo root — workspace isolation breaks

This is a **codex profile/instruction gap**, not an AOM architectural gap.

---

## Recommended Next Actions

### P0 — Strengthen AGENTS.md CWD instruction for codex workspace agents

Add to `builder.md.tmpl` and `frontend.md.tmpl`:
```markdown
## Workspace Isolation (IMPORTANT for codex)
Your workspace is: `<workspace_path>` (shown in your task.md Artifact Root)
- Verify your CWD with `pwd` before creating any files
- ALL files must be created inside this directory
- Do NOT use absolute paths pointing to parent directories
- Do NOT `cd` to the git repo root or any path outside your workspace
```

### P1 — Add handoff.md to aom workflow step in base.md.tmpl

Before `aom task signal task.completed`, add:
```
3b. Update .agent/handoff.md: what was done, files changed, what comes next
```

### P2 — Add codex CWD preflight preamble to buildCodexWrapperPreamble

Inject a `cd <workspace_path>` at the start of the codex launch preamble to ensure
codex starts in the workspace even if it later navigates away on restart.

---

## Conclusion

**Phase 3.3 status: ⚠️ Partial Pass**

- AOM control plane handles cross-provider teams ✅
- claude workspace isolation works correctly ✅  
- codex workspace isolation broken — navigates to project root ❌
- All work reached main (via different paths) ✅
- No catastrophic failures; no data loss; no session crashes ✅

**Root blocker:** codex ignores workspace CWD → Profile fix required (P0) before a clean cross-provider E2E is possible without `--force`.
