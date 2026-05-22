#!/usr/bin/env bash
export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom
WS=/tmp/aom-g1g2g3-test
cd "$WS"

echo '======================================================='
echo 'Q: workspace agent — ยัง work กับ task+worktree ได้ไหม?'
echo '======================================================='

echo ''
echo '--- Task list ---'
$AOM task list 2>&1

echo ''
echo '--- Worktree list ---'
$AOM worktree list 2>&1

echo ''
echo '--- Task artifacts (workspace path) ---'
echo "workspace/.agent/:"
find "$WS/.aom/agents/frontend-main/workspace" -maxdepth 3 -type f 2>/dev/null

echo ''
echo '--- Task artifacts (central .aom/tasks) ---'
find "$WS/.aom/tasks" -maxdepth 3 -type f 2>/dev/null

echo ''
echo '--- Task show (detail) ---'
TASK_ID=$($AOM task list 2>&1 | grep -oE 'TASK-[0-9]+-[0-9]+' | head -1)
$AOM task show "$TASK_ID" 2>&1

echo ''
echo '======================================================='
echo 'สร้าง 2nd task ให้ backend-main (codex, no workspace) → ต้องมี worktree'
echo '======================================================='
$AOM task create "Backend API stub for login" --agent backend-main --priority normal 2>&1
echo ''
echo 'Worktrees after backend task:'
$AOM worktree list 2>&1

echo ''
echo '======================================================='
echo 'branch graph: workspace branch vs main'
echo '======================================================='
git -C "$WS" branch -a 2>&1
git -C "$WS/.aom/agents/frontend-main/workspace" log --oneline -5 2>&1

echo ''
echo '======================================================='
echo 'Merge check: workspace agent → agents/frontend-main branch'
echo '======================================================='
$AOM merge check "$TASK_ID" 2>&1 || true
