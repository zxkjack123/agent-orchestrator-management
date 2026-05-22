#!/usr/bin/env bash
export PATH=/usr/local/go/bin:/tmp:$PATH
AOM=/tmp/aom
WS=/tmp/aom-g1g2g3-test
cd "$WS"

echo '======================================================='
echo 'Current sessions'
echo '======================================================='
$AOM session list 2>&1

echo ''
echo '======================================================='
echo 'aom session resume frontend-main (by agent name)'
echo '   Expected: picks NEWEST session (task-bound one)'
echo '======================================================='
$AOM session resume frontend-main 2>&1 || true

echo ''
echo '======================================================='
echo 'aom session resume from INSIDE workspace dir'
echo '======================================================='
cd "$WS/.aom/agents/frontend-main/workspace"
$AOM session resume frontend-main 2>&1 || true
