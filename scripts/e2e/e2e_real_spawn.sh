#!/usr/bin/env bash
export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom
WS=/tmp/aom-g1g2g3-test
cd "$WS"

echo '======================================================='
echo 'Step 1: stop all existing Idle sessions for frontend-main'
echo '======================================================='
SESS_IDS=$($AOM session list 2>&1 | grep 'frontend-main' | grep -oE 'SESS-[0-9]+')
for sid in $SESS_IDS; do
  echo "Stopping $sid ..."
  $AOM session stop "$sid" 2>&1 || true
done
echo "Done stopping."

echo ''
echo '======================================================='
echo 'Step 2: spawn --real with task'
echo '======================================================='
TASK_ID=$($AOM task list 2>&1 | grep -oE 'TASK-[0-9]+-[0-9]+' | head -1)
echo "Task: $TASK_ID"
SPAWN_OUT=$($AOM session spawn frontend-main --real --task "$TASK_ID" 2>&1)
echo "$SPAWN_OUT"
NEW_SESS=$(echo "$SPAWN_OUT" | grep -oE 'SESS-[0-9]+' | head -1)
echo "New session: $NEW_SESS"

echo ''
echo '======================================================='
echo 'Step 3: wait 8s for claude to boot and create session'
echo '======================================================='
sleep 8

echo ''
echo '======================================================='
echo 'Step 4: capture pane output (what claude is showing)'
echo '======================================================='
$AOM capture "$NEW_SESS" 2>&1 | head -40

echo ''
echo '======================================================='
echo 'Step 5: check vendor session ID captured'
echo '======================================================='
$AOM session show "$NEW_SESS" 2>&1

echo ''
echo '======================================================='
echo 'Step 6: check ~/.claude/projects/ for workspace entry'
echo '======================================================='
WS_HASH=$(echo -n "$(realpath "$WS/.aom/agents/frontend-main/workspace")" | sha256sum | cut -c1-8)
echo "Workspace: $(realpath "$WS/.aom/agents/frontend-main/workspace")"
echo "~/.claude/projects contents:"
ls ~/.claude/projects/ 2>/dev/null | head -20 || echo "(no projects dir)"

echo ''
echo '======================================================='
echo 'Step 7: claude -r from INSIDE workspace (non-interactive list)'
echo '======================================================='
cd "$WS/.aom/agents/frontend-main/workspace"
echo "CWD: $(pwd)"
# Run claude --resume in a subshell that exits immediately to just show available sessions
timeout 5 claude --resume --print "just list sessions" 2>&1 | head -20 || true
