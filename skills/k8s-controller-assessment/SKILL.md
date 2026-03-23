---
name: k8s-controller-assessment
description: Assess a Kubernetes controller implementation against upstream conventions, kubebuilder best practices, and production readiness criteria.
---

# Kubernetes Controller Assessment

Perform a comprehensive assessment of a Kubernetes controller implementation.

## Inputs

- If `$ARGUMENTS` is provided, treat it as scope (files, package, controller name, or assessment focus).
- If no arguments are provided, assess current repository changes from git diff. If there are no changes, assess the full codebase.

## References

Use these upstream references as the authoritative source for conventions:

- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [OpenShift Conventions](https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [controller-tools](https://github.com/kubernetes-sigs/controller-tools)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Kubernetes Logging Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md)
- [Structured Logging Migration](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/migration-to-structured-logging.md)
- [client-go](https://github.com/kubernetes-sigs/controller-runtime)
- [Server-Side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/)

## Assessment Areas

### 1. Reconciliation Idempotency and State Handling

- Reconcile function produces the same result regardless of how many times it runs for the same input state
- State is reconstructed from observed state, not cached or assumed from previous reconciliation
- No side effects on no-op reconciliations (no unnecessary updates, patches, or event emissions)
- Handles resource-not-found gracefully (deleted between queue and reconciliation)
- Uses `apierrors.IsNotFound()` to detect deleted resources and returns nil to stop reconciliation
- Properly handles optimistic concurrency via `resourceVersion` conflicts (re-fetch and retry)

### 2. Error Handling and Requeue Strategy

- Distinguishes between recoverable (transient) and non-recoverable (permanent) errors
- Transient errors (API server unavailable, conflict) return error to trigger exponential backoff requeue
- Permanent errors (invalid spec, missing referenced resource) update status conditions without returning error
- Uses `ctrl.Result{RequeueAfter: duration}` for time-based scheduling instead of polling loops
- Uses `ctrl.Result{}` with nil error for graceful stop (no unnecessary requeue)
- Never silently swallows errors — either returns them, logs them, or records them in status
- Wraps errors with `fmt.Errorf("context: %w", err)` for debuggable error chains

### 3. Resource Management and API Efficiency

- Minimizes API server calls: uses cached client for reads, direct client only when strong consistency is needed
- Uses field indexers (`mgr.GetFieldIndexer().IndexField()`) for efficient filtered list operations
- Batches related operations where possible
- Avoids unnecessary status updates (compares before updating)
- Uses Server-Side Apply (SSA) where appropriate with descriptive `fieldManager` names
- When using SSA: includes all managed fields in each Apply, omits unmanaged fields, handles conflicts gracefully
- Does not manually edit `.metadata.managedFields`
- Uses `ctrl.SetControllerReference()` for ownership, letting garbage collector handle cleanup
- Declares `.Owns()` in controller setup for automatic watch on owned resources

### 4. RBAC Least Privilege and Security

- RBAC markers (`//+kubebuilder:rbac`) grant minimum required permissions
- No wildcard verbs or resource grants (`*` on verbs or resources)
- Status subresource has separate RBAC from the main resource (`status` subresource permission)
- Event recording permissions are declared if events are emitted
- No cluster-scoped permissions when namespace-scoped suffices
- Secrets access is scoped to specific secrets if possible, not blanket access
- RBAC markers match actual API calls in the code (no stale or missing markers)

### 5. Status, Conditions, and Observed Generation

- Status is updated via the status subresource (`r.Status().Update()` or `r.Status().Patch()`)
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

### 6. Finalizers, Cleanup Logic, and Owner References

- Finalizers are only used when strictly necessary — specifically for cleanup of external resources not managed by Kubernetes garbage collection (e.g., cloud infrastructure, external databases, DNS records). If all child resources are Kubernetes-native and have owner references, finalizers add unnecessary complexity and should not be used
- When a finalizer is needed, it is added early (before creating external resources) and removed only after cleanup succeeds
- Finalizer name follows convention: `<group>/<finalizer-name>` (e.g., `mygroup.example.com/cleanup`)
- Cleanup logic is idempotent (safe to run multiple times)
- Owner references are set on child resources for automatic garbage collection
- Does not set cross-namespace owner references (not supported)
- Uses `controllerutil.AddFinalizer()` / `controllerutil.RemoveFinalizer()` helpers

### 7. Test Coverage

- Unit tests for reconciliation logic covering:
  - Happy path (resource created, updated, deleted)
  - Error paths (API errors, missing dependencies, invalid spec)
  - Idempotency (running reconcile twice produces same result)
  - Finalizer flow (add, cleanup, remove)
  - Status condition transitions
- Integration tests using envtest for realistic API server interaction
- Tests use `gomega` matchers with `Eventually`/`Consistently` for async assertions
- No flaky tests relying on timing or sleep
- Edge cases: concurrent modifications, rapid creation/deletion, large resource counts

### 8. Observability (Events, Logs, Metrics)

**Events:**
- Records events for significant state changes (`r.Recorder.Event()` or `r.Recorder.Eventf()`)
- Uses `Normal` type for successful operations, `Warning` for failures
- Event reasons are CamelCase, messages are human-readable
- Does not emit events on every reconciliation (only meaningful transitions)

**Logging:**
- Uses structured logging (`log.FromContext(ctx)` from controller-runtime, or `klog.InfoS`/`klog.ErrorS`)
- No unstructured logging (`klog.Infof`, `fmt.Printf`, `log.Printf`)
- Key names use lowerCamelCase (`podName`, `namespace`, not `pod_name`)
- Messages start with capital letter, no ending punctuation, past tense ("Deleted pod" not "Deleting pod...")
- Uses `klog.KObj()` / `klog.KRef()` for Kubernetes object references in log values
- Appropriate verbosity levels:
  - V(0): Critical errors, startup info
  - V(1): Configuration, expected repeated errors
  - V(2): Default operational level — state changes, reconciliation events
  - V(4): Debug-level detail
- Libraries return errors instead of logging them (caller controls output)
- Does not log-and-return-error (pick one)

**Metrics:**
- Exposes custom metrics for controller-specific business logic if applicable
- Uses prometheus client conventions (snake_case metric names, proper label cardinality)
- Includes reconciliation duration and error rate metrics where appropriate

### 9. Performance and Cache Usage

- Uses informer cache (default client) for reads; avoids direct API calls unless required
- Watches are scoped to relevant namespaces or label selectors when possible
- Predicates filter irrelevant events (`WithEventFilter`, `GenerationChangedPredicate`)
- `GenerationChangedPredicate` skips reconciliation for metadata-only changes (e.g., label updates not relevant to the controller)
- Does not block reconciliation with long-running operations (offload to jobs or async)
- Rate limiting is configured appropriately for the controller's workload
- Leader election is configured for HA deployments (LeaseDuration=137s, RenewDeadline=107s, RetryPeriod=26s recommended by OpenShift)

### 10. Cache Consistency and Client Type Alignment

controller-runtime maintains separate caches for typed, unstructured, and partial (PartialObjectMetadata) objects. Mixing client types for the same resource causes redundant watches, cache misses, or runtime panics.

**Client type consistency:**
- If a resource is watched or owned as `Unstructured`, all Get/List/Watch calls for that resource must also use `Unstructured` — never mix typed and unstructured access for the same GVK
- If a resource is watched or owned as a typed object (e.g., `&appsv1.Deployment{}`), all Get/List calls must use the same typed object
- If using `PartialObjectMetadata`, the cache must be explicitly configured to handle that GVK as metadata-only (via `cache.Options.ByObject` or `cache.Options.DefaultTransform`)

**Cache configuration alignment:**
- If `Unstructured` objects are used, verify the manager's cache is configured to support unstructured caching for those GVKs
- Every `client.Get()` or `client.List()` call on a resource must have a corresponding Watch (via `.Watches()`, `.Owns()`, or `.For()`) or explicit cache configuration — otherwise the informer cache is not populated and reads will fail or hit the API server directly
- If a resource is only read (Get/List) but never watched, it either needs a watch added or should use a direct (non-cached) client explicitly

**Prefer PartialObjectMetadata:**
- When the controller only needs metadata (labels, annotations, owner references, name/namespace), use `PartialObjectMetadata` instead of the full typed object to reduce memory consumption and API payload size
- Ensure watches and cache are configured for metadata-only access by using `.Watches()` with `PartialObjectMetadata` objects

**Avoid caching Secrets and ConfigMaps:**
- Do not cache Secrets and ConfigMaps unless strictly necessary — they are often large, numerous, and contain sensitive data that increases memory footprint and security exposure
- When access is needed, prefer direct (uncached) client reads or scope the cache to specific namespaces/labels
- If caching is unavoidable, restrict it via `cache.Options.ByObject` with label or field selectors

### 11. Kubernetes API Conventions Compliance

- CRD follows Kubernetes API conventions:
  - `spec` for desired state, `status` for observed state
  - Uses `int32`/`int64` for integers, avoids unsigned types
  - Enums are string-typed CamelCase values
  - Lists of named subobjects preferred over maps of subobjects
  - All optional fields have `+optional` marker and `omitempty` JSON tag
  - Required fields are validated with kubebuilder markers (`+kubebuilder:validation:Required`)
- API versioning follows Kubernetes conventions (v1alpha1 → v1beta1 → v1)
- Webhooks (if present): defaulting webhook sets sensible defaults, validating webhook rejects invalid input
- Printer columns (`+kubebuilder:printcolumn`) show useful summary info in `kubectl get`

## Output Format

Produce the assessment in this format:

### Summary
2-3 sentences describing the overall quality and maturity of the controller implementation.

### Critical Issues (must fix)
Issues that will cause bugs, data loss, security vulnerabilities, or API violations in production. Each issue includes:
- **What**: Description of the problem
- **Where**: File and line reference
- **Why**: Why this is critical (with reference to upstream convention if applicable)
- **Fix**: Concrete suggested change

### Major Issues (should fix)
Issues that indicate poor practices, potential reliability problems, or convention violations. Same format as critical.

### Minor Issues (nice to improve)
Improvements that would increase code quality, observability, or maintainability. Same format as critical.

### Positive Highlights
Things the implementation does well, patterns worth preserving or replicating.
