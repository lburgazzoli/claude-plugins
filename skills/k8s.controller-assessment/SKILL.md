---
name: k8s.controller-assessment
description: Comprehensive Kubernetes controller assessment combining architecture review, API conventions audit, lifecycle safety, and production readiness evaluation.
user-invocable: true
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash
  - Agent
  - Skill
  - mcp__k8s-controller-analyzer__analyze_controller
  - mcp__gopls__go_file_context
  - mcp__gopls__go_symbol_references
  - mcp__gopls__go_search
  - mcp__gopls__go_package_api
  - LSP
---

# Kubernetes Controller Assessment

Perform a comprehensive assessment of a Kubernetes controller by invoking four focused skills and merging their results into one deterministic report.

## References

Consult [analyzer-output-schema.md](../../references/analyzer-output-schema.md) for the analyzer JSON input contract.
Consult [validation-output-schema.md](../../references/validation-output-schema.md) for the canonical report model.
Consult [reproducible-assessments.md](../../references/reproducible-assessments.md) for deterministic execution rules.

## Inputs

- `$ARGUMENTS` may contain:
  - scope text such as files, packages, controller names, or GitHub repositories
  - `--scope=<list>`
  - `--mode=deterministic`
  - `--mode=exploratory`
- Accepted `--scope` values are `architecture`, `api`, `lifecycle`, and `production-readiness`.
- Default mode is `--mode=deterministic`.
- If no `--scope` is provided, run all four sub-skills.

## Input Validation

