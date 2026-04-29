---
name: Build Derived Context
description: Build interaction map, detect restart triggers, read odh-cli checks, finalize context.md
scope: [static]
state-status: context_complete
requires: [context-draft.md]
---

# Build Derived Context

Follow **Phase 3** of `${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md`. This will:
- Read verified `context-draft.md` from the run directory
- Build Cross-Component Interaction Map
- Build Accidental Data-Plane Restart Triggers table
- Read odh-cli check registry and perform version gate analysis
- Add Persona Routing table and Reference Paths
- Rename `context-draft.md` to `context.md` (adding the derived sections)
- Print summary (run ID, versions, component counts, CRD changes, discrepancies)
