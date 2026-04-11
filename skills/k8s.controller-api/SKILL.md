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
Consult [validation-output-schema.md](../../references/validation-output-schema.md) for the canonical finding, evidence, highlight, and validation model used by validation-style skills.

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

- API versioning often follows a maturation path such as `v1alpha1` → `v1beta1` → `v1`, but not every API needs every stage. Review whether version naming, stability expectations, and migration semantics are clear and consistent
- For multi-version CRDs, `served`, `storage`, and deprecation semantics are intentional and documented
- Deprecated versions have a clear migration path and are not left served indefinitely without justification
- Multi-version CRDs use an appropriate conversion strategy. Use a conversion webhook when schema or semantic differences require it; do not assume every multi-version CRD needs a webhook
- Conversions preserve semantic meaning across versions and avoid silent data loss on round-trip conversion
- Validation and defaulting changes across versions are compatibility-safe: avoid tightening validation, changing defaults, or dropping fields in ways that break existing stored objects or upgrades
- Favor additive schema evolution where possible. Treat breaking field renames, enum changes, or required-field additions as high-risk unless there is an explicit migration or compatibility strategy
- `+kubebuilder:storageversion` marker is present on exactly one version when multiple API versions coexist (`must`). For single-version CRDs the marker is recommended but its absence is not an error (`contextual`).

### 3. Webhooks (if present)

Skip this area entirely if the project has no admission webhooks. Only assess when webhook configurations or webhook handler code exist.

- Defaulting webhook sets sensible defaults
- Validating webhook rejects invalid input
- Failure policy is explicitly set (`Fail` or `Ignore`) based on criticality
- `sideEffects` is declared (typically `None`)
- `timeoutSeconds` is explicitly set and kept small enough to avoid stalling admission
- `admissionReviewVersions` is current and compatible with the target clusters
- `matchPolicy`, rules, and any namespace or object selectors are intentional and do not accidentally bypass required admission coverage
- Webhook scope is tight: only the intended groups, versions, resources, and operations are intercepted
- Dry-run behavior is consistent with the declared `sideEffects`
- Avoid long-running logic or external dependencies on the admission path unless they are clearly necessary and failure-tolerant
- For simple validation logic, consider whether `ValidatingAdmissionPolicy` is a better fit than a validating webhook. Treat this as a recommendation (`should`/`contextual`), not a blocker
- Prefer `ValidatingAdmissionPolicy` for declarative, object-local validation rules; reserve validating webhooks for logic that needs custom code, cross-object awareness, mutation/defaulting, or behavior not expressible cleanly in policy/CEL
- Webhook certificate rotation is handled

### 4. CRD Generation and Marker Correctness

- Consider explicitly marking fields with kubebuilder/controller-gen validation markers such as `+kubebuilder:validation:Required` and `+optional` to avoid ambiguity
- Be aware that zero values pass required field validation (OpenAPI checks non-null only) — use `MinLength`, `Minimum` markers when meaningful
- Inspect generated CRD manifests — controller-gen may silently ignore unrecognized markers:
  - Check for typos in `+kubebuilder:` markers by comparing against the known set (`validation`, `default`, `printcolumn`, `rbac`, `object`, `subresource`, etc.) — any unrecognized marker is silently dropped
  - Verify marker/field type alignment — e.g., `+kubebuilder:validation:Minimum=1` on a string field produces no validation in the generated CRD
  - Check `+kubebuilder:validation:Enum` values match the corresponding const block defining valid values
