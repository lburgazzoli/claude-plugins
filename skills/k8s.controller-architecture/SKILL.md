---
name: k8s.controller-architecture
description: Assess, review, or audit a Kubernetes controller architecture for upstream conventions, kubebuilder best practices, and correctness.
---

# Kubernetes Controller Architecture Assessment

Perform a comprehensive architecture assessment of a Kubernetes controller implementation.
This skill is primarily designed for Go controllers built with `controller-runtime`/kubebuilder patterns.

## Inputs

- If `$ARGUMENTS` is provided, treat it as scope (files, package, controller name, assessment focus, or GitHub repository).
- GitHub repository inputs may be full URLs (for example, `https://github.com/org/repo`) or shorthand (`org/repo`).
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source and do not apply local `git diff` defaults.
- If no arguments are provided, assess current repository changes from git diff.
- If there are no changes, start with controller packages and API types first; expand to the full codebase only when needed for evidence.
- If the project has no controller/operator implementation assets in scope (for example, no controller reconciler code and no relevant controller/runtime manifests), skip this skill and report `Not applicable`.
- `--details` includes a full breakdown of each finding (Why, Fix, metadata) after the summary tables. Without this flag, only the summary tables are produced.

## Input Validation

The only recognized flag is `--details`. If `$ARGUMENTS` contains any unrecognized `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.
When this skill is invoked by `/k8s.controller-assessment`, it should receive only scope text and optional `--details` (orchestration flags such as `--scope` are not valid here).

## References

Consult [k8s-upstream.md](../../references/k8s-upstream.md) for the authoritative source of conventions and high-quality reference implementations.

## Assessment Areas

### 1. Reconciliation Idempotency and State Handling

- Each controller should have a single responsibility with clear inputs/outputs — follow Unix philosophy where each controller does one thing well
- Adopt a consistent reconcile structure: fetch resource, handle finalization, initialize conditions, reconcile, patch status (Knative pattern — see [cluster-api](https://github.com/kubernetes-sigs/cluster-api) reconcilers for a well-structured example)
- Reconcile function produces the same result regardless of how many times it runs for the same input state
- State is reconstructed from observed state, not cached or assumed from previous reconciliation
- No side effects on no-op reconciliations (no unnecessary updates, patches, or event emissions)
- Labels and annotations must not be blindly propagated from parent resources to child templates (e.g., from a CR to a Deployment's `spec.template.metadata`), as any change to the parent's labels would trigger a rolling restart of the underlying pods. If label/annotation propagation is intentional, it should use an explicit opt-in allowlist of keys to propagate, not copy-all with an opt-out denylist
- Handles resource-not-found gracefully (deleted between queue and reconciliation)
- Uses `apierrors.IsNotFound()` to detect deleted resources and returns nil to stop reconciliation
- Properly handles optimistic concurrency via `resourceVersion` conflicts (re-fetch and retry)

### 2. Error Handling and Requeue Strategy

- Distinguishes between recoverable (transient) and non-recoverable (permanent) errors
- Transient errors (API server unavailable, conflict) return error to trigger exponential backoff requeue
- Permanent errors (invalid spec, missing referenced resource) update status conditions without returning error
- Uses `ctrl.Result{RequeueAfter: duration}` for time-based scheduling instead of polling loops
- Uses `ctrl.Result{}` with nil error for graceful stop (no unnecessary requeue)
- Do not use `Requeue: true` as a substitute for returning an error — return errors directly for failures (controller-runtime handles requeue with backoff automatically); reserve `Requeue: true` for in-progress operations that need default backoff without an error
- Prefer letting the reconcile loop handle conflicts naturally over using `retry.RetryOnConflict` (`k8s.io/client-go/util/retry`) — returning an error already re-enqueues the resource with exponential backoff, and the next reconciliation will re-fetch the latest object state. `RetryOnConflict` retries inline against a potentially stale read, risking decisions based on outdated cluster state. Classify as `contextual` (Minor) rather than a hard requirement
- Never silently swallows errors — either returns them, logs them, or records them in status
- Wraps errors with `fmt.Errorf("context: %w", err)` for debuggable error chains

### 3. Resource Management and API Efficiency

- Minimizes API server calls: uses cached client for reads, direct client only when strong consistency is needed
- Uses field indexers (`mgr.GetFieldIndexer().IndexField()`) for efficient filtered list operations
- Batches related operations where possible
- Avoids unnecessary status updates (compares before updating); avoids expensive operations (external API calls, status updates) on every reconciliation when nothing changed — use `status.observedGeneration` to detect if actual work is needed
- Uses Server-Side Apply (SSA) where appropriate with descriptive `fieldManager` names
- When using SSA: includes all managed fields in each Apply, omits unmanaged fields, handles conflicts gracefully
- Does not manually edit `.metadata.managedFields`

### 4. RBAC Least Privilege and Security

- RBAC markers (`//+kubebuilder:rbac`) grant minimum required permissions
- No wildcard verbs or resource grants (`*` on verbs or resources)
- Status subresource has separate RBAC from the main resource (`status` subresource permission)
- Event recording permissions are declared if events are emitted
- No cluster-scoped permissions when namespace-scoped suffices
- Secrets access is scoped to specific secrets if possible, not blanket access
- RBAC markers match actual API calls in the code (no stale or missing markers)
- **Meta-operators** (operators that manage other operators, e.g., cluster-api-operator, OLM): broader RBAC is expected because the operator must manage resources of its sub-operators. When a meta-operator is detected, verify that permission boundaries match the actual sub-operator manifests being managed, in addition to the operator's own resources — do not flag these broader permissions as least-privilege violations

