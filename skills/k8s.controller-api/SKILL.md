---
name: k8s.controller-api
description: Review Kubernetes CRD definitions, API types, webhooks, and markers for compliance with upstream Kubernetes API conventions.
---

# Kubernetes API Conventions Assessment

Review Kubernetes Custom Resource Definitions (CRDs), API types, webhooks, and kubebuilder markers for compliance with upstream Kubernetes API conventions.
This skill is primarily designed for Go controllers built with `controller-runtime` and kubebuilder patterns.

## References

Consult [k8s-upstream.md](../../references/k8s-upstream.md) for upstream conventions.
Consult [analyzer-output-schema.md](../../references/analyzer-output-schema.md) for the analyzer JSON input contract.
Consult [validation-output-schema.md](../../references/validation-output-schema.md) for the canonical report model.
Consult [reproducible-assessments.md](../../references/reproducible-assessments.md) for deterministic execution rules.

## Inputs

- `$ARGUMENTS` may contain:
  - scope text such as files, packages, API type names, or GitHub repositories
  - `--mode=deterministic`
  - `--mode=exploratory`
- Default mode is `--mode=deterministic`.
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source.
- If no API types, CRD manifests, or webhook assets are found in the resolved scope, return `Not applicable`.

## Input Validation

- The only recognized flags are `--mode=deterministic` and `--mode=exploratory`.
- If `$ARGUMENTS` contains any other `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.

## Static Analyzer

In deterministic mode, build and run the static analyzer to extract structured facts from Go code and YAML manifests. This is the single evidence source.

Treat [analyzer-output-schema.md](../../references/analyzer-output-schema.md) as the normative schema for the analyzer JSON envelope and fact payloads.

### Run

```bash
<plugin-root>/scripts/k8s-controller-analyzer.sh <repo-root> --skill api --format json
```

If the orchestrator (`k8s.controller-assessment`) has already run the analyzer and the JSON is loaded in context, skip the run step.

### Load and use

Load the full JSON output into context. The output includes:

- A `manifest` section with `count`, `hash` (MANIFEST_HASH), and categorized file `entries` — include `manifest.hash` verbatim in the report for auditability
- A `facts` array with all extracted evidence

If `manifest.count` is 0, return `Not applicable`.

This is the **primary evidence source** for all analysis. The output contains facts with these kinds:

- `crd_type` — root CRD type markers (root marker, status subresource, resource scope, print columns, unsigned fields)
- `crd_field` — field-level markers (optional/required, validation, JSON tags, omitempty, field types)
- `crd_version` — CRD version storage/hub/spoke info
- `webhook` — webhook type, path, failurePolicy, sideEffects, timeoutSeconds (from Go markers)
- `crd_manifest` — CRD YAML: versions, conversion strategy, scope
- `webhook_manifest` — webhook YAML: failurePolicy, sideEffects, timeoutSeconds, scopes

Do not re-derive facts that the analyzer already provides.

### Fact-to-checklist mapping

| Analyzer Facts | Checklist Items |
|----------------|-----------------|
| `crd_type.has_status_field` + `crd_type.has_status_subresource` | 1a, 1b |
| `crd_type.resource_scope` | 1c |
| `crd_field.is_optional` + `crd_field.has_omitempty` | 1d |
| `crd_field.is_required` + `crd_field.markers` | 1e |
| `crd_type.unsigned_fields` | 1f |
| `crd_field.markers` (enum markers) | 1g |
| `crd_type.print_columns` | 1h |
| `crd_type.has_status_field` + `crd_field` (status struct fields) | 1i |
| `crd_version.storage` + `crd_version.hub` + `crd_version.spoke` | 2a-d |
| `webhook.failure_policy` + `webhook.side_effects` + `webhook.timeout_seconds` | 3a-c |
| `crd_type.has_root_marker` | 4e |
| `crd_field.markers` + `crd_type` | 4a-d |

## gopls Verification Protocol

Use gopls MCP tools to verify and enrich the static analyzer's structural findings. These verification steps are **mandatory** in deterministic mode.

### Field type verification (checklist 1d-g)

For each `crd_field` fact:

1. If a field has validation markers that might mismatch its type (e.g., `Minimum` on a string):
   - Call `go_file_context` on the API types file to confirm the field's Go type
   - Cross-reference marker applicability with the resolved type

2. For enum fields with `kubebuilder:validation:Enum` markers:
   - Call `go_symbol_references` for the enum type to find all declared constants
   - Verify marker enum values match the declared constants

### Version conversion verification (checklist 2a-d)

For each set of `crd_version` facts for the same kind:

1. If multiple versions exist and hub/spoke flags are set:
   - Call `go_search` for `ConvertTo` and `ConvertFrom` methods to verify conversion implementations
   - Verify schema differences between versions warrant conversion

### Webhook handler verification (checklist 3e)

For each `webhook` fact:

1. Call `go_symbol_references` for the webhook handler methods (`Default`, `ValidateCreate`, etc.)
2. In the handler body, check for external calls that could block:
   - Use `go_file_context` on the handler file to see imports (HTTP clients, external services)

### Marker correctness verification (checklist 4a-f)

For `crd_type` and `crd_field` facts:

1. Use `go_file_context` on API type files to see full type context
2. For 4f (generated output matches committed): compare `crd_type` facts against committed CRD YAML from the manifest

## Assessment Areas

Use area names exactly as written in the section headings below.

### 1. CRD Structure and Field Conventions

- **1a. Spec and status follow Kubernetes conventions**
  - title: "Spec/status structure violates Kubernetes conventions"
  - finding: `spec` or `status` struct layout does not follow Kubernetes conventions (e.g., status contains desired-state fields, spec contains observed-state fields) (`Major`)
  - pass: `spec` holds desired state and `status` holds observed state consistent with Kubernetes conventions
  - not-observed: no `spec` or `status` structs in scope

- **1b. Status subresource marker is present**
  - title: "Missing status subresource marker"
  - finding: a `Status` field exists on the root type but `+kubebuilder:subresource:status` is absent (`Major`)
  - pass: `+kubebuilder:subresource:status` is present when a `Status` field exists, or no `Status` field exists
  - not-observed: no root types with `Status` fields in scope

- **1c. Resource scope is explicitly set**
  - title: "Resource scope not explicitly set"
  - finding: `+kubebuilder:resource:scope=` is absent on a root type (`Minor`)
  - pass: `+kubebuilder:resource:scope=` is explicitly set on all root types
  - not-observed: no root types in scope

- **1d. Optional fields have +optional and omitempty**
  - title: "Optional field missing +optional or omitempty"
  - finding: an optional field lacks `+optional` marker or `omitempty` JSON tag (`Major`)
  - pass: all optional fields have both `+optional` and `omitempty`
  - not-observed: no optional fields in scope

- **1e. Required fields use explicit validation markers**
  - title: "Required field lacks effective validation"
  - finding: a required field has no `+kubebuilder:validation:Required` marker or equivalent schema enforcement (`Major`)
  - pass: all required fields use explicit validation markers
  - not-observed: no required fields in scope

- **1f. Numeric fields avoid unsigned types**
  - title: "Numeric field uses unsigned type"
  - finding: a CRD field uses an unsigned integer type (`uint`, `uint32`, `uint64`) which is not representable in JSON Schema (`Minor`)
  - pass: all numeric fields use signed types
  - not-observed: no numeric fields in scope

- **1g. Enums are string-typed CamelCase values**
  - title: "Enum not string-typed CamelCase"
  - finding: an enum field is not string-typed or its values do not follow CamelCase convention (`Major`)
  - pass: all enum fields are string-typed with CamelCase values
  - not-observed: no enum fields in scope

- **1h. Printer columns are defined**
  - title: "Missing printer columns"
  - finding: no `+kubebuilder:printcolumn` markers are defined on a root type (`Minor`)
  - pass: printer columns are defined on root types
  - not-observed: no root types in scope

- **1i. Status struct includes observedGeneration**
  - title: "Status struct missing observedGeneration field"
  - finding: the status struct does not include an `observedGeneration` field to signal which `.metadata.generation` the controller last processed (`Major`)
  - pass: the status struct includes an `observedGeneration int64` field with a `+optional` marker
  - not-observed: no status struct in scope

### 2. API Versioning

- **2a. Multi-version CRDs have one explicit storage version**
  - title: "Multi-version CRD missing explicit storage version"
  - finding: a CRD serves multiple versions but does not mark exactly one as the storage version (`Critical`)
  - pass: exactly one version is marked as storage version, or only one version exists
  - not-observed: no multi-version CRDs in scope

- **2b. Served and deprecated versions are intentional**
  - title: "Unintentional version serving configuration"
  - finding: a version is served or deprecated without clear intent (e.g., a deprecated version is still marked served with no migration note) (`Major`)
  - pass: served and deprecated flags are consistent and intentional
  - not-observed: no multi-version CRDs in scope

- **2c. Conversion strategy matches schema differences**
  - title: "Conversion strategy mismatches schema differences"
  - finding: versions have schema differences but the CRD uses `None` conversion strategy, or a webhook conversion is configured but schemas are identical (`Critical`)
  - pass: conversion strategy is appropriate for the schema differences between versions
  - not-observed: no multi-version CRDs in scope

- **2d. Version evolution is additive**
  - title: "Non-additive version evolution without migration"
  - finding: a new version removes or renames fields from a prior version without a documented migration path (`Major`)
  - pass: version evolution is additive, or a migration path is documented
  - not-observed: no version evolution evidence in scope

### 3. Webhooks

Skip this area if no admission webhook assets exist in scope.

- **3a. Failure policy is explicit**
  - title: "Webhook failure policy not explicit"
  - finding: a webhook configuration omits `failurePolicy`, relying on the server default (`Major`)
  - pass: `failurePolicy` is explicitly set on all webhook configurations
  - not-observed: no webhook configurations in scope

- **3b. sideEffects is explicit**
  - title: "Webhook sideEffects not explicit"
  - finding: a webhook configuration omits `sideEffects`, relying on the server default (`Major`)
  - pass: `sideEffects` is explicitly set on all webhook configurations
  - not-observed: no webhook configurations in scope

- **3c. timeoutSeconds is explicit**
  - title: "Webhook timeoutSeconds not explicit"
  - finding: a webhook configuration omits `timeoutSeconds`, relying on the server default (`Minor`)
  - pass: `timeoutSeconds` is explicitly set on all webhook configurations
  - not-observed: no webhook configurations in scope

- **3d. Webhook match scope is narrow**
  - title: "Webhook match scope too broad"
  - finding: a webhook matches more resources or namespaces than the controller manages (`Major`)
  - pass: webhook match rules are scoped to the resources and namespaces the controller owns
  - not-observed: no webhook configurations in scope

- **3e. No long-running or externally dependent admission logic**
  - title: "Long-running or externally dependent admission logic"
  - finding: admission webhook handler performs long-running operations or calls external services that can block API server requests (`Major`)
  - pass: admission logic is fast and self-contained
  - not-observed: no webhook handler implementations in scope

### 4. Marker and Generated Output Correctness

- **4a. Marker names are valid and spelled correctly**
  - title: "Invalid or misspelled marker name"
  - finding: a kubebuilder marker name is invalid, misspelled, or not recognized by controller-gen (`Critical`)
  - pass: all marker names are valid and correctly spelled
  - not-observed: no kubebuilder markers in scope

- **4b. Marker type usage matches field types**
  - title: "Marker type mismatch with field type"
  - finding: a validation marker is applied to a field type it does not support (e.g., `+kubebuilder:validation:Minimum` on a string field) (`Major`)
  - pass: all marker types are compatible with their field types
  - not-observed: no validation markers in scope

- **4c. Enum marker values match declared constants**
  - title: "Enum marker values do not match declared constants"
  - finding: `+kubebuilder:validation:Enum` values do not match the declared Go constants for the type (`Major`)
  - pass: enum marker values match declared constants
  - not-observed: no enum markers in scope

- **4d. Nested defaults are reachable**
  - title: "Unreachable nested default value"
  - finding: a `+kubebuilder:default` marker on a nested field is unreachable because a parent field lacks a default or is optional without a default (`Major`)
  - pass: all nested defaults are reachable through parent defaults or required fields
  - not-observed: no nested default markers in scope

- **4e. Root markers are present on root types**
  - title: "Missing root markers on root types"
  - finding: a root type (the type that becomes a CRD) is missing `+kubebuilder:object:root=true` or equivalent root marker (`Major`)
  - pass: all root types have root markers
  - not-observed: no root types in scope

- **4f. Generated CRD output matches committed manifests**
  - title: "Generated CRD output differs from committed manifest"
  - finding: the committed CRD manifest does not match what the current markers and types would generate (`Major`)
  - pass: committed CRD manifests are consistent with the current type definitions and markers
  - not-observed: no committed CRD manifests in scope

## Deterministic Procedure

Run the assessment in this exact order:

1. Resolve scope from explicit scope text or `$ARGUMENTS`.
2. Run the static analyzer with `--skill api`. Load the full JSON output into context.
3. If `manifest.count` is 0, return `Not applicable`.
4. Run the gopls verification protocol for checklist areas 1, 2, 3, and 4 as specified above.
5. Walk every checklist item (1a through 4f) in order. For each item, record a disposition: `finding`, `pass`, or `not-observed` using the criteria defined above, the analyzer facts, and gopls verification results. Do not skip items.
6. Draft findings only from items with `finding` disposition. Every finding must trace to a specific checklist item ID (e.g., "1d"). Observations outside the checklist may appear in a `Notes` section but do not receive an ID, severity, or score impact.
7. Run one deterministic leaf validation pass in the same session:
   - dismiss findings unsupported by cited evidence
   - dismiss any anti-finding violations
   - lower severity only when the written criteria support the lower level
   - remove or reword highlights that contradict findings
8. Sort findings using the shared schema rules. Assign final IDs after sorting.
9. Compute score and severity counts.

Use area names exactly as written in the section headings above.

In deterministic mode do not:

- default to `git diff`
- expand scope beyond what the evidence manifest discovered
- launch subagents
- run `controller-gen` or other code generators
- use ad-hoc `sg` (ast-grep) queries for facts the analyzer already provides
- skip gopls verification steps defined in the protocol

`--mode=exploratory` may widen scope after the manifest and may run additional searches, but the report format and scoring stay the same.

## Severity Mapping

Severity for each checklist item is defined inline in the assessment areas above. When evidence fits two adjacent levels and the criteria do not force the higher level, choose the lower level.

## Scoring

1. Start at 100.
2. Subtract:
   - `Critical`: 20
   - `Major`: 10
   - `Minor`: 3
3. Floor score at 0.
4. Show arithmetic in the report.

Interpretation:

- `90-100`: Production-ready with minor polish
- `75-89`: Solid baseline, a few important gaps
- `50-74`: Significant issues to address before production
- `<50`: High operational risk; major redesign or fixes recommended

## Output Format

Produce the assessment using the canonical model from [validation-output-schema.md](../../references/validation-output-schema.md).

Output conventions:

- `scope` should use a URI-like string such as `repo://org/repo`, `dir://api`, or `api://group/version/Kind`
- `where` should use repo-relative GitHub-style location strings
- Sort findings by severity, area, `where`, and title before assigning final IDs

