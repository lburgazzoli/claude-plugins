---
name: k8s.controller-production-readiness
description: Evaluate a Kubernetes controller's production readiness by reviewing test coverage, observability, and deployment hardening.
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

# Kubernetes Controller Production Readiness Assessment

Evaluate a Kubernetes controller's production readiness focusing on test coverage, observability, and operational hardening.
This skill is primarily designed for Go controllers built with `controller-runtime` and kubebuilder patterns.

## References

Consult [k8s-upstream.md](../../references/k8s-upstream.md) for upstream conventions.
Consult [analyzer-output-schema.md](../../references/analyzer-output-schema.md) for the analyzer JSON input contract.
Consult [validation-output-schema.md](../../references/validation-output-schema.md) for the canonical report model.
Consult [reproducible-assessments.md](../../references/reproducible-assessments.md) for deterministic execution rules.

## Inputs

- `$ARGUMENTS` may contain:
  - scope text such as files, packages, controller names, or GitHub repositories
  - `--mode=deterministic`
  - `--mode=exploratory`
- Default mode is `--mode=deterministic`.
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source.
- If no controller implementation assets, relevant tests, or deployment manifests are found in the resolved scope, return `Not applicable`.

## Input Validation

- The only recognized flags are `--mode=deterministic` and `--mode=exploratory`.
- If `$ARGUMENTS` contains any other `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.

## Static Analyzer

In deterministic mode, build and run the static analyzer to extract structured facts from Go code, YAML manifests, and test file discovery. This is the single evidence source.

Treat [analyzer-output-schema.md](../../references/analyzer-output-schema.md) as the normative schema for the analyzer JSON envelope and fact payloads.

### Run

Use the `analyze_controller` MCP tool with `repo_path` set to the repository root and `skill` set to `production-readiness`.

If the orchestrator (`k8s.controller-assessment`) has already run the analyzer and the JSON is loaded in context, skip the run step.

### Load and use

Load the full JSON output into context. The output includes:

- A `manifest` section with `count`, `hash` (MANIFEST_HASH), and categorized file `entries` — include `manifest.hash` verbatim in the report for auditability
- A `facts` array with all extracted evidence

If `manifest.count` is 0, return `Not applicable`.

The analyzer provides facts relevant to this skill:

- `import_analysis` — unstructured logging calls, metrics package detection
- `controller` — event recorder usages
- `deployment_manifest` — Deployment/StatefulSet YAML: resource requests/limits, security context, health probes
- `networkpolicy_manifest` — NetworkPolicy YAML: presence and policy types
- `test_discovery` — discovered _test.go files and count

### Fact-to-checklist mapping

| Analyzer Facts | Checklist Items |
|----------------|-----------------|
| `test_discovery` | 1a-c (Test coverage) |
| `import_analysis.unstructured_logging` | 2a (Structured logging) |
| `controller.event_usages` | 2b (Event usage) |
| `import_analysis.has_metrics` | 2c (Metrics coverage) |
| `deployment_manifest.containers[].requests/limits` | 3a (Resource requests/limits) |
| `deployment_manifest.security_context` + `containers[].security_context` | 3b (Security context) |
| `deployment_manifest.containers[].has_liveness/has_readiness` | 3c (Health probes) |
| `networkpolicy_manifest` | 3d (Network policy) |

## gopls Verification Protocol

Use gopls MCP tools to verify the static analyzer's findings. These steps are **mandatory** in deterministic mode.

If the `gopls-lsp` MCP server is unavailable, use the built-in `LSP` tool with gopls as a fallback:

| MCP tool | LSP equivalent |
|----------|---------------|
| `go_file_context` | `documentSymbol` on the file, then `hover` on specific symbols |
| `go_symbol_references` | `findReferences` at the symbol's position |
| `go_search` | `workspaceSymbol` with the symbol name |
| `go_package_api` | `documentSymbol` on the package's Go files |

### Logging verification (checklist 2a)

If `import_analysis` facts show `unstructured_logging` calls:

1. Call `go_file_context` on the flagged file to confirm the call is in production code (not a CLI tool, test helper, or behind a build tag)

### Event verification (checklist 2b)

If controller facts show `event_usages`:

1. Use `go_file_context` on the controller file to understand the event call site context
2. Check whether events are inside conditional blocks (state transitions) or unconditional paths (every reconcile)

## Assessment Areas

Use area names exactly as written in the section headings below.

### 1. Test Coverage

- **1a. Reconcile logic has unit or table-driven tests**
  - title: "Reconcile logic lacks unit or table-driven tests"
  - finding: no unit or table-driven tests exist for reconcile happy path, error path, idempotency, finalizers, or status transitions (`Major`)
  - pass: reconcile logic has unit or table-driven tests covering happy path, error path, and at least one of idempotency, finalizers, or status transitions
  - not-observed: no reconcile logic in scope

- **1b. Integration test coverage is present**
  - title: "No integration test coverage for controller-runtime patterns"
  - finding: no integration tests (e.g., `envtest`) exist when the project uses controller-runtime patterns extensively (`Major`)
  - pass: integration test coverage is present, OR the project does not use controller-runtime patterns extensively enough to warrant it
  - not-observed: no controller-runtime usage in scope

- **1c. No sleep-based or timing-fragile tests**
  - title: "Sleep-based or timing-fragile tests"
  - finding: tests use fixed sleeps or timing-dependent assertions (`Minor`; escalate to `Major` when these tests are clearly relied upon for correctness validation)
  - pass: tests use polling, Eventually/Consistently, or other non-timing-dependent patterns
  - not-observed: no tests in scope

### 2. Observability

- **2a. Logs are structured**
  - title: "Unstructured logging"
  - finding: controller uses `fmt.Printf`, `log.Println`, or other unstructured logging instead of controller-runtime or `klog` structured APIs (`Major`)
  - pass: all logging uses structured APIs (controller-runtime logger, `klog`, or equivalent)
  - not-observed: no logging calls in scope

- **2b. Events are meaningful and not excessive**
  - title: "Events emitted on every reconcile"
  - finding: events are emitted on every reconcile iteration regardless of state changes (`Minor`)
  - pass: events are emitted only on meaningful state transitions, or no events are used
  - not-observed: no event recorder usage in scope

- **2c. Metrics cover reconcile or business-critical behavior**
  - title: "No reconcile or business-critical metrics"
  - finding: no custom metrics cover reconcile duration or business-critical controller behavior (`Minor`; escalate to `Major` only when the controller has a specific code path worth tracking — e.g., external service calls, rendering operations, or data pipeline stages — and no custom metric records its frequency, duration, or error rate)
  - pass: metrics cover reconcile duration or business-critical behavior, OR the controller has no specific code paths that would benefit from custom metrics beyond framework defaults (controller-runtime default metrics are sufficient for simple reconcile loops)
  - not-observed: no metrics registration in scope

### 3. Deployment Hardening

- **3a. Manager manifests define resource requests and limits**
  - title: "Manager manifest missing resource requests or limits"
  - finding: the manager deployment manifest omits resource requests or limits (`Major`)
  - pass: resource requests and limits are defined in the manager manifest
  - not-observed: no manager deployment manifests in scope

- **3b. Security context trends toward restricted defaults**
  - title: "Security context does not trend toward restricted defaults"
  - finding: the manager deployment does not set a restrictive security context (e.g., missing `runAsNonRoot`, `readOnlyRootFilesystem`, or `allowPrivilegeEscalation: false`) (`Major`)
  - pass: security context sets restricted defaults
  - not-observed: no manager deployment manifests in scope

- **3c. Health probes exist and match manager setup**
  - title: "Missing or misconfigured health probes"
  - finding: the manager deployment lacks liveness or readiness probes, or probes do not match the manager's health endpoint configuration (`Major`)
  - pass: health probes exist and match the manager setup
  - not-observed: no manager deployment manifests in scope

- **3d. Network policy is present in hardened environments**
  - title: "Missing network policy in hardened environment"
  - finding: no network policy exists and the repository clearly targets hardened or multi-tenant environments (`Minor`)
  - pass: network policy is defined, OR the repository does not target hardened/multi-tenant environments
  - not-observed: cannot determine target environment from scope
  - > **Deterministic mode**: the analyzer detects `networkpolicy_manifest` presence. Whether the repo targets "hardened" or "multi-tenant" environments requires human judgment. Default to `not-observed` if no NetworkPolicy manifests exist and the target environment cannot be determined from repository metadata alone.

### 4. OpenShift TLS Configuration Compliance

Apply this area only when repository evidence shows the controller targets OpenShift.

> **Deterministic mode**: no OpenShift-specific facts are extracted by the analyzer. If no OpenShift targeting evidence is found (no `openshift` in go.mod dependencies, no openshift-specific deployment files, no `oc` CLI references), default the entire Area 4 to `not-observed` without further analysis. Identifying OpenShift targeting requires exploratory-mode grep for `openshift` in imports, Makefile, and deployment manifests.

- **4a. No hardcoded TLS settings overriding platform-managed TLS**
  - title: "Hardcoded TLS settings override platform-managed TLS"
  - finding: TLS cipher suites, protocol versions, or certificate paths are hardcoded when local pinned references in scope show platform-managed TLS is required (`Critical`)
  - pass: TLS configuration defers to the platform or uses configurable settings that align with platform-managed TLS
  - not-observed: no TLS configuration in scope, or controller does not target OpenShift

## Anti-Findings

Do not emit findings for:

- **4b**: if the necessary OpenShift guidance is not available locally, mark area 4 `Not verified` — do not create a finding
- **4c**: do not use live platform guidance or time-sensitive web lookups in deterministic mode

## Deterministic Procedure

Run the assessment in this exact order:

1. Resolve scope from explicit scope text or `$ARGUMENTS`.
2. Run the `analyze_controller` MCP tool with `skill=production-readiness`. Load the full JSON output into context.
3. If `manifest.count` is 0, return `Not applicable`.
4. For test coverage checks (1a-c), read the test files listed in the `test_discovery` fact from the manifest entries to examine test patterns, table-driven tests, and timing usage.
5. Run the gopls verification protocol for checklist areas 2a and 2b as specified above.
6. Walk every checklist item (1a through 4a) in order. For each item, record a disposition: `finding`, `pass`, or `not-observed` using the criteria defined above, the analyzer facts, and gopls verification results. Do not skip items.
7. Draft findings only from items with `finding` disposition. Every finding must trace to a specific checklist item ID (e.g., "3b"). Observations outside the checklist may appear in a `Notes` section but do not receive an ID, severity, or score impact.
8. Apply anti-finding rules to dismiss any violations.
9. Run one deterministic leaf validation pass in the same session:
   - dismiss findings unsupported by cited evidence
   - dismiss any anti-finding violations
   - lower severity only when the written criteria support the lower level
   - remove or reword highlights that contradict findings
10. Sort findings using the shared schema rules. Assign final IDs after sorting.
11. Compute score and severity counts.

Use area names exactly as written in the section headings above.

In deterministic mode do not:

- default to `git diff`
- expand scope beyond what the evidence manifest discovered
- launch subagents
- use current platform guidance from outside the repository
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

- `scope` should use a URI-like string such as `repo://org/repo`, `dir://controllers`, or `controller://MyReconciler`
- `where` should use repo-relative GitHub-style location strings
- Sort findings by severity, area, `where`, and title before assigning final IDs

### Evidence Manifest

Paste the `manifest` section from the analyzer JSON output, including `count`, `hash`, and the categorized file entries. Do not edit, reformat, or summarize the output. This section makes the report auditable and verifiable.

### Summary

Write 2-3 sentences describing production readiness.

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
- **Checklist item**: the item ID (e.g., 3b)
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
