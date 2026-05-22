#!/usr/bin/env bash
export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom
WS=/tmp/aom-g1g2g3-test
cd "$WS"

echo '======================================================='
echo '[Task list] — see actual task IDs'
echo '======================================================='
$AOM task list 2>&1

echo ''
echo '======================================================='
echo '[Bug 4] spawn --task → CLAUDE.md must go to workspace'
echo '======================================================='
TASK_ID=$($AOM task list 2>&1 | grep -oE 'TASK-[0-9]+-[0-9]+' | head -1)
echo "Using task: $TASK_ID"

# Remove CLAUDE.md from repo root (left over from earlier test) to get a clean read
rm -f "$WS/CLAUDE.md" "$WS/AGENTS.md"
echo "Cleaned up repo-root identity files"

if [ -n "$TASK_ID" ]; then
  $AOM session spawn frontend-main --mock --task "$TASK_ID" 2>&1 || true
  echo ''
  echo "Repo-root CLAUDE.md ($WS/CLAUDE.md):"
  ls "$WS/CLAUDE.md" 2>/dev/null && echo 'PRESENT (BAD — written to wrong place!)' || echo 'ABSENT (correct — workspace agent uses workspace)'
  echo "Workspace CLAUDE.md:"
  ls "$WS/.aom/agents/frontend-main/workspace/CLAUDE.md" 2>/dev/null && echo 'PRESENT (correct)' || echo 'ABSENT'
  echo "Workspace AGENTS.md:"
  ls "$WS/.aom/agents/frontend-main/workspace/AGENTS.md" 2>/dev/null && echo 'PRESENT (correct)' || echo 'ABSENT'
  echo ""
  echo "Workspace contents:"
  ls -la "$WS/.aom/agents/frontend-main/workspace/" 2>/dev/null | head -20
else
  echo 'SKIPPED: no task found'
fi
