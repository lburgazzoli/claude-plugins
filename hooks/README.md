# Hooks Directory

This directory contains custom Claude Code hooks.

## What are Hooks?

Hooks are shell commands that execute in response to Claude Code events like:
- Tool calls
- User prompt submissions
- Session starts/ends

## Hook Structure

Hooks are configured in `~/.claude/settings.json` under the `hooks` section.

Example hook configuration:
```json
{
  "hooks": {
    "user-prompt-submit": "echo 'User submitted: {{prompt}}'"
  }
}
```

## Creating Hooks

1. Create a shell script in this directory
2. Make it executable: `chmod +x hooks/your-hook.sh`
3. Document what event it responds to
4. Add installation instructions to registry

## Example Hooks

Add your custom hooks here with documentation on:
- What event triggers them
- What they do
- How to install them
