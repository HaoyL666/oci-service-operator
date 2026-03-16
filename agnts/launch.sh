#!/bin/bash
#
# Agent Launch Script
# Launches 3 Codex agents in tmux: planner, coder, reviewer
#
# Usage:
#   ./agnts/launch.sh                  # Launch all 3 agents (single planner)
#   ./agnts/launch.sh multi            # Launch all 3 agents (multi-agent planner with sub-agents)
#   ./agnts/launch.sh planner          # Launch only planner (single)
#   ./agnts/launch.sh planner-multi    # Launch only planner (multi-agent orchestrator)
#   ./agnts/launch.sh planner_draft    # Launch only planner_draft (standalone)
#   ./agnts/launch.sh planner_review   # Launch only planner_review (standalone)
#   ./agnts/launch.sh coder            # Launch only coder
#   ./agnts/launch.sh reviewer         # Launch only reviewer
#

set -euo pipefail

PROJECT_DIR="/Users/ethan/Desktop/projects/oci-service-operator"
SESSION="oci-agents"
PROFILE="${CODEX_PROFILE:-gpt-5-4}"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

log() { echo -e "${GREEN}[agents]${NC} $1"; }
warn() { echo -e "${YELLOW}[agents]${NC} $1"; }

banner() {
    echo ""
    echo -e "${CYAN}${BOLD}"
    echo "    ╔═══════════════════════════════════════════════════╗"
    echo "    ║                                                   ║"
    echo "    ║        🤖  OCI Service Operator Agents  🤖        ║"
    echo "    ║                                                   ║"
    echo -e "    ╚═══════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "    ${DIM}Session:${NC}  ${BOLD}$SESSION${NC}"
    echo -e "    ${DIM}Profile:${NC}  ${BOLD}$PROFILE${NC}"
    echo -e "    ${DIM}Project:${NC}  ${BOLD}$(basename "$PROJECT_DIR")${NC}"
    echo ""
}

countdown() {
    local secs=$1
    for ((i=secs; i>0; i--)); do
        printf "\r    ${YELLOW}⏳ Launching agents in ${BOLD}%d${NC}${YELLOW}s...${NC}  " "$i"
        sleep 1
    done
    printf "\r    ${GREEN}🚀 Launching agents now!${NC}          \n"
    echo ""
}

command -v tmux >/dev/null 2>&1 || { echo "tmux is required"; exit 1; }
command -v codex >/dev/null 2>&1 || { echo "codex is required"; exit 1; }
command -v bd >/dev/null 2>&1 || { echo "bd is required"; exit 1; }

start_dolt() {
    if bd list --all >/dev/null 2>&1; then
        return 0
    fi
    log "Starting beads Dolt server..."
    bd dolt start 2>&1 || true
    sleep 1
}

ROLE="${1:-all}"

