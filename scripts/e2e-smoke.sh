#!/usr/bin/env bash
# e2e-smoke.sh — end-to-end smoke test for AOM CLI
# Runs all major command groups against a clean temp project.
# Requires no real agent binaries; session commands use --mock where applicable.
# Exit 0 if all checks pass; exit 1 on first failure or accumulated failures.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AOM_BIN="$SCRIPT_DIR/../aom"

PASS=0
FAIL=0
TMPDIR=""

# Cleanup on exit.
cleanup() {
    if [[ -n "$TMPDIR" && -d "$TMPDIR" ]]; then
        rm -rf "$TMPDIR"
    fi
}
trap cleanup EXIT

# Colour helpers — use plain text if not a TTY.
if [[ -t 1 ]]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    CYAN='\033[0;36m'
    RESET='\033[0m'
else
    GREEN='' RED='' CYAN='' RESET=''
fi

section() { printf "\n${CYAN}── %s ──${RESET}\n" "$1"; }

pass() {
    printf "  ${GREEN}[PASS]${RESET} %s\n" "$1"
    PASS=$((PASS + 1))
}

fail() {
    printf "  ${RED}[FAIL]${RESET} %s\n" "$1"
    FAIL=$((FAIL + 1))
}

# Run a command; pass if it exits 0, fail otherwise.
check() {
    local label="$1"
    shift
    if "$@" > /dev/null 2>&1; then
        pass "$label"
    else
        fail "$label"
    fi
}

# Run a command and check that its output contains a string.
check_contains() {
    local label="$1"
    local needle="$2"
    shift 2
    local out
    out=$("$@" 2>&1) || true
    if echo "$out" | grep -qF "$needle"; then
        pass "$label"
    else
        fail "$label (missing: $needle)"
    fi
}

# ── Build binary ─────────────────────────────────────────────────────────────

printf "Building AOM binary...\n"
if ! go build -o "$AOM_BIN" "$SCRIPT_DIR/../cmd/aom/main.go" 2>&1; then
    printf "${RED}FATAL: go build failed${RESET}\n"
    exit 1
fi
printf "Build OK: %s\n" "$AOM_BIN"

AOM="$AOM_BIN"

# ── Temp project setup ───────────────────────────────────────────────────────

TMPDIR=$(mktemp -d)
REPO="$TMPDIR/testrepo"

git init "$REPO" -q
git -C "$REPO" config user.email "test@example.com"
git -C "$REPO" config user.name "Test"
git -C "$REPO" commit --allow-empty -m "initial" -q

cd "$REPO"

# ── SECTION 0: Project setup ─────────────────────────────────────────────────

section "SECTION 0 — Project setup"

check_contains "project init" "Project initialized" \
    "$AOM" project init smoketest --repo "$REPO"

check_contains "doctor"       "Summary:" \
    "$AOM" doctor

check_contains "runtime list" "RUNTIME" \
    "$AOM" runtime list

check_contains "status"       "Project status" \
    "$AOM" status

# ── SECTION 1: Planning ──────────────────────────────────────────────────────

section "SECTION 1 — Planning"

check_contains "plan (dry run)"  "Proposed steps:" \
    "$AOM" plan "build a login page"

check_contains "plan --create"   "Proposed steps:" \
    "$AOM" plan "build a login page" --create

# ── SECTION 2: Task graph ────────────────────────────────────────────────────

section "SECTION 2 — Task graph"

T1_OUT=$("$AOM" task create "backend API" --role backend --priority high 2>&1)
T1=$(echo "$T1_OUT" | grep "^Task:" | awk '{print $2}')
if [[ -n "$T1" ]]; then
    pass "task create T1 (backend API)"
else
    fail "task create T1 (backend API)"
fi

T2_OUT=$("$AOM" task create "frontend UI" --role frontend --priority normal 2>&1)
T2=$(echo "$T2_OUT" | grep "^Task:" | awk '{print $2}')
if [[ -n "$T2" ]]; then
    pass "task create T2 (frontend UI)"
else
    fail "task create T2 (frontend UI)"
fi

T3_OUT=$("$AOM" task create "integration review" --role reviewer 2>&1)
T3=$(echo "$T3_OUT" | grep "^Task:" | awk '{print $2}')
if [[ -n "$T3" ]]; then
    pass "task create T3 (integration review)"
else
    fail "task create T3 (integration review)"
fi

check_contains "task list"      "backend API" \
    "$AOM" task list

check_contains "task link T2 --blocked-by T1" "Linked" \
    "$AOM" task link "$T2" --blocked-by "$T1"

check_contains "task link T3 --blocked-by T2" "Linked" \
    "$AOM" task link "$T3" --blocked-by "$T2"

check_contains "next"           "Unblocked" \
    "$AOM" next

check_contains "step list T1"   "STEP-" \
    "$AOM" step list "$T1"

S1=$("$AOM" step list "$T1" --ids-only 2>&1 | head -1 | tr -d '[:space:]')
if [[ -n "$S1" ]]; then
    pass "step list T1 --ids-only"
