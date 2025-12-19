#!/bin/bash

# List all available plugins in the marketplace

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
REGISTRY_FILE="$REPO_DIR/registry.json"

if [ ! -f "$REGISTRY_FILE" ]; then
    echo "Error: Registry file not found: $REGISTRY_FILE"
    exit 1
fi

echo "=== Claude Plugin Marketplace ==="
echo ""

# Check if jq is available
if command -v jq &> /dev/null; then
    echo "Skills:"
    jq -r '.plugins[] | select(.type=="skill") | "  - \(.name) (v\(.version)): \(.description)"' "$REGISTRY_FILE"

    echo ""
    echo "Hooks:"
    jq -r '.hooks[] | "  - \(.name) (v\(.version)): \(.description)"' "$REGISTRY_FILE" 2>/dev/null || echo "  (none)"

    echo ""
    echo "MCP Servers:"
    jq -r '.mcpServers[] | "  - \(.name) (v\(.version)): \(.description)"' "$REGISTRY_FILE" 2>/dev/null || echo "  (none)"
else
    echo "Install 'jq' for formatted output, or view registry.json manually"
    cat "$REGISTRY_FILE"
fi
