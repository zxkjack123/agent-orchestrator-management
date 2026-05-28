#!/usr/bin/env bash
set -e
export PATH="/tmp/aom-e2e-xprovider2:/usr/local/bin:/usr/bin:/bin:$PATH"
cd /tmp/e2e-xprovider2

BACK_TASK="TASK-1779803976183425824-1"
FRONT_TASK="TASK-1779803976234789567-1"

echo "=== spawn backend2 (codex) ==="
aom session spawn backend2 --task "$BACK_TASK" --real

echo ""
echo "=== spawn frontend2 (claude) ==="
aom session spawn frontend2 --task "$FRONT_TASK" --real

echo ""
echo "=== session list ==="
aom session list

echo ""
echo "=== git worktree list ==="
git worktree list