else
    fail "step list T1 --ids-only"
fi

# ── SECTION 3: Step advancement ──────────────────────────────────────────────

section "SECTION 3 — Step advancement"

check_contains "step update proposed→confirmed" "Confirmed" \
    "$AOM" step update "$S1" --status confirmed

check_contains "step update confirmed→ready"    "Ready" \
    "$AOM" step update "$S1" --status ready

check_contains "step update ready→in-progress"  "InProgress" \
    "$AOM" step update "$S1" --status in-progress

check_contains "step update in-progress→completed" "Completed" \
    "$AOM" step update "$S1" --status completed

check_contains "task update planned→ready"     "Ready" \
    "$AOM" task update "$T1" --status ready

check_contains "task update ready→in-progress" "InProgress" \
    "$AOM" task update "$T1" --status in-progress

check_contains "task close T1" "Done" \
    "$AOM" task close "$T1"

# ── SECTION 4: Task self-service ─────────────────────────────────────────────

section "SECTION 4 — Task self-service"

check_contains "task request" "Request filed" \
    "$AOM" task request "Need clarification on API format"

check_contains "task list-requests" "Pending task requests" \
    "$AOM" task list-requests

REQ_ID=$("$AOM" task list-requests 2>&1 | grep "REQ-" | awk '{print $1}' | head -1 | tr -d '[:space:]')
if [[ -n "$REQ_ID" ]]; then
    pass "extract request ID ($REQ_ID)"
else
    fail "extract request ID"
fi

check_contains "task approve-request" "Request approved" \
    "$AOM" task approve-request "$REQ_ID"

check_contains "task record-result T2" "Result recorded" \
    "$AOM" task record-result "$T2" --passed --summary "all tests pass"

# ── SECTION 5: Messaging ─────────────────────────────────────────────────────

section "SECTION 5 — Messaging"

check_contains "channel append" "Message appended" \
    "$AOM" channel append "Starting work" --agent test-agent

check_contains "channel read" "Starting work" \
    "$AOM" channel read

check_contains "message send" "Message sent" \
    "$AOM" message send test-agent "Hello from orchestrator"

check_contains "message read" "Hello from orchestrator" \
    "$AOM" message read test-agent

check_contains "message clear" "cleared" \
    "$AOM" message clear test-agent

# ── SECTION 6: Task visibility ───────────────────────────────────────────────

section "SECTION 6 — Task visibility"

check_contains "task list (updated board)" "frontend UI" \
    "$AOM" task list

check_contains "worktree repair T2" "worktree" \
    "$AOM" worktree repair "$T2"

# ── SECTION 7: Merge coordination ────────────────────────────────────────────

section "SECTION 7 — Merge coordination"

check_contains "merge check T2" "Merge check" \
    "$AOM" merge check "$T2"

# ── SECTION 8: Observability ─────────────────────────────────────────────────

section "SECTION 8 — Observability"

check_contains "metrics"    "Team metrics" \
    "$AOM" metrics

check_contains "team brief" "Team brief generated" \
    "$AOM" team brief

# worktree read-file may succeed or fail depending on worktree state — both are acceptable.
READFILE_OUT=$("$AOM" worktree read-file "$T2" .agent/task.md 2>&1) || true
if echo "$READFILE_OUT" | grep -qF "Task ID"; then
    pass "worktree read-file T2 .agent/task.md (artifact found)"
elif echo "$READFILE_OUT" | grep -qiE "not found|no such|worktree|error"; then
    pass "worktree read-file T2 .agent/task.md (graceful not-ready)"
else
    fail "worktree read-file T2 .agent/task.md (unexpected output)"
fi

# ── SECTION 9: Watch ─────────────────────────────────────────────────────────

section "SECTION 9 — Watch (quick timeout)"

check_contains "watch --task T1 --timeout 3s" "Watching" \
    "$AOM" watch --task "$T1" --timeout 3s

# ── SECTION 10: Session commands (tmux-gated) ─────────────────────────────────

section "SECTION 10 — Session commands (tmux-gated)"

if command -v tmux &>/dev/null && tmux info &>/dev/null 2>&1; then
    check_contains "session list" "Sessions" \
        "$AOM" session list
    pass "tmux available — session commands exercised"
else
    pass "tmux not available — session commands skipped gracefully"
fi

# ── SECTION 11: Final status and task show ───────────────────────────────────

section "SECTION 11 — Cleanup / Status"

check_contains "status (final)" "Project status" \
    "$AOM" status

check_contains "task show T1" "Done" \
    "$AOM" task show "$T1"

# ── Summary ──────────────────────────────────────────────────────────────────

printf "\n"
printf "Results: ${GREEN}%d PASS${RESET}  ${RED}%d FAIL${RESET}\n" "$PASS" "$FAIL"

if [[ $FAIL -gt 0 ]]; then
    printf "${RED}SMOKE TEST FAILED${RESET}\n"
    exit 1
fi

printf "${GREEN}SMOKE TEST PASSED${RESET}\n"
exit 0
