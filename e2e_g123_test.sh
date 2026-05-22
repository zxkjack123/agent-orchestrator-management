#!/usr/bin/env bash

export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom

WS=/tmp/aom-g1g2g3-test
rm -rf "$WS"
mkdir -p "$WS"

cd "$WS"
git init -b main -q
git config user.name 'Test User'
git config user.email 'test@test.com'
echo '# Test Repo' > README.md
git add README.md
git commit -q -m 'initial'

echo '======================================================='
echo '[G3] aom project init — should show provision hints'
echo '======================================================='
$AOM project init 'g1g2g3-test' --repo . --default-branch main \
  --agents 'frontend-main:builder:claude,reviewer-main:reviewer:claude,backend-main:builder:codex' 2>&1
echo "exit: $?"

echo ''
echo '======================================================='
echo '[G2] aom doctor — WARN: 2 claude agents lack workspace'
echo '======================================================='
$AOM doctor 2>&1 || true

echo ''
echo '======================================================='
echo '[G1] spawn frontend-main --mock (no workspace → WARN)'
echo '======================================================='
$AOM session spawn frontend-main --mock 2>&1 || true

echo ''
echo '======================================================='
echo '[provision] aom agent provision frontend-main'
echo '======================================================='
$AOM agent provision frontend-main 2>&1

echo ''
echo '======================================================='
echo '[Bug 1] re-open project: workspace_path persists'
echo '======================================================='
$AOM agent list 2>&1

echo ''
echo '======================================================='
echo '[G2 partial] doctor after provisioning frontend only'
echo '======================================================='
$AOM doctor 2>&1 || true

echo ''
echo '======================================================='
echo '[G1 partial] spawn reviewer-main --mock'
echo '     (reviewer still has no ws, frontend has ws)'
echo '     => G1 should NOT warn: frontend now has ws'
echo '======================================================='
$AOM session spawn reviewer-main --mock 2>&1 || true

echo ''
echo '======================================================='
echo '[provision all] aom agent provision reviewer-main'
echo '======================================================='
$AOM agent provision reviewer-main 2>&1

echo ''
echo '======================================================='
echo '[G2 clean] doctor — all claude agents have workspaces'
echo '======================================================='
$AOM doctor 2>&1 || true

echo ''
echo '======================================================='
echo '[Bug 2] task create w/ provisioned agent → no worktree'
echo '======================================================='
TASK_OUT=$($AOM task create "Test task for Bug2" --agent frontend-main --priority medium 2>&1)
echo "$TASK_OUT"
TASK_ID=$(echo "$TASK_OUT" | grep -oE 'TASK-[0-9]+' | head -1)
echo "Task ID: $TASK_ID"
echo "Worktrees in .aom/worktrees/:"
ls .aom/worktrees/ 2>/dev/null || echo "(worktrees dir absent — correct: no per-task worktree created)"

echo ''
echo '======================================================='
echo '[Bug 4] spawn with --task: CLAUDE.md goes to WORKSPACE'
echo '======================================================='
if [ -n "$TASK_ID" ]; then
  $AOM session spawn frontend-main --mock --task "$TASK_ID" 2>&1 || true
  echo ""
  echo "Repo-root CLAUDE.md (should be ABSENT — no conflict):"
  ls "$WS/CLAUDE.md" 2>/dev/null && echo "PRESENT (unexpected!)" || echo "ABSENT (correct)"
  echo "Workspace CLAUDE.md (should be PRESENT):"
  ls "$WS/.aom/agents/frontend-main/workspace/CLAUDE.md" 2>/dev/null && echo "PRESENT (correct)" || echo "ABSENT (unexpected!)"
else
  echo "SKIPPED: task creation failed"
fi

echo ''
echo '======================================================='
echo 'DONE. Test workspace: '$WS
echo '======================================================='
