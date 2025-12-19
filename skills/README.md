# Skills Directory

This directory contains custom Claude Code skills.

## Skill Structure

Each skill should be a markdown file with:

1. **Metadata section**: Name, description, category, version
2. **Usage section**: How to invoke the skill
3. **Implementation section**: The actual prompt/instructions for Claude

## Creating a New Skill

1. Create a new `.md` file in this directory
2. Follow the structure in `example-skill.md`
3. Add the skill to the marketplace registry
4. Install it to your Claude Code skills directory

## Installation

Skills are installed to `~/.claude/skills/` by default. Use the installation script:

```bash
./scripts/install-skill.sh skills/your-skill.md
```
