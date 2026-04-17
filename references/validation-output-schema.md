# Validation Output Schema

Use this reference for validation-style skills that produce findings, highlights, validation adjustments, and evidence metadata.

When a skill also consumes `k8s-controller-analyzer` JSON as input evidence, pair
this file with [analyzer-output-schema.md](./analyzer-output-schema.md). This
file defines report output semantics; the analyzer schema file defines analyzer
input semantics.

This schema remains documentation-first. It defines a canonical information model that skills can render as Markdown reports. It is not a strict JSON Schema or parser contract. However, validation-style skills in this repository should treat the ordering, casing, and normalization rules below as normative so repeated runs converge on byte-identical Markdown when the underlying evidence does not change.

## Goals

- Keep validation skills consistent without duplicating the same report contract in every `SKILL.md`
- Provide a canonical finding model that orchestration skills can merge and validate
- Preserve readable Markdown output while allowing optional JSON-like serialization when helpful
- Make report ordering and identifiers deterministic enough for golden testing

## Canonical Report Envelope

Use these top-level fields when describing structured output:

- `schemaVersion`
- `skill`
- `scope`
- `status`
- `scores`
- `severityCounts`
- `summary`
- `findings`
- `highlights`
- `validation`
- `extensions` (optional, for skill-specific metadata)

### Field Notes

- `schemaVersion`: schema identifier, for example `1.0`
- `skill`: skill name, for example `k8s.controller-api`
- `scope`: URI-like string describing what was assessed
- `status`: usually `completed` or `not_applicable`
- `scores`: skill-specific scoring object; do not force all skills to use the same score fields
- `severityCounts`: counts for critical, major, and minor findings
- `summary`: short narrative summary
- `findings`: list of canonical finding objects
- `highlights`: list of canonical highlight objects
- `validation`: list of validation outcomes, usually populated when findings were confirmed, adjusted, or dismissed

## Deterministic Rendering Rules

Validation-style skills in this repository should follow these rules unless a skill explicitly opts into exploratory behavior:

1. Resolve scope using a fixed search order documented in the skill file.
2. Normalize `scope` to a URI-like string whenever enough information is available.
3. Normalize every `where` entry to repo-relative `path#Lstart-Lend`, `path#Lstart`, or plain `path`.
4. Sort findings before assigning final IDs using:
   - severity order: `critical`, `major`, `minor`
   - `area`
   - first `where` entry
   - `title`
5. Sort highlights using:
   - `sourceSkill`
   - `description`
6. Assign final finding IDs only after sorting and after any validation adjustments have been applied.
7. Use stable display casing:
   - severities: `Critical`, `Major`, `Minor`
   - confidence: `High`, `Medium`, `Low`
8. Always render empty `highlights` and `validation` sections consistently rather than conditionally changing structure unless the skill explicitly documents otherwise.

## Canonical Finding Object

Use these fields for each finding:

- `id`
- `title`
- `severity`
- `area`
- `what`
- `where`
- `why`
- `fix`
- `confidence`
- `notVerified`

### Finding Field Notes

- `id`: stable identifier within the report, for example `API-001`
- `title`: short human-readable title
- `severity`: `critical`, `major`, or `minor`
- `area`: review area name used by the skill
- `what`: one-line statement of the issue
- `where`: list of repo-relative evidence locations
- `why`: explanation of impact and reasoning
- `fix`: concrete suggestion
- `confidence`: `high`, `medium`, or `low`
- `notVerified`: assumptions, runtime checks not performed, or unresolved unknowns; use an empty list or `[]` when there are none. Do not use this field for validation history; keep that in `validation`

### `scope`

`scope` is a URI-like string. Prefer:

- `repo://openshift/cluster-version-operator`
- `dir://controllers`
- `file://api/v1alpha1/myresource_types.go`
- `package://api/v1alpha1`
- `controller://ClusterDeploymentReconciler`
- `api://infrastructure.cluster.x-k8s.io/v1beta1/AWSCluster`
- `diff://HEAD`

If no scheme is present, treat the value as legacy free text for backward compatibility.

### `where`

`where` is a list of repo-relative GitHub-style location strings:

- `controllers/myresource_controller.go#L118-L146`
- `api/v1alpha1/myresource_types.go#L42-L49`
- `config/crd/bases/example.io_myresources.yaml#L88-L102`

Rules:

- Prefer `path#Lstart-Lend` when line ranges are known
- Use `path#Lstart` for single-line references
- Use plain `path` only when exact lines are unavailable
- Use multiple entries when a finding depends on more than one location

## Highlights

Use these fields for each positive highlight:

- `id`
- `sourceSkill`
- `description`

