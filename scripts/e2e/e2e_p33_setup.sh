#!/usr/bin/env bash
set -e
export PATH="/tmp/aom-e2e-xprovider2:/usr/local/bin:/usr/bin:/bin:$PATH"
cd /tmp/e2e-xprovider2

echo "=== create backend task ==="
BACK_OUT=$(aom task create "Build a simple Python HTTP server with GET /hello returning JSON {\"message\": \"hello\"}" --agent backend2)
echo "$BACK_OUT"
BACK_TASK=$(echo "$BACK_OUT" | grep "^Task:" | awk '{print $2}')
echo "Backend task: $BACK_TASK"
aom task ready "$BACK_TASK"

echo ""
echo "=== create frontend task ==="
FRONT_OUT=$(aom task create "Build index.html page that fetches GET /hello and displays the JSON result" --agent frontend2)
echo "$FRONT_OUT"
FRONT_TASK=$(echo "$FRONT_OUT" | grep "^Task:" | awk '{print $2}')
echo "Frontend task: $FRONT_TASK"
aom task ready "$FRONT_TASK"

echo ""
echo "=== task list ==="
aom task list

echo ""
echo "BACK_TASK=$BACK_TASK"
echo "FRONT_TASK=$FRONT_TASK"
