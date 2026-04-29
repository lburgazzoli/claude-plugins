---
name: Verify and Cross-Validate CRDs
description: Run anomaly detection on raw CRD inventory, verify against component repos, diff schemas
scope: [static]
state-status: crds_verified
requires: [context-draft.md, discrepancies.yaml]
---

# Verify and Cross-Validate CRDs

Spawn a single Opus agent to execute Phase 2 of the context builder:

```
Agent(
  description="RHOAI upgrade assessment — context builder phase 2",
  model="opus",
  prompt="You are verifying and cross-validating CRDs for an RHOAI upgrade assessment.

Read these files in this order:
1. ${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md — read Shared Rules + Phase 2 only
2. The project CLAUDE.md at the vault root — for repository cloning and refs rules
3. {run_dir}/context-draft.md — the raw inventories from Phase 1

Follow Phase 2 instructions to:
- Run anomaly detection on the CRD Inventory
- Clone component repos and verify CRDs against actual YAML
- Extract spec.versions[] arrays, check conversion strategy
- Diff OpenAPI schemas for version-changed CRDs
- Update CRD Inventory in context-draft.md with Conversion and Schema Delta columns
- Append discrepancies to discrepancies.yaml
- Run dependency operator disruption analysis

Run directory: {run_dir}
Source version: {source}
Target version: {target}"
)
```