### 5. Status, Conditions, and Observed Generation

- Status is updated via the status subresource (`r.Status().Update()` or `r.Status().Patch()`)
- Consider using Server-Side Apply for status updates (`r.Status().Patch()` with `client.Apply`) — this enables cooperative controllers to contribute to different status fields without conflicting, since each controller owns only its `fieldManager`-scoped fields. In the single-controller case the behavior is equivalent to a full status update, so there is no downside
- Resource is re-fetched before status update to avoid "object has been modified" conflicts
- Uses standard condition types following Kubernetes conventions:
  - `type`: PascalCase (e.g., `Ready`, `Available`, `Degraded`, `Progressing`)
  - `status`: `True`, `False`, or `Unknown`
  - `reason`: one-word CamelCase describing why the condition is set
  - `message`: human-readable details
  - `lastTransitionTime`: updated only when `status` changes
  - `observedGeneration`: set to `.metadata.generation` to indicate which spec version the condition reflects
- Conditions are set on first visit, even with `Unknown` status
- Uses `meta.SetStatusCondition()` or equivalent for proper condition management
- OpenShift operators: reports `Available`, `Progressing`, `Degraded` conditions on ClusterOperator

### 6. Ownership, Finalizers, and Cleanup Logic

- Uses `ctrl.SetControllerReference()` for ownership, letting garbage collector handle cleanup
- Declares `.Owns()` in controller setup for automatic watch on owned resources
- Set `ReaderFailOnMissingInformer: true` on the manager to prevent hidden on-the-fly informer creation from undeclared resource queries (see also Area 8 for cache alignment implications)
- Owner references are set on child resources for automatic garbage collection
- Does not set cross-namespace owner references (not supported)
- Finalizers are only used when strictly necessary — specifically for cleanup of external resources not managed by Kubernetes garbage collection (e.g., cloud infrastructure, external databases, DNS records). If all child resources are Kubernetes-native and have owner references, finalizers add unnecessary complexity and should not be used
- When a finalizer is needed, it is added early (before creating external resources) and removed only after cleanup succeeds
- Finalizer name follows convention: `<group>/<finalizer-name>` (e.g., `mygroup.example.com/cleanup`)
- Cleanup logic is idempotent (safe to run multiple times)
- Uses `controllerutil.AddFinalizer()` / `controllerutil.RemoveFinalizer()` helpers

