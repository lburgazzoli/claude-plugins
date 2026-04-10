---
name: k8s.controller-assessment
description: Comprehensive Kubernetes controller assessment combining architecture review, API conventions audit, and production readiness evaluation.
---

# Kubernetes Controller Assessment

Perform a comprehensive assessment of a Kubernetes controller by invoking three focused skills in parallel and merging their results into a single report.

## Inputs

- If `$ARGUMENTS` is provided, parse orchestration flags first, then pass only the remaining scope text to each selected sub-skill.
- GitHub repository inputs may be full URLs (for example, `https://github.com/org/repo`) or shorthand (`org/repo`). These are forwarded as scope text to sub-skills, which handle them directly.
- If `$ARGUMENTS` is a GitHub repository, do not apply local `git diff` defaults in sub-skills.
- If no arguments are provided, each sub-skill will fall back to its own defaults (git diff, then focused directories, then expand as needed for evidence).
- `--scope=<list>` limits which sub-skills to run. Accepted values: `architecture`, `api`, `production-readiness` (comma-separated). Examples:
  - `--scope=architecture` — run only the architecture review
  - `--scope=architecture,api` — run architecture and API conventions, skip production readiness
  - If `--scope` is omitted, all three sub-skills run.
  - When scope restricts to a subset, scoring uses only the included sub-skills (renormalize weights as described in Scoring).
- A sub-skill may return `Not applicable` (for example, API review when a project has no Kubernetes APIs/CRDs).
- `--details` includes a full breakdown of each finding (Why, Fix, metadata) after the summary tables. This flag is passed through to each sub-skill. Without this flag, only the summary tables are produced.

## Input Validation