- The only recognized flags are `--scope=<list>`, `--mode=deterministic`, and `--mode=exploratory`.
- If `$ARGUMENTS` contains any other `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.
- Validate `--scope=<list>` strictly:
  - split by comma
  - trim whitespace
  - accepted values are exactly `architecture`, `api`, `lifecycle`, `production-readiness`
  - if any value is invalid, stop and ask for confirmation

## Pre-Assessment Setup

Before invoking any child skill, run the static analyzer once. This avoids triple-running across child skills.

Treat [analyzer-output-schema.md](../../references/analyzer-output-schema.md) as the normative schema for the analyzer JSON envelope and fact payloads passed to child skills.

Use the `analyze_controller` MCP tool with `repo_path` set to the repository root (omit `skill` since each child skill needs different manifest categories).

Load the full JSON output into context. Child skills detect the already-loaded analyzer JSON and skip their own run step. Each child skill will reference the loaded facts directly.

## Child Invocation Rules

Resolve selected sub-skills first:

- `architecture` -> `/k8s.controller-architecture`
- `api` -> `/k8s.controller-api`
- `lifecycle` -> `/k8s.controller-lifecycle`
- `production-readiness` -> `/k8s.controller-production-readiness`

Build child arguments in this order:

1. Remove `--scope=<list>`.
2. Preserve `--mode=...`.
3. Treat remaining non-flag text as shared child scope text.

## Deterministic Execution

In deterministic mode:

0. Run Pre-Assessment Setup (build and run static analyzer, load JSON into context).
1. Run child skills sequentially in this fixed order:
   - `k8s.controller-architecture`
   - `k8s.controller-api`
   - `k8s.controller-lifecycle`
   - `k8s.controller-production-readiness`


If a sub-skill is not selected by `--scope`, skip it.
If a sub-skill returns `Not applicable`, keep that status and continue.

In deterministic mode do not:

- run sub-skills in parallel
- re-open scope inside the orchestrator
- launch validator subagents
- adopt the child skill's output format when only one child is selected

`--mode=exploratory` may run the same sub-skills with broader local exploration, but the merge algorithm stays the same.

## Finding Enrichment Before Merge

Before merging, enrich each leaf finding with orchestrator fields:

1. Set `sources` to a single-element list containing the originating skill's canonical identifier (e.g., `["k8s.controller-architecture"]`).
2. Set `primarySource` to the same skill identifier.
3. Set `checklistItem` to the leaf finding's checklist item ID (e.g., `"5c"`).

These fields are required inputs to the merge algorithm. After deduplication, `sources` is the union of all contributing skills and `primarySource` is selected by the tie-breaking rules below.

## Merge And Normalization

After child skills complete and findings are enriched, merge in this exact order:

1. Combine all findings from selected sub-skills.
2. Combine all highlights from selected sub-skills.
3. Normalize each finding:
   - normalize `where`
   - preserve `sources`
   - preserve candidate `primarySource`
4. Deduplicate with this fingerprint:
   - `area`
   - normalized problem class: the checklist item `title` from the originating leaf skill, used verbatim without paraphrasing
   - primary `where` path
   - primary line range when available
5. If fingerprints match, merge findings.
6. If merged severities differ, keep the higher severity.
7. If merged severities tie, use this `primarySource` priority:
   - `k8s.controller-architecture`
   - `k8s.controller-api`
   - `k8s.controller-lifecycle`
   - `k8s.controller-production-readiness`
8. Copy `title`, `what`, `area`, `why`, `fix`, and `confidence` from the `primarySource`.
9. Union `where` and `notVerified`, preserving sorted order.

If fingerprints differ, keep findings separate. Do not dedupe on prose similarity alone.

## Orchestrator Validation

Run one deterministic second pass after merging:

1. Re-check dedupe decisions against the fingerprint rules.
2. Re-check cross-skill highlight contradictions.
3. Recompute severity counts and scores from the final merged findings.
4. Dismiss or adjust findings only when the merge rules require it.

The orchestrator second pass must not introduce new findings.

## Scoring

Recompute child scores from final merged findings:

- Architecture uses its category-weighted model. Map each finding's `area` to a category:
  - Category A (weight 0.60): Reconciliation Idempotency and State Handling, Error Handling and Requeue Strategy, RBAC Least Privilege and Security, Status, Conditions, and Observed Generation, Ownership, Finalizers, and Cleanup Logic, Portability and Vendor API Dependencies, Spec-Status Contract Boundary, Concurrency Safety
  - Category B (weight 0.40): Resource Management and API Efficiency, Performance and Cache Usage, Cache Consistency and Client Type Alignment
  - If an Architecture finding's area does not match any of these, keep the finding in the report but note reduced-confidence scoring in the summary
- API uses flat deductions (Critical -20, Major -10, Minor -3, floor 0)
- Lifecycle uses flat deductions (Critical -20, Major -10, Minor -3, floor 0)
- Production Readiness uses flat deductions (Critical -20, Major -10, Minor -3, floor 0)

Overall weights:

- Architecture: `0.35`
- API: `0.20`
- Lifecycle: `0.25`
- Production Readiness: `0.20`

If one or more sub-skills are `Not applicable`, renormalize the remaining weights to sum to `1.0`.
Show arithmetic for each contributing score and for the weighted overall score.

Interpretation:

- `90-100`: Production-ready with minor polish
- `75-89`: Solid baseline, a few important gaps
- `50-74`: Significant issues to address before production
- `<50`: High operational risk; major redesign or fixes recommended

## Output Format

Produce the merged assessment using the canonical model from [validation-output-schema.md](../../references/validation-output-schema.md). When `--scope` selects a single child skill, use this orchestrator output format — not the child skill's native format. Unselected dimensions show `Not applicable`.

Output conventions:

- `scope` should use a URI-like string such as `repo://org/repo`, `dir://controllers`, or `controller://MyReconciler`
- `where` should use repo-relative GitHub-style location strings
- Sort merged findings by severity, area, `where`, and title before assigning final IDs
- Sort highlights by `sourceSkill` and description

### Summary

Write 2-3 sentences describing overall controller maturity.
If the overall score excludes one or more sub-skills, state which dimensions contributed.

Score table:

| Metric | Value |
|--------|-------|
| **Overall Score** | 0-100 or `Not applicable` |
| **Architecture** | 0-100 or `Not applicable` |
| **API Conventions** | 0-100 or `Not applicable` |
| **Lifecycle** | 0-100 or `Not applicable` |
| **Production Readiness** | 0-100 or `Not applicable` |

Severity count table:

| Severity | Count |
|----------|-------|
| Critical | _n_ |
| Major | _n_ |
| Minor | _n_ |

Findings summary table:

| # | Severity | Area | What | Where | Confidence | Source |
|---|----------|------|------|-------|------------|--------|

### Findings

For each finding include:

- Severity
- Source
- Area
- Where
- Confidence
- Not verified
- Why
- Fix

### Validation

Include this section when one or more findings or highlights changed during the orchestrator second pass. Otherwise render "No adjustments."

### Highlights

Include only positive highlights that do not contradict any merged finding.
