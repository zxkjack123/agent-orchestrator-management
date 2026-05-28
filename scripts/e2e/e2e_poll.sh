#!/usr/bin/env bash
export PATH="/tmp/aom-e2e-xprovider2:/usr/local/bin:/usr/bin:/bin:$PATH"
cd /tmp/e2e-xprovider3

echo "=== $(date +%H:%M:%S) ==="

echo "--- workspace files ---"
echo -n "codex-be:  "
ls .aom/agents/codex-be/workspace/ 2>/dev/null | grep -Ev "^(AGENTS\.md|README\.md|\.git|\.codex|\.agent)$" | tr '\n' ' '
echo ""
echo -n "claude-fe: "
ls .aom/agents/claude-fe/workspace/ 2>/dev/null | grep -Ev "^(CLAUDE\.md|README\.md|\.git|\.agent)$" | tr '\n' ' '
echo ""

echo ""
echo "--- channel (last 8 msgs) ---"
aom channel read 2>/dev/null | grep "Summary:" | tail -8

echo ""
echo "--- git log (new commits) ---"
git log --all --oneline | grep -v "^e2647fe" | head -6

echo ""
echo "--- task logs ---"
echo "codex-be:"
grep -E "task\.(completed|closed)|step\.completed" .aom/tasks/TASK-1779805949642500627-1/log.md 2>/dev/null | tail -3 || echo "  (none)"
echo "claude-fe:"
grep -E "task\.(completed|closed)|step\.completed" .aom/tasks/TASK-1779805949712291112-1/log.md 2>/dev/null | tail -3 || echo "  (none)"

echo ""
echo "--- session list ---"
aom session list 2>/dev/null | grep -E "agent=|readiness=" | head -10