The only recognized flags are `--scope=<list>` and `--details`. If `$ARGUMENTS` contains any unrecognized `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.

Validate `--scope=<list>` values strictly:

- Split by comma and trim whitespace.
- Accepted values are exactly: `architecture`, `api`, `production-readiness`.
- If any value is unknown, stop before running the assessment and ask the user to confirm whether it is intentional or a typo.
- If `--scope` is present but resolves to an empty list after parsing, stop and ask for a valid scope.

## Execution

Resolve selected sub-skills from `--scope` first:

- If `--scope` is omitted, select all three sub-skills.
- If `--scope` is present, select only the listed sub-skills.

Build child arguments before invocation:

1. Parse and remove `--scope=<list>` from `$ARGUMENTS` (orchestrator-only flag; never forwarded).
2. Preserve `--details` only when present.
3. Treat remaining non-flag text as the shared child scope string.
4. Forward to each selected sub-skill using: `<child-scope> [--details]`.

Run the selected skills in parallel, passing only child arguments to each selected skill:

1. `/k8s.controller-architecture` — reconciliation, error handling, RBAC, status, finalizers, performance, cache
2. `/k8s.controller-api` — CRD structure, API versioning, webhooks, marker correctness
3. `/k8s.controller-production-readiness` — test coverage, observability (events, logs, metrics)

Wait for all selected sub-skills to complete before proceeding to the merge phase.

If a sub-skill returns `Not applicable`, include that result in the report and continue without treating it as a failure.
If parallel execution is unavailable, run the selected sub-skills sequentially using the same child-argument and merge rules.

## Merge and Severity Normalization

After sub-skills complete, merge and normalize findings in this exact order:

1. Combine all findings from selected sub-skills into one list.
2. Combine all positive highlights from selected sub-skills into one list, preserving the source sub-skill for each highlight.
3. Deduplicate overlapping findings — two findings are considered duplicates when they reference the same code location (`where`) and describe the same underlying problem. When in doubt, prefer to keep both as separate findings rather than incorrectly merging distinct issues.
4. For each merged finding, keep a `sources` list containing every sub-skill that reported it.
5. If two or more sub-skills report the same issue with different severities, keep the higher severity.
6. Set `primarySource` for every finding using priority: `Architecture` > `API` > `Production Readiness`. For single-source findings, use that source. For multi-source findings, use the source that reported the higher severity; if severities tied, use the priority order.
7. Run validation on the merged findings and highlights, and apply validation adjustments.
8. Compute severity counts and scores only after the final merged severities are settled.

## Validation Phase

> This phase **always runs** after merging and deduplication. Its results (severity adjustments and dismissals) are applied before scoring regardless of flags. The detailed Validation output section is only included in the report when `--details` is passed.

Each individual sub-skill now performs its own adversarial self-validation before returning findings. This orchestrator-level validation serves as a **cross-skill consistency check** — deduplicating overlapping findings, resolving conflicting severities across skills, and catching any issues that only become apparent when viewing results holistically.

After findings are merged and deduplicated, launch an **adversarial validation subagent** with a clean context. The validator's purpose is to independently verify whether each finding represents a real behavioral issue in the controller.

### Validator subagent instructions

Launch a separate subagent with the following brief. Do **not** share the assessment agents' context — the validator must read the code independently to avoid confirmation bias.

> **Role**: You are a skeptical reviewer. Your job is to challenge each finding from a controller assessment and determine whether it actually impacts the controller's runtime behavior, correctness, or operational safety. You also verify that positive highlights do not contradict the findings.
>
> **Inputs you receive**:
> - The merged findings list (each with: findingId, severity, primarySource, sources, area, what, where, confidence)
> - The merged positive highlights list (each with: highlightId, source sub-skill, description)
> - The scope (`$ARGUMENTS`) so you can read the same code
>
> **Your task**:
>
> **Part 1 — Validate findings:**
> For each finding, independently read the code at the referenced location and evaluate:
> 1. **Is the finding accurate?** Does the code actually exhibit the described problem?
> 2. **Does it affect behavior?** Would fixing this change how the controller behaves at runtime, or is it purely stylistic / cosmetic / theoretical?
> 3. **Is the severity appropriate?** A code pattern that looks non-ideal but cannot cause incorrect reconciliation, data loss, or operational failure should be downgraded.
>
> **Part 2 — Validate highlights against findings:**
> For each positive highlight, check whether it contradicts any finding:
> - A highlight must not praise a pattern that is also flagged as a finding. For example: praising "Controller X uses RetryOnConflict" while finding "Controllers Y and Z lack RetryOnConflict" is a contradiction — if the absence of the pattern is a problem, its presence in one place is not a highlight, it is the expected baseline.
> - If a highlight praises a pattern whose absence is flagged as a finding anywhere in the report, mark the highlight for removal or rewording.
> - If the same concept is used correctly in some code paths and incorrectly in others, the correct usage is not a highlight — only the incorrect usage should appear (as a finding).
> - Highlights that do not conflict with any finding should be kept as-is.
>
> **Downgrade rules** (findings only):
> - A finding that is factually correct but has **no behavioral impact** (e.g., a minor style deviation, an extra log line, a slightly verbose but functionally correct pattern) → downgrade by one level (Critical→Major, Major→Minor, Minor→dismiss)
> - A finding where the described problem **cannot occur** given the surrounding code (e.g., "missing nil check" when the value is guaranteed non-nil by an earlier guard) → dismiss entirely
> - A finding where the severity is appropriate and the behavioral impact is real → keep as-is
>
> **Output schema**:
>
> Finding validations (one entry per finding):
> ```
> findingId: <number matching the merged findings list>
> originalSeverity: Critical / Major / Minor
> validatedSeverity: Critical / Major / Minor / Dismissed
> verdict: Confirmed / Downgraded / Dismissed
> reason: <1-2 sentences explaining why — reference specific code evidence>
> ```
>
> Highlight validations (one entry per highlight that needs adjustment):
> ```
> highlightId: <number matching the merged highlights list>
> verdict: Keep / Remove / Reword
> reason: <1-2 sentences explaining the contradiction with a specific finding>
> suggestedRewording: <if verdict is Reword, the revised text — omit if Remove or Keep>
> ```

### Applying validation results

After the validator returns:

**Finding adjustments:**

1. Update each finding's severity to the `validatedSeverity`.
2. Remove any finding with `validatedSeverity: Dismissed`.
3. For downgraded findings, append the validator's reason to the finding's `Not verified` field as: `Downgraded from <original> — <reason>`.
4. Recompute severity counts and scores using the validated severities.
5. If `--details` is active, include a **Validation** section after the Findings section listing all verdict changes (downgrades and dismissals) with reasons.

**Highlight adjustments:**

6. Remove any highlight with `verdict: Remove`.
7. Replace any highlight with `verdict: Reword` with the validator's `suggestedRewording`.
8. Keep all highlights with `verdict: Keep` unchanged.

### Skipping validation

If the merged findings list is empty (all sub-skills returned no findings or `Not applicable`), skip the validation phase entirely.

## Scoring

Compute scores **after** the validation phase (using validated severities). Recompute sub-skill scores by re-applying the scoring formula to the validated merged findings for each sub-skill, then compute the overall score:

- Each merged finding contributes to every sub-skill listed in its `sources`.
- Use the merged finding's final severity (post-validation) for every contributing sub-skill.
- Do not duplicate a finding more than once within the same sub-skill score.

Sub-skill scoring models:
- **Architecture**: uses weighted categories — Category A (Areas 1, 2, 4, 5, 6) at 60% and Category B (Areas 3, 7, 8) at 40%. Start each category at 100, deduct per finding, floor at 0, then compute `Architecture = A × 0.60 + B × 0.40`.
- **API Conventions** and **Production Readiness**: use flat scoring — start at 100, deduct per finding (Critical -20, Major -10, Minor -3), floor at 0.

- **Architecture** (from `/k8s.controller-architecture`): **50%**
- **API Conventions** (from `/k8s.controller-api`): **25%**
- **Production Readiness** (from `/k8s.controller-production-readiness`): **25%**

`Overall = Architecture × 0.50 + API Conventions × 0.25 + Production Readiness × 0.25`

If one or more sub-skills are `Not applicable`, exclude their weights and renormalize the remaining weights to sum to 1.0 before calculating `Overall`.
If all sub-skills are `Not applicable`, do not compute `Overall`; report `Overall Score: Not applicable`.

Example:
- If `API Conventions` is `Not applicable`, renormalize `Architecture` and `Production Readiness` from `0.50/0.25` to `0.667/0.333`.
- Then compute `Overall = Architecture × 0.667 + Production Readiness × 0.333`.
- If `--scope=architecture`, compute `Overall = Architecture × 1.0`.
- If `--scope=api,production-readiness`, renormalize `API/Production Readiness` from `0.25/0.25` to `0.50/0.50`.

When the overall score reflects fewer than all three sub-skills (due to `--scope` or `Not applicable`), include a note in the Summary stating which dimensions contributed to the score.

Interpretation:

- **90-100**: Production-ready with minor polish
- **75-89**: Solid baseline, a few important gaps
- **50-74**: Significant issues to address before production
- **<50**: High operational risk; major redesign/fixes recommended

## Output Format

Merge findings from selected sub-skills into a single report. Deduplicate overlapping findings that refer to the same underlying issue. Preserve full source attribution per finding (`sources`). If skills conflict on severity for the same issue, prefer the higher severity. If severity still ties, use source priority for display (`primarySource`): `Architecture` > `API` > `Production Readiness`.

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

Findings summary table (one row per finding, sorted by severity then by primary source):

| # | Severity | Source | Area | What | Where | Confidence |
|---|----------|--------|------|------|-------|------------|

- **Source**: Display `primarySource` (Architecture, API, Production Readiness). If multiple sub-skills identified the same merged finding, optionally append `(+N more)` and list all contributors in `--details`.
- **Where** must include a concrete file path and line reference for every Critical and Major finding.

### Findings (only with `--details`)

This section is only included when the `--details` flag is passed.

For each finding (numbered to match the summary table), produce:

#### _N_. _Finding title_

| | |
|---|---|
| **Severity** | Critical / Major / Minor |
| **Source** | Architecture / API / Production Readiness |
| **Also reported by** | Additional sources from `sources` beyond `primarySource` (or `—`) |
| **Area** | Assessment area name |
| **Where** | File and line reference |
| **Confidence** | High / Medium / Low |
| **Not verified** | Any assumptions or runtime checks not validated (or `—`) |

**Why**: Explanation of why this is an issue.

**Fix**: Concrete suggested change.

---

### Validation (only with `--details`)

**Do NOT include this section unless `--details` is passed.** When `--details` is active and the validation phase produced changes, list each downgraded or dismissed finding:

| # | Original Severity | Validated Severity | Verdict | Reason |
|---|-------------------|--------------------|---------|--------|

### Positive Highlights

Notable strengths from selected sub-skills.
