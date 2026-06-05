#!/bin/bash
# Capture web UI screenshots for README demo GIF
set -e

BASE_URL="http://localhost:7777"
# Use the first project ID from the running server, or override manually
PROJECT_ID="${AOM_PROJECT_ID:-825427da}"
OUT_DIR="docs/assets/screenshots"
mkdir -p "$OUT_DIR"

WIN_W=1440
WIN_H=860

capture_page() {
  local name="$1"
  local path="$2"
  local delay="${3:-3}"
  local url="${BASE_URL}/projects/${PROJECT_ID}/${path}"
  echo "→ Capturing: $name ($url)"

  # Navigate via open, then screencapture after delay
  open -a Safari "$url"
  sleep "$delay"

  # Capture full screen then crop with ffmpeg (Retina 2x: 3600x2338 physical)
  screencapture -x /tmp/aom_capture_tmp.png
  ffmpeg -y -i /tmp/aom_capture_tmp.png -vf "crop=2880:1600:100:200,scale=1200:-1" "${OUT_DIR}/${name}.png" 2>/dev/null
  echo "  ✓ ${OUT_DIR}/${name}.png ($(du -sh "${OUT_DIR}/${name}.png" | cut -f1))"
}

echo "=== AOM Demo Screenshot Capture ==="

# Set Safari window size via osascript (best effort)
osascript <<'SCPT' 2>/dev/null || true
tell application "Safari"
  activate
  set bounds of front window to {50, 50, 1490, 910}
end tell
SCPT
sleep 1

capture_page "01-dashboard"  "dashboard"   4
capture_page "02-agents"     "agents"      3
capture_page "03-roles"      "roles"       3
capture_page "04-sessions"   "sessions"    3
capture_page "05-tasks"      "tasks"       3
capture_page "06-mailbox"    "mailbox"     3
capture_page "07-channel"    "events"      3
capture_page "08-warroom"    "war-room"    4

echo ""
echo "=== Preparing frames (already 1200px from capture) ==="
for f in "${OUT_DIR}"/0*.png; do
  base="${f%.png}"
  if [[ "$base" != *"_r" ]]; then
    cp "$f" "${base}_r.png"
    echo "  ✓ $(basename ${base}_r.png)"
  fi
done

echo ""
echo "=== Building animated GIF ==="
ffmpeg -y \
  -framerate 0.35 \
  -pattern_type glob \
  -i "${OUT_DIR}/0*_r.png" \
  -vf "fps=10,scale=1200:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=256[p];[s1][p]paletteuse=dither=bayer:bayer_scale=5" \
  -loop 0 \
  "docs/assets/demo.gif" 2>&1 | tail -2

echo "  ✓ docs/assets/demo.gif ($(du -sh docs/assets/demo.gif | cut -f1))"

echo ""
echo "=== Optimizing GIF ==="
gifsicle --optimize=3 --lossy=30 -o "docs/assets/demo-optimized.gif" "docs/assets/demo.gif"
echo "  ✓ docs/assets/demo-optimized.gif ($(du -sh docs/assets/demo-optimized.gif | cut -f1))"

echo ""
echo "Done. Use in README:"
echo '  ![AOM Demo](docs/assets/demo-optimized.gif)'
