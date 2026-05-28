#!/usr/bin/env bash
set -e
export PATH="/tmp/aom-e2e-xprovider2:/usr/local/bin:/usr/bin:/bin:$PATH"

# Clean previous run
rm -rf /tmp/e2e-xprovider3
mkdir -p /tmp/e2e-xprovider3

cd /tmp/e2e-xprovider3
git init -b main .
git config user.email "test@aom.dev"
git config user.name "AOM Test"
echo "# E2E Cross-Provider v3" > README.md
git add README.md
git commit -m "init"

aom project init e2e-xprovider3 --repo . --default-branch main
echo "=== project init done ==="

aom agent add codex-be --role backend --class builder --runtime codex
aom agent add claude-fe --role frontend --class frontend --runtime claude

aom agent provision codex-be
aom agent provision claude-fe

echo ""
echo "=== doctor ==="
aom doctor

echo ""
echo "=== worktree list ==="
git worktree list
