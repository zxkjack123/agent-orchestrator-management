#!/bin/bash
# War Room team grid — instant render, no typing animation
# Designed for screencapture as static image

C_RESET="\033[0m"; C_BOLD="\033[1m"; C_DIM="\033[2m"
C_CYAN="\033[1;36m"; C_YELLOW="\033[1;33m"
C_GREEN="\033[1;32m"; C_MAGENTA="\033[1;35m"
C_BLUE="\033[1;34m"; C_OK="\033[0;32m"; C_WARN="\033[0;33m"

clear

printf "${C_BOLD}${C_BLUE}  AOM War Room${C_RESET}  ${C_DIM}aom orchestrate --layout tiled --real  ·  4 agents active${C_RESET}\n"
printf "${C_DIM}$(printf '─%.0s' $(seq 1 118))${C_RESET}\n\n"

# Row 1
printf "${C_CYAN}┌─ orchestrator-main  [claude · orchestrator] ─── Working ──────────┐${C_RESET}  "
printf "${C_YELLOW}┌─ backend-main  [codex · backend] ────────────── Working ──────────┐${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  TASK-003 · coordinate handoff & assign next tasks             ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  TASK-001 · implement user authentication REST API       ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}                                                               ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}                                                         ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_OK}→${C_RESET} reading team channel...                              ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_OK}✓${C_RESET} internal/handler/auth.go                        ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_DIM}backend-main: TASK-001 step 4/5 complete${C_RESET}              ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_OK}✓${C_RESET} internal/service/auth_service.go                ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_DIM}frontend-main: dashboard component shipped${C_RESET}            ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_WARN}○${C_RESET} internal/db/migrations/001_users.sql            ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_DIM}reviewer-main: review done, ready to merge${C_RESET}            ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}                                                         ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}                                                               ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_OK}\$${C_RESET} ${C_DIM}git add -A && git commit -m \"[TASK-001] auth\"${C_RESET}  ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_OK}\$${C_RESET} ${C_DIM}aom message send reviewer-main \"please review TASK-001\"${C_RESET}   ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_DIM}[main 7dc3f09] [TASK-001] add auth service${C_RESET}       ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_DIM}  → message queued ✓${C_RESET}                                 ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_DIM}  2 files changed, 187 insertions(+)${C_RESET}             ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                     ${C_CYAN}│${C_RESET}  "
printf "${C_YELLOW}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                   ${C_YELLOW}│${C_RESET}\n"

printf "${C_CYAN}└──────────────────────────────────────────────────────────────────────┘${C_RESET}  "
printf "${C_YELLOW}└──────────────────────────────────────────────────────────────────┘${C_RESET}\n\n"

# Row 2
printf "${C_GREEN}┌─ frontend-main  [claude · frontend] ─────────── Working ──────────┐${C_RESET}  "
printf "${C_MAGENTA}┌─ reviewer-main  [claude · reviewer] ─────────── Working ──────────┐${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  TASK-002 · build agent dashboard UI components              ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  TASK-001 review · handoff received from backend-main  ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}                                                               ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}                                                         ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_OK}✓${C_RESET} src/components/AgentCard.tsx                       ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_OK}[PASS]${C_RESET} commit tagged [TASK-001] ✓               ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_OK}✓${C_RESET} src/components/StatusBadge.tsx                     ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_OK}[PASS]${C_RESET} handoff.md present ✓                     ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_OK}✓${C_RESET} src/hooks/useAgents.ts                             ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_OK}[PASS]${C_RESET} all 5 steps completed ✓                  ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_WARN}○${C_RESET} src/features/sessions/SessionsView.tsx              ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_WARN}[WARN]${C_RESET} 2 minor style suggestions                ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}                                                               ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}                                                         ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_OK}\$${C_RESET} ${C_DIM}aom task signal TASK-002 step.completed${C_RESET}            ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_OK}\$${C_RESET} ${C_DIM}aom message send orchestrator-main \"done\"${C_RESET}  ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_DIM}  → event logged to .agent/log.md ✓${C_RESET}                 ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_DIM}  → ready for aom merge prepare${C_RESET}               ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                     ${C_GREEN}│${C_RESET}  "
printf "${C_MAGENTA}│${C_RESET}  ${C_BOLD}▌${C_RESET}                                                   ${C_MAGENTA}│${C_RESET}\n"

printf "${C_GREEN}└──────────────────────────────────────────────────────────────────────┘${C_RESET}  "
printf "${C_MAGENTA}└──────────────────────────────────────────────────────────────────┘${C_RESET}\n\n"

printf "${C_DIM}  aom switch <name>   jump to any pane  ·  aom channel read   team log  ·  aom status --action-items   what needs attention${C_RESET}\n"

# Keep terminal open for screenshot
sleep 30
