---
name: Resolve Repos and Build Inventories
description: Clone/update repos, read architecture docs, build raw resource inventories
scope: [static]
state-status: repos_resolved
---

# Resolve Repos and Build Inventories

The run directory already exists and state is initialized (done by the orchestrator before the step loop). The run directory path is in `state.yaml`.

Follow **Phase 1** of `${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md`. This will:
- Clean stale clones, update/clone repos (architecture-context, odh-cli, odh-gitops, ADRs)
- Clone version-specific odh-gitops branches for source and target
- Resolve architecture directories for both versions
- Read PLATFORM.md files, compute Component Diff
- Read architecture docs and odh-gitops manifests
- Build raw resource inventories (CRDs, Deployments, Endpoints, Webhooks, Workload Types, PVCs, Dependencies)
- Write `context-draft.md` with all raw inventories (no interaction map, no verified CRDs, no odh-cli checks yet)
- Write empty `discrepancies.yaml`

After the phase completes, verify `context-draft.md` exists in the run directory.
