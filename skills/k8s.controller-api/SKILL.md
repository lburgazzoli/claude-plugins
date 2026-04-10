---
name: k8s.controller-api
description: Review or audit Kubernetes CRD definitions, API types, webhooks, and markers for compliance with upstream Kubernetes API conventions.
---

# Kubernetes API Conventions Assessment

Review Kubernetes Custom Resource Definitions (CRDs), API types, webhooks, and kubebuilder markers for compliance with upstream Kubernetes API conventions.
This skill is primarily designed for Go controllers built with `controller-runtime`/kubebuilder patterns.

## Inputs

- If `$ARGUMENTS` is provided, treat it as scope (files, package, API type name, or GitHub repository).
- GitHub repository inputs may be full URLs (for example, `https://github.com/org/repo`) or shorthand (`org/repo`).
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source and do not apply local `git diff` defaults.
- If no arguments are provided, assess current repository changes from git diff.
- If there are no changes, start with `api/`, `pkg/apis/`, and `config/crd/` directories first; expand only when needed for evidence.
- If the project does not define Kubernetes APIs/CRDs (no API types, no CRD manifests, and no related markers), skip this skill entirely and report `Not applicable`.
- `--details` includes a full breakdown of each finding (Why, Fix, metadata) after the summary tables. Without this flag, only the summary tables are produced.

## Input Validation

