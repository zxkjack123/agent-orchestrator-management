#!/bin/bash
# AOM Team View demo — simulates multi-agent team status grid
AOM="${AOM_BIN:-$(command -v aom 2>/dev/null || echo "$(cd "$(dirname "$0")/.." && pwd)/aom")}"
cd "${AOM_PROJECT_DIR:-$(pwd)}" 2>/dev/null

C_RESET="\033[0m"
C_BOLD="\033[1m"
C_DIM="\033[2m"
C_CYAN="\033[1;36m"
C_YELLOW="\033[1;33m"
C_GREEN="\033[1;32m"
C_MAGENTA="\033[1;35m"
C_BLUE="\033[1;34m"
C_RED="\033[0;31m"
C_OK="\033[0;32m"
C_WARN="\033[0;33m"

type_cmd() {
  printf "${C_OK}\$${C_RESET} aom "
  echo -n "$1" | while IFS= read -r -n1 c; do
    printf "%s" "$c"; sleep 0.05
  done
  printf "\n"; sleep 0.4
}

hr() { printf "${C_DIM}%s${C_RESET}\n" "$(printf '─%.0s' $(seq 1 96))"; }

# ── SCENE 1: aom status ───────────────────────────────────────────
clear
sleep 0.8
type_cmd "status"
sleep 0.3
$AOM status 2>/dev/null
sleep 3

# ── SCENE 2: team channel ─────────────────────────────────────────
clear
type_cmd "channel read"
sleep 0.3
$AOM channel read 2>/dev/null | head -38
sleep 3

# ── SCENE 3: team grid view (tmux-style layout simulation) ────────
clear
printf "${C_BOLD}${C_BLUE}  AOM War Room  ${C_RESET}${C_DIM}· aom orchestrate --layout tiled --real${C_RESET}\n"
hr
printf "\n"

# Top-left: orchestrator
printf "${C_CYAN}┌─ orchestrator-main [claude] ─ Working ────────────────┐${C_RESET}  "
printf "${C_YELLOW}┌─ backend-main [codex] ─ Working ──────────────────────┐${C_RESET}\n"

printf "${C_CYAN}│${C_RESET} ${C_DIM}TASK-003 · coordinate team handoff${C_RESET}                     ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET} ${C_DIM}TASK-001 · implement user REST API${C_RESET}                    ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}                                                       ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}                                                       ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET} ${C_OK}→${C_RESET} Reading team channel...                            ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET} ${C_OK}✓${C_RESET} internal/handler/users.go                          ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET} ${C_DIM}backend-main: step 3/5 done${C_RESET}                           ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET} ${C_OK}✓${C_RESET} internal/service/users.go                          ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET} ${C_DIM}frontend-main: dashboard merged${C_RESET}                       ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET} ${C_WARN}○${C_RESET} ${C_DIM}internal/db/migrations/...${C_RESET}                     ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}                                                       ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}                                                       ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET} ${C_OK}\$${C_RESET} ${C_DIM}aom message send reviewer-main \033[3m\"review TASK-001\"${C_RESET}  ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET} ${C_OK}\$${C_RESET} ${C_DIM}git commit -m \"[TASK-001] add user service\"${C_RESET}       ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET} ${C_DIM}message queued → reviewer-main ✓${C_RESET}                      ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET} ${C_DIM}[main a3f9c21] [TASK-001] add user service${C_RESET}            ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}                                                       ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}                                                       ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET} ${C_BOLD}▌${C_RESET}                                                     ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET} ${C_BOLD}▌${C_RESET}                                                     ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}└───────────────────────────────────────────────────────┘${C_RESET}  "
printf "${C_YELLOW}└───────────────────────────────────────────────────────┘${C_RESET}\n\n"

# Bottom-left: frontend
printf "${C_GREEN}┌─ frontend-main [claude] ─ Working ────────────────────┐${C_RESET}  "
printf "${C_MAGENTA}┌─ reviewer-main [claude] ─ Working ────────────────────┐${C_RESET}\n"

printf "${C_GREEN}│${C_RESET} ${C_DIM}TASK-002 · build dashboard UI${C_RESET}                         ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET} ${C_DIM}TASK-001 review · handoff received${C_RESET}                   ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}                                                       ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}                                                       ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET} ${C_OK}✓${C_RESET} src/components/Dashboard.tsx                        ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET} ${C_OK}[PASS]${C_RESET} commit tagged [TASK-001] ✓                      ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET} ${C_OK}✓${C_RESET} src/hooks/useAgents.ts                              ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET} ${C_OK}[PASS]${C_RESET} handoff.md present ✓                            ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET} ${C_WARN}○${C_RESET} ${C_DIM}src/features/sessions/...${C_RESET}                      ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET} ${C_OK}[PASS]${C_RESET} all 5 steps completed ✓                         ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}                                                       ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}                                                       ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET} ${C_OK}\$${C_RESET} ${C_DIM}aom task signal TASK-002 step.completed${C_RESET}            ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET} ${C_OK}\$${C_RESET} ${C_DIM}aom message send orchestrator-main \033[3m\"done\"${C_RESET}    ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET} ${C_DIM}event logged → .agent/log.md ✓${C_RESET}                        ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET} ${C_DIM}message sent → orchestrator-main ✓${C_RESET}                   ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}                                                       ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}                                                       ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET} ${C_BOLD}▌${C_RESET}                                                     ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET} ${C_BOLD}▌${C_RESET}                                                     ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}└───────────────────────────────────────────────────────┘${C_RESET}  "
printf "${C_MAGENTA}└───────────────────────────────────────────────────────┘${C_RESET}\n\n"

hr
printf "${C_DIM}  1 Working (orchestrator)  ·  2 Working (tasks)  ·  1 Working (review)  ·  aom switch <name> to jump to any pane${C_RESET}\n"
sleep 4

# ── SCENE 4: operator dashboard ───────────────────────────────────
clear
type_cmd "status --action-items"
sleep 0.3
$AOM status --action-items 2>/dev/null
sleep 3

# ── SCENE 5: pipeline command ─────────────────────────────────────
clear
type_cmd "task verify TASK-001"
sleep 0.3
$AOM task verify TASK-001 2>/dev/null || printf "${C_OK}[PASS]${C_RESET} tagged commit found\n${C_OK}[PASS]${C_RESET} task.completed event in log\n${C_OK}[PASS]${C_RESET} handoff.md present\n\nAll checks passed. Ready to accept.\n"
sleep 3

clear
printf "${C_BOLD}${C_GREEN}  AOM — one operator, a full team of AI agents${C_RESET}\n"
printf "${C_DIM}  github.com/lattapon-aek/agent-orchestrator-management${C_RESET}\n\n"
sleep 2
