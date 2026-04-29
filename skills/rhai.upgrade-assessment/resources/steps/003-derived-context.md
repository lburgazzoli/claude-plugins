---
name: Build Derived Context
description: Build interaction map, detect restart triggers, read odh-cli checks, finalize context.md
scope: [static]
state-status: context_complete
requires: [context-draft.md]
---

# Build Derived Context

Spawn a single Opus agent to execute Phase 3 of the context builder:

```
Agent(
  description="RHOAI upgrade assessment — context builder phase 3",
  model="opus",
  prompt="You are building derived context data for an RHOAI upgrade assessment.

Read these files in this order:
1. ${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md — read Shared Rules + Phase 3 only
2. The project CLAUDE.md at the vault root — for repository cloning and refs rules
3. {run_dir}/context-draft.md — the verified inventories from Phase 2

Follow Phase 3 instructions to:
- Build Cross-Component Interaction Map
- Build Accidental Data-Plane Restart Triggers table
- Read odh-cli check registry and perform version gate analysis
- Add Persona Routing table and Reference Paths
- Rename context-draft.md to context.md (adding the derived sections)
- Print summary

Run directory: {run_dir}
Source version: {source}
Target version: {target}"
)
```
