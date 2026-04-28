---
name: Handle Dry Run
description: If --dry-run is set, print context and stop without spawning personas
scope: [static]
state-status: null
---

# Handle Dry Run

If `--dry-run` is set:
- Print the `context.md` content to the user
- Print which personas would run
- **Stop. Do not proceed to the next step.**
