#!/usr/bin/env bash
# install.sh — build and install aom inside WSL
#
# Usage (from WSL terminal, any directory):
#   cd /mnt/c/Users/lattapon.kea/Desktop/agents-orchestrator-management-private
#   ./scripts/install.sh          # build + install
#   ./scripts/install.sh --test   # build + run quick tests + install
#   ./scripts/install.sh --dry    # build only, don't install
#
# The binary is installed to /usr/local/bin/aom (system-wide, needs sudo).
# If sudo is not available the script falls back to ~/.local/bin/aom.

set -euo pipefail

# ── Colour helpers ────────────────────────────────────────────────────────────

if [[ -t 1 ]]; then
    GREEN='\033[0;32m'; YELLOW='\033[0;33m'
    RED='\033[0;31m';   CYAN='\033[0;36m'; RESET='\033[0m'
else
    GREEN=''; YELLOW=''; RED=''; CYAN=''; RESET=''
fi

info()    { printf "${CYAN}[aom-install]${RESET} %s\n" "$*"; }
success() { printf "${GREEN}[aom-install]${RESET} %s\n" "$*"; }
warn()    { printf "${YELLOW}[aom-install]${RESET} %s\n" "$*"; }
fatal()   { printf "${RED}[aom-install] FATAL:${RESET} %s\n" "$*" >&2; exit 1; }

# ── Resolve project root ──────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── Parse flags ───────────────────────────────────────────────────────────────

RUN_TESTS=false
DRY_RUN=false

for arg in "$@"; do
    case "$arg" in
        --test|-t)  RUN_TESTS=true ;;
        --dry|-d)   DRY_RUN=true ;;
        --help|-h)
            echo "Usage: $0 [--test] [--dry]"
            echo "  --test  Run unit tests before installing"
            echo "  --dry   Build only, skip installation"
            exit 0
            ;;
        *) fatal "Unknown argument: $arg" ;;
    esac
done

# ── Verify Go is available ────────────────────────────────────────────────────

if ! command -v go &>/dev/null; then
    fatal "go is not in PATH — make sure Go is installed and PATH is set in ~/.bashrc or ~/.profile"
fi

GO_VERSION=$(go version 2>&1)
info "Using $GO_VERSION"

# ── Build ─────────────────────────────────────────────────────────────────────

OUT="/tmp/aom-build-$$"
trap 'rm -f "$OUT"' EXIT

info "Building from $PROJECT_ROOT ..."

GOTOOLCHAIN=local \
GOCACHE=/tmp/aom-gocache \
GOMODCACHE=/tmp/aom-gomodcache \
GOTELEMETRY=off \
    go build -o "$OUT" "$PROJECT_ROOT/cmd/aom/main.go"

BUILD_SIZE=$(du -h "$OUT" | cut -f1)
success "Build OK  ($BUILD_SIZE)"

# ── Tests (optional) ─────────────────────────────────────────────────────────

if [[ "$RUN_TESTS" == true ]]; then
    info "Running unit tests (skipping slow integration tests)..."
    GOTOOLCHAIN=local \
    GOCACHE=/tmp/aom-gocache \
    GOMODCACHE=/tmp/aom-gomodcache \
    GOTELEMETRY=off \
        go test \
            -timeout 5m \
            -run "TestExecuteProjectInit|TestExecuteStatus|TestCaptureAll|TestHelp|TestTask|TestStep|TestChannel|TestMessage|TestSession$|TestSessionList|TestSessionStop|TestSessionArchive|TestDB" \
            "$PROJECT_ROOT/internal/..." 2>&1 | grep -E "^(ok|FAIL|---)" || true

    # Fail the install if any test package failed
    if GOTOOLCHAIN=local GOCACHE=/tmp/aom-gocache GOMODCACHE=/tmp/aom-gomodcache GOTELEMETRY=off \
        go test -timeout 5m \
            -run "TestExecuteProjectInit|TestExecuteStatus|TestCaptureAll|TestHelp|TestTask|TestStep|TestChannel|TestMessage|TestSession$|TestSessionList|TestSessionStop|TestSessionArchive|TestDB" \
            "$PROJECT_ROOT/internal/..." &>/dev/null; then
        success "Tests passed"
    else
        fatal "Tests failed — fix before installing (or use without --test to skip)"
    fi
fi

# ── Dry run exit ─────────────────────────────────────────────────────────────

if [[ "$DRY_RUN" == true ]]; then
    warn "Dry run — skipping installation. Binary is at: $OUT"
    info "To install manually: sudo cp $OUT /usr/local/bin/aom"
    exit 0
fi

# ── Install ───────────────────────────────────────────────────────────────────

SYSTEM_BIN="/usr/local/bin/aom"
USER_BIN="$HOME/.local/bin/aom"

# Show what's currently installed (if anything).
if [[ -f "$SYSTEM_BIN" ]]; then
    PREV_SIZE=$(du -h "$SYSTEM_BIN" | cut -f1)
    PREV_DATE=$(stat -c '%y' "$SYSTEM_BIN" 2>/dev/null | cut -d'.' -f1 || echo "unknown")
    info "Replacing: $SYSTEM_BIN ($PREV_SIZE, modified $PREV_DATE)"
fi

if sudo -n true 2>/dev/null; then
    # Passwordless sudo available — use system path.
    sudo cp "$OUT" "$SYSTEM_BIN"
    sudo chmod +x "$SYSTEM_BIN"
    INSTALLED_AT="$SYSTEM_BIN"
elif sudo true 2>/dev/null; then
    # Interactive sudo — ask once.
    sudo cp "$OUT" "$SYSTEM_BIN"
    sudo chmod +x "$SYSTEM_BIN"
    INSTALLED_AT="$SYSTEM_BIN"
else
    # No sudo — fall back to user-local bin.
    warn "sudo not available — installing to $USER_BIN instead"
    mkdir -p "$(dirname "$USER_BIN")"
    cp "$OUT" "$USER_BIN"
    chmod +x "$USER_BIN"
    INSTALLED_AT="$USER_BIN"
    # Remind user to add ~/.local/bin to PATH if needed.
    if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
        warn "Add this to ~/.bashrc:  export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
fi

# ── Verify ────────────────────────────────────────────────────────────────────

INSTALLED_SIZE=$(du -h "$INSTALLED_AT" | cut -f1)
INSTALLED_DATE=$(stat -c '%y' "$INSTALLED_AT" 2>/dev/null | cut -d'.' -f1 || echo "unknown")

success "Installed: $INSTALLED_AT ($INSTALLED_SIZE, $INSTALLED_DATE)"
success "Ready.  Run: aom --help"
