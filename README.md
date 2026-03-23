# lburgazzoli

A personal marketplace for Claude Code skills, hooks, and MCP servers.

## Overview

This repository serves as a centralized marketplace for custom Claude Code extensions:

- **Skills**: Contextual knowledge and instructions for specific tools/frameworks
- **Hooks**: Event-driven scripts that respond to Claude Code events
- **MCP Servers**: Model Context Protocol servers that extend Claude's capabilities

## Installation

Install the marketplace plugin:

```bash
claude plugin install <repo-url>
```

Or test locally:

```bash
claude --plugin-dir /path/to/claude-plugins
```

## Available Skills

| Skill | Description |
|-------|-------------|
| `testcontainers` | Guidance for running tests with Testcontainers and Podman/Docker |

## Directory Structure

```
.
├── .claude-plugin/
│   ├── plugin.json          # Plugin manifest
│   └── marketplace.json     # Marketplace catalog
├── skills/                  # Skill definitions (auto-discovered)
├── hooks/                   # Hook scripts (optional)
├── CLAUDE.md               # Project instructions for Claude Code
└── README.md
```

## Creating New Extensions

### Skills

Create a directory in `skills/` with a `SKILL.md` file:

```markdown
---
name: my-skill
description: What the skill provides
---

Skill content and instructions...
```

### Hooks

Add hook scripts to `hooks/` and configure in `hooks/hooks.json`.

### MCP Servers

Configure in `.mcp.json` at the repository root.

## Development

Test changes locally:

```bash
claude --plugin-dir .
```

Reload after changes:

```
/reload-plugins
```

## License

See [LICENSE](LICENSE) file for details.
