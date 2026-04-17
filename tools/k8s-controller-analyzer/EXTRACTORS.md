# Extractor ↔ Rule Contract

This file documents which extractor fields feed which skill rules.
When a new rule is added to a skill SKILL.md, update this file and
add the corresponding extraction to the relevant extractor file.

## Controller Extractor (`extractor/controllers.go`)

| Rule ID                  | Skill                              | Fields used                                                             | Added |
|--------------------------|------------------------------------|-------------------------------------------------------------------------|-------|
| rbac-coverage            | k8s.controller-architecture (3a-d) | rbac_markers, owns, watches, api_calls                                  | v0.1  |
| requeue-safety           | k8s.controller-architecture (2a-c) | error_returns, requeue_ops                                              | v0.1  |
| finalizer-safety         | k8s.controller-architecture (5a-c) | finalizer_ops, owner_ref_ops, external_write_ops                        | v0.1  |
| status-conditions        | k8s.controller-architecture (4a-d) | status_condition_sets, status_update_sites, retry_ops, reconciles.kind  | v0.2  |
| library-rendering        | k8s.controller-architecture (7d-e) | library_invocations                                                     | v0.3  |
| watch-owns-alignment     | k8s.controller-architecture (8a-b) | owns, watches, predicate_usages                                        | v0.1  |

### Controller fact fields

| Field                 | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| name                  | Reconciler struct name                                                      |
| reconciles.kind       | Inferred CRD kind from struct name                                          |
| reconciles.group      | CRD group from package markers                                              |
| reconciles.version    | CRD version from package path                                               |
| owns                  | Types passed to `.Owns()` in SetupWithManager                               |
| watches               | Types passed to `.Watches()` in SetupWithManager                            |
| rbac_markers          | Parsed +kubebuilder:rbac markers (verbs, resource, group, resourceNames)    |
| finalizer_ops         | AddFinalizer/RemoveFinalizer calls with values                              |
| owner_ref_ops         | SetControllerReference/SetOwnerReference/finalizer calls                    |
| external_write_ops    | Create/Update/Patch/Delete calls in Reconcile                               |
| api_calls             | All candidate k8s client API calls (method, receiver, obj_type)             |
| status_condition_sets | meta.SetStatusCondition calls with condition type and ObservedGeneration     |
| status_update_sites   | Status().Update()/Status().Patch() calls with retry guard metadata          |
| retry_ops            | Retry wrapper usage (RetryOnConflict/OnError), backoff kind, wrapped calls |
| library_invocations  | Helm/Kustomize call sites with repo-local reconcile-loop reachability metadata |
| event_usages          | Event/Eventf recorder calls                                                 |
| not_found_handlers    | IsNotFound/IgnoreNotFound calls                                             |
| predicate_usages      | Predicate types and WithEventFilter calls in SetupWithManager               |
| requeue_ops           | Requeue/RequeueAfter in return statements                                   |
| error_returns         | Error returns with requeue presence                                         |

## CRD Version Extractor (`extractor/crd_versions.go`)

| Rule ID                  | Skill                         | Fields used                              | Added |
|--------------------------|-------------------------------|------------------------------------------|-------|
| crd-version-coverage     | k8s.controller-api (2a-d)     | kind, version, storage, hub, spoke       | v0.1  |

## API Fields Extractor (`extractor/api_fields.go`)

| Rule ID                  | Skill                         | Fields used                                                | Added |
|--------------------------|-------------------------------|------------------------------------------------------------|-------|
| crd-structure            | k8s.controller-api (1a-h)     | crd_type: root/status markers, scope, print_columns        | v0.2  |
| field-conventions        | k8s.controller-api (1d-g)     | crd_field: json_tag, omitempty, optional, required, type    | v0.2  |
| marker-correctness       | k8s.controller-api (4a-f)     | crd_field: markers, crd_type: root/status markers          | v0.2  |

## Webhook Extractor (`extractor/webhooks.go`)

| Rule ID                  | Skill                         | Fields used                                                | Added |
|--------------------------|-------------------------------|------------------------------------------------------------|-------|
| webhook-auth             | k8s.controller-api (3a-e)     | type, failure_policy, side_effects, timeout_seconds         | v0.1  |

## Scheme Extractor (`extractor/scheme.go`)

| Rule ID                  | Skill                         | Fields used                              | Added |
|--------------------------|-------------------------------|------------------------------------------|-------|
| scheme-registration      | k8s.controller-architecture   | package, call                            | v0.1  |

## Import Analysis Extractor (`extractor/imports.go`)

| Rule ID                  | Skill                              | Fields used                              | Added |
|--------------------------|------------------------------------|------------------------------------------|-------|
| vendor-isolation         | k8s.controller-architecture (6a-b) | vendor_imports (path, vendor)            | v0.2  |
| library-imports          | k8s.controller-architecture        | library_imports (family, path, line)     | v0.3  |
| structured-logging       | k8s.controller-prod-readiness (2a) | unstructured_logging (call, line)        | v0.2  |
| metrics-coverage         | k8s.controller-prod-readiness (2c) | has_metrics, metrics_package             | v0.2  |

### Import analysis fact fields

| Field               | Description                                                       |
|---------------------|-------------------------------------------------------------------|
| vendor_imports      | Vendor cloud/platform imports (`path`, `vendor`, `line`)          |
| library_imports     | Helm/Kustomize imports (`family`, `path`, `line`)                 |
| unstructured_logging| Unstructured logging calls (`call`, `line`)                       |
| has_metrics         | Whether a metrics package import was detected                     |
| metrics_package     | Metrics package import path                                       |

## Common fact envelope

Every emitted fact uses the same outer shape:

- `rules`: ordered `[]string`; single-rule facts still serialize as `["rule-id"]`
- `kind`: fact kind identifier
- `file`, `line`: source location for the fact
- `data`: typed payload for the fact kind

## RBAC reasoning contract

For architecture assessments:

- Use `rbac_manifest` as the primary evidence for effective committed permissions.
- Use `controller.rbac_markers` as secondary evidence for generator intent and drift checks.
- Use `controller.api_calls` to determine what permissions the controller actually needs.

## How to add a new rule

1. Add the rule description to the relevant skill's SKILL.md under
   `## Assessment Areas`, following the existing format.
2. Identify what facts the rule needs to reason about.
3. If an existing extractor already emits those facts, add the new
   rule ID to the `rules` array in that extractor's output.
4. If new facts are needed, add a new extractor file or extend an
   existing one.
5. Add the rule ID constant to `consts.go`.
6. Update this table with the new row.
7. Run `make test` to verify nothing is broken.
