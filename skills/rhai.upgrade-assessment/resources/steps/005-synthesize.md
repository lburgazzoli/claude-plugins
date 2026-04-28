---
name: Synthesize
description: Pre-compute report data and spawn synthesis agent to write final report
scope: [static]
state-status: synthesis_complete
---

# Synthesize

## Phase A: Pre-compute report data

Verify that `context.md` still exists in the run directory. If it was deleted or overwritten, print an error and stop.

Run the synthesis script to pre-compute tables, aggregates, and cross-persona analysis from the structured metadata:

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/synthesize.py {run_dir} \
    --source {source} --target {target} \
    --personas {personas} --strict
```

This reads each persona's `.yaml` sidecar file and writes `{run_dir}/synthesis.yaml` with:
- Pre-rendered verdict table and persona assessments table
- Aggregate finding counts across all personas
- Blocking findings list
- Corroboration matches (XREFs matched to owning persona's findings)
- Disagreement detections (XREF severity differs from owner's finding severity)
- Missing personas list

If the script reports errors (missing `.yaml` files, validation failures), read the affected persona's `.md` file, extract the data, write the `.yaml` file via `metadata.py write`, then re-run synthesis.

## Phase B: Write report

Spawn a synthesis agent with a clean context to write the final report:

```
Agent(
  description="Synthesize upgrade assessment report",
  model="opus",
  prompt="You are writing the final synthesis report for an RHOAI upgrade assessment.

Read these files:
1. {run_dir}/context.md — read only the header section (up to ## Component Diff) for repo SHAs and platform metadata
2. {run_dir}/synthesis.yaml — pre-computed tables, aggregates, corroborations, disagreements
3. Each persona's .md file in {run_dir}/ — prose findings with evidence

Write {run_dir}/report.md with this structure:

# Upgrade Assessment Report: RHOAI {source} -> {target}

**Date**: {date from synthesis.yaml}
**Scope**: {scope}
**Method**: Independent clean-context agent personas, each isolated.
**Repos**: {copy the Repos line from context.md header}

## Executive Summary
3-5 sentences: overall risk, key blockers, recommended path. Use the aggregate
counts and blocking_findings from synthesis.yaml to ground the summary.

## Verdict Table
Copy the verdict_table from synthesis.yaml verbatim.

**Overall recommendation**: synthesize the persona verdicts without averaging
or harmonizing. State the recommendation and the key factors driving it.

## Blocking Prerequisites
Use blocking_findings from synthesis.yaml. For each, read the corresponding
persona's .md file to add the mitigation details and evidence.

---

## Persona Assessments
Copy the persona_table from synthesis.yaml verbatim.

---

## Cross-Persona Corroboration
Use corroborations from synthesis.yaml. For each match, read the relevant
persona .md files for detail. Note any XREF that adds a concern the owner
did not cover.

## Disagreements
Use disagreements from synthesis.yaml. Read both persona .md files to present
both views. Preserve both — do not pick a winner.

## Unclaimed Findings
Read persona .md files for findings outside ownership boundaries.

## Coverage Gaps
List missing_personas from synthesis.yaml. Read persona .md files for domains
inadequately covered.

Preserve disagreements. The user decides. Unclaimed findings signal ownership
taxonomy gaps to fix."
)
```
