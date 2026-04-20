---
name: k8s.controller-api
description: Review Kubernetes CRD definitions, API types, webhooks, and markers for compliance with upstream Kubernetes API conventions.
user-invocable: true
allowed-tools:
  - Read
  - Grep
  - Glob
  - mcp__k8s-controller-analyzer__analyze_controller
  - mcp__gopls__go_file_context
  - mcp__gopls__go_symbol_references
  - mcp__gopls__go_search
  - mcp__gopls__go_package_api
  - LSP
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

Use the `analyze_controller` MCP tool with `repo_path` set to the repository root and `skill` set to `api`.

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
| `crd_field.list_type` + `crd_field.list_map_keys` | 5a-b |
| `crd_field.cel_rules` + `crd_type.cel_rules` | 6a-c |
| `webhook` (validating) + `crd_field.cel_rules` | 6a |
| `webhook` (defaulting) + `crd_field.markers` (defaults) | 7a-c |
| `webhook_manifest` (mutating) | 7d |
| `crd_field.cel_rules` + `crd_field.field_type` + `crd_field.markers` (MaxItems) | 8a |
| `crd_field.cel_rules` + `crd_field.is_optional` | 8b |

## gopls Verification Protocol

Use gopls MCP tools to verify and enrich the static analyzer's structural findings. These verification steps are **mandatory** in deterministic mode.

If the `gopls-lsp` MCP server is unavailable, use the built-in `LSP` tool with gopls as a fallback:

| MCP tool | LSP equivalent |
|----------|---------------|
| `go_file_context` | `documentSymbol` on the file, then `hover` on specific symbols |
| `go_symbol_references` | `findReferences` at the symbol's position |
| `go_search` | `workspaceSymbol` with the symbol name |
| `go_package_api` | `documentSymbol` on the package's Go files |

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

### List type verification (checklist 5a-b)

For each `crd_field` fact where `field_type` starts with `[]`:

1. If `list_type` is empty, check whether the field is a slice of a struct type (as opposed to `[]string` or `[]int` which are typically atomic)
   - Call `go_file_context` on the API types file to confirm the element type
   - Struct-element slices without `+listType` are a 5a finding
2. If `list_type` is "map" and `list_map_keys` is empty, this is a 5b finding

### CEL and immutability verification (checklist 6a-c)

1. For each `webhook` fact with type "validating":
   - Call `go_file_context` on the webhook handler to inspect validation logic
   - If the validation is simple enough for CEL (field range, enum, cross-field comparison), assess for 6a
2. For each `crd_field.cel_rules` entry where `uses_old_self` is true:
   - Check whether a non-transition validation exists for the same field (another CEL rule without `oldSelf`, or a webhook) for 6b
3. For 6c, use `go_search` for comments or documentation mentioning "immutable" near spec fields, then verify enforcement exists

### Defaulting webhook verification (checklist 7a-c)

For each `webhook` fact with type "defaulting":

1. Call `go_file_context` on the webhook handler file to find the `Default()` method
2. For 7a: inspect whether field assignments in `Default()` are guarded by nil/zero checks (e.g., `if r.Spec.Replicas == nil { r.Spec.Replicas = ptr.To(int32(1)) }`)
3. For 7b: compare default values set in `Default()` against `+kubebuilder:default` markers on the same fields from `crd_field.markers`
4. For 7c: identify fields set by `Default()` and check whether they are spec fields that a user would normally control (fields without `+kubebuilder:default` and without clear "system default" semantics)

### Webhook reinvocation verification (checklist 7d)

For each `webhook_manifest` entry with mutating webhooks:

1. Check whether `reinvocationPolicy` is present in the webhook configuration YAML

### CEL cost and safety verification (checklist 8a-b)

For each `crd_field` fact with `cel_rules`:

1. For 8a: if `field_type` starts with `[]` or `map[`, check whether the field has a `+kubebuilder:validation:MaxItems` or `+kubebuilder:validation:MaxProperties` marker in its `markers` list. If no size bound exists and the CEL rule uses iteration patterns (`self.items.all`, `self.exists`, `self.filter`, `self.map`), flag it.
2. For 8b: if the field is `is_optional` and the CEL rule text references sub-fields of the optional field without a `has()` guard, flag it. Use `go_file_context` on the API type file to confirm the field is a pointer type.

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

- **1i. Status struct includes ObservedGeneration**
  - title: "Status struct missing ObservedGeneration field"
  - finding: the status struct does not include an `ObservedGeneration` field to signal which `.metadata.generation` the controller last processed (`Major`)
  - pass: the status struct includes an `ObservedGeneration int64` field with a `+optional` marker
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
  - > **Ownership**: this skill owns the design question (is the strategy appropriate for the schema diff?). Lifecycle 4a owns the operational question (will upgrade silently lose data?). When both fire, the orchestrator merges with this skill as primarySource.
  - > **Deterministic mode**: assess only the coarse signal — if `has_multiple_served` is true and `conversion_strategy` is `None`, raise a finding. The analyzer does not perform schema-diff between CRD versions, so detailed schema comparison requires exploratory mode with `go_file_context` on API type files across versions.

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
  - > **Deterministic mode**: skip this item entirely. Verification requires running `controller-gen` to produce expected output and comparing against committed manifests. In exploratory mode, compare `crd_type` facts against committed CRD YAML from the manifest.

### 5. List Type and Merge Semantics

- **5a. Slice fields have +listType marker**
  - title: "Slice field missing +listType marker"
  - finding: a slice field in a CRD type lacks a `+listType` marker, causing undefined merge behavior with Server-Side Apply and strategic merge patch (`Major`)
  - pass: all slice fields have `+listType={atomic,set,map}` marker
  - not-observed: no slice fields in scope