The only recognized flag is `--details`. If `$ARGUMENTS` contains any unrecognized `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.
When this skill is invoked by `/k8s.controller-assessment`, it should receive only scope text and optional `--details` (orchestration flags such as `--scope` are not valid here).

## References

Consult [k8s-upstream.md](../../references/k8s-upstream.md) for the authoritative source of conventions and high-quality reference implementations.

## Assessment Areas

### 1. CRD Structure and Field Conventions

- CRD follows Kubernetes API conventions:
  - `spec` for desired state, `status` for observed state
  - `+kubebuilder:subresource:status` marker is present on types with a `Status` field to enable the status subresource (required for `r.Status().Update()` and separate status RBAC)
  - `+kubebuilder:resource:scope=` is explicitly set (`Cluster` or `Namespaced`) — do not rely on the implicit default. Cluster-scoped resources should be justified (most resources should be namespace-scoped). Verify scope aligns with RBAC markers and owner reference patterns
  - Uses `int32`/`int64` for integers, avoids unsigned types
  - Enums are string-typed CamelCase values
  - Lists of named subobjects preferred over maps of subobjects
  - All optional fields have `+optional` marker and `omitempty` JSON tag
  - Required fields are validated with kubebuilder markers (`+kubebuilder:validation:Required`)
- Printer columns (`+kubebuilder:printcolumn`) show useful summary info in `kubectl get`

### 2. API Versioning

- API versioning follows Kubernetes conventions (v1alpha1 → v1beta1 → v1)
- Conversion webhooks are implemented when multiple versions coexist
- `+kubebuilder:storageversion` marker is present on exactly one version when multiple API versions coexist (`must`). For single-version CRDs the marker is recommended but its absence is not an error (`contextual`).

### 3. Webhooks (if present)

Skip this area entirely if the project has no admission webhooks. Only assess when webhook configurations or webhook handler code exist.

- Defaulting webhook sets sensible defaults
- Validating webhook rejects invalid input
- Failure policy is explicitly set (`Fail` or `Ignore`) based on criticality
- `sideEffects` is declared (typically `None`)
- Webhook certificate rotation is handled

### 4. CRD Generation and Marker Correctness

- Consider explicitly marking fields as `+required` or `+optional` to avoid ambiguity
- Be aware that zero values pass required field validation (OpenAPI checks non-null only) — use `MinLength`, `Minimum` markers when meaningful
- Inspect generated CRD manifests — controller-gen may silently ignore unrecognized markers:
  - Check for typos in `+kubebuilder:` markers by comparing against the known set (`validation`, `default`, `printcolumn`, `rbac`, `object`, `subresource`, etc.) — any unrecognized marker is silently dropped
  - Verify marker/field type alignment — e.g., `+kubebuilder:validation:Minimum=1` on a string field produces no validation in the generated CRD
  - Check `+kubebuilder:validation:Enum` values match the corresponding const block defining valid values
- Watch out for nested defaulting: when a nested struct field has `+kubebuilder:default:` markers, the parent field must have `+kubebuilder:default:{}` or the nested defaults are never applied
- Verify `+kubebuilder:object:root:=true` is present on root types and that generated deepcopy files (`zz_generated.deepcopy.go`) are not stale relative to current type definitions
- Always review generated output rather than trusting markers blindly

## Assessment Procedure

Use this repeatable workflow:

1. Determine scope from `$ARGUMENTS`, git diff, or targeted API type packages.
   - If `$ARGUMENTS` points to a GitHub repository, prioritize `api/`, `config/crd/`, and webhook directories as initial evidence sources.
2. If no Kubernetes APIs/CRDs are present in scope, stop and return `Not applicable`.
3. Collect evidence first (specific files and call sites), then classify issues by impact.
4. Mark each finding as `must`, `should`, or `contextual` based on production risk.
5. Map internal labels to report severities:
   - `must` -> `Critical`
   - `should` -> `Major`
   - `contextual` -> `Minor`
6. If generated CRD manifests are unavailable in scope (or `controller-gen` cannot be executed), perform static marker/source review and mark generated-manifest checks as `Not verified` with reduced confidence.
7. **Adversarial self-validation**: Before producing output, critically review each finding:
   - **Accuracy**: Does the code actually exhibit the described problem? Re-read the referenced location independently.
   - **Behavioral impact**: Would fixing this change runtime behavior, correctness, or operational safety — or is it purely stylistic/cosmetic/theoretical?
   - **Severity check**: A pattern that looks non-ideal but cannot cause incorrect reconciliation, data loss, or operational failure should be downgraded.
   - Downgrade rules:
     - Factually correct but **no behavioral impact** → downgrade one level (Critical→Major, Major→Minor, Minor→dismiss)
     - Described problem **cannot occur** given surrounding code → dismiss entirely
     - Severity appropriate with real impact → keep as-is
   - **Consistency check**: Cross-reference findings against positive highlights — a pattern must not appear as both a finding and a positive highlight. If a pattern is flagged as a finding, remove or reword any positive highlight that praises the same or contradictory pattern. If the same concept is used correctly in some code paths and incorrectly in others, do not praise it as a positive — only flag the problematic usage.
   - Remove dismissed findings and adjust severities before scoring.
   - Validation **always runs** and its results (severity adjustments and dismissals) are applied before scoring regardless of flags. The detailed Validation output section is only included in the report when `--details` is passed.
8. Generate output with severity, concrete fix, confidence, and any unverified assumptions.

## Scoring

1. Start at 100.
2. For each finding, subtract points based on severity:
   - Critical: **-20** per finding
   - Major: **-10** per finding
   - Minor: **-3** per finding
3. Floor score at 0.

Interpretation:

- **90-100**: Production-ready with minor polish
- **75-89**: Solid baseline, a few important gaps
- **50-74**: Significant issues to address before production
- **<50**: High operational risk; major redesign/fixes recommended

## Output Format

Produce the assessment in this format. All sections are always included unless noted otherwise.

### Summary

2-3 sentences describing the API design quality and conventions compliance.

Score table:

| Metric | Value |
|--------|-------|
| **Score** | 0-100 (or `Not applicable`) |
| **Interpretation** | One of: Production-ready with minor polish / Solid baseline, a few important gaps / Significant issues to address before production / High operational risk |

Severity count table:

| Severity | Count |
|----------|-------|
| Critical | _n_ |
| Major | _n_ |
| Minor | _n_ |

Findings summary table (one row per finding, sorted by severity then by area):

| # | Severity | Area | What | Where | Confidence |
|---|----------|------|------|-------|------------|

- **Where** must include a concrete file path and line reference for every Critical and Major finding.

### Findings (only with `--details`)

This section is only included when the `--details` flag is passed.

For each finding (numbered to match the summary table), produce:

#### _N_. _Finding title_

| | |
|---|---|
| **Severity** | Critical / Major / Minor |
| **Area** | Assessment area name |
| **Where** | File and line reference |
| **Confidence** | High / Medium / Low |
| **Not verified** | Any assumptions or runtime checks not validated (or `—`) |

**Why**: Explanation of why this is an issue, with reference to upstream convention if applicable.

**Fix**: Concrete suggested change.

---

### Validation (only with `--details`)

**Do NOT include this section unless `--details` is passed.** When `--details` is active and the validation phase produced changes, list each downgraded or dismissed finding:

| # | Original Severity | Validated Severity | Verdict | Reason |
|---|-------------------|--------------------|---------|--------|

### Positive Highlights

Things the API design does well, patterns worth preserving or replicating.