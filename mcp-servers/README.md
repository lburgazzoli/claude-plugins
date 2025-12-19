# MCP Servers Directory

This directory contains Model Context Protocol (MCP) server configurations.

## What are MCP Servers?

MCP servers extend Claude Code's capabilities by providing additional tools and context. They can:
- Add custom tools
- Provide domain-specific knowledge
- Integrate with external services

## Structure

MCP servers are configured in `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "server-name": {
      "command": "node",
      "args": ["/path/to/server/index.js"],
      "env": {
        "API_KEY": "your-key"
      }
    }
  }
}
```

## Adding MCP Servers

1. Create a configuration file for your MCP server
2. Document installation steps
3. Include any required environment variables
4. Add to the registry

## Available Servers

Add your MCP server configurations and documentation here.
