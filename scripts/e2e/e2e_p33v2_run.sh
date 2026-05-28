#!/usr/bin/env bash
set -e
export PATH="/tmp/aom-e2e-xprovider2:/usr/local/bin:/usr/bin:/bin:$PATH"
cd /tmp/e2e-xprovider3

echo "=== create tasks ==="
BACK_OUT=$(aom task create "Build a simple Python HTTP server with GET /hello returning JSON {\"message\": \"hello\", \"status\": \"ok\"}" --agent codex-be)
BACK_TASK=$(echo "$BACK_OUT" | grep "^Task:" | awk '{print $2}')
aom task ready "$BACK_TASK"
echo "Backend task: $BACK_TASK"

FRONT_OUT=$(aom task create "Build index.html page that fetches GET /hello and displays JSON result with error handling" --agent claude-fe)
FRONT_TASK=$(echo "$FRONT_OUT" | grep "^Task:" | awk '{print $2}')
aom task ready "$FRONT_TASK"
echo "Frontend task: $FRONT_TASK"

echo ""
echo "=== spawn codex-be ==="
aom session spawn codex-be --task "$BACK_TASK" --real

echo ""
echo "=== spawn claude-fe ==="
aom session spawn claude-fe --task "$FRONT_TASK" --real

echo ""
echo "=== session list ==="
aom session list

echo "BACK_TASK=$BACK_TASK"
echo "FRONT_TASK=$FRONT_TASK"
