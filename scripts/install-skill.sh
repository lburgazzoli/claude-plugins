#!/bin/bash

# Install a skill from this marketplace to Claude Code

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <skill-file>"
    echo "Example: $0 skills/example-skill.md"
    exit 1
fi

SKILL_FILE="$1"
SKILL_NAME=$(basename "$SKILL_FILE")
CLAUDE_SKILLS_DIR="$HOME/.claude/skills"

if [ ! -f "$SKILL_FILE" ]; then
    echo "Error: Skill file not found: $SKILL_FILE"
    exit 1
fi

# Create skills directory if it doesn't exist
mkdir -p "$CLAUDE_SKILLS_DIR"

# Copy skill file
cp "$SKILL_FILE" "$CLAUDE_SKILLS_DIR/$SKILL_NAME"

echo "✓ Skill installed successfully: $SKILL_NAME"
echo "Location: $CLAUDE_SKILLS_DIR/$SKILL_NAME"
echo ""
echo "You can now use this skill in Claude Code with:"
echo "  /${SKILL_NAME%.md}"
