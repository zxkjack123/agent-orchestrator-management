#!/bin/bash
# AOM CLI demo script for asciinema recording
AOM="${AOM_BIN:-$(command -v aom 2>/dev/null || echo "$(cd "$(dirname "$0")/.." && pwd)/aom")}"
cd "${AOM_PROJECT_DIR:-$(pwd)}"

# Fake typing effect
type_cmd() {
  printf "\e[1;32m\$\e[0m aom "
  echo -n "$1" | while IFS= read -r -n1 c; do
    printf "%s" "$c"
    sleep 0.04
  done
  echo ""
  sleep 0.3
}

clear
printf "\e[1;36m# AOM — Agent Orchestrator Management\e[0m\n"
printf "\e[2m# project: test-aom-001\e[0m\n\n"
sleep 1.5

# --- status ---
type_cmd "status"
$AOM status 2>/dev/null
sleep 2.5

clear
# --- agent list ---
type_cmd "agent list"
$AOM agent list 2>/dev/null
sleep 2.5

clear
# --- role list ---
type_cmd "role list"
$AOM role list 2>/dev/null
sleep 2.5

clear
# --- class list ---
type_cmd "class list"
$AOM class list 2>/dev/null
sleep 2.5

clear
# --- session list (recent 8) ---
type_cmd "session list --active"
$AOM session list 2>/dev/null | head -18
sleep 2.5

clear
# --- channel read (top messages) ---
type_cmd "channel read"
$AOM channel read 2>/dev/null | head -40
sleep 3

clear
# --- doctor ---
type_cmd "doctor"
$AOM doctor 2>/dev/null
sleep 2

clear
printf "\e[1;32m# AOM — manage your AI agent team from one place\e[0m\n"
printf "\e[2m  github.com/lattapon-aek/agent-orchestrator-management\e[0m\n\n"
sleep 2