- **5b. Map-type lists have +listMapKey**
  - title: "Map-type list missing +listMapKey"
  - finding: a slice field with `+listType=map` lacks a `+listMapKey` marker, preventing SSA from identifying individual list elements for merge (`Critical`)
  - pass: all `+listType=map` fields have corresponding `+listMapKey` markers
  - not-observed: no map-type list fields in scope

### 6. CEL Validation Rules

- **6a. CEL rules are present where validation webhooks could be replaced**
  - title: "Validation webhook used where CEL rule would suffice"
  - finding: a validating webhook performs simple field validation (range checks, enum enforcement, cross-field comparisons) that could be expressed as a CEL validation rule, avoiding webhook round-trip overhead (`Minor`)
  - pass: CEL rules are used for simple validations, OR webhook validation involves complex logic requiring Go code
  - not-observed: no validation webhooks or CEL rules in scope

- **6b. CEL transition rules use oldSelf correctly**
  - title: "CEL transition rule missing CREATE-path validation"
  - finding: a CEL transition rule using `oldSelf` is the sole validation for a field, leaving the CREATE path unvalidated since transition rules only fire on UPDATE (`Major`)
  - pass: transition rules are complemented by non-transition validation for the CREATE path
  - not-observed: no CEL transition rules in scope

- **6c. Field immutability is enforced**
  - title: "Immutable field lacks enforcement"
  - finding: a spec field is treated as immutable by the controller (ignored on update, or documented as immutable) but has no enforcement via CEL transition rule or validating webhook (`Major`)
  - pass: immutable fields are enforced via `+kubebuilder:validation:XValidation:rule="self == oldSelf"` or equivalent webhook validation
  - not-observed: no evidence of immutable fields in scope

### 7. Defaulting and Admission Semantic Correctness

- **7a. Defaulting webhook is idempotent**
  - title: "Non-idempotent defaulting webhook"
  - finding: the `Default()` method modifies fields unconditionally without checking their current value, meaning applying it twice produces different results or overwrites user-set values (`Major`)
  - pass: `Default()` checks whether fields are already set (nil/zero checks) before applying defaults
  - not-observed: no defaulting webhook in scope
  - > **Deterministic mode**: requires gopls inspection of the `Default()` handler body. Use `go_file_context` on the webhook handler file identified by `webhook` facts with `type == "defaulting"`. If gopls is unavailable, defaults to `not-observed`.

- **7b. Schema defaults are consistent with webhook defaults**
  - title: "Schema defaults inconsistent with webhook defaults"
  - finding: `+kubebuilder:default` markers set a different value than the `Default()` webhook method for the same field, creating order-dependent behavior (`Major`)
  - pass: schema defaults and webhook defaults agree, OR only one mechanism is used
  - not-observed: no defaulting webhook or no schema defaults in scope
  - > **Deterministic mode**: requires gopls inspection of the `Default()` handler body to compare against `crd_field.markers` defaults. Use `go_file_context` on the webhook handler file. If gopls is unavailable, defaults to `not-observed`.

- **7c. Defaulting does not set user-owned fields**
  - title: "Defaulting webhook sets user-owned spec fields"
  - finding: the `Default()` method unconditionally writes to spec fields that the user should control, causing SSA field ownership conflicts where the webhook field manager competes with the user (`Major`)
  - pass: `Default()` only sets fields that have no user-meaningful zero value, OR uses SSA-aware patterns
  - not-observed: no defaulting webhook in scope
  - > **Deterministic mode**: requires gopls inspection of the `Default()` handler body. Use `go_file_context` on the webhook handler file. If gopls is unavailable, defaults to `not-observed`.

- **7d. Webhook reinvocation policy is explicit**
  - title: "Mutating webhook missing reinvocation policy"
  - finding: a mutating webhook configuration omits `reinvocationPolicy`, meaning mutations may be overwritten by later webhooks without re-evaluation (`Minor`)
  - pass: `reinvocationPolicy` is explicitly set on mutating webhook configurations
  - not-observed: no mutating webhook configurations in scope

### 8. CEL Cost and Safety

- **8a. CEL rules on unbounded collections have size limits**
  - title: "CEL rule on unbounded collection without size limit"
  - finding: a CEL validation rule operates on a slice or map field that lacks `+kubebuilder:validation:MaxItems` or `+kubebuilder:validation:MaxProperties`, causing the API server to reject the CRD at registration due to cost budget overflow (`Critical`)
  - pass: all fields with CEL rules that iterate over collections have explicit size bounds
  - not-observed: no CEL rules on collection fields in scope

- **8b. CEL expressions guard against null optional fields**
  - title: "CEL expression accesses optional field without null guard"
  - finding: a CEL expression accesses a field marked `+optional` (pointer type) without a `has()` guard, causing CEL evaluation failures on nil values (`Major`)
  - pass: all CEL expressions on optional fields use `has(self.field)` or equivalent null guards
  - not-observed: no CEL rules on optional fields in scope

## Deterministic Procedure

Run the assessment in this exact order:

1. Resolve scope from explicit scope text or `$ARGUMENTS`.
2. Run the `analyze_controller` MCP tool with `skill=api`. Load the full JSON output into context.
3. If `manifest.count` is 0, return `Not applicable`.
4. Run the gopls verification protocol for checklist areas 1, 2, 3, 4, 5, 6, 7, and 8 as specified above.
5. Walk every checklist item (1a through 8b) in order. For each item, record a disposition: `finding`, `pass`, or `not-observed` using the criteria defined above, the analyzer facts, and gopls verification results. Do not skip items.
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
