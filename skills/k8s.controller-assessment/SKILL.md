---
name: k8s.controller-assessment
description: Comprehensive Kubernetes controller assessment combining architecture review, API conventions audit, and production readiness evaluation.
---

# Kubernetes Controller Assessment

Perform a comprehensive assessment of a Kubernetes controller by invoking three focused skills in parallel and merging their results into a single report.

## Inputs

- If `$ARGUMENTS` is provided, pass it through to each sub-skill as scope.
- If no arguments are provided, each sub-skill will fall back to its own defaults (git diff, then full codebase scan).
- `--scope=<list>` limits which sub-skills to run. Accepted values: `architecture`, `api`, `production-readiness` (comma-separated). Examples:
  - `--scope=architecture` — run only the architecture review
  - `--scope=architecture,api` — run architecture and API conventions, skip production readiness
  - If `--scope` is omitted, all three sub-skills run.
  - When scope restricts to a subset, scoring uses only the included sub-skills (renormalize weights as described in Scoring).
- A sub-skill may return `Not applicable` (for example, API review when a project has no Kubernetes APIs/CRDs).
- `--detail` includes a full breakdown of each finding (Why, Fix, metadata) after the summary tables. This flag is passed through to each sub-skill. Without this flag, only the summary tables are produced.

## Execution

Run all three skills in parallel, passing `$ARGUMENTS` (including `--detail` if present) to each:

1. `/k8s.controller-architecture` — reconciliation, error handling, RBAC, status, finalizers, performance, cache
2. `/k8s.controller-api` — CRD structure, API versioning, webhooks, marker correctness
3. `/k8s.controller-production-readiness` — test coverage, observability (events, logs, metrics)

Wait for all three to complete before proceeding to the validation phase.

If a sub-skill returns `Not applicable`, include that result in the report and continue without treating it as a failure.
If parallel execution is unavailable, run the three sub-skills sequentially using the same scope and merge rules.

## Validation Phase

After the three assessment sub-skills complete and their findings are merged and deduplicated, launch an **adversarial validation subagent** with a clean context. The validator's purpose is to independently verify whether each finding represents a real behavioral issue in the controller.

### Validator subagent instructions

Launch a separate subagent with the following brief. Do **not** share the assessment agents' context — the validator must read the code independently to avoid confirmation bias.

> **Role**: You are a skeptical reviewer. Your job is to challenge each finding from a controller assessment and determine whether it actually impacts the controller's runtime behavior, correctness, or operational safety.
>
> **Inputs you receive**:
> - The merged findings list (each with: severity, source, area, what, where, confidence)
> - The scope (`$ARGUMENTS`) so you can read the same code
>
> **Your task**:
> For each finding, independently read the code at the referenced location and evaluate:
> 1. **Is the finding accurate?** Does the code actually exhibit the described problem?
> 2. **Does it affect behavior?** Would fixing this change how the controller behaves at runtime, or is it purely stylistic / cosmetic / theoretical?
> 3. **Is the severity appropriate?** A code pattern that looks non-ideal but cannot cause incorrect reconciliation, data loss, or operational failure should be downgraded.
>
> **Downgrade rules**:
> - A finding that is factually correct but has **no behavioral impact** (e.g., a minor style deviation, an extra log line, a slightly verbose but functionally correct pattern) → downgrade by one level (Critical→Major, Major→Minor, Minor→dismiss)
> - A finding where the described problem **cannot occur** given the surrounding code (e.g., "missing nil check" when the value is guaranteed non-nil by an earlier guard) → dismiss entirely
> - A finding where the severity is appropriate and the behavioral impact is real → keep as-is
>
> **Output schema** (return one entry per finding):
> ```
> findingId: <number matching the merged findings list>
> originalSeverity: Critical / Major / Minor
> validatedSeverity: Critical / Major / Minor / Dismissed
> verdict: Confirmed / Downgraded / Dismissed
> reason: <1-2 sentences explaining why — reference specific code evidence>
> ```

