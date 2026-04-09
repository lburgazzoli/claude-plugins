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

## Execution

Run all three skills in parallel, passing `$ARGUMENTS` to each:

1. `/k8s.controller-architecture` — reconciliation, error handling, RBAC, status, finalizers, performance, cache
2. `/k8s.controller-api` — CRD structure, API versioning, webhooks, marker correctness
3. `/k8s.controller-production-readiness` — test coverage, observability (events, logs, metrics)

Wait for all three to complete before producing the merged report.

If a sub-skill returns `Not applicable`, include that result in the report and continue without treating it as a failure.
If parallel execution is unavailable, run the three sub-skills sequentially using the same scope and merge rules.

## Scoring

Compute the overall score from the three sub-skill scores:

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

### Summary
2-3 sentences describing the overall quality and maturity of the controller.
- `Overall Score`: 0-100 (or `Not applicable`)
- `Sub-scores`: Architecture / API Conventions / Production Readiness (`Not applicable` allowed)
- `Not applicable`: List any sub-skills that were skipped as `Not applicable`

### Critical Issues (must fix)
All critical findings from the three skills, grouped by source skill. Each issue includes:
- **Source**: Which skill identified this
- **What**: Description of the problem
- **Where**: File and line reference
- **Why**: Why this is critical
- **Fix**: Concrete suggested change
- **Confidence**: High/Medium/Low
- **Not verified**: Any assumptions or runtime checks not validated
- Evidence rule: include at least one concrete file path and line reference for every Critical finding.

### Major Issues (should fix)
Same format as critical.
- Evidence rule: include at least one concrete file path and line reference for every Major finding.

### Minor Issues (nice to improve)
Same format as critical.

### Positive Highlights
Notable strengths from all three skills.
