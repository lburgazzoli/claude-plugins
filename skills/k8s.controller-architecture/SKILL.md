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
Consult [validation-output-schema.md](../../references/validation-output-schema.md) for the canonical finding, evidence, highlight, and validation model used by validation-style skills.

## Assessment Areas

### 1. Reconciliation Idempotency and State Handling

- Each controller should ideally have a clear responsibility with understandable inputs and outputs. Treat this as a strong default rather than a universal rule; broader controllers can still be valid when their boundaries are intentional and operationally coherent
- Prefer a consistent reconcile structure such as: fetch resource, handle finalization, initialize conditions, reconcile, patch status. Patterns used by projects such as Knative and [cluster-api](https://github.com/kubernetes-sigs/cluster-api) are useful references, but review for internal consistency rather than strict adherence to one framework-specific shape
- Reconcile function produces the same result regardless of how many times it runs for the same input state
- State is reconstructed from observed state, not cached or assumed from previous reconciliation
- No side effects on no-op reconciliations (no unnecessary updates, patches, or event emissions)
- Labels and annotations must not be blindly propagated from parent resources to child templates (e.g., from a CR to a Deployment's `spec.template.metadata`), as any change to the parent's labels would trigger a rolling restart of the underlying pods. If label/annotation propagation is intentional, it should use an explicit opt-in allowlist of keys to propagate, not copy-all with an opt-out denylist
- Handles resource-not-found gracefully (deleted between queue and reconciliation)
- Uses `apierrors.IsNotFound()` to detect deleted resources and returns nil to stop reconciliation
- Properly handles optimistic concurrency via `resourceVersion` conflicts. Prefer returning the conflict error and letting controller-runtime requeue with backoff so the next reconcile starts from fresh state. Use `retry.RetryOnConflict` only when strictly needed for a narrowly bounded inline retry section. The absence of `RetryOnConflict` is **not** a finding when the controller returns errors normally — the requeue loop inherently resolves conflicts. Only flag conflict handling when there is evidence of **silent data loss**, **swallowed conflict errors**, or **writes on knowingly stale objects without returning the error**

### 2. Error Handling and Requeue Strategy

- Distinguishes between recoverable (transient) and non-recoverable (permanent) errors
- Transient errors (API server unavailable, conflict) return error to trigger exponential backoff requeue
- For user-fixable or long-lived spec issues (for example, invalid spec or missing referenced resources), prefer surfacing the problem in status conditions and avoid hot-looping on repeated errors. Returning an error can still be appropriate when it materially improves retry behavior or observability
- Uses `ctrl.Result{RequeueAfter: duration}` for time-based scheduling instead of polling loops
- Uses `ctrl.Result{}` with nil error for graceful stop (no unnecessary requeue)
- Do not use `Requeue: true` as a substitute for returning an error — return errors directly for failures (controller-runtime handles requeue with backoff automatically); reserve `Requeue: true` for in-progress operations that need default backoff without an error
- Conflict handling is contextual. Many controller-runtime reconcilers correctly handle optimistic concurrency by returning patch or update errors and relying on the next reconcile to re-fetch fresh state. **This is the default valid pattern** — do not flag the absence of `RetryOnConflict` as a finding when the controller propagates conflict errors normally
- Treat `retry.RetryOnConflict` (`k8s.io/client-go/util/retry`) as an exception pattern, not a default. Prefer returning the conflict error and letting the next reconcile start from fresh state
- Use `retry.RetryOnConflict` only when strictly needed for a narrowly bounded read-modify-write section where immediate local retry is materially better than reconcile-level retry
- Define "strictly needed" as: a narrow, side-effect-free read-modify-write section where immediate local retry materially reduces risk or churn compared with exiting reconcile and requeueing
- Example strict-need case: a small status-only merge that must preserve controller-owned fields and can safely retry after a fresh re-fetch, without re-running broader reconcile logic
- Non-example: wrapping broad reconcile flow (especially with external calls, child resource creation, or multi-step mutations) in `RetryOnConflict`; in those cases, return the conflict error and let the next reconcile start from fresh state
- When in doubt, do not use `RetryOnConflict`
- Whenever `retry.RetryOnConflict` is used, require all safeguards:
  - re-fetch inside the retry closure before mutation
  - merge only controller-owned fields (never wholesale replacement)
  - avoid side effects inside the retry loop
  - avoid decisions from stale assumptions across retries
  - preserve reconcile-entry generation semantics for `observedGeneration` (see Area 5)
- Use [cluster-api-operator](https://github.com/kubernetes-sigs/cluster-api-operator) as a reference: its status update helpers re-fetch inside the retry closure and merge only the fields the controller owns rather than replacing the entire status struct
- Never praise `RetryOnConflict` as a positive highlight — at best it is a neutral implementation choice that requires scrutiny
- Review whether the controller's chosen pattern is coherent with its write style:
  - controllers that accumulate changes in memory and write them back with a single patch near the end of reconcile often return the conflict error and let the next reconcile fetch fresh state instead of retrying inline (for example, the common Cluster API-style deferred patch pattern)
  - SSA-based writes may encounter field-manager conflicts instead of classic update conflicts
  - explicit `RetryOnConflict` loops should be rare and must satisfy the strict-use and safeguard rules above
- Classify this area as `contextual` (Minor) unless there is evidence of lost updates, stale-state decisions, or hot-looping
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
- Consider Server-Side Apply for status updates (`r.Status().Patch()` with `client.Apply`) when multiple actors may contribute to status and field ownership needs to stay explicit. In single-controller cases this can still be a good fit, but treat it as a design choice rather than a universal upgrade over ordinary status patch/update flows
- Resource is re-fetched before status update to avoid "object has been modified" conflicts
- `status.observedGeneration` should be set to `.metadata.generation` captured at the start of the reconcile to indicate which spec version the status reflects. Without this field, consumers cannot determine whether the status is current or stale. The same rule applies to condition-level `observedGeneration`
- For status updates, apply the same default: use `retry.RetryOnConflict` only when strict necessity is demonstrated; when in doubt, return the conflict and reconcile again from fresh state
- When using `retry.RetryOnConflict` for status updates, first verify strict necessity (prefer returning conflicts and reconciling with fresh state). If used, verify that the retry closure does not wholesale-replace `latest.Status` with an in-memory copy built before the retry. A full replacement silently overwrites status fields set by other actors between the original fetch and the retry. Prefer merging only the fields the controller owns onto the re-fetched object, or use Server-Side Apply status patches which handle field ownership natively. See [cluster-api-operator](https://github.com/kubernetes-sigs/cluster-api-operator) for a gold-standard implementation of this pattern
- When using `retry.RetryOnConflict`, `status.observedGeneration` (and condition-level `observedGeneration`) must be set to the generation captured at the start of the reconcile — not to the re-fetched object's `.metadata.generation` inside the retry closure. A re-fetch may return a newer generation if the spec was updated between retries; using that value falsely claims the controller has reconciled a spec it never processed. If the status lacks `observedGeneration` entirely and the controller uses `RetryOnConflict`, flag it as `should` (Major): without this field, consumers cannot determine whether the status reflects the current spec or a stale one, and the retry loop compounds the risk by potentially writing status against a spec version the controller never processed. Classify as `must` (Critical) when combined with wholesale status replacement
- Uses standard condition types following Kubernetes conventions:
  - `type`: PascalCase (e.g., `Ready`, `Available`, `Degraded`, `Progressing`)
  - `status`: `True`, `False`, or `Unknown`
  - `reason`: one-word CamelCase describing why the condition is set
  - `message`: human-readable details
  - `lastTransitionTime`: updated only when `status` changes
  - `observedGeneration`: set to `.metadata.generation` captured at reconcile entry to indicate which spec version the condition reflects — never from a re-fetched object inside a retry loop
- Conditions are set on first visit, even with `Unknown` status
- Uses `meta.SetStatusCondition()` or equivalent for proper condition management
- OpenShift operators: reports `Available`, `Progressing`, `Degraded` conditions on ClusterOperator

### 6. Ownership, Finalizers, and Cleanup Logic

- Uses `ctrl.SetControllerReference()` for ownership, letting garbage collector handle cleanup
- Declares `.Owns()` in controller setup for automatic watch on owned resources
- Set `ReaderFailOnMissingInformer: true` on the manager to prevent hidden on-the-fly informer creation from undeclared resource queries (see also Area 8 for cache alignment implications)
- Owner references are set on child resources for automatic garbage collection
- Does not set cross-namespace owner references (not supported)
- Finalizers are used when deletion must be gated on cleanup, ordered teardown, or asynchronous work completion. Cleanup of external resources is the most common case, but not the only legitimate one
- If child resources are fully Kubernetes-native and owner references are sufficient, a finalizer may be unnecessary. Treat this as a design choice to evaluate, not a blanket anti-pattern
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

### 9. Portability and Vendor API Dependencies

Treat this area as contextual by default; escalate when unguarded vendor API usage causes runtime failures on vanilla Kubernetes.

- Identify imports and usage of vendor-specific API groups (e.g., `openshift.io`, `route.openshift.io`, `security.openshift.io`, `config.openshift.io`, `rancher.cattle.io`, `run.tanzu.vmware.com`) and flag them as distribution-specific dependencies
- Check whether vendor-specific API usage is guarded by runtime discovery — for example, using the discovery client (`discovery.ServerResourcesForGroupVersion()`) or checking API group availability before registering watches or reconciling vendor resources. Unguarded usage causes the controller to crash or fail to start on clusters that do not have those APIs installed
- When vendor APIs are used, check whether the controller gracefully degrades (skips vendor-specific logic, logs a warning) when the API is unavailable, or whether it hard-fails
- Flag vendor-specific resource types used where a portable Kubernetes-native equivalent exists and the vendor type is not strictly required:
  - `Route` (OpenShift) vs `Ingress` or Gateway API
  - `DeploymentConfig` (OpenShift, deprecated) vs `Deployment`
  - `SecurityContextConstraints` (OpenShift) vs `PodSecurityStandards` / `PodSecurity` admission
- Check whether vendor-specific logic is isolated into separate packages or files, making it possible to build and run the controller without the vendor dependency. Mixing vendor-specific and portable logic in the same reconciler makes the controller harder to port
- Distinguish between build-time coupling (importing vendor types — the binary links against vendor libraries) and runtime coupling (discovering vendor APIs dynamically). Runtime discovery is preferred when the vendor integration is optional
- If the controller is explicitly designed for a single vendor platform (e.g., an OpenShift-only operator that manages `ClusterOperator` conditions), vendor API usage is expected — flag it as informational rather than a finding, but still verify that the vendor requirement is documented
- Severity guidance:
  - `must` (Critical): vendor API used without any availability guard, causing hard crash or startup failure on vanilla Kubernetes
  - `should` (Major): vendor API usage is not isolated or documented, making portability difficult without code changes
  - `contextual` (Minor): vendor API usage is intentional and documented but could benefit from better isolation or graceful degradation

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
6. **Adversarial validation**: After merging Category A and B results (see [Check Categories and Parallel Execution](#check-categories-and-parallel-execution)), launch a clean-context validator subagent with the merged draft findings, draft highlights, and scope. See [Leaf Validator Subagent](#leaf-validator-subagent) for the validator brief and isolation rules.
7. Apply validation results: update severities, remove dismissed findings, adjust or remove highlights per validator verdicts. Validation **always runs** and its results are applied before scoring regardless of flags. The detailed Validation output section is only included in the report when `--details` is passed.
8. Generate output with severity, concrete fix, confidence, and any unverified assumptions.

## Check Categories and Parallel Execution

Categorize checks into these groups before deep analysis:

- **Category A - Correctness, lifecycle, RBAC, security, and portability:** Areas 1, 2, 4, 5, 6, 9
- **Category B - Performance and cache behavior:** Areas 3, 7, 8

Execution model:

1. Launch one subagent per category and run the two category checks in parallel.
2. Give each subagent explicit scope, expected evidence format (file path plus line references), and severity expectations.
3. Require each subagent to return findings using this schema:
   - `id`, `title`, `area`, `severity`, `what`, `where`, `why`, `fix`, `confidence`, `notVerified`
   - `where` should use repo-relative GitHub-style location string(s), especially for each `Critical` or `Major` finding
   - Assign category-prefixed IDs (`A-1`, `A-2`, ... for Category A; `B-1`, `B-2`, ... for Category B). The parent agent reassigns sequential IDs after merge.
4. Require each subagent to return positive highlights.
5. Merge results in the parent agent, deduplicate overlapping findings, normalize severity using the mapping above, and produce one final report in the output format below.
6. If categories conflict on severity, prefer the higher severity and explain the rationale in the final report.
7. If parallel subagent execution is unavailable, run Category A and Category B sequentially using the same evidence and severity rules.

## Leaf Validator Subagent

After merging Category A and B results, launch a **separate subagent** to adversarially validate the merged findings and highlights. The validator operates with a clean context — it does not receive the primary reviewer's or category subagents' reasoning or intermediate notes.

### Isolation rules

- Run in a separate subagent
- Receive only: the scope (from `$ARGUMENTS` or resolved defaults), merged draft findings, and merged draft highlights
- Re-read code independently from the evidence locations in each finding's `where` field
- Do not rely on the category subagents' internal reasoning

### Validator brief

> **Role**: You are a skeptical reviewer. Your job is to challenge each finding from a controller architecture assessment and determine whether it actually impacts runtime behavior, correctness, or operational safety. You also verify that positive highlights do not contradict the findings.
>
> **Inputs you receive**:
> - The merged draft findings list (each following the canonical finding model from `validation-output-schema.md`)
> - The merged draft positive highlights list (each with: `id`, `sourceSkill`, `description`). For this leaf skill, `sourceSkill` is always `k8s.controller-architecture`
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

Use this weighting to keep assessments consistent:

- **Category A** — Correctness, data safety, RBAC, security, and portability (Areas 1, 2, 4, 5, 6, 9): **60%**
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
Use the canonical report, finding, highlight, and validation model from [validation-output-schema.md](../../references/validation-output-schema.md).

Output conventions:

- `scope` should follow the shared URI-like form when expressed structurally (for example, `diff://working-tree`, `repo://org/repo`, `controller://MyReconciler`)
- `where` should use repo-relative GitHub-style location string(s) (for example, `controllers/myresource_controller.go#L118-L146`)
- Use the shared `notVerified` concept consistently; render it in Markdown as `Not verified`

### Summary

2-3 sentences describing the overall quality and maturity of the controller implementation.

Score table:

| Metric | Value |
|--------|-------|
| **Score** | 0-100 (or `Not applicable`) |
| **Category A** (Correctness, RBAC, Security, Portability) | 0-100 |
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

Things the implementation does well, patterns worth preserving or replicating.
