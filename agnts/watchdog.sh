#!/bin/bash
#
# Watchdog — periodically nudges idle coder/reviewer agents
#
# Usage:
#   ./agnts/watchdog.sh         # Run in foreground
#   ./agnts/watchdog.sh &       # Run in background
#   ./agnts/watchdog.sh stop    # Kill running watchdog
#

set -euo pipefail

SESSION="oci-agents"
INTERVAL="${WATCHDOG_INTERVAL:-60}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
DIM='\033[2m'
NC='\033[0m'

log() { echo -e "${GREEN}[watchdog]${NC} $(date '+%H:%M:%S') $1"; }

# Stop mode
if [ "${1:-}" = "stop" ]; then
    pkill -f "agnts/watchdog.sh" 2>/dev/null && echo "Watchdog stopped." || echo "No watchdog running."
    exit 0
fi

# Verify tmux session exists
if ! tmux has-session -t "$SESSION" 2>/dev/null; then
    echo "Error: tmux session '$SESSION' not found. Launch agents first."
    exit 1
fi

log "Started — nudging coder/reviewer every ${INTERVAL}s"
log "${DIM}Ctrl+C to stop${NC}"
echo ""

while true; do
    sleep "$INTERVAL"

    # Nudge coder (pane 1)
    if tmux list-panes -t "$SESSION:agents" -F '#{pane_index}' 2>/dev/null | grep -q '^1$'; then
        tmux send-keys -l -t "$SESSION:agents.1" "If you have an ongoing task, continue with it. Otherwise check beads for new work."
        sleep 0.2
        tmux send-keys -t "$SESSION:agents.1" Enter
        log "Nudged ${YELLOW}coder${NC}"
    fi

    # Nudge reviewer (pane 2)
    if tmux list-panes -t "$SESSION:agents" -F '#{pane_index}' 2>/dev/null | grep -q '^2$'; then
        tmux send-keys -l -t "$SESSION:agents.2" "If you are mid-review, continue with it. Otherwise check beads for new review."
        sleep 0.2
        tmux send-keys -t "$SESSION:agents.2" Enter
        log "Nudged ${YELLOW}reviewer${NC}"
    fi
done