### 7. Performance and Cache Usage

Treat this area as workload-sensitive guidance: classify findings here as `contextual` unless you have concrete evidence of correctness or operational risk impact.

- Uses informer cache (default client) for reads; avoids direct API calls unless required
- Watches are scoped to relevant namespaces or label selectors when possible
- Predicates filter irrelevant events (`builder.WithPredicates()` or legacy `WithEventFilter`, `GenerationChangedPredicate`)
- `GenerationChangedPredicate` skips reconciliation for metadata-only changes (e.g., label updates not relevant to the controller)
- Does not block reconciliation with long-running operations (offload to jobs or async)
- Rate limiting is configured appropriately for the controller's workload
- Monitor reconciliation latency and queue depth; understand that periodic resyncs enqueue all objects and can create backlogs that delay processing of new events
- Consider priority queue features (controller-runtime v0.20+) to deprioritize resync-triggered reconciliations vs edge-triggered ones
- Consider the "expectations pattern" only when reconciliation logic creates resources and immediately lists them to decide further actions — track pending operations in-memory and wait for cache catch-up to avoid acting on stale list results (e.g., creating duplicate replicas)
- Leader election is configured for HA deployments; tune lease duration, renew deadline, and retry period based on cluster network characteristics and failover tolerance

### 8. Cache Consistency and Client Type Alignment

Treat optimization-only findings in this area as `contextual`; escalate to `should` or `must` only when there is clear impact on correctness, scalability, or memory behavior.

Area 7 covers performance-oriented cache tuning (scoping, predicates, rate limiting). This area covers correctness of client-type alignment and informer configuration. If a finding is purely about efficiency, it belongs in Area 7.

- Pick one access pattern (typed, unstructured, or metadata) per GVK — use it consistently across watches, Get, and List calls. Mixing patterns creates duplicate informers (separate watch connections, doubled memory) even though both receive correct selector config.
- Match read client to watch type: if a resource is watched as `Unstructured`, read it via the cached client with an unstructured object, not a typed clientset. Vice versa for typed watches.
- Reading via a raw kubernetes clientset (e.g., `client.AppsV1().Deployments().List()`) bypasses the cache entirely — prefer the controller-runtime cached client.
- `cache.Options.ByObject` entries match by GVK, so they apply regardless of typed vs unstructured access — no special configuration needed.
- Resources accessed via `client.Get()`/`client.List()` should have a corresponding watch (`.Watches()`, `.Owns()`, `.For()`) or use a direct client explicitly. If a resource is only read but never watched, it needs a watch added or an explicit uncached client.
- Ensure `ReaderFailOnMissingInformer: true` is set (see Area 6) so undeclared cache access surfaces immediately.

**Prefer PartialObjectMetadata:**
- When the controller only needs metadata (labels, annotations, owner references, name/namespace), use `PartialObjectMetadata` instead of the full typed object to reduce memory consumption and API payload size
- Ensure watches and cache are configured for metadata-only access by using `.Watches()` with `PartialObjectMetadata` objects

**Avoid caching Secrets and ConfigMaps:**
- Do not cache Secrets and ConfigMaps unless strictly necessary — they are often large, numerous, and contain sensitive data that increases memory footprint and security exposure
- When access is needed, prefer direct (uncached) client reads or scope the cache to specific namespaces/labels
- If caching is unavoidable, restrict it via `cache.Options.ByObject` with label or field selectors

## Assessment Procedure

Use this repeatable workflow:

1. Determine scope from `$ARGUMENTS`, git diff, or targeted controller/API packages.
   - If `$ARGUMENTS` points to a GitHub repository, prioritize `api/`, `controllers/`, `config/rbac/`, `config/crd/`, and test directories as initial evidence sources.
