---
name: Verify Unverified Claims
description: Spawn verification subagents for unverified persona claims
scope: [static]
state-status: verification_complete
---

# Verify Unverified Claims

After all personas return, run the metadata CLI to find which personas have unverified claims:

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/metadata.py unverified {run_dir}
```

If the metadata CLI reports `none`, skip verification and proceed.

If any persona reports unverified claims, read the `## Unverified Claims` section from that persona's `.md` file to get the claim details. For each unverified claim, spawn a **focused verification subagent** via the Agent tool. The subagent's instructions are in `${CLAUDE_SKILL_DIR}/resources/prompts/verification-agent.md`.

```
Agent(
  description="Verify: {brief claim description}",
  prompt="Read ${CLAUDE_SKILL_DIR}/resources/prompts/verification-agent.md for your instructions, then verify:

claim: {the claim text}
context: {why verification is needed}
check_path: {the repo/path the persona provided}"
)
```

Spawn all verification subagents in a single message (parallel). After they return, update each persona's finding:
- **CONFIRMED**: promote from Unverified Claims to a numbered finding with HIGH confidence
- **REFUTED**: drop the claim entirely and note it in the report as "claim refuted by verification"
- **INCONCLUSIVE**: keep in Runtime Verification Checklist (requires cluster access)

Write verification results to `{run_dir}/verifications.md`.

**Update metadata after verification**: for each persona whose findings changed (confirmed claims added, refuted claims removed), re-run `metadata.py write` to update the `.yaml` sidecar with the corrected findings list and `--unverified_claims 0`. This ensures `synthesize.py` uses accurate data.
