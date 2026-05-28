# AOM E2E Test Report — Phase 3.3 Clean Re-run (Post P0 Fix)

**Date:** 2026-05-26  
**Environment:** Windows 11 + WSL2 Ubuntu  
**Agents:** backend2 (codex / default), frontend2 (claude / default)  
**Project:** e2e-xprovider2 (Python server + HTML page)  
**Binary:** Cross-compiled Windows Go → Linux amd64 (with P0 preamble cd fix)  
**Purpose:** Verify P0 fix resolves codex workspace isolation regression

---

## P0 Fix Verification — Primary Goal ✅ CONFIRMED

The P0 fix adds `cd "<WorktreePath>"` as the first preamble statement in the codex launch command, before `export AOM_RUNTIME=codex`. This forces codex to start in the agent's permanent workspace directory rather than navigating to the git repo root.

**Before fix (Phase 3.3 original run):**
- codex committed to `main` branch
- `server.py` wrote to `/tmp/e2e-xprovider/` (project root)
- `aom task verify` 2/5 pass

**After fix (this run):**
```
git log --all --oneline:
816c22e (agents/backend2) [TASK-1779803976183425824-1] implement hello http server   ← CORRECT
0daa206 (agents/frontend2) [TASK-1779803976234789567-1] add index.html               ← CORRECT
```
- ✅ codex committed to `agents/backend2` (not `main`)
- ✅ `server.py` written to `/tmp/e2e-xprovider2/.aom/agents/backend2/workspace/`
- ✅ `test_server.py` written to workspace (not project root)
- ✅ Both worktrees on correct branches throughout

**Launch command with P0 fix applied:**
```sh
sh -lc '...
  cd "/tmp/e2e-xprovider2/.aom/agents/backend2/workspace";   ← NEW: first statement
  export AOM_RUNTIME=codex;
  export PYTHONDONTWRITEBYTECODE=1;
  ...
  exec nice -n 19 codex --dangerously-bypass-approvals-and-sandbox ...'
```

---

## Results Summary

| Check | Result |
|-------|--------|
| **P0: codex starts in workspace CWD** | ✅ **FIXED** — cd prepended in preamble |
| **P0: codex commits to correct branch** | ✅ **FIXED** — `816c22e` on `agents/backend2` |
| codex files in workspace (not root) | ✅ server.py, test_server.py in workspace |
| [TASK-xxx] prefix on codex commit | ✅ Yes |
| claude files in workspace | ✅ index.html on `agents/frontend2` |
| [TASK-xxx] prefix on claude commit | ✅ Yes |
| task.completed (claude) | ✅ Signaled correctly |
| task.completed (codex) | ⚠️ Used `aom task close` (existing behavior) |
| handoff.md filled — codex workspace | ✅ Filled (From Role / To Role / Completed / Files Changed) |
| handoff.md filled — verify check | ⚠️ 0/2 pass (reads task artifact, not workspace) |
| state.md — codex | ✅ Filled (Completed Work / Touched Files / Open Questions) |
| state.md — claude | ⚠️ Not updated (shows 'None recorded yet') |
| backend verify gate | ⚠️ 3/5 pass (commits on branch ✅, [TASK-xxx] ✅, state.md ✅) |
| frontend verify gate | ⚠️ 3/5 pass (commits on branch ✅, [TASK-xxx] ✅, task.completed ✅) |
| Tests pass on merged code | ✅ 2/2 unittest pass |
| Both tasks merged to main | ✅ server.py + test_server.py + index.html on main |

---

## What Worked Well ✅

### 1. P0 Fix: codex workspace isolation fully resolved

```
agents/backend2 workspace:
  AGENTS.md  README.md  server.py  test_server.py  .gitignore  .agent/
```

codex stayed in `/tmp/e2e-xprovider2/.aom/agents/backend2/workspace/` for the entire session.
No navigation to project root. The `cd` in preamble held.

### 2. Correct branch isolation — both providers

```
git branch output from within backend2/workspace:
  * agents/backend2    ← correct
  + agents/frontend2
  + main

git branch output from within frontend2/workspace:
  + agents/backend2
  * agents/frontend2   ← correct
  + main
```

### 3. [TASK-xxx] tagged commits — both providers

- `816c22e [TASK-1779803976183425824-1] implement hello http server` (codex)
- `0daa206 [TASK-1779803976234789567-1] add index.html` (claude)

### 4. Code quality: excellent from both agents

**backend2 (codex):**
- `ThreadingHTTPServer` with factory pattern (`create_server()`)
- Tests use `port=0` (OS-assigned, no port conflicts)
- Proper setUp/tearDown lifecycle
- 2/2 tests pass

**frontend2 (claude):**
- CSS variables / design tokens
- ARIA-accessible button
- Loading/error states with fetch
- JSON pretty-print display

### 5. handoff.md filled by codex — workspace format correct

codex filled in `.agent/handoff.md` inside its workspace with:
- From Role / To Role
- Completed (concrete summary with test command)
- Remaining: none
- Files Changed (list)

This is the correct content — see Issue 2 below for the path mismatch.

### 6. Channel communication — both providers

```
backend2: STARTING → step done → task done
frontend2: STARTING → task done
```

Both agents broadcast to team channel. Messages staged to outbox and flushed.

### 7. Merge via `--prefer-branch`

Both `.gitignore` add/add conflicts auto-resolved. Backend and frontend merged cleanly:
```
b31b8d0 Merge agents/frontend2 into main
83e3acd Merge agents/backend2 into main
```