2. If no controller/operator implementation assets are present in scope, stop and return `Not applicable`.
3. Collect evidence first (specific files and call sites), then classify issues by impact.
4. Mark each finding as `must`, `should`, or `contextual` based on production risk.
5. Map internal labels to report severities:
   - `must` -> `Critical`
   - `should` -> `Major`
   - `contextual` -> `Minor`
6. **Adversarial self-validation**: Before producing output, critically review each finding:
   - **Accuracy**: Does the code actually exhibit the described problem? Re-read the referenced location independently.
   - **Behavioral impact**: Would fixing this change runtime behavior, correctness, or operational safety — or is it purely stylistic/cosmetic/theoretical?
   - **Severity check**: A pattern that looks non-ideal but cannot cause incorrect reconciliation, data loss, or operational failure should be downgraded.
   - Downgrade rules:
     - Factually correct but **no behavioral impact** → downgrade one level (Critical→Major, Major→Minor, Minor→dismiss)
     - Described problem **cannot occur** given surrounding code → dismiss entirely
     - Severity appropriate with real impact → keep as-is
   - Remove dismissed findings and adjust severities before scoring.
   - Validation **always runs** and its results (severity adjustments and dismissals) are applied before scoring regardless of flags. The detailed Validation output section is only included in the report when `--details` is passed.
7. Generate output with severity, concrete fix, confidence, and any unverified assumptions.

## Check Categories and Parallel Execution

Categorize checks into these groups before deep analysis:

- **Category A - Correctness, lifecycle, RBAC, and security:** Areas 1, 2, 4, 5, 6
- **Category B - Performance and cache behavior:** Areas 3, 7, 8

Execution model:

1. Launch one subagent per category and run the two category checks in parallel.
2. Give each subagent explicit scope, expected evidence format (file path plus line references), and severity expectations.
3. Require each subagent to return findings using this schema:
   - `findingId`, `area`, `severity`, `what`, `where`, `why`, `fix`, `confidence`, `unknowns`
   - `where` must include concrete file path and line reference for each `Critical` or `Major` finding
   - Assign category-prefixed IDs (`A-1`, `A-2`, ... for Category A; `B-1`, `B-2`, ... for Category B). The parent agent reassigns sequential IDs after merge.
4. Require each subagent to return positive highlights.
5. Merge results in the parent agent, deduplicate overlapping findings, normalize severity using the mapping above, and produce one final report in the output format below.
6. If categories conflict on severity, prefer the higher severity and explain the rationale in the final report.
7. If parallel subagent execution is unavailable, run Category A and Category B sequentially using the same evidence and severity rules.

## Scoring

Use this weighting to keep assessments consistent:

- **Category A** — Correctness, data safety, RBAC, and security (Areas 1, 2, 4, 5, 6): **60%**
- **Category B** — Performance and scalability (Areas 3, 7, 8): **40%**

Scoring procedure:

1. Start each category at 100.
2. For each finding, subtract points based on severity:
   - Critical: **-20** per finding
   - Major: **-10** per finding
   - Minor: **-3** per finding
3. Floor each category score at 0.
4. Compute the overall score as the weighted sum: `Overall = A × 0.60 + B × 0.40`.
5. Report both the per-category scores and the overall score.

Interpretation of overall score:

- **90-100**: Production-ready with minor polish
- **75-89**: Solid baseline, a few important gaps
- **50-74**: Significant issues to address before production
- **<50**: High operational risk; major redesign/fixes recommended

When Category A and Category B scores diverge by more than 20 points, call out the divergence in the Summary.

## Output Format

Produce the assessment in this format. All sections are always included unless noted otherwise.

### Summary

2-3 sentences describing the overall quality and maturity of the controller implementation.

Score table:

| Metric | Value |
|--------|-------|
| **Score** | 0-100 (or `Not applicable`) |
| **Category A** (Correctness, RBAC, Security) | 0-100 |
| **Category B** (Performance, Scalability) | 0-100 |
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

Things the implementation does well, patterns worth preserving or replicating.
