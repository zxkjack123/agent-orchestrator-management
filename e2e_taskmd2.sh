#!/usr/bin/env bash
export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom
WS=/tmp/aom-g1g2g3-test
cd "$WS"

echo '======================================================='
echo 'Create new task for workspace agent (frontend-main/claude)'
echo '======================================================='
FE_OUT=$($AOM task create "Build login page UI" --agent frontend-main --priority normal 2>&1)
echo "$FE_OUT"
FE_TASK=$(echo "$FE_OUT" | grep -oE 'TASK-[0-9]+-[0-9]+' | head -1)
echo "Task: $FE_TASK"

echo ''
echo '======================================================='
echo 'Create new task for traditional agent (backend-main/codex)'
echo '======================================================='
BE_OUT=$($AOM task create "Build login API endpoint" --agent backend-main --priority normal 2>&1)
echo "$BE_OUT"
BE_TASK=$(echo "$BE_OUT" | grep -oE 'TASK-[0-9]+-[0-9]+' | head -1)
echo "Task: $BE_TASK"

echo ''
echo '--- Worktrees after both tasks ---'
git -C "$WS" branch -a 2>&1

echo ''
echo '======================================================='
echo "task.md — WORKSPACE agent (frontend-main): $FE_TASK"
echo "          should show ABSOLUTE path + workspace note"
echo '======================================================='
cat "$WS/.aom/tasks/$FE_TASK/task.md" 2>/dev/null | head -18

echo ''
echo '======================================================='
echo "task.md — TRADITIONAL agent (backend-main): $BE_TASK"
echo "          should show worktree path + CWD note"
echo '======================================================='
BE_WT=$(git -C "$WS" worktree list --porcelain 2>/dev/null | grep "aom/task" | head -1 | awk '{print $2}')
# artifacts are in worktree .agent/ dir
find "$WS/.aom/worktrees" -name "task.md" 2>/dev/null | head -5
WTDIR=$(find "$WS/.aom" -path "*/worktrees/*/task.md" 2>/dev/null | grep "$BE_TASK" | head -1)
if [ -n "$WTDIR" ]; then
  cat "$WTDIR" | head -18
else
  echo "(task.md in worktree .agent/)"
  find "$WS/.aom" -name "task.md" 2>/dev/null | head -5
fi
