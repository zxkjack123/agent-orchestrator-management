#!/usr/bin/env bash
# get.sh — install or update aom from pre-built GitHub Release binaries
# Supports: macOS (Intel + Apple Silicon), Linux (amd64 + arm64)
# Windows: download the .zip from https://github.com/lattapon-aek/agent-orchestrator-management/releases
#
# One-line install:
#   curl -fsSL https://raw.githubusercontent.com/lattapon-aek/agent-orchestrator-management/main/scripts/get.sh | sh
#
# Install a specific version:
#   curl -fsSL ...get.sh | sh -s -- --version v0.2.0

set -euo pipefail

REPO="lattapon-aek/agent-orchestrator-management"

# Colour helpers (disabled when not a TTY)
if [ -t 1 ]; then
  GREEN='\033[0;32m' YELLOW='\033[0;33m' RED='\033[0;31m' CYAN='\033[0;36m' RESET='\033[0m'
else
  GREEN='' YELLOW='' RED='' CYAN='' RESET=''
fi
info()    { printf "${CYAN}[aom]${RESET} %s\n" "$*"; }
success() { printf "${GREEN}[aom]${RESET} %s\n" "$*"; }
warn()    { printf "${YELLOW}[aom]${RESET} %s\n" "$*"; }
fatal()   { printf "${RED}[aom] error:${RESET} %s\n" "$*" >&2; exit 1; }

# Parse flags
TARGET_VERSION=""
for arg in "$@"; do
  case "$arg" in
    --version=*) TARGET_VERSION="${arg#*=}" ;;
    --version)   shift; TARGET_VERSION="${1:-}" ;;
    --help|-h)
      echo "Usage: $0 [--version <tag>]"
      echo "  --version  Install a specific release tag (default: latest)"
      exit 0 ;;
    *) fatal "Unknown argument: $arg" ;;
  esac
done

# Detect OS
OS="$(uname -s 2>/dev/null || echo unknown)"
case "$OS" in
  Linux)  OS_ID="linux" ;;
  Darwin) OS_ID="darwin" ;;
  *) fatal "Unsupported OS: $OS — Windows users: download from https://github.com/${REPO}/releases" ;;
esac

# Detect architecture
ARCH="$(uname -m 2>/dev/null || echo unknown)"
case "$ARCH" in
  x86_64)        ARCH_ID="amd64" ;;
  aarch64|arm64) ARCH_ID="arm64" ;;
  *) fatal "Unsupported architecture: $ARCH" ;;
esac

# Resolve latest version if not specified
if [ -z "$TARGET_VERSION" ]; then
  info "Fetching latest release version..."
  if command -v curl >/dev/null 2>&1; then
    API_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null)"
  elif command -v wget >/dev/null 2>&1; then
    API_JSON="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null)"
  else
    fatal "curl or wget is required"
  fi
  TARGET_VERSION="$(printf '%s' "$API_JSON" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/' | head -1)"
  [ -n "$TARGET_VERSION" ] || fatal "Could not determine latest version — check internet connection or specify --version"
fi

VERSION_BARE="${TARGET_VERSION#v}"
ARCHIVE="aom_${VERSION_BARE}_${OS_ID}_${ARCH_ID}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TARGET_VERSION}"

info "Installing aom ${TARGET_VERSION} (${OS_ID}/${ARCH_ID})"

# Download to temp dir
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

info "Downloading ${ARCHIVE}..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL --progress-bar -o "${TMP}/${ARCHIVE}" "${BASE_URL}/${ARCHIVE}" || fatal "Download failed: ${BASE_URL}/${ARCHIVE}"
  curl -fsSL -o "${TMP}/checksums.txt" "${BASE_URL}/checksums.txt" 2>/dev/null || true
else
  wget -q --show-progress -O "${TMP}/${ARCHIVE}" "${BASE_URL}/${ARCHIVE}" || fatal "Download failed: ${BASE_URL}/${ARCHIVE}"
  wget -qO "${TMP}/checksums.txt" "${BASE_URL}/checksums.txt" 2>/dev/null || true
fi

# Verify checksum (best-effort — skip gracefully if tools unavailable)
if [ -f "${TMP}/checksums.txt" ]; then
  EXPECTED="$(grep "  ${ARCHIVE}$" "${TMP}/checksums.txt" 2>/dev/null | awk '{print $1}')"
  if [ -n "$EXPECTED" ]; then
    ACTUAL=""
    if command -v sha256sum >/dev/null 2>&1; then
      ACTUAL="$(sha256sum "${TMP}/${ARCHIVE}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      ACTUAL="$(shasum -a 256 "${TMP}/${ARCHIVE}" | awk '{print $1}')"
    fi
    if [ -n "$ACTUAL" ]; then
      [ "$ACTUAL" = "$EXPECTED" ] || fatal "Checksum mismatch — download may be corrupted\n  expected: $EXPECTED\n  got:      $ACTUAL"
      success "Checksum verified"
    fi
  fi
fi

# Extract
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"
SRC="${TMP}/aom"
[ -f "$SRC" ] || fatal "Binary 'aom' not found in archive"
chmod +x "$SRC"

# Install — try system path, then fall back to user-local
SYSTEM_DIR="/usr/local/bin"
USER_DIR="$HOME/.local/bin"
DEST=""

if [ -w "$SYSTEM_DIR" ]; then
  cp "$SRC" "${SYSTEM_DIR}/aom"
  DEST="${SYSTEM_DIR}/aom"
elif sudo -n true 2>/dev/null; then
  sudo cp "$SRC" "${SYSTEM_DIR}/aom"
  sudo chmod +x "${SYSTEM_DIR}/aom"
  DEST="${SYSTEM_DIR}/aom"
else
  mkdir -p "$USER_DIR"
  cp "$SRC" "${USER_DIR}/aom"
  DEST="${USER_DIR}/aom"
  if ! printf '%s' "$PATH" | grep -q "$USER_DIR"; then
    warn "Add to your shell profile:  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
fi

success "Installed: $DEST"
"$DEST" version | sed 's/^/[aom] /'
success "Done. Run: aom --help"