For leaf skills, `sourceSkill` is usually the same as `skill`. For orchestration skills, preserve the originating sub-skill using the canonical skill identifier.

## Validation Results

Use these fields when a validator confirms, adjusts, or dismisses findings:

- `findingId`
- `originalSeverity`
- `validatedSeverity`
- `verdict`
- `reason`
- `validationLayer` (optional)

Recommended values:

- `originalSeverity`: `critical`, `major`, `minor`
- `validatedSeverity`: `critical`, `major`, `minor`, `dismissed`
- `verdict`: `confirmed`, `adjusted`, `dismissed`
- `validationLayer`: `leaf`, `orchestrator`

### Validation Layers

Validation may operate in two complementary layers:

- `leaf`: per-finding deterministic second-pass review inside a single skill. The reviewer re-checks evidence, severity, and highlight contradictions using the same fixed checklist.
- `orchestrator`: cross-skill consistency review after merging findings from multiple skills. It checks deduplication correctness, cross-skill contradictions, and score recomputation.

Validation layers must not invent new findings during the second pass. They may only keep, adjust, or dismiss draft findings.

### Highlight Validation

When a validator reviews highlights, use:

- `highlightId`
- `verdict`
- `reason`
- `suggestedRewording` (optional)

Recommended values:

- `verdict`: `keep`, `remove`, `reword`

## Orchestrator Extensions

Orchestration skills may extend the base finding object with:

- `sources`: list of contributing skills
- `primarySource`: skill chosen for display and tie-breaking

Use canonical skill identifiers for these fields, for example:

- `k8s.controller-architecture`
- `k8s.controller-api`
- `k8s.controller-production-readiness`

Human-readable labels such as `Architecture` or `Production Readiness` should be derived only at render time. These fields are optional for leaf skills and expected for merged findings.

## Example Finding

```json
{
  "id": "API-001",
  "title": "Required field lacks effective validation",
  "severity": "major",
  "area": "CRD Structure and Field Conventions",
  "what": "`spec.endpoint` is optional in the schema but treated as required by the reconciler",
  "where": [
    "api/v1alpha1/myresource_types.go#L42-L49",
    "config/crd/bases/example.io_myresources.yaml#L88-L102"
  ],
  "why": "The API server can accept an object that later fails during reconciliation because the controller assumes the field is always present.",
  "fix": "Add explicit schema validation using `+kubebuilder:validation:Required` and regenerate the CRD.",
  "confidence": "high",
  "notVerified": []
}
```

## Example Leaf Report

```json
{
  "schemaVersion": "1.0",
  "skill": "k8s.controller-api",
  "scope": "dir://api",
  "status": "completed",
  "scores": {
    "score": 82,
    "interpretation": "Solid baseline, a few important gaps"
  },
  "severityCounts": {
    "critical": 0,
    "major": 2,
    "minor": 1
  },
  "summary": "The API design is mostly aligned with Kubernetes conventions, but the CRD has a few validation and versioning gaps that could cause upgrade or runtime issues.",
  "findings": [],
  "highlights": [],
  "validation": []
}
```

## Example Merged Finding

```json
{
  "id": "F-001",
  "title": "Finalizer cleanup can leave external state behind",
  "severity": "critical",
  "area": "Ownership, Finalizers, and Cleanup Logic",
  "what": "Cleanup removes the finalizer before confirming external deletion succeeded.",
  "where": [
    "controllers/myresource_controller.go#L118-L146"
  ],
  "why": "A failed cleanup may orphan external resources while allowing Kubernetes deletion to complete.",
  "fix": "Only remove the finalizer after cleanup succeeds or after the resource is proven safe to delete.",
  "confidence": "high",
  "notVerified": [],
  "sources": [
    "k8s.controller-architecture",
    "k8s.controller-production-readiness"
  ],
  "primarySource": "k8s.controller-architecture"
}
```

## Markdown Rendering Guidance

Validation skills may keep their current Markdown report style. When doing so:

- Render severities for humans as `Critical`, `Major`, `Minor`
- Render confidence for humans as `High`, `Medium`, `Low`
- Render `validatedSeverity` and verdicts in the validation table using display casing that matches the surrounding report
- Keep `where` readable by rendering the GitHub-style strings directly
- When a finding has multiple `where` entries, render them as a semicolon-separated list in summary tables and include the full list in detailed finding blocks

## Adoption Guidance

- Use this schema for validation-style skills that produce findings and supporting evidence
- Keep skill-specific scoring logic local to each skill
- Prefer `notVerified` everywhere instead of introducing alternative field names such as `unknowns`
- Treat this file as the shared contract; individual skills should only document the parts of output that are truly skill-specific