case "$ROLE" in
    all)
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            warn "Killing existing session '$SESSION'..."
            tmux kill-session -t "$SESSION"
        fi

        banner
        start_dolt

        # Create session with 3 panes (planner on top, coder+reviewer on bottom)
        tmux new-session -d -s "$SESSION" -n agents -c "$PROJECT_DIR"
        tmux set-option -t "$SESSION" pane-border-status top 2>/dev/null || true
        tmux set-option -t "$SESSION" pane-border-format " #{pane_title} " 2>/dev/null || true

        tmux split-window -v -t "$SESSION:agents" -c "$PROJECT_DIR"
        tmux split-window -h -t "$SESSION:agents.1" -c "$PROJECT_DIR"

        tmux select-pane -t "$SESSION:agents.0" -T "PLANNER"
        tmux select-pane -t "$SESSION:agents.1" -T "CODER"
        tmux select-pane -t "$SESSION:agents.2" -T "REVIEWER"

        echo -e "    ${DIM}Agents:${NC}"
        echo -e "    ${MAGENTA}◆${NC} ${BOLD}Planner${NC}   ${DIM}— designs + creates beads directly${NC}"
        echo -e "    ${GREEN}◆${NC} ${BOLD}Coder${NC}     ${DIM}— implements + submits for review${NC}"
        echo -e "    ${BLUE}◆${NC} ${BOLD}Reviewer${NC}  ${DIM}— quality gates + approve/reject${NC}"
        echo ""

        countdown 3

        echo -e "    ${MAGENTA}▸${NC} Starting ${BOLD}Planner${NC}..."
        tmux send-keys -t "$SESSION:agents.0" "codex -s danger-full-access -p $PROFILE 'You are the PLANNER agent. Read agnts/roles/planner-single.md FIRST, then wait for me to give you a task.'" Enter
        echo -e "    ${GREEN}▸${NC} Starting ${BOLD}Coder${NC}..."
        tmux send-keys -t "$SESSION:agents.1" "codex -s danger-full-access -p $PROFILE 'You are the CODER agent. Read agnts/roles/coder.md FIRST. Then run bd ready --json to find work. If no work, wait 30s and check again. Begin now.'" Enter
        echo -e "    ${BLUE}▸${NC} Starting ${BOLD}Reviewer${NC}..."
        tmux send-keys -t "$SESSION:agents.2" "codex -s danger-full-access -p $PROFILE 'You are the REVIEWER agent. Read agnts/roles/reviewer.md FIRST. Then check bd list --label=needs-review --json. If nothing to review, wait 30s and check again. Begin now.'" Enter

        tmux select-pane -t "$SESSION:agents.0"

        echo ""
        echo -e "    ${GREEN}${BOLD}✓ All agents launched!${NC}"
        echo ""
        echo -e "    ${CYAN}┌───────────────────────────────────────┐${NC}"
        echo -e "    ${CYAN}│${NC}${MAGENTA}${BOLD}           PLANNER                     ${NC}${CYAN}│${NC}"
        echo -e "    ${CYAN}├───────────────────┬───────────────────┤${NC}"
        echo -e "    ${CYAN}│${NC}${GREEN}${BOLD}      CODER        ${NC}${CYAN}│${NC}${BLUE}${BOLD}    REVIEWER       ${NC}${CYAN}│${NC}"
        echo -e "    ${CYAN}└───────────────────┴───────────────────┘${NC}"
        echo ""
        echo -e "    ${DIM}Keybindings:${NC}"
        echo -e "    ${YELLOW}Ctrl+b ↑/↓/←/→${NC}  switch panes"
        echo -e "    ${YELLOW}Ctrl+b z${NC}        zoom/unzoom pane"
        echo -e "    ${YELLOW}Ctrl+b d${NC}        detach session"
        echo ""
        echo -e "    ${DIM}Attaching to tmux session...${NC}"
        sleep 1

        tmux attach -t "$SESSION"
        ;;
    multi)
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            warn "Killing existing session '$SESSION'..."
            tmux kill-session -t "$SESSION"
        fi

        banner
        start_dolt

        # Create session with 3 panes (planner orchestrator on top, coder+reviewer on bottom)
        # Planner uses Codex multi-agent internally (planner_draft + planner_review sub-agents)
        tmux new-session -d -s "$SESSION" -n agents -c "$PROJECT_DIR"
        tmux set-option -t "$SESSION" pane-border-status top 2>/dev/null || true
        tmux set-option -t "$SESSION" pane-border-format " #{pane_title} " 2>/dev/null || true

        tmux split-window -v -t "$SESSION:agents" -c "$PROJECT_DIR"
        tmux split-window -h -t "$SESSION:agents.1" -c "$PROJECT_DIR"

        tmux select-pane -t "$SESSION:agents.0" -T "PLANNER (multi)"
        tmux select-pane -t "$SESSION:agents.1" -T "CODER"
        tmux select-pane -t "$SESSION:agents.2" -T "REVIEWER"

        echo -e "    ${DIM}Agents (multi-agent planner mode):${NC}"
        echo -e "    ${MAGENTA}◆${NC} ${BOLD}Planner${NC}   ${DIM}— orchestrates draft + review sub-agents${NC}"
        echo -e "    ${GREEN}◆${NC} ${BOLD}Coder${NC}     ${DIM}— implements + submits for review${NC}"
        echo -e "    ${BLUE}◆${NC} ${BOLD}Reviewer${NC}  ${DIM}— quality gates + approve/reject${NC}"
        echo ""

        countdown 3

        echo -e "    ${MAGENTA}▸${NC} Starting ${BOLD}Planner${NC} (orchestrates draft + review sub-agents)..."
        tmux send-keys -t "$SESSION:agents.0" "codex -s danger-full-access -p $PROFILE 'You are the PLANNER orchestrator. Read agnts/roles/planner.md FIRST, then wait for me to give you a task. You will spawn planner_draft and planner_review sub-agents.'" Enter
        echo -e "    ${GREEN}▸${NC} Starting ${BOLD}Coder${NC}..."
        tmux send-keys -t "$SESSION:agents.1" "codex -s danger-full-access -p $PROFILE 'You are the CODER agent. Read agnts/roles/coder.md FIRST. Then run bd ready --json to find work. If no work, wait 30s and check again. Begin now.'" Enter
        echo -e "    ${BLUE}▸${NC} Starting ${BOLD}Reviewer${NC}..."
        tmux send-keys -t "$SESSION:agents.2" "codex -s danger-full-access -p $PROFILE 'You are the REVIEWER agent. Read agnts/roles/reviewer.md FIRST. Then check bd list --label=needs-review --json. If nothing to review, wait 30s and check again. Begin now.'" Enter

        tmux select-pane -t "$SESSION:agents.0"

        echo ""
        echo -e "    ${GREEN}${BOLD}✓ All agents launched! (multi-agent planner)${NC}"
        echo ""
        echo -e "    ${CYAN}┌───────────────────────────────────────┐${NC}"
        echo -e "    ${CYAN}│${NC}${MAGENTA}${BOLD}   PLANNER (draft ↔ review sub-agents) ${NC}${CYAN}│${NC}"
        echo -e "    ${CYAN}├───────────────────┬───────────────────┤${NC}"
        echo -e "    ${CYAN}│${NC}${GREEN}${BOLD}      CODER        ${NC}${CYAN}│${NC}${BLUE}${BOLD}    REVIEWER       ${NC}${CYAN}│${NC}"
        echo -e "    ${CYAN}└───────────────────┴───────────────────┘${NC}"
        echo ""
        echo -e "    ${DIM}Attaching to tmux session...${NC}"
        sleep 1

        tmux attach -t "$SESSION"
        ;;
    planner)
        start_dolt
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            tmux new-window -t "$SESSION" -n planner -c "$PROJECT_DIR"
        else
            tmux new-session -d -s "$SESSION" -n planner -c "$PROJECT_DIR"
        fi
        tmux send-keys -t "$SESSION:planner" "codex -s danger-full-access -p $PROFILE 'You are the PLANNER agent. Read agnts/roles/planner-single.md FIRST, then wait for me to give you a task.'" Enter
        tmux attach -t "$SESSION:planner"
        ;;
    planner-multi)
        start_dolt
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            tmux new-window -t "$SESSION" -n planner-multi -c "$PROJECT_DIR"
        else
            tmux new-session -d -s "$SESSION" -n planner-multi -c "$PROJECT_DIR"
        fi
        tmux send-keys -t "$SESSION:planner-multi" "codex -s danger-full-access -p $PROFILE 'You are the PLANNER orchestrator. Read agnts/roles/planner.md FIRST, then wait for me to give you a task. You will spawn planner_draft and planner_review sub-agents.'" Enter
        tmux attach -t "$SESSION:planner-multi"
        ;;
    planner_draft)
        start_dolt
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            tmux new-window -t "$SESSION" -n planner_draft -c "$PROJECT_DIR"
        else
            tmux new-session -d -s "$SESSION" -n planner_draft -c "$PROJECT_DIR"
        fi
        tmux send-keys -t "$SESSION:planner_draft" "codex -s danger-full-access -p $PROFILE 'You are the PLANNER DRAFT agent. Read agnts/roles/planner_draft.md FIRST, then wait for me to give you a task.'" Enter
        tmux attach -t "$SESSION:planner_draft"
        ;;
    planner_review)
        start_dolt
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            tmux new-window -t "$SESSION" -n planner_review -c "$PROJECT_DIR"
        else
            tmux new-session -d -s "$SESSION" -n planner_review -c "$PROJECT_DIR"
        fi
        tmux send-keys -t "$SESSION:planner_review" "codex -s danger-full-access -p $PROFILE 'You are the PLANNER REVIEW agent. Read agnts/roles/planner_review.md FIRST. Then check if agnts/plans/draft.md exists with Status: DRAFT. If not, wait 30s and check again. Begin now.'" Enter
        tmux attach -t "$SESSION:planner_review"
        ;;
    coder)
        start_dolt
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            tmux new-window -t "$SESSION" -n coder -c "$PROJECT_DIR"
        else
            tmux new-session -d -s "$SESSION" -n coder -c "$PROJECT_DIR"
        fi
        tmux send-keys -t "$SESSION:coder" "codex -s danger-full-access -p $PROFILE 'You are the CODER agent. Read agnts/roles/coder.md FIRST. Then run bd ready --json to find work. If no work, wait 30s and check again. Begin now.'" Enter
        tmux attach -t "$SESSION:coder"
        ;;
    reviewer)
        start_dolt
        if tmux has-session -t "$SESSION" 2>/dev/null; then
            tmux new-window -t "$SESSION" -n reviewer -c "$PROJECT_DIR"
        else
            tmux new-session -d -s "$SESSION" -n reviewer -c "$PROJECT_DIR"
        fi
        tmux send-keys -t "$SESSION:reviewer" "codex -s danger-full-access -p $PROFILE 'You are the REVIEWER agent. Read agnts/roles/reviewer.md FIRST. Then check bd list --label=needs-review --json. If nothing to review, wait 30s and check again. Begin now.'" Enter
        tmux attach -t "$SESSION:reviewer"
        ;;
    *)
        echo "Usage: $0 [all|multi|planner|planner-multi|planner_draft|planner_review|coder|reviewer]"
        exit 1
        ;;
esac
