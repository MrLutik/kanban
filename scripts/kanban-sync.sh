#!/bin/bash
# Kanban sync script - run via systemd timer
# Usage: kanban-sync.sh [ORG] [REPO|--all]

set -euo pipefail

ORG="${1:-}"
REPO="${2:---all}"
CONFIG="${KANBAN_CONFIG:-$HOME/.kanban.yaml}"
KANBAN="${KANBAN_BIN:-kanban}"

if [[ -z "$ORG" ]]; then
    if [[ -f "$CONFIG" ]]; then
        ORG=$(grep -m1 '^organization:' "$CONFIG" | awk '{print $2}' | tr -d '"')
    fi
fi

if [[ -z "$ORG" ]]; then
    echo "Error: No org specified and none found in config"
    exit 1
fi

$KANBAN sync --org "$ORG" $REPO --config "$CONFIG" --with-timeline