### Applying validation results

After the validator returns:

1. Update each finding's severity to the `validatedSeverity`.
2. Remove any finding with `validatedSeverity: Dismissed`.
3. For downgraded findings, append the validator's reason to the finding's `Not verified` field as: `Downgraded from <original> — <reason>`.
4. Recompute severity counts and scores using the validated severities.
5. If `--detail` is active, include a **Validation** section after the Findings section listing all verdict changes (downgrades and dismissals) with reasons.

### Skipping validation

If the merged findings list is empty (all sub-skills returned no findings or `Not applicable`), skip the validation phase entirely.

## Scoring

Compute scores **after** the validation phase (using validated severities). Recompute sub-skill scores by re-applying the scoring formula to the validated findings for each sub-skill, then compute the overall score:

- **Architecture** (from `/k8s.controller-architecture`): **50%**
- **API Conventions** (from `/k8s.controller-api`): **25%**
- **Production Readiness** (from `/k8s.controller-production-readiness`): **25%**

`Overall = Architecture × 0.50 + API Conventions × 0.25 + Production Readiness × 0.25`

If one or more sub-skills are `Not applicable`, exclude their weights and renormalize the remaining weights to sum to 1.0 before calculating `Overall`.
If all sub-skills are `Not applicable`, do not compute `Overall`; report `Overall Score: Not applicable`.

Example:
- If `API Conventions` is `Not applicable`, renormalize `Architecture` and `Production Readiness` from `0.50/0.25` to `0.667/0.333`.
- Then compute `Overall = Architecture × 0.667 + Production Readiness × 0.333`.

Interpretation:

- **90-100**: Production-ready with minor polish
- **75-89**: Solid baseline, a few important gaps
- **50-74**: Significant issues to address before production
- **<50**: High operational risk; major redesign/fixes recommended

## Output Format

Merge findings from all three skills into a single report. Deduplicate overlapping findings. If skills conflict on severity for the same issue, prefer the higher severity.

All sections are always included unless noted otherwise.

### Summary

2-3 sentences describing the overall quality and maturity of the controller.

Score table:

| Metric | Value |
|--------|-------|
| **Overall Score** | 0-100 (or `Not applicable`) |
| **Architecture** | 0-100 (or `Not applicable`) |
| **API Conventions** | 0-100 (or `Not applicable`) |
| **Production Readiness** | 0-100 (or `Not applicable`) |
| **Interpretation** | One of: Production-ready with minor polish / Solid baseline, a few important gaps / Significant issues to address before production / High operational risk |

Severity count table:

| Severity | Count |
|----------|-------|
| Critical | _n_ |
| Major | _n_ |
| Minor | _n_ |

Findings summary table (one row per finding, sorted by severity then by source):

| # | Severity | Source | Area | What | Where | Confidence |
|---|----------|--------|------|------|-------|------------|

- **Source**: Which sub-skill identified the finding (Architecture, API, Production Readiness).
- **Where** must include a concrete file path and line reference for every Critical and Major finding.
- Evidence rule: include at least one concrete file path and line reference for every Critical and Major finding.

### Findings (only with `--detail`)

This section is only included when the `--detail` flag is passed.

For each finding (numbered to match the summary table), produce:

#### _N_. _Finding title_

| | |
|---|---|
| **Severity** | Critical / Major / Minor |
| **Source** | Architecture / API / Production Readiness |
| **Area** | Assessment area name |
| **Where** | File and line reference |
| **Confidence** | High / Medium / Low |
| **Not verified** | Any assumptions or runtime checks not validated (or `—`) |

**Why**: Explanation of why this is an issue.

**Fix**: Concrete suggested change.

---

### Validation (only with `--detail`)

This section is only included when the `--detail` flag is passed and the validation phase produced changes.

List each downgraded or dismissed finding:

| # | Original Severity | Validated Severity | Verdict | Reason |
|---|-------------------|--------------------|---------|--------|

### Positive Highlights

Notable strengths from all three skills.
