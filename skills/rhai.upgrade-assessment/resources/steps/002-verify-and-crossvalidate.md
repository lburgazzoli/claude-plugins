---
name: Verify and Cross-Validate CRDs
description: Run anomaly detection on raw CRD inventory, verify against component repos, diff schemas
scope: [static]
state-status: crds_verified
requires: [context-draft.md, discrepancies.yaml]
---

# Verify and Cross-Validate CRDs

Follow **Phase 2** of `${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md`. This will:
- Read `context-draft.md` from the run directory
- Run anomaly detection (smell test) on the raw CRD Inventory
- For each red flag: clone the component repo, verify against actual CRD YAML
- Extract full `spec.versions[]` arrays, verify scope, check conversion strategy
- Diff OpenAPI schemas for version-changed CRDs
- Update CRD Inventory in `context-draft.md` with Conversion and Schema Delta columns
- Append discrepancies to `discrepancies.yaml`
- Run dependency operator disruption analysis (web search for changelogs)
