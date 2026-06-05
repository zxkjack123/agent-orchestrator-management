#!/bin/bash
# AOM Quick Start demo — shows real project with real output
# Record: asciinema rec --cols 110 --rows 40 --command "bash scripts/demo-quickstart.sh" /tmp/demo-qs.cast
# AOM binary: use AOM_BIN env var, or look in PATH, or fall back to ../aom relative to this script
AOM="${AOM_BIN:-$(command -v aom 2>/dev/null || echo "$(cd "$(dirname "$0")/.." && pwd)/aom")}"
# Project dir: use AOM_PROJECT_DIR env var, or current working directory
PROJECT_DIR="${AOM_PROJECT_DIR:-$(pwd)}"

C_RESET="\033[0m"; C_BOLD="\033[1m"; C_DIM="\033[2m"
C_GREEN="\033[0;32m"; C_CYAN="\033[1;36m"; C_YELLOW="\033[1;33m"
C_MAGENTA="\033[1;35m"; C_BLUE="\033[1;34m"

prompt() { printf "\n${C_GREEN}\$${C_RESET} ${C_BOLD}aom $1${C_RESET}\n"; sleep 0.4; }
hr()     { printf "${C_DIM}$(printf '─%.0s' $(seq 1 108))${C_RESET}\n"; }

cd "$PROJECT_DIR"
clear

# ── INTRO ────────────────────────────────────────────────────────
printf "${C_BOLD}${C_BLUE}  AOM — Agent Orchestrator Management${C_RESET}\n"
printf "${C_DIM}  Coordinate a team of AI agents from the terminal${C_RESET}\n\n"
sleep 1.5

# ── SCENE 1: aom status ──────────────────────────────────────────
clear
prompt "status"
$AOM status 2>/dev/null
sleep 3.5

# ── SCENE 2: aom channel read ────────────────────────────────────
clear
prompt "channel read"
$AOM channel read 2>/dev/null
sleep 3.5

# ── SCENE 3: aom task list ───────────────────────────────────────
clear
prompt "task list"
$AOM task list 2>/dev/null
sleep 2.5

# ── SCENE 4: team grid (aom team view → tiled tmux layout) ───────
clear
printf "${C_BOLD}${C_GREEN}\$${C_RESET} ${C_BOLD}aom team view${C_RESET}\n"
printf "${C_DIM}  Opening tiled tmux layout (iTerm2 native panes via tmux -CC)...${C_RESET}\n"
sleep 0.8

printf "\n"
hr

printf "${C_BOLD}${C_BLUE}  AOM War Room${C_RESET}  "
printf "${C_DIM}test-aom-004  ·  4 agents  ·  tmux tiled layout${C_RESET}\n"
hr
printf "\n"

# Row 1
printf "${C_CYAN}┌─ orchestrator-main  [claude · orchestrator] ────── Idle ──────────┐${C_RESET}  "
printf "${C_YELLOW}┌─ backend-main  [codex · backend] ───────────── Done ──────────────┐${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  Idle — no task assigned — standby                          ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  TASK-001 · GET /hello API → Hello World              ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}                                                             ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}                                                        ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  \033[0;32m→\033[0m aom channel read                                   ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  \033[0;32m✓\033[0m GET /hello route implemented                 ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  \033[2mbackend-main: task DONE ✓\033[0m                             ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  \033[0;32m✓\033[0m npm test passes                               ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  \033[2mawaiting next task from operator\033[0m                    ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  \033[2m[main a3f9c21] [TASK-001]\033[0m                           ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}                                                             ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}                                                        ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  \033[0;32m\$\033[0m \033[2maom message send backend-main \"assign TASK-002\"\033[0m    ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  \033[0;32m\$\033[0m \033[2mgit commit -m \"[TASK-001] add /hello\"\033[0m         ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                   ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                   ${C_YELLOW}│${C_RESET}\n"
printf "${C_CYAN}└─────────────────────────────────────────────────────────────────────┘${C_RESET}  "
printf "${C_YELLOW}└──────────────────────────────────────────────────────────────────┘${C_RESET}\n\n"

# Row 2
printf "${C_GREEN}┌─ frontend-main  [claude · frontend] ─────────── Idle ─────────────┐${C_RESET}  "
printf "${C_MAGENTA}┌─ reviewer-main  [claude · reviewer] ─────────── Idle ─────────────┐${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  Idle — no task assigned — standby                          ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  Idle — awaiting review request                       ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}                                                             ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}                                                        ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  \033[0;32m→\033[0m aom message read                                   ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  \033[0;32m→\033[0m aom message read                             ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  \033[2mMailbox empty. Waiting for task.\033[0m                    ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  \033[2mMailbox empty. Waiting for review.\033[0m              ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}                                                             ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}                                                        ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  \033[0;32m\$\033[0m \033[2maom channel read\033[0m                                   ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  \033[0;32m\$\033[0m \033[2maom channel read\033[0m                             ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                   ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                   ${C_MAGENTA}│${C_RESET}\n"
printf "${C_GREEN}└─────────────────────────────────────────────────────────────────────┘${C_RESET}  "
printf "${C_MAGENTA}└──────────────────────────────────────────────────────────────────┘${C_RESET}\n\n"

hr
printf "${C_DIM}  aom switch <name>   jump to agent pane  ·  aom status   team overview  ·  aom channel read   team log${C_RESET}\n"
sleep 5
