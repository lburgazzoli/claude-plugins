# Analyzer Output Schema

Use this reference when a skill consumes JSON emitted by `k8s-controller-analyzer`.

This file is the normative input contract for skills. It defines the analyzer
report envelope, the common fact shape, and the payload shape per fact kind at a
level suitable for prompt instructions. For detailed field-to-rule mapping, see
`tools/k8s-controller-analyzer/EXTRACTORS.md`.

## Schema Version

Current analyzer schema version: `v3`

This schema is intentionally documentation-first, but skills should treat the
field names, casing, and structural rules below as normative.

## Report Envelope

The analyzer emits a JSON object with these top-level fields:

- `schema_version`
- `repo_path`
- `extracted_at`
- `manifest` (optional)
- `facts`

### Field Notes

- `schema_version`: currently `v3`
- `repo_path`: absolute path to the analyzed repository root
- `extracted_at`: UTC RFC3339 timestamp
- `manifest`: present when the analyzer runs with `--skill`; omitted otherwise
- `facts`: array of extracted fact objects

## Manifest Shape

When present, `manifest` has this shape:

- `skill`: skill name used for manifest selection
- `count`: number of manifest entries
- `hash`: deterministic manifest hash
- `entries`: ordered list of manifest entries

Each manifest entry has:

- `category`
- `path`

## Fact Envelope

Every fact in `facts` has this common shape:

- `rules`
- `kind`
- `file`
- `line`
- `data`

### Field Notes

- `rules`: ordered `[]string`; single-rule facts still use an array
- `kind`: fact kind identifier; skills should dispatch on this field
- `file`: repo-relative path for the evidence source; may be empty for repo-wide facts
- `line`: 1-based line number; `0` means the fact is not tied to one specific line
- `data`: typed payload whose shape depends on `kind`

## Skill Consumption Rules

Skills consuming analyzer output should follow these rules:

1. Treat `rules` as metadata, not the primary dispatch key. Branch on `kind`,
   then read the corresponding `data` shape.
2. Never assume `rules` is a string. It is always an array in `v3`.
3. Ignore unknown fact kinds unless the skill explicitly requires them.
4. Treat `manifest.count == 0` as "no relevant scope discovered" for
   manifest-aware skill runs.
5. For RBAC reasoning:
   - use `rbac_manifest` as the primary evidence for effective committed permissions
   - use `controller.rbac_markers` as secondary evidence for source/generator intent
   - use `controller.api_calls[].required_permissions` to determine what permissions are needed when the analyzer can resolve them concretely
   - use `controller.ambiguity_signals` to detect unresolved receiver, unstructured, or rendered-object cases before scoring unused-RBAC findings
6. For controller identity, use `controller.data.reconciles.kind`,
   `controller.data.reconciles.group`, and `controller.data.reconciles.version`.
   Do not look for legacy flat `reconciles_kind`, `reconciles_group`, or
   `reconciles_version` fields.

## Fact Kinds

### `controller`

Payload fields:

- `name`
- `reconciles`
- `owns`
- `watches`
- `rbac_markers`
- `finalizer_ops`
- `owner_ref_ops`
- `external_write_ops`
- `api_calls`
- `status_condition_sets`
- `status_update_sites`
- `retry_ops`
- `library_invocations`
- `event_usages`
- `not_found_handlers`
- `predicate_usages`
- `requeue_ops`
- `error_returns`
- `ambiguity_signals`
- `max_concurrent_reconciles`

`reconciles` is a nested object with:

- `group`
- `version`
- `kind`

RBAC-relevant nested signals:

- `rbac_markers[].permissions` — normalized permission tuples derived from source markers
- `api_calls[].operation_class` — normalized intent such as `read`, `write`, or `statusWrite`
- `api_calls[].required_permissions` — normalized permission tuples inferred from concrete API usage
- `api_calls[].receiver_resolution` / `api_calls[].object_resolution` — resolution metadata for confidence and ambiguity handling
- `event_usages[].required_permissions` — normalized permission tuples for emitted events
- `ambiguity_signals[]` — explicit unresolved-signal records such as `receiverUnresolved`, `usesUnstructured`, or `usesRenderedObjects`

### `crd_version`

Payload fields:

- `kind`
- `group`
- `version`
- `storage`
- `served`
- `hub`
- `spoke`

### `crd_type`

Payload fields:

- `kind`
- `has_root_marker`
- `has_status_subresource`
- `has_status_field`
- `resource_scope`
- `print_columns`
- `fields`
- `unsigned_fields`
- `status_field_type`
- `cel_rules`

### `crd_field`

Payload fields:

- `type_name`
- `field_name`
- `field_type`
- `json_tag`
- `has_omitempty`
- `is_optional`
- `is_required`
- `list_type`
- `list_map_keys`
- `cel_rules`
- `has_max_items`
- `has_max_properties`
- `markers`

### `webhook`

Payload fields:

- `kind`
- `type`
- `path`
- `failure_policy`
- `side_effects`
- `timeout_seconds`
- `has_auth_annotation`

### `scheme_registration`

Payload fields:

- `package`
- `call`

### `import_analysis`

Payload fields:

- `vendor_imports`
- `library_imports`
- `unstructured_logging`
- `has_metrics`
- `metrics_package`

### `rbac_manifest`

Payload fields:

- `name`
- `kind`
- `namespace`
- `rules`
- `permissions`
- `has_wildcard`
- `has_wildcard_group`
- `has_wildcard_resource`
- `has_wildcard_verb`
- `has_events`

### `crd_manifest`

Payload fields:

- `name`
- `group`
- `kind`
- `scope`
- `versions`
- `conversion_strategy`
- `served_version_count`
- `has_multiple_served`

### `webhook_manifest`

Payload fields:

- `name`
- `kind`
- `webhooks`

Each webhook entry includes `reinvocation_policy` (for mutating webhooks).

### `deployment_manifest`

Payload fields:

- `name`
- `kind`
- `namespace`
- `containers`
- `security_context`

### `networkpolicy_manifest`

Payload fields:

- `name`
- `namespace`
- `policy_types`

### `manager_config`

Payload fields:

- `leader_election`
- `leader_election_id`
- `leader_election_resource_lock`
- `leader_election_release_on_cancel`
- `has_signal_handler`
- `graceful_shutdown_timeout`

### `test_discovery`

Payload fields:

- `files`
- `count`

## Minimal Example

```json
{
  "schema_version": "v3",
  "repo_path": "/abs/path/to/repo",
  "extracted_at": "2026-04-17T12:00:00Z",
  "manifest": {
    "skill": "architecture",
    "count": 2,
    "hash": "abc123",
    "entries": [
      {
        "category": "controller",
        "path": "controllers/foo_controller.go"
      },
      {
        "category": "rbac",
        "path": "config/rbac/role.yaml"
      }
    ]
  },
  "facts": [
    {
      "rules": [
        "rbac-coverage",
        "requeue-safety"
      ],
      "kind": "controller",
      "file": "controllers/foo_controller.go",
      "line": 42,
      "data": {
        "name": "FooReconciler",
        "reconciles": {
          "group": "example.com",
          "version": "v1alpha1",
          "kind": "Foo"
        }
      }
    }
  ]
}
```

## Relationship To Other References

- Use this file for analyzer input semantics.
- Use `references/k8s-controller/report-schema.md` for skill report output semantics.
- Use `tools/k8s-controller-analyzer/EXTRACTORS.md` for rule-to-field mapping and
  extractor coverage details.
