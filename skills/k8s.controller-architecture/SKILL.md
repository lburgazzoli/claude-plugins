---
name: k8s.controller-architecture
description: Assess a Kubernetes controller architecture for upstream conventions, kubebuilder practices, and correctness.
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

# Kubernetes Controller Architecture Assessment

Assess the controller architecture of a Kubernetes operator or controller implementation.
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
- If no controller implementation assets or controller-runtime manifests are found in the resolved scope, return `Not applicable`.

## Input Validation

- The only recognized flags are `--mode=deterministic` and `--mode=exploratory`.
- If `$ARGUMENTS` contains any other `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.

## Static Analyzer

In deterministic mode, build and run the static analyzer to extract structured facts from Go code and YAML manifests. This is the single evidence source — it replaces manual file discovery, ast-grep queries, and YAML file reading.

Treat [analyzer-output-schema.md](../../references/analyzer-output-schema.md) as the normative schema for the analyzer JSON envelope and fact payloads.

### Run

Use the `analyze_controller` MCP tool with `repo_path` set to the repository root and `skill` set to `architecture`.

If the orchestrator (`k8s.controller-assessment`) has already run the analyzer and the JSON is loaded in context, skip the run step.

### Load and use

Load the full JSON output into context. The output includes:

- A `manifest` section with `count`, `hash` (MANIFEST_HASH), and categorized file `entries` — include `manifest.hash` verbatim in the report for auditability
- A `facts` array with all extracted evidence

If `manifest.count` is 0, return `Not applicable`.

This is the **primary evidence source** for all analysis. The output contains facts with these kinds:

- `controller` — reconciler structs, RBAC markers (with normalized permissions), API calls (with operation class, resolution metadata, and normalized required permissions), finalizer/owner-ref ops, status conditions (with ObservedGeneration), status update sites (with retry guard detection), retry ops, library invocation sites, event usages, not-found handlers, predicate usages, requeue ops, error returns, owns/watches, and ambiguity signals
- `crd_version` — CRD version storage/hub/spoke info
- `import_analysis` — vendor imports, library imports, unstructured logging, metrics
- `scheme_registration` — scheme registrations in main/cmd
- `rbac_manifest` — Role/ClusterRole YAML: rules, normalized permissions, split wildcard metadata, event permissions
- `crd_manifest` — CRD YAML: versions, conversion strategy, scope

Do not re-derive facts that the analyzer already provides. Do not use `sg` (ast-grep) queries for patterns the analyzer extracts. The analyzer uses `go/packages` for accurate AST parsing — it is more reliable than syntax-only pattern matching.

### Fact-to-checklist mapping

| Analyzer Facts | Checklist Items |
|----------------|-----------------|
| `controller.rbac_markers` + `controller.api_calls` + `controller.event_usages` + `controller.ambiguity_signals` + `rbac_manifest` | 3a-d (RBAC) |
| `controller.error_returns` + `controller.requeue_ops` | 2a-c (Error handling) |
| `controller.finalizer_ops` + `controller.owner_ref_ops` + `controller.external_write_ops` | 5a-c (Finalizers) |
| `controller.status_condition_sets` + `controller.status_update_sites` + `controller.retry_ops` | 4a-d (Status) |
| `controller.owns` + `controller.watches` + `controller.predicate_usages` | 8a-b (Watches) |
| `controller.api_calls` + `controller.not_found_handlers` + `controller.external_write_ops` | 1a-c (Idempotency) |
| `controller.library_invocations` | 7d-e (Rendering/caching) |
| `import_analysis.vendor_imports` | 6a-b (Vendor isolation) |
| `scheme_registration` | Informational for 9a-b |
| `controller.api_calls` (target matches `controller.reconciles`) | 10a (Spec writes) |
| `controller.max_concurrent_reconciles` | 11a (Concurrency safety) |

For RBAC checklist items, reason in this order:

1. Treat `rbac_manifest.permissions` as the primary evidence for effective committed permissions.
2. Use `controller.api_calls[].required_permissions` plus `controller.event_usages[].required_permissions` to determine what permissions the controller actually needs when they are concrete.
3. Use `controller.ambiguity_signals` to detect unresolved receiver, unstructured, or rendered-object cases before scoring unused-RBAC findings.
4. Use `controller.rbac_markers[].permissions` as secondary evidence for drift checks between source markers and generated/committed RBAC.
5. Fall back to raw `rbac_manifest.rules`, `controller.rbac_markers`, or raw `obj_type` strings only when the normalized signals are empty and the gap is itself the thing being verified.

## Assessment Areas

Use area names exactly as written in the section headings below.

### Category A: Correctness, Lifecycle, and Security

#### 1. Reconciliation Idempotency and State Handling

- **1a. Reconcile logic is idempotent**
  - title: "Non-idempotent reconcile path"
  - finding: a reconcile path creates, updates, or deletes resources unconditionally without checking current state (`Major`)
  - pass: reconcile paths check current state before writes (create-or-update, SSA, or equivalent)
  - not-observed: no resource writes in scope

- **1b. No-op reconciles avoid unnecessary writes**
  - title: "No-op reconciles trigger unnecessary writes"
  - finding: every reconcile triggers writes even when the resource is already at desired state (`Minor`)
  - pass: reconcile short-circuits or skips writes when nothing changed
  - not-observed: cannot determine from code whether writes occur on no-ops

- **1c. Not-found objects are handled gracefully**
  - title: "Not-found error not handled gracefully"
  - finding: a not-found error causes a requeue or error log instead of a clean return (`Major`)
  - pass: `IsNotFound` or equivalent check returns `ctrl.Result{}, nil`
  - not-observed: no Get/read calls in reconcile scope

- **1d. RetryOnConflict absence**
  - Anti-finding: do not create a finding for the mere absence of `RetryOnConflict`. Do not list `RetryOnConflict` as a positive highlight.

#### 2. Error Handling and Requeue Strategy

- **2a. Transient errors return for requeue with backoff**
  - title: "Transient errors swallowed, preventing requeue"
  - finding: transient errors are swallowed (logged but not returned) preventing requeue (`Major`)
  - pass: transient errors are returned so the controller requeues with backoff
  - not-observed: no error paths in scope

- **2b. Time-based scheduling uses RequeueAfter**
  - title: "Polling without RequeueAfter duration"
  - finding: polling uses `Requeue: true` without a duration, causing tight-loop reconciliation (`Minor`)
  - pass: time-based scheduling uses `RequeueAfter` with an explicit duration
  - not-observed: no time-based scheduling in scope

- **2c. Permanent errors are surfaced coherently**
  - title: "Permanent errors cause infinite requeue"
  - finding: permanent errors are returned for infinite requeue without a terminal condition or status update (`Major`)
  - pass: permanent errors update status conditions or use an explicit terminal path
  - not-observed: no permanent error paths in scope

#### 3. RBAC Least Privilege and Security

- **3a. RBAC markers match actual API calls**
  - title: "RBAC permissions do not match actual API usage"
  - finding: RBAC grants permissions for resources the controller never accesses, OR the controller accesses resources without corresponding RBAC (`Major`)
  - pass: RBAC markers align with actual client calls
  - not-observed: no RBAC markers or no client calls in scope
  - deterministic rule: treat the two branches differently. Missing required RBAC may be reported directly from concrete `controller.api_calls[].required_permissions` evidence. Extra or apparently unused RBAC should only be reported when the controller's concrete resource usage is observable from `controller.api_calls[].required_permissions`, `controller.event_usages[].required_permissions`, and manifests. If `controller.ambiguity_signals`, reconcile-loop rendering, or generic `unstructured.Unstructured` apply paths make the concrete rendered GVK set ambiguous, do not emit a scored unused-RBAC finding from the absence of matching `required_permissions` alone; use `not-observed`, or a `Low` confidence finding only when the ambiguity itself is the main point.
  - matching rule: compare normalized permission tuples by `group`, `resource`, `subresource`, and verb coverage. A manifest permission satisfies a required permission when the tuple matches exactly or the manifest verb set is a strict superset for the same resource tuple.

- **3b. No wildcard RBAC**
  - title: "Wildcard RBAC verbs or resources"
  - finding: RBAC uses `*` for verbs or resources (`Major`)
  - pass: all RBAC entries use explicit verbs and resource names
  - not-observed: no RBAC markers in scope
  - evidence: use `rbac_manifest.has_wildcard_group`, `has_wildcard_resource`, and `has_wildcard_verb` first; use source markers only for drift context when no manifest exists

- **3c. Event permissions match event usage**
  - title: "Event RBAC does not match event usage"
  - finding: controller emits events but lacks RBAC for events, OR has event RBAC but never emits events (`Minor`)
  - pass: event RBAC matches actual event recorder usage
  - not-observed: no events or event RBAC in scope
  - evidence: compare `controller.event_usages[].required_permissions` against `rbac_manifest.permissions`; use `rbac_manifest.has_events` only as a coarse fallback when normalized permission tuples are absent

- **3d. Cluster-scoped permissions are justified**
  - title: "Unjustified cluster-scoped RBAC"
  - finding: cluster-scoped RBAC (ClusterRole) grants broad access without clear need from the controller logic (`Major`)
  - pass: cluster-scoped permissions correspond to cluster-scoped resources the controller manages
  - not-observed: no cluster-scoped RBAC in scope
  - evidence: start from `rbac_manifest.kind == ClusterRole` plus `rbac_manifest.permissions`; justify them with concrete `required_permissions`, known cluster-scoped resources, or scope evidence from CRD/YAML manifests

#### 4. Status, Conditions, and Observed Generation

- **4a. Conditions are initialized and updated coherently**
  - title: "Inconsistent condition updates across reconcile paths"
  - finding: conditions are set in some paths but not others, leaving stale or uninitialized conditions (`Major`)
  - pass: all reconcile exit paths set or preserve conditions consistently
  - not-observed: no status conditions in scope
  - evaluation rule: collect the set of distinct condition types from `controller.status_condition_sets` (e.g., `{Available, Progressing}`). For each reconcile exit path that updates status or returns an error, check whether ALL condition types in the set are set. If any exit path sets only a subset of the condition types, that is a finding. A condition type that appears on some paths but not others can retain a stale value from a previous reconcile — this is the definition of "inconsistent" regardless of whether the omitted type is set to True or False when present.

- **4b. observedGeneration is set on conditions**
  - title: "ObservedGeneration not set on status conditions"
  - finding: status subresource is enabled and `metav1.Condition` structs omit `ObservedGeneration` (`Major`)
  - pass: `ObservedGeneration` is set on all condition writes, OR status subresource is not used
  - not-observed: no status condition writes in scope
  - evidence: use `controller.status_condition_sets[].has_observed_generation` from the analyzer. When the status type uses `[]metav1.Condition`, the `ObservedGeneration` field is available and should be set. Use gopls (`go_file_context`) on the status type (identified by `crd_type.status_field_type`) to confirm the condition type.

- **4c. Status writes do not overwrite unrelated data**
  - title: "Status writes overwrite unrelated controller data"
  - finding: status update overwrites fields managed by another controller or the user (`Major`)
  - pass: status updates target only controller-owned fields (conditions, a specific status struct)
  - not-observed: no status writes in scope

- **4d. Status updates use consistent conflict handling**
  - title: "Inconsistent conflict handling across status update paths"
  - finding: some status update sites use `RetryOnConflict` while others write directly without retry, creating inconsistent conflict resilience (`Minor`)
  - pass: all status update sites use the same conflict handling strategy, OR the unguarded sites are in error paths where the error itself triggers a requeue
  - not-observed: fewer than two status update sites in scope
  - evidence: use `controller.status_update_sites[].is_guarded` and `controller.status_update_sites[].guard_kind` from the analyzer. Compare guarded and unguarded sites. Sites in the same method with mixed strategies indicate inconsistency. Sites in error paths that return the error (triggering requeue) may be acceptable without retry.

#### 5. Ownership, Finalizers, and Cleanup Logic

- **5a. Owner references are set where appropriate**
  - title: "Child resources missing owner references"
  - finding: controller creates child resources without setting owner references, risking orphaned resources (`Major`)
  - pass: child resources have owner references, OR the controller uses an alternative cleanup mechanism (finalizers, explicit GC)
  - not-observed: no child resource creation in scope

- **5b. Finalizers gate cleanup when needed**
  - title: "External cleanup not gated by finalizer"
  - finding: controller performs external cleanup on deletion but does not use a finalizer to gate it (`Major`)
  - pass: finalizer is added during reconcile and removed only after cleanup succeeds
  - not-observed: no deletion handling in scope

- **5c. Finalizer is not removed before cleanup succeeds**
  - title: "Finalizer removed before cleanup completes"
  - finding: finalizer is removed before external cleanup completes or before error checking (`Critical`)
  - pass: finalizer removal is the last operation after all cleanup succeeds
  - not-observed: no finalizer logic in scope

#### 6. Portability and Vendor API Dependencies

- **6a. Vendor APIs are isolated**
  - title: "Unguarded vendor API dependency"
  - finding: vendor-specific API calls are made unconditionally without capability detection or build guards (`Major`)
  - pass: vendor APIs are behind runtime discovery, build tags, or feature gates
  - not-observed: no vendor API usage in scope

- **6b. Vendor-only startup dependencies**
  - title: "Controller fails to start without vendor CRDs"
  - finding: controller fails to start on non-vendor platforms due to unguarded CRD/API dependency (`Critical`)
  - pass: startup handles missing vendor CRDs gracefully (skip watch, log warning)
  - not-observed: no vendor-specific startup dependencies in scope

### Category B: Resource Management and Cache Behavior

#### 7. Resource Management and API Efficiency

- **7a. Cached reads are preferred**
  - title: "Direct API reads for watched resources"
  - finding: controller uses direct API reads (non-cached client) for resources that are also watched (`Minor`)
  - pass: reads use the cached client by default
  - not-observed: cannot determine client type from code
  - > **Deterministic mode**: defaults to `not-observed`. The analyzer does not track which client type (cached vs. direct) is used per API call. In exploratory mode, use `go_file_context` on the reconciler struct to inspect client field types.

- **7b. Status updates are avoided when unchanged**
  - title: "Unconditional status update on every reconcile"
  - finding: status is updated on every reconcile even when no fields changed (`Minor`)
  - pass: status update is skipped or guarded by a comparison when nothing changed
  - not-observed: no status updates in scope
  - > **Deterministic mode**: defaults to `not-observed`. The analyzer tracks status update sites but does not detect whether updates are guarded by equality comparison. In exploratory mode, use `go_file_context` on status update call sites to inspect guard logic.

- **7c. SSA usage is coherent**
  - title: "Incoherent SSA field manager usage"
  - finding: SSA is used with conflicting field managers or mixed with non-SSA updates on the same resource (`Major`)
  - pass: SSA field owner is consistent and not mixed with Update/Patch on the same fields
  - not-observed: no SSA usage in scope
  - > **Deterministic mode**: defaults to `not-observed`. The analyzer does not extract SSA `Apply()` calls or field manager strings. In exploratory mode, use `go_search` for `client.Apply` and `FieldOwner` patterns.

- **7d. Helm/Kustomize rendering in reconcile loop is clearly gated**
  - title: "Helm/Kustomize rendering runs in reconcile loop without clear gating"
  - finding: a Helm/Kustomize library call is reachable from `Reconcile` and there is no clear state or input gate preventing fresh render work on repeated reconciles (`Minor`; escalate to `Major` only when the same chart or base is loaded from disk AND rendered on every reconcile iteration with no generation, content-hash, or resource-version guard on the render path)
  - pass: render calls are outside the reconcile loop, or reconcile-path rendering is clearly gated by generation, content hash, resource version, or equivalent input-change checks
  - not-observed: no `controller.library_invocations` entries with `invoked_in_reconcile_loop=true`
  - evidence: use `controller.library_invocations` to identify render-related call sites in `Reconcile` or repo-local helpers/functions/methods reached from `Reconcile`

- **7e. Rendered artifacts are reused or cached coherently**
  - title: "Rendered artifacts are recomputed without reuse"
  - finding: the reconcile loop performs fresh Helm/Kustomize render work for stable inputs and there is no clear reuse, memoization, or cached rendered-output path (`Minor`)
  - pass: rendered artifacts are reused, memoized, or clearly bounded to input changes rather than recomputed on repeated reconciles
  - not-observed: no `controller.library_invocations` entries with `invoked_in_reconcile_loop=true`
  - evidence: start from `controller.library_invocations` and verify whether render results are persisted or reused across reconcile iterations

#### 8. Performance and Cache Usage

- **8a. Watches and predicates avoid churn**
  - title: "Watches without predicates cause reconcile churn"
  - finding: watches lack predicates, causing reconciliation on every update to watched resources (`Minor`)
  - pass: watches use predicates or event filters to reduce unnecessary reconciles
  - not-observed: no watches configured in scope

- **8b. Hidden informer creation**
  - title: "Informers created outside manager cache"
  - finding: code creates additional informers or list/watches outside the manager cache, duplicating API server load (`Minor` unless it clearly affects correctness or memory materially, then `Major`)
  - pass: all watches go through the manager cache
  - not-observed: no evidence of informer creation outside the manager

#### 9. Cache Consistency and Client Type Alignment

- **9a. Access pattern per GVK is coherent**
  - title: "Mixed cached and uncached reads for same GVK"
  - finding: same GVK is read via both cached and uncached clients without clear justification (`Major`)
  - pass: each GVK uses a consistent client type
  - not-observed: cannot determine client type per GVK
  - > **Deterministic mode**: defaults to `not-observed`. The analyzer records `api_calls` but does not distinguish cached vs. uncached client receivers. In exploratory mode, use `go_file_context` on the reconciler struct to resolve client field types per call.

- **9b. Cached reads match watched resources**
  - title: "Cached reads for unwatched resources"
  - finding: controller reads from cache for a resource it does not watch, getting stale data (`Major`)
  - pass: all cached reads are for watched resources
  - not-observed: cannot determine watch-to-read mapping
  - > **Deterministic mode**: defaults to `not-observed`. The analyzer provides `owns`, `watches`, and `api_calls` but cannot confirm client type per call. In exploratory mode, cross-reference resolved client types with watch/owns lists.

- **9c. Mixed typed and unstructured informers**
  - title: "Duplicate informers from mixed typed and unstructured watches"
  - finding: same resource is watched via both typed and unstructured informers, causing double cache entries (`Major` only when it creates concrete duplication or correctness risk, otherwise `Minor`)
  - pass: each resource uses one informer type
  - not-observed: no mixed informer patterns in scope
  - > **Deterministic mode**: defaults to `not-observed`. The analyzer does not track `unstructured.Unstructured` usage in watch/informer setup. In exploratory mode, use `go_search` for `unstructured.Unstructured` in watch setup code.

#### 10. Spec-Status Contract Boundary

- **10a. Controller does not write to spec**
  - title: "Controller writes to primary resource spec"
  - finding: controller issues Update or Patch calls targeting the primary resource's spec (not status subresource), violating the spec-status contract where spec is user-owned (`Major`)
  - pass: controller only writes to status subresource and metadata (labels, annotations, finalizers) on the primary resource
  - not-observed: no Update/Patch calls on the primary resource in scope
  - evidence: use `controller.api_calls` to find calls where the target GVK matches `controller.reconciles` and `operation_class` is Update or Patch (not StatusUpdate or StatusPatch)

#### 11. Concurrency Safety

- **11a. Reconciler struct is safe for concurrent use**
  - title: "Reconciler struct has unsynchronized mutable state with concurrent reconciles"
  - finding: `MaxConcurrentReconciles` is set > 1 and the reconciler struct contains mutable fields (maps, slices, counters) without visible synchronization primitives (sync.Mutex, sync.RWMutex, sync.Map, atomic) (`Major`)
  - pass: `MaxConcurrentReconciles` is 1 (default), OR mutable shared state is protected with synchronization, OR the reconciler struct contains only immutable fields (client, logger, scheme, recorder)
  - not-observed: no `MaxConcurrentReconciles` configuration in scope
  - evidence: use `controller.max_concurrent_reconciles` from the analyzer. When > 1, use gopls to inspect the reconciler struct definition for mutable fields.

## Anti-Findings

Do not emit findings for:

- absence of `RetryOnConflict` when conflict errors are returned normally
- `RetryOnConflict` as a positive highlight
- purely stylistic reconcile shape differences without an operational effect
- controllers that only write to metadata (annotations, labels, finalizers) on the primary resource — this is not a spec write

## gopls Verification Protocol

The `gopls-lsp` plugin provides MCP tools for Go semantic analysis. Use these to verify and enrich the static analyzer's structural findings. These verification steps are **mandatory** in deterministic mode — not optional fallbacks.

If the `gopls-lsp` MCP server is unavailable, use the built-in `LSP` tool with gopls as a fallback:

| MCP tool | LSP equivalent |
|----------|---------------|
| `go_file_context` | `documentSymbol` on the file, then `hover` on specific symbols |
| `go_symbol_references` | `findReferences` at the symbol's position |
| `go_search` | `workspaceSymbol` with the symbol name |
| `go_package_api` | `documentSymbol` on the package's Go files |

### RBAC verification (checklist 3a-d)

For each controller fact, verify RBAC-to-API-call alignment:

1. Build the concrete required-permission set from `controller.api_calls[].required_permissions` and `controller.event_usages[].required_permissions`.
   - Treat these normalized tuples as the default matching input for checklist 3a and 3c.
   - Do not re-derive a different permission tuple from raw call text when the normalized tuple is already present.

2. For each `api_calls` entry where `required_permissions` is empty but `obj_type` suggests a variable-backed object:
   - Call `go_file_context` on the controller file to resolve the variable's actual Go type
   - Use that result to refine the concrete permission tuple before scoring RBAC coverage

3. For each `rbac_manifest.permissions` tuple that has no matching required-permission tuple:
   - Check `controller.ambiguity_signals` first. If unresolved receiver, unstructured, or rendered-object signals are present on the reconcile path, prefer `not-observed` for the unused-RBAC side unless the manifest grant is independently unjustified.
   - Only continue to unused-RBAC verification when the required-permission set is concrete enough to enumerate controller usage.

4. For each `rbac_markers` permission tuple that has no matching required-permission tuple or committed manifest permission:
   - Call `go_symbol_references` for the resource type name in the controller file
   - If no references found, do not immediately emit a 3a finding. First check whether the controller has `library_invocations` with `invoked_in_reconcile_loop=true` or generic apply helpers that may apply rendered `unstructured.Unstructured` objects.
   - Use `controller.ambiguity_signals` plus `go_file_context` / `go_search` on the reconcile path to look for generic apply helpers, `unstructured.Unstructured`, or rendered object slices/maps flowing into apply logic.
   - If rendered/generic apply paths are present and the concrete GVK set cannot be enumerated from evidence, treat the extra-RBAC side of 3a as ambiguous: prefer `not-observed`, or at most `Low` confidence with an explicit `notVerified` note that rendered resources could consume the permission.
   - Only emit a scored unused-RBAC finding when no controller references, no committed-manifest match, and no rendered/generic apply ambiguity are visible.

5. For any `api_calls` entry whose `receiver_resolution` is ambiguous (not clearly `r.Client` or similar):
   - Call `go_search` for the reconciler struct name to verify it embeds `client.Client`

6. When `rbac_markers` and `rbac_manifest` disagree:
   - Treat the manifest as the effective permission set for checklist 3a-d
   - Treat the marker/manifest mismatch as generator drift evidence, not as a reason to ignore the manifest

7. For wildcard and event checks:
   - Use `rbac_manifest.has_wildcard_*` for 3b
   - Use `controller.event_usages[].required_permissions` matched against `rbac_manifest.permissions` for 3c
   - Use `rbac_manifest.kind`, `rbac_manifest.permissions`, and scope evidence from API/manifests for 3d

### Error propagation verification (checklist 2a-c)

For each controller fact:

1. If `error_returns` exist but reconcile paths appear to call helper functions:
   - Call `go_symbol_references` for helper functions called from Reconcile
   - Trace whether their error returns propagate up to the controller runtime

### Status condition verification (checklist 4a-b, 4d)

For each controller fact with `status_condition_sets`:

1. If any condition has `has_observed_generation: false`:
   - Use the `crd_type.status_field_type` from the analyzer to identify the status struct name
   - Call `go_file_context` on the API types file to inspect the status struct and verify whether it uses `[]metav1.Condition` (which has ObservedGeneration) or a custom type
   - If the struct uses `metav1.Condition`, the omission is a 4b finding

2. For checklist 4d, use `controller.status_update_sites` directly:
   - Compare `is_guarded` and `guard_kind` across all sites
   - Sites with `is_guarded: true` use retry wrappers; sites with `is_guarded: false` do not
   - If both guarded and unguarded sites exist, check whether unguarded sites are in error paths that return the error (triggering controller-runtime requeue)
   - Use `controller.retry_ops` to understand what retry strategy is in use (function, backoff kind, wrapped calls)

### Vendor isolation verification (checklist 6a-b)

For each `import_analysis` fact with `vendor_imports`:

1. Call `go_package_api` on the vendor import path to understand what types are used
2. Check whether usage is behind build tags or runtime discovery by reading the import file context

### Library rendering verification (checklist 7d-e)

For each `controller` fact with one or more `library_invocations` entries where `invoked_in_reconcile_loop=true`:

1. Use `go_file_context` on the controller file and the referenced method to confirm the library call sits in `Reconcile` or a repo-local helper/function/method reached from `Reconcile`
2. Use `go_symbol_references` on the containing helper method (when not `Reconcile`) to verify it is actually called from reconcile-path methods
3. Use `go_search` on the reconciler struct to inspect whether it stores cached rendered output, memoized inputs, or renderer state used to avoid fresh render work on repeated reconciles
4. If no cache or reuse path is visible and the same render helper is reachable on repeated reconcile paths, assess it as fresh read+render per reconcile

### Cache and client type verification (checklist 9a-b)

1. Call `go_search` for the reconciler struct to see if it has both cached and uncached client fields
2. For ambiguous `api_calls`, use `go_file_context` to determine which client field the receiver references

### Spec-status contract verification (checklist 10a)

For each controller fact:

1. Check `controller.api_calls` for Update or Patch calls where the target GVK matches `controller.reconciles`
2. If found, use `go_file_context` on the call site to distinguish:
   - Status subresource updates (`.Status().Update()`, `.Status().Patch()`) — these are fine
   - Metadata-only updates (annotations, labels, finalizers) — these are fine
   - Spec field updates — these are a finding

### Concurrency safety verification (checklist 11a)

For each controller fact where `max_concurrent_reconciles > 1`:

1. Call `go_file_context` on the reconciler struct definition file
2. Inspect struct fields for mutable types: `map[...]...`, `[]...` (not from embedded interfaces), counters (`int`, `int64` without atomic)
3. Check for synchronization primitives: `sync.Mutex`, `sync.RWMutex`, `sync.Map`, `atomic.*`
4. Immutable fields are safe: `client.Client`, `logr.Logger`, `*runtime.Scheme`, `record.EventRecorder`

## Deterministic Procedure

Run the assessment in this exact order:

1. Resolve scope from explicit scope text or `$ARGUMENTS`.
2. Run the `analyze_controller` MCP tool with `skill=architecture`. Load the full JSON output into context.
3. If `manifest.count` is 0, return `Not applicable`.
4. Run the gopls verification protocol for checklist areas 2, 3, 4, 6, 7, 9, 10, and 11 as specified above.
5. Walk every checklist item (1a through 11a, including 4d, 7d, and 7e) in order. For each item, record one disposition: `finding`, `pass`, or `not-observed` using the checklist criteria, the analyzer facts, and gopls verification results. Do not skip items.
6. Draft findings only from items with `finding` disposition. Every finding must trace to a specific checklist item ID (e.g., "4b"). Observations outside the checklist may appear in a `Notes` section but do not receive an ID, severity, or score impact.
7. Apply anti-finding rules to dismiss any violations.
8. Run one deterministic leaf validation pass in the same session:
   - dismiss findings unsupported by cited evidence
   - dismiss any anti-finding violations
   - lower severity only when the written criteria support the lower level
   - remove or reword highlights that contradict findings
9. Sort findings using the shared schema rules. Assign final IDs after sorting.
10. Compute category scores and overall score.

In deterministic mode do not:

- default to `git diff`
- launch category subagents
- run validator fan-out
- expand scope beyond what the evidence manifest discovered
- use ad-hoc `sg` (ast-grep) queries for facts the analyzer already provides
- skip gopls verification steps defined in the protocol

`--mode=exploratory` may widen scope after the manifest and may run additional searches, but the report format and scoring stay the same.

## Severity Mapping

Severity for each checklist item is defined inline in the assessment areas above. When evidence fits two adjacent levels and the criteria do not force the higher level, choose the lower level.

## Scoring

1. Start Category A at 100.
2. Start Category B at 100.
3. Subtract from each category:
   - `Critical`: 20
   - `Major`: 10
   - `Minor`: 3
4. Floor each category score at 0.
5. Compute `Architecture = Category A x 0.60 + Category B x 0.40`.
6. Findings with `confidence: Low` do not contribute to score deductions.
7. Show arithmetic in the report.

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

Write 2-3 sentences describing controller architecture quality.

Score table:

| Metric | Value |
|--------|-------|
| **Score** | 0-100 or `Not applicable` |
| **Category A** | 0-100 or `Not applicable` |
| **Category B** | 0-100 or `Not applicable` |
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
- **Checklist item**: the item ID (e.g., 4b)
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
