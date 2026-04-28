---
name: Build Context
description: Build shared context.md by resolving repos, reading architecture docs, cross-validating CRDs, and reading odh-cli checks
scope: [static]
state-status: context_complete
---

# Build Context

Follow the instructions in `${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md` to build the shared context. This will:
- Resolve repos, clone version-specific branches
- Read architecture docs, cross-validate CRDs against actual component code
- Read odh-cli checks with version gate analysis
- Write `context.md` and `discrepancies.yaml` to the run directory

After the context build completes:
1. Verify `context.md` exists in the run directory (`.context/tmp/upgrade-assessments/{run_id}/`).
2. Initialize state persistence:
   ```bash
   python3 ${CLAUDE_SKILL_DIR}/scripts/state.py init {run_dir} \
       --source {source} --target {target} --scope {scope} --personas {personas}
   ```
