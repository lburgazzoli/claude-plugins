---
name: Resolve Repos and Build Inventories
description: Clone/update repos, read architecture docs, build raw resource inventories
scope: [static]
state-status: repos_resolved
---

# Resolve Repos and Build Inventories

The run directory already exists and state is initialized (done by the orchestrator before the step loop). The run directory path is in `state.yaml`.

Spawn a single Opus agent to execute Phase 1 of the context builder:

```
Agent(
  description="RHOAI upgrade assessment — context builder phase 1",
  model="opus",
  prompt="You are building the resource inventory for an RHOAI upgrade assessment.

Read these files in this order:
1. ${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md — read Shared Rules + Phase 1 only
2. The project CLAUDE.md at the vault root — for repository cloning and refs rules

Follow Phase 1 instructions to:
- Clean stale clones, update/clone repos
- Clone version-specific odh-gitops branches
- Resolve architecture directories
- Read PLATFORM.md files, compute Component Diff
- Build all raw resource inventories
- Write context-draft.md and empty discrepancies.yaml to the run directory

Run directory: {run_dir}
Source version: {source}
Target version: {target}"
)
```

After the agent returns, verify `context-draft.md` exists in the run directory.