### Evidence Manifest

Paste the `manifest` section from the analyzer JSON output, including `count`, `hash`, and the categorized file entries. Do not edit, reformat, or summarize the output. This section makes the report auditable and verifiable.

### Summary

Write 2-3 sentences describing API quality and conventions compliance.

Score table:

| Metric | Value |
|--------|-------|
| **Score** | 0-100 or `Not applicable` |
| **Interpretation** | Production-ready with minor polish / Solid baseline, a few important gaps / Significant issues to address before production / High operational risk |

Severity count table:

| Severity | Count |
|----------|-------|
| Critical | _n_ |
| Major | _n_ |
| Minor | _n_ |

Findings summary table (use the checklist `title` in the What column):

| # | Severity | Area | What | Where | Confidence |
|---|----------|------|------|-------|------------|

### Findings

For each finding, use the canonical text from the checklist item:

- **Title**: use the `title` from the checklist item verbatim
- **What**: use the `finding` text from the checklist item verbatim
- **Checklist item**: the item ID (e.g., 1d)
- **Severity**: from the checklist item
- **Area**: exact section heading
- **Where**: repo-relative paths with line ranges from the evidence
- **Confidence**: `High` if all `where` entries have line ranges, `Medium` if any are path-only, `Low` if no `where` references
- **Not verified**: assumptions or checks not performed
- **Why**: explain how the specific code triggers this checklist criterion — reference the actual function names, variables, and control flow found in the evidence
- **Fix**: concrete suggested change using the specific types and patterns from the codebase

### Validation

Include this section when one or more findings or highlights changed during the deterministic second pass. Otherwise render "No adjustments."

### Highlights

Include only positive highlights that do not contradict any finding.

### Notes

Optional. Observations outside the checklist that may be useful but do not receive IDs, severity, or score impact.
