---
name: k8s.controller-api
description: Review or audit Kubernetes CRD definitions, API types, webhooks, and markers for compliance with upstream Kubernetes API conventions.
---

# Kubernetes API Conventions Review

Review Kubernetes Custom Resource Definitions (CRDs), API types, webhooks, and kubebuilder markers for compliance with upstream Kubernetes API conventions.

## Inputs

- If `$ARGUMENTS` is provided, treat it as scope (files, package, API type name, or GitHub repository).
- GitHub repository inputs may be full URLs (for example, `https://github.com/org/repo`) or shorthand (`org/repo`).
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source and do not apply local `git diff` defaults.
- If no arguments are provided, assess current repository changes from git diff.
- If there are no changes, start with `api/` and `config/crd/` directories first; expand only when needed for evidence.
- If the project does not define Kubernetes APIs/CRDs (no API types, no CRD manifests, and no related markers), skip this skill entirely and report `Not applicable`.
- `--detail` includes a full breakdown of each finding (Why, Fix, metadata) after the summary tables. Without this flag, only the summary tables are produced.

## References

Consult [k8s-upstream.md](../../references/k8s-upstream.md) for the authoritative source of conventions and high-quality reference implementations.

## Assessment Areas

### 1. CRD Structure and Field Conventions

- CRD follows Kubernetes API conventions:
  - `spec` for desired state, `status` for observed state
  - Uses `int32`/`int64` for integers, avoids unsigned types
  - Enums are string-typed CamelCase values
  - Lists of named subobjects preferred over maps of subobjects
  - All optional fields have `+optional` marker and `omitempty` JSON tag
  - Required fields are validated with kubebuilder markers (`+kubebuilder:validation:Required`)
- Printer columns (`+kubebuilder:printcolumn`) show useful summary info in `kubectl get`

### 2. API Versioning

- API versioning follows Kubernetes conventions (v1alpha1 → v1beta1 → v1)
- Conversion webhooks are implemented when multiple versions coexist
- Storage version is explicitly marked (only relevant when multiple versions exist — skip for single-version CRDs)

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
7. Generate output with severity, concrete fix, confidence, and any unverified assumptions.

## Scoring

1. Start at 100.
2. For each finding, subtract points based on severity:
   - Critical: **-20** per finding
   - Major: **-10** per finding
   - Minor: **-3** per finding
3. Floor score at 0.

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
- Evidence rule: include at least one concrete file path and line reference for every Critical and Major finding.

### Findings (only with `--detail`)

This section is only included when the `--detail` flag is passed.

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

### Positive Highlights

Things the API design does well, patterns worth preserving or replicating.