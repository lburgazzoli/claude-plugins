# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**lburgazzoli** is a personal marketplace for Claude Code extensions including skills, hooks, and MCP servers. It provides a structured way to manage and distribute custom Claude Code plugins.

## Repository Structure

```
├── .claude-plugin/
│   ├── plugin.json              # Plugin manifest (name, version, author)
│   └── marketplace.json         # Marketplace catalog
├── skills/                      # Claude Code skill definitions (directory per skill with SKILL.md)
├── hooks/                       # Event-driven hook scripts (hooks.json when populated)
├── scripts/                     # Helper scripts (Python, shell) for skill validation and tooling
├── CLAUDE.md
└── README.md
```

## Core Architecture

### Plugin System

This repo serves as both a **marketplace** (`.claude-plugin/marketplace.json`) and a **plugin** (`.claude-plugin/plugin.json`). The marketplace references the plugin via `"source": "./"`.

- **Plugin manifest** (`.claude-plugin/plugin.json`): Defines the plugin identity. Only `name` is required.
- **Marketplace catalog** (`.claude-plugin/marketplace.json`): Lists available plugins with `name`, `owner`, and `plugins` array.
- Components (skills) in default directories are **auto-discovered** — no need to list them explicitly.

### Skills

Skills are directories containing a `SKILL.md` file with YAML frontmatter. They:
- Live in the `skills/` directory (e.g., `skills/testcontainers/SKILL.md`)
- Are auto-discovered from the default directory

### Hooks

Hooks are configured via `hooks/hooks.json`. They:
- Live in the `hooks/` directory
- Must be executable (`chmod +x`)
- Respond to events like tool calls, prompt submissions, etc.

### Scripts

Helper scripts for skill validation, evaluation, and tooling live in the `scripts/` directory. These can be Python, shell, or any language appropriate for the task.

### MCP Servers

MCP servers are configured via `.mcp.json` at the plugin root. They:
- Extend Claude Code with custom tools and context
- Can be written in any language (Node.js, Python, etc.)

## Adding New Plugins

### Adding a Skill

1. Create `skills/your-skill/SKILL.md` with YAML frontmatter (`name`, `description`)
2. Test with `claude --plugin-dir .`
3. Verify with `/reload-plugins`

### Adding a Hook

1. Create the hook script in `hooks/`
2. Make executable: `chmod +x hooks/your-hook.sh`
3. Add configuration to `hooks/hooks.json`

### Adding an MCP Server

1. Create `.mcp.json` at the plugin root
2. Define server configuration with command and args

## Development Guidelines

- **Repository cloning**: when a repo needs to be cloned, always clone into `.context/repos/<repo-name>` with `--depth 1 --single-branch` (default branch only, shallow clone)
- Keep skills focused on a single task
- Document all environment variables and dependencies
- Test with `claude --plugin-dir .` before committing
- Use `/reload-plugins` to hot-reload during development
- Use `jq` instead of Python for JSON manipulation in scripts and tool invocations when possible
