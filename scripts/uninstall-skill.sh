#!/bin/bash

# Uninstall a skill from Claude Code

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <skill-name>"
    echo "Example: $0 example-skill"
    exit 1
fi

SKILL_NAME="$1"
CLAUDE_SKILLS_DIR="$HOME/.claude/skills"
SKILL_PATH="$CLAUDE_SKILLS_DIR/${SKILL_NAME}.md"

if [ ! -f "$SKILL_PATH" ]; then
    echo "Error: Skill not found: $SKILL_PATH"
    exit 1
fi

rm "$SKILL_PATH"
echo "✓ Skill uninstalled successfully: $SKILL_NAME"
