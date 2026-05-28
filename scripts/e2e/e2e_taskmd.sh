#!/usr/bin/env bash
export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom
WS=/tmp/aom-g1g2g3-test
cd "$WS"

TASK_ID=$($AOM task list 2>&1 | grep -oE 'TASK-[0-9]+-[0-9]+' | head -1)
BACKEND_TASK=$($AOM task list 2>&1 | grep -oE 'TASK-[0-9]+-[0-9]+' | tail -1)

echo '======================================================='
echo "[workspace agent] task.md for $TASK_ID (frontend-main)"
echo '======================================================='
# Trigger artifact refresh so task.md is regenerated with new code
$AOM task show "$TASK_ID" > /dev/null 2>&1 || true
cat "$WS/.aom/tasks/$TASK_ID/task.md" 2>/dev/null | head -20

echo ''
echo '======================================================='
echo "[traditional agent] task.md for $BACKEND_TASK (backend-main)"
echo '======================================================='
$AOM task show "$BACKEND_TASK" > /dev/null 2>&1 || true
cat "$WS/.aom/tasks/$BACKEND_TASK/task.md" 2>/dev/null | head -20
