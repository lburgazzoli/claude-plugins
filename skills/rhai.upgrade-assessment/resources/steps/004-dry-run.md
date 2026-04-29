---
name: Handle Dry Run
description: If --dry-run is set, print context and stop without spawning personas
scope: [static]
state-status: dry_run_complete
requires: [context.md]
stop: true
stop-condition: dry-run
---

# Handle Dry Run

If `--dry-run` is set:
- Print the `context.md` content to the user
- Print which personas would run

Stop enforcement is machine-enforced: `steps.py next --flags dry-run` returns `done` after this step completes, preventing the loop from continuing.

If `--dry-run` is **not** set, this step is a no-op. `steps.py next` (without the `dry-run` flag) will continue to the next step.
