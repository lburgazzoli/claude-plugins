# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**lburgazzolim** is a personal marketplace for Claude Code extensions including skills, hooks, and MCP servers. It provides a structured way to manage and distribute custom Claude Code plugins.

## Repository Structure

```
├── skills/              # Claude Code skill definitions (markdown files)
├── hooks/               # Event-driven hook scripts
├── mcp-servers/         # MCP server configurations and documentation
├── scripts/             # Management utilities (install, uninstall, list)
├── marketplace.json     # Marketplace metadata and configuration
├── registry.json        # Central registry of all available plugins
└── README.md           # User-facing documentation
```

## Core Architecture

### Plugin Registry System

The `registry.json` file is the source of truth for all available plugins. Each entry contains:
- `name`: Unique identifier
- `type`: Plugin type (skill, hook, mcp-server)
- `path`: Relative path to the plugin file
- `description`: What the plugin does
- `version`: Semantic version
- `category`: Organization category
- `author`: Plugin author
- `installPath`: Where the plugin gets installed

### Skills

Skills are markdown files that contain prompts/instructions for Claude Code. They:
- Live in the `skills/` directory
- Follow a structured format (metadata, usage, implementation)
- Get installed to `~/.claude/skills/`
- Are invoked using `/skill-name` in Claude Code

### Hooks

Hooks are shell scripts that execute on Claude Code events. They:
- Live in the `hooks/` directory
- Must be executable (`chmod +x`)
- Are configured in `~/.claude/settings.json`
- Respond to events like tool calls, prompt submissions, etc.

### MCP Servers

MCP servers extend Claude Code with custom tools and context. They:
- Have configuration documented in `mcp-servers/`
- Are configured in `~/.claude/settings.json` under `mcpServers`
- Can be written in any language (Node.js, Python, etc.)

## Common Commands

### List all available plugins
```bash
./scripts/list-plugins.sh
```

### Install a skill
```bash
./scripts/install-skill.sh skills/<skill-name>.md
```

### Uninstall a skill
```bash
./scripts/uninstall-skill.sh <skill-name>
```

### Make scripts executable (if needed)
```bash
chmod +x scripts/*.sh
```

## Adding New Plugins

### Adding a Skill

1. Create `skills/your-skill.md` following the example-skill structure
2. Add entry to `registry.json` in the `plugins` array
3. Test installation: `./scripts/install-skill.sh skills/your-skill.md`
4. Verify in Claude Code: `/your-skill`

### Adding a Hook

1. Create `hooks/your-hook.sh` with proper shebang
2. Make executable: `chmod +x hooks/your-hook.sh`
3. Document the hook's purpose and event trigger
4. Add entry to `registry.json` in the `hooks` array
5. Provide installation instructions in the hook's README

### Adding an MCP Server

1. Document server configuration in `mcp-servers/`
2. Include setup instructions and environment variables
3. Add entry to `registry.json` in the `mcpServers` array
4. Provide example settings.json configuration

## Development Guidelines

- Keep skills focused on a single task
- Document all environment variables and dependencies
- Version all plugins semantically
- Update registry.json when adding/modifying plugins
- Test installations before committing
- Skills should be self-contained and portable

## File Formats

### Skill Structure
```markdown
# Skill Name

Description

## Metadata
- **Name**: skill-name
- **Description**: What it does
- **Category**: category
- **Version**: x.y.z

## Usage
/skill-name [args]

## Implementation
[Claude instructions]
```

### Registry Entry
```json
{
  "name": "plugin-name",
  "type": "skill|hook|mcp-server",
  "path": "category/plugin-name.ext",
  "description": "Brief description",
  "version": "1.0.0",
  "category": "utilities",
  "author": "lburgazzoli",
  "installPath": "~/.claude/skills/plugin-name.md"
}
```