- Review CEL validation (`+kubebuilder:validation:XValidation` / `x-kubernetes-validations`) when cross-field invariants, conditional requirements, or immutability rules exist
- Prefer CEL for declarative validation that is local to the object; keep validating webhooks for cases that need external lookups, mutation, or more complex programmatic checks
- Verify CEL rules remain compatible with version evolution and do not unintentionally reject older persisted objects after schema changes
- Watch out for nested defaulting: when a nested struct field has `+kubebuilder:default:` markers, the parent field must have `+kubebuilder:default:{}` or the nested defaults are never applied
- Verify `+kubebuilder:object:root=true` is present on root types and that generated deepcopy files (`zz_generated.deepcopy.go`) are not stale relative to current type definitions
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
7. **Adversarial validation**: Launch a clean-context validator subagent with the draft findings, draft highlights, and scope. See [Leaf Validator Subagent](#leaf-validator-subagent) for the validator brief and isolation rules.
8. Apply validation results: update severities, remove dismissed findings, adjust or remove highlights per validator verdicts. Validation **always runs** and its results are applied before scoring regardless of flags. The detailed Validation output section is only included in the report when `--details` is passed.
9. Generate output with severity, concrete fix, confidence, and any unverified assumptions.

## Leaf Validator Subagent

After the primary analysis produces draft findings and draft highlights, launch a **separate subagent** to adversarially validate the results. The validator operates with a clean context — it does not receive the primary reviewer's reasoning or intermediate notes.

### Isolation rules

- Run in a separate subagent
- Receive only: the scope (from `$ARGUMENTS` or resolved defaults), draft findings, and draft highlights
- Re-read code independently from the evidence locations in each finding's `where` field
- Do not rely on the primary reviewer's internal reasoning

### Validator brief

> **Role**: You are a skeptical reviewer. Your job is to challenge each finding from an API conventions assessment and determine whether it actually impacts runtime behavior, correctness, or operational safety. You also verify that positive highlights do not contradict the findings.
>
> **Inputs you receive**:
> - The draft findings list (each following the canonical finding model from `validation-output-schema.md`)
> - The draft positive highlights list (each with: `id`, `sourceSkill`, `description`). For this leaf skill, `sourceSkill` is always `k8s.controller-api`
> - The scope so you can read the same code
>
> **Your task**:
>
> **Part 1 — Validate findings:**
> For each finding, independently read the code at the referenced location and evaluate:
> 1. **Is the finding accurate?** Does the code actually exhibit the described problem?
> 2. **Does it affect behavior?** Would fixing this change runtime behavior, correctness, or operational safety — or is it purely stylistic / cosmetic / theoretical?
> 3. **Is the severity appropriate?** A pattern that looks non-ideal but cannot cause incorrect behavior, data loss, or operational failure should be downgraded.
>
> **Part 2 — Validate highlights against findings:**
> For each positive highlight, check whether it contradicts any finding:
> - A highlight must not praise a pattern that is also flagged as a finding.
> - If a highlight praises a pattern whose absence is flagged as a finding anywhere in the report, mark the highlight for removal or rewording.
> - If the same concept is used correctly in some code paths and incorrectly in others, the correct usage is not a highlight — only the incorrect usage should appear (as a finding).
> - Highlights that do not conflict with any finding should be kept as-is.
>
> **Downgrade rules** (findings only):
> - A finding that is factually correct but has **no behavioral impact** → downgrade by one level (Critical→Major, Major→Minor, Minor→dismiss)
> - A finding where the described problem **cannot occur** given the surrounding code → dismiss entirely
> - A finding where the severity is appropriate and the behavioral impact is real → keep as-is
>
> **Output schema**:
>
> Finding validations (one entry per finding):
> ```
> findingId: <id matching the draft findings list>
> originalSeverity: critical / major / minor
> validatedSeverity: critical / major / minor / dismissed
> verdict: confirmed / downgraded / dismissed
> reason: <1-2 sentences explaining why — reference specific code evidence>
> validationLayer: leaf
> ```
>
> Highlight validations (one entry per highlight that needs adjustment):
> ```
> highlightId: <id matching the draft highlights list>
> verdict: keep / remove / reword
> reason: <1-2 sentences explaining the contradiction with a specific finding>
> suggestedRewording: <if verdict is reword, the revised text — omit if remove or keep>
> ```
>
> **Verdict vocabulary**: Leaf validators use `confirmed`, `downgraded`, or `dismissed`. The orchestrator validator uses `confirmed`, `adjusted`, or `dismissed` instead, because it operates on already-validated findings and makes cross-skill adjustments rather than per-finding accuracy checks.
>
> Do **not** produce new findings. Your role is to validate, not to review.

### Applying validation results

1. Update each finding's severity to the `validatedSeverity`.
2. Remove any finding with `validatedSeverity: dismissed`.
3. Remove any highlight with `verdict: remove`.
4. Replace any highlight with `verdict: reword` with the validator's `suggestedRewording`.
5. Keep downgrade and dismissal rationale for the `validation` results. Do not append validator provenance into the finding's `notVerified` field.
6. Recompute severity counts using the validated severities.

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
Use the canonical report, finding, highlight, and validation model from [validation-output-schema.md](../../references/validation-output-schema.md).

Output conventions:

- `scope` should follow the shared URI-like form when expressed structurally (for example, `diff://working-tree`, `repo://org/repo`, `api://group/version/Kind`)
- `where` should use repo-relative GitHub-style location string(s) (for example, `api/v1alpha1/myresource_types.go#L42-L49`)
- Use the shared `notVerified` concept consistently; render it in Markdown as `Not verified`

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

- **Where** must include repo-relative GitHub-style location string(s) for every Critical and Major finding.

### Findings (only with `--details`)

This section is only included when the `--details` flag is passed.

For each finding (numbered to match the summary table), produce:

#### _N_. _Finding title_

| | |
|---|---|
| **Severity** | Critical / Major / Minor |
| **Area** | Assessment area name |
| **Where** | GitHub-style location string(s) |
| **Confidence** | High / Medium / Low |
| **Not verified** | Shared `notVerified` content rendered for humans (or `—`) |

**Why**: Explanation of why this is an issue, with reference to upstream convention if applicable.

**Fix**: Concrete suggested change.

---

### Validation (only with `--details`)

**Do NOT include this section unless `--details` is passed.** When `--details` is active and the validation phase produced changes, list each downgraded or dismissed finding:

| # | Original Severity | Validated Severity | Verdict | Reason |
|---|-------------------|--------------------|---------|--------|

Highlight validation changes (only when one or more highlights were removed or reworded):

| Highlight | Verdict | Reason | Suggested Rewording |
|-----------|---------|--------|---------------------|

### Positive Highlights

Things the API design does well, patterns worth preserving or replicating.