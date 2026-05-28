#!/usr/bin/env bash
export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom
WS=/tmp/aom-g1g2g3-test
cd "$WS"

echo '======================================================='
echo '[Bug 2] task create w/ provisioned agent'
echo '        → should NOT create per-task worktree'
echo '======================================================='
TASK_OUT=$($AOM task create 'Test task for Bug2' --agent frontend-main --priority normal 2>&1)
echo "$TASK_OUT"
TASK_ID=$(echo "$TASK_OUT" | grep -oE 'TASK-[0-9]+' | head -1)
echo "Task ID: $TASK_ID"
echo "Worktrees in .aom/worktrees/ (should be empty — no per-task worktree):"
ls "$WS/.aom/worktrees/" 2>/dev/null || echo "(worktrees dir absent — correct)"

echo ''
echo '======================================================='
echo '[Bug 4] spawn --task → CLAUDE.md lands in WORKSPACE'
echo '======================================================='
if [ -n "$TASK_ID" ]; then
  $AOM session spawn frontend-main --mock --task "$TASK_ID" 2>&1 || true
  echo ''
  echo "Repo-root CLAUDE.md ($WS/CLAUDE.md):"
  ls "$WS/CLAUDE.md" 2>/dev/null && echo 'PRESENT (unexpected conflict!)' || echo 'ABSENT (correct)'
  echo "Workspace CLAUDE.md:"
  ls "$WS/.aom/agents/frontend-main/workspace/CLAUDE.md" 2>/dev/null && echo 'PRESENT (correct)' || echo 'ABSENT (unexpected!)'
  echo "Workspace AGENTS.md:"
  ls "$WS/.aom/agents/frontend-main/workspace/AGENTS.md" 2>/dev/null && echo 'PRESENT (correct)' || echo 'ABSENT'
else
  echo 'SKIPPED: no task ID'
fi
