# lburgazzolim

A personal marketplace for Claude Code skills, hooks, and MCP servers.

## Overview

This repository serves as a centralized marketplace for custom Claude Code extensions:

- **Skills**: Custom commands and workflows for Claude Code
- **Hooks**: Event-driven scripts that respond to Claude Code events
- **MCP Servers**: Model Context Protocol servers that extend Claude's capabilities

## Quick Start

### List Available Plugins

```bash
./scripts/list-plugins.sh
```

### Install a Skill

```bash
./scripts/install-skill.sh skills/example-skill.md
```

Then use it in Claude Code:

```
/example-skill
```

### Uninstall a Skill

```bash
./scripts/uninstall-skill.sh example-skill
```

## Directory Structure

```
.
├── skills/              # Custom Claude Code skills
├── hooks/               # Event-driven hooks
├── mcp-servers/         # MCP server configurations
├── scripts/             # Installation and management scripts
├── marketplace.json     # Marketplace metadata
└── registry.json        # Plugin registry
```

## Creating Your Own Plugins

### Skills

1. Create a new markdown file in `skills/`
2. Follow the structure in `skills/example-skill.md`
3. Add an entry to `registry.json`
4. Install with `./scripts/install-skill.sh`

See [skills/README.md](skills/README.md) for details.

### Hooks

1. Create a shell script in `hooks/`
2. Make it executable: `chmod +x hooks/your-hook.sh`
3. Configure in `~/.claude/settings.json`

See [hooks/README.md](hooks/README.md) for details.

### MCP Servers

1. Create a configuration file in `mcp-servers/`
2. Document installation and setup
3. Configure in `~/.claude/settings.json`

See [mcp-servers/README.md](mcp-servers/README.md) for details.

## Registry Format

The `registry.json` file tracks all available plugins:

```json
{
  "plugins": [
    {
      "name": "plugin-name",
      "type": "skill",
      "path": "skills/plugin-name.md",
      "description": "What the plugin does",
      "version": "1.0.0",
      "category": "utilities",
      "author": "lburgazzoli",
      "installPath": "~/.claude/skills/plugin-name.md"
    }
  ]
}
```

## Contributing

This is a personal marketplace, but feel free to fork and create your own!

## License

See [LICENSE](LICENSE) file for details.