---

## Remaining Issues ⚠️

### Issue 1 (Medium): `handoff.md` path mismatch — verify reads task artifact, agents write workspace copy

**Observed:** Both agents filled `.agent/handoff.md` inside their workspace directory (correct from their perspective). But `aom task verify` reads `.aom/tasks/<task-id>/handoff.md` (the task artifact), which still contained template text.

**Root cause:** The profile says "Fill in `.agent/handoff.md`" — from within the workspace, this is `<workspace>/.agent/handoff.md`. The task artifact handoff.md lives at a different path: `.aom/tasks/<task-id>/handoff.md`. The verify check reads the task artifact, not the workspace copy.

**Impact:** `handoff.md filled` verify check always fails for workspace agents unless they also update the task artifact path explicitly.

**Fix directions:**
1. **Profile fix (short-term):** Tell agents to update BOTH files, or give the exact task artifact path from `task.md` "Artifact Root".
2. **AOM fix (preferred):** When `aom task signal task.completed` is called and the task artifact handoff.md still contains template text, auto-copy from the workspace `.agent/handoff.md` if it has real content.
3. **Verify fix (alternative):** Also accept workspace handoff.md as a fallback when task artifact still has template text.

---

### Issue 2 (Low): `aom task close` vs `aom task signal task.completed`

**Observed:** codex used `aom task close` which creates `task.closed` event. The `task.completed` verify check looks for `task.completed` event specifically.

**Impact:** `task.completed in log` verify check fails for codex (1/5 fail). Task transitions to Done correctly via `task.closed`.

**Fix direction:** Profile instruction could clarify: "Use `aom task signal task.completed` — do NOT use `aom task close`". Or the verify check could also accept `task.closed`.

---

### Issue 3 (Low): claude did not update `state.md`

**Observed:** frontend2/claude committed `index.html` and signaled `task.completed` but left `state.md` as "None recorded yet".

**Impact:** `state.md updated` verify check fails (1/5 fail).

**Fix direction:** Profile already has state.md update instructions. May need stronger language: "You MUST update state.md before committing — `aom task verify` will fail if you skip this."

---

## Verify Score Comparison (Before vs After P0 Fix)

| Check | Before (codex) | After (codex) | Change |
|-------|---------------|--------------|--------|
| commits on branch | ❌ (committed to main) | ✅ | Fixed by P0 |
| [TASK-xxx] tagged commit | ✅ | ✅ | — |
| state.md updated | ❌ | ✅ | Fixed by P0 (correct CWD) |
| handoff.md filled | ❌ | ⚠️ (path mismatch) | New insight |
| task.completed in log | ❌ | ❌ | Pre-existing (aom task close) |

**Before: 2/5** → **After: 3/5** (P0 fix resolved 2 checks, revealed 1 new path issue)

---

## Phase 3.3 Checklist — Post P0 Fix

```
Setup
[✅] aom project init succeeded
[✅] aom doctor 14 passed, 2 warnings (default agents lacking workspace — not used)
[✅] provision created workspaces for backend2 and frontend2
[✅] git worktree list shows 2 workspace worktrees on correct branches

backend2 (codex) workflow
[✅] session spawn --real succeeded, native ID auto-detected
[✅] cd <WorktreePath> in launch preamble — P0 fix applied
[✅] codex wrote server.py + test_server.py to WORKSPACE (not project root)
[✅] commit has [TASK-xxx] prefix
[✅] commit on correct branch agents/backend2 (NOT main)  ← KEY FIX
[✅] state.md updated with Completed Work and Touched Files
[✅] handoff.md filled in workspace copy
[⚠️] handoff.md task artifact still has template (path mismatch — Issue 1)
[⚠️] task.closed (not task.completed) in log (Issue 2)
[✅] channel.md shows backend2 step + task done messages

frontend2 (claude) workflow
[✅] session spawn --real succeeded, native ID auto-detected
[✅] claude wrote index.html to workspace
[✅] commit has [TASK-xxx] prefix on agents/frontend2
[⚠️] state.md not updated (Issue 3)
[⚠️] handoff.md task artifact not filled (Issue 1)
[✅] task.completed in log

Verify and accept
[⚠️] aom task verify backend2 → 3/5 pass (accepted with --force)
[⚠️] aom task verify frontend2 → 3/5 pass (accepted with --force)

Merge
[✅] aom merge check backend2 → Green
[✅] aom merge check frontend2 → Green
[✅] aom merge commit backend2 --prefer-branch → Merged
[✅] aom merge commit frontend2 --prefer-branch → Merged
[✅] git log main shows both [TASK-xxx] commits
[✅] server.py + test_server.py + index.html present on main
[✅] python3 -m unittest test_server → 2/2 PASS
```

---

## Conclusion

**Phase 3.3 status: ✅ Pass (P0 Fix Validated)**

- P0 fix confirmed: codex starts in workspace, commits to correct branch ✅
- AOM control plane handles cross-provider teams correctly ✅
- Code quality excellent from both providers ✅
- 3 remaining verify failures are all profile/path issues, not AOM architecture ✅

**Next actions (priority order):**
1. **P1 (Medium):** Fix handoff.md path mismatch — auto-copy from workspace on signal, or update profile path reference
2. **P2 (Low):** Enforce `aom task signal task.completed` vs `aom task close` in codex profile
3. **P3 (Low):** Strengthen state.md update instruction in claude profile
