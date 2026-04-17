# Build: operator-analyzer — Operator CPG Extraction Tool

## What you are building

A Go CLI tool that parses a Kubernetes operator codebase and extracts
structured facts about its architecture. The tool does NOT validate or
judge — it extracts. The LLM using it applies the validation rules.

Each extracted fact is tagged with the skill rule it feeds, so the LLM
knows which lens to apply when reasoning about it.

The tool lives inside the `lburgazzoli/claude-plugins` repository as a
Go module co-located with the skills it serves. It co-evolves with the
skills: when a new rule is added to a skill, the tool gains a new extractor.

---

## Repository layout

Place the tool at this path inside `lburgazzoli/claude-plugins`:

```
plugins/
  k8s.controller/
    skills/
      k8s.controller-analyze/
        SKILL.md                  ← existing skill (update in Phase 3)
    tools/
      operator-analyzer/
        go.mod                    ← module: github.com/lburgazzoli/claude-plugins/tools/operator-analyzer
        go.sum
        main.go                   ← CLI entry point
        EXTRACTORS.md             ← living contract: rule ↔ extractor mapping
        pkg/
          loader/
            loader.go             ← load Go packages via go/packages
          extractor/
            controllers.go        ← Extracted for: rbac-coverage, requeue-safety, finalizer-safety, status-conditions
            crd_versions.go       ← Extracted for: crd-version-coverage
            webhooks.go           ← Extracted for: webhook-registration
            scheme.go             ← Extracted for: scheme-registration
          output/
            json.go               ← JSON serialisation of OperatorFacts
```

---

## Go module setup

```
module github.com/lburgazzoli/claude-plugins/tools/operator-analyzer

go 1.22

require (
    golang.org/x/tools v0.20.0
)
```

Only `golang.org/x/tools` is needed. All parsing uses `go/ast`,
`go/packages`, and `go/token` from stdlib + x/tools. No tree-sitter,
no external graph libraries.

---

## Marker extraction — how to read kubebuilder markers from comments

Kubebuilder markers are structured comments. They are **not** Go annotations
or struct tags — they live in `*ast.CommentGroup` nodes attached to
declarations. Every marker line starts with `// +`.

This section defines the canonical extraction pattern all extractors must
follow. Do not invent alternatives — use these patterns everywhere.

### The `Doc` field is the primary source

`go/ast` attaches the comment group immediately preceding a declaration
(with no blank line between them) to the declaration's `Doc` field:

```go
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
type Foo struct { ... }   // ← ast.TypeSpec.Doc contains both markers above
```

Always prefer `Doc` over scanning `ast.File.Comments` — `Doc` is already
resolved to the correct declaration. Use `ast.File.Comments` only for
package-level markers (see below).

### Core extraction helper

Add this to `pkg/extractor/markers.go` and use it everywhere:

```go
package extractor

import (
    "go/ast"
    "strings"
)

const markerPrefix = "// +"

// Marker represents a single parsed kubebuilder marker.
type Marker struct {
    Raw    string            // full text after "// +", e.g. "kubebuilder:rbac:groups=apps,..."
    Name   string            // marker name, e.g. "kubebuilder:rbac"
    Args   map[string]string // parsed key=value pairs
    Line   int               // source line number
}

// ExtractMarkersFromDoc extracts all kubebuilder markers from a comment group.
// Pass node.Doc for any ast.GenDecl, ast.FuncDecl, ast.TypeSpec, or ast.Field.
func ExtractMarkersFromDoc(cg *ast.CommentGroup, fset *token.FileSet) []Marker {
    if cg == nil {
        return nil
    }
    var markers []Marker
    for _, c := range cg.List {
        if !strings.HasPrefix(c.Text, markerPrefix) {
            continue
        }
        raw := strings.TrimPrefix(c.Text, "// +")
        markers = append(markers, Marker{
            Raw:  raw,
            Name: markerName(raw),
            Args: markerArgs(raw),
            Line: fset.Position(c.Slash).Line,
        })
    }
    return markers
}

// markerName returns the marker identifier before the first colon that
// precedes a key=value section, e.g. "kubebuilder:rbac" from
// "kubebuilder:rbac:groups=apps,resources=deployments,verbs=get"
func markerName(raw string) string {
    // Split on ":" — name is everything up to the last segment that
    // contains "=" (which is the args section).
    parts := strings.SplitN(raw, ":", -1)
    for i, p := range parts {
        if strings.Contains(p, "=") {
            return strings.Join(parts[:i], ":")
        }
    }
    return raw // no args, whole thing is the name (e.g. "kubebuilder:storageversion")
}

// markerArgs parses the key=value,key2=value2 section of a marker.
// Values containing ";" are left as-is (caller splits if needed).
func markerArgs(raw string) map[string]string {
    args := map[string]string{}
    idx := strings.Index(raw, ":")
    if idx == -1 {
        return args
    }
    // find the last ":" before key=value section
    argStr := raw
    parts := strings.SplitN(raw, ":", -1)
    for i, p := range parts {
        if strings.Contains(p, "=") {
            argStr = strings.Join(parts[i:], ":")
            break
        }
        _ = i
    }
    for _, kv := range strings.Split(argStr, ",") {
        if eq := strings.Index(kv, "="); eq != -1 {
            args[kv[:eq]] = kv[eq+1:]
        }
    }
    return args
}
```

### Where to call it per declaration type

#### On type declarations (CRD types, webhook types)

```go
// Walk ast.File.Decls looking for *ast.GenDecl with tok == token.TYPE
ast.Inspect(file, func(n ast.Node) bool {
    gd, ok := n.(*ast.GenDecl)
    if !ok || gd.Tok != token.TYPE {
        return true
    }
    for _, spec := range gd.Specs {
        ts := spec.(*ast.TypeSpec)
        // Doc can be on the GenDecl OR on the TypeSpec — check both.
        // TypeSpec.Doc wins when the marker is directly above the type name
        // inside a grouped decl; GenDecl.Doc wins for standalone decls.
        doc := ts.Doc
        if doc == nil {
            doc = gd.Doc
        }
        markers := ExtractMarkersFromDoc(doc, fset)
        // ... use markers
    }
    return true
})
```

#### On function declarations (Reconcile, SetupWithManager, webhook methods)

```go
ast.Inspect(file, func(n ast.Node) bool {
    fd, ok := n.(*ast.FuncDecl)
    if !ok {
        return true
    }
    markers := ExtractMarkersFromDoc(fd.Doc, fset)
    // ... use markers
    return true
})
```

#### On struct fields (validation markers, optional/required)

```go
ast.Inspect(file, func(n ast.Node) bool {
    st, ok := n.(*ast.StructType)
    if !ok {
        return true
    }
    for _, field := range st.Fields.List {
        markers := ExtractMarkersFromDoc(field.Doc, fset)
        // field.Doc is the comment group immediately above the field
        // field.Comment is the inline comment after the field (rarely has markers)
        // ... use markers
    }
    return true
})
```

#### Package-level markers (`+groupName=`, `+versionName=`)

Package-level markers live in the package comment, not attached to any
declaration. They appear in `doc.go` or `groupversion_info.go` as the
file's leading comment group before the `package` statement.

```go
// In ast.File, the package comment is file.Doc (not file.Comments[0]).
// It is the comment group immediately above the package keyword.

func extractPackageMarkers(file *ast.File, fset *token.FileSet) []Marker {
    return ExtractMarkersFromDoc(file.Doc, fset)
}
```

Example source this handles:

```go
// +groupName=example.com
// +versionName=v1alpha1
package v1alpha1
```

### Marker name reference

The most common markers and their canonical names for use in
`Marker.Name` matching:

| Source marker text | `Marker.Name` |
|---|---|
| `+kubebuilder:object:root=true` | `kubebuilder:object:root` |
| `+kubebuilder:storageversion` | `kubebuilder:storageversion` |
| `+kubebuilder:subresource:status` | `kubebuilder:subresource:status` |
| `+kubebuilder:resource:path=foos,scope=Namespaced` | `kubebuilder:resource` |
| `+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get` | `kubebuilder:rbac` |
| `+kubebuilder:webhook:path=/mutate-...,mutating=true,...` | `kubebuilder:webhook` |
| `+kubebuilder:validation:XValidation:rule="..."` | `kubebuilder:validation:XValidation` |
| `+kubebuilder:validation:Required` | `kubebuilder:validation:Required` |
| `+kubebuilder:validation:Optional` | `kubebuilder:validation:Optional` |
| `+kubebuilder:printcolumn:name="Age",...` | `kubebuilder:printcolumn` |
| `+groupName=example.com` | `groupName` |
| `+versionName=v1alpha1` | `versionName` |
| `+operator-sdk:csv:customresourcedefinitions:...` | `operator-sdk:csv:customresourcedefinitions` |

### Key pitfall: blank lines break Doc attachment

`go/ast` only attaches a comment group to a declaration's `Doc` field if
there is **no blank line** between the last comment line and the declaration.

```go
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get
                                    ← blank line here breaks attachment
func (r *FooReconciler) Reconcile(...) { ... }
// ↑ fd.Doc will be nil — the marker is in ast.File.Comments but not in Doc
```

To handle this robustly, fall back to scanning `ast.File.Comments` for
markers that are within a small line window (≤3 lines) above the
declaration start position when `Doc` is nil:

```go
func docOrNearby(file *ast.File, fset *token.FileSet, pos token.Pos, doc *ast.CommentGroup) *ast.CommentGroup {
    if doc != nil {
        return doc
    }
    declLine := fset.Position(pos).Line
    for _, cg := range file.Comments {
        lastLine := fset.Position(cg.End()).Line
        if lastLine >= declLine-3 && lastLine < declLine {
            return cg
        }
    }
    return nil
}
```

Call this instead of using `fd.Doc` / `ts.Doc` directly in all extractors.

---

## Output schema

The tool emits a single JSON document to stdout. Structure:

```json
{
  "schema_version": "v3",
  "repo_path": "/abs/path/to/repo",
  "extracted_at": "2026-04-16T10:00:00Z",
  "facts": [
    {
      "rules": ["<rule-id>"],
      "kind": "<fact-kind>",
      "file": "relative/path/to/file.go",
      "line": 42,
      "data": { }
    }
  ]
}
```

`rules` is the ordered list of skill rules this fact feeds. Single-rule facts
still serialize as an array (for example: `"rules": ["rbac-coverage"]`).

`kind` is a short label describing what was found (e.g. `"controller"`,
`"rbac_marker"`, `"crd_version"`, `"watch_call"`, `"finalizer_op"`).

`data` is rule-specific — see extractor specs below.

---

## Extractor specifications

Implement each extractor in its own file under `pkg/extractor/`.

### controllers.go

**Rules fed**: `rbac-coverage`, `requeue-safety`, `finalizer-safety`,
`status-conditions`, `watch-owns-alignment`

Walk all Go packages looking for structs that have a `Reconcile` method
with the signature `func (r *XxxReconciler) Reconcile(ctx context.Context,
req reconcile.Request) (reconcile.Result, error)`.

For each reconciler found, emit a fact with kind `"controller"`:

```json
{
  "rules": ["rbac-coverage", "requeue-safety", "finalizer-safety",
            "status-conditions", "watch-owns-alignment"],
  "kind": "controller",
  "file": "controllers/foo_controller.go",
  "line": 42,
  "data": {
    "name": "FooReconciler",
    "reconciles": {
      "kind": "Foo",
      "group": "example.com",
      "version": "v1alpha1"
    },
    "owns": ["Deployment", "Service"],
    "watches": ["ConfigMap", "Secret"],
    "rbac_markers": [
      {"verbs": "get,list,watch", "resource": "foos", "group": "example.com", "line": 38},
      {"verbs": "create,update,patch,delete", "resource": "deployments", "group": "apps", "line": 39}
    ],
    "finalizer_ops": [
      {"op": "AddFinalizer", "value": "example.com/cleanup", "line": 91},
      {"op": "RemoveFinalizer", "value": "example.com/cleanup", "line": 112}
    ],
    "external_write_ops": [
      {"call": "r.Create", "line": 95},
      {"call": "r.Patch", "line": 108}
    ],
    "status_condition_sets": [
      {"condition": "Ready", "line": 134},
      {"condition": "Degraded", "line": 156}
    ],
    "requeue_ops": [
      {"kind": "RequeueAfter", "line": 178},
      {"kind": "Requeue", "line": 195}
    ],
    "error_returns": [
      {"line": 88, "has_requeue": false},
      {"line": 102, "has_requeue": true}
    ]
  }
}
```

**How to extract**:

- Struct name ending in `Reconciler` → controller candidate
- `+kubebuilder:rbac` markers: scan comment groups above the `Reconcile`
  method and above the struct declaration; parse `groups=`, `resources=`,
  `verbs=` from the marker text
- `Owns()`/`Watches()` calls: walk `SetupWithManager` function body, look
  for `.Owns(&TypeX{})` and `.Watches(...)` call expressions; extract the
  type argument
- `controllerutil.AddFinalizer` / `controllerutil.RemoveFinalizer` calls:
  walk `Reconcile` body, record call site line numbers
- `r.Create` / `r.Patch` / `r.Update` / `r.Delete` calls on the client:
  walk `Reconcile` body, record line numbers
- `meta.SetStatusCondition` or `conditions.Set` calls: walk all functions
  reachable from `Reconcile`, record condition type string argument
- `reconcile.Result{Requeue: true}` and `reconcile.Result{RequeueAfter: ...}`
  returns: walk `Reconcile` return statements
- Error returns: any `return ..., err` where err != nil; check if the
  Result in that same return has Requeue set

---

### crd_versions.go

**Rules fed**: `crd-version-coverage`

Scan `api/` directory for Go files containing kubebuilder type markers.

For each CRD kind found, emit a fact with kind `"crd_version"`:

```json
{
  "rules": ["crd-version-coverage"],
  "kind": "crd_version",
  "file": "api/v1alpha1/foo_types.go",
  "line": 28,
  "data": {
    "kind": "Foo",
    "group": "example.com",
    "version": "v1alpha1",
    "storage": false,
    "served": true,
    "hub": false,
    "spoke": false
  }
}
```

**How to extract**:

- Find files with `+kubebuilder:object:root=true` marker
- Parse `+kubebuilder:storageversion` → `storage: true`
- Parse `+kubebuilder:subresource:status` as informational
- Look for `Hub()` method on the type → `hub: true`
- Look for `ConvertTo`/`ConvertFrom` methods → `spoke: true`
- Group from `+groupName=` in `doc.go` or `groupversion_info.go`

---

### webhooks.go

**Rules fed**: `webhook-auth`

Scan for webhook setup functions (files matching `*_webhook.go` or
containing `webhook.Register` / `ctrl.NewWebhookManagedBy` calls).

Emit a fact with kind `"webhook"` per webhook found:

```json
{
  "rules": ["webhook-auth"],
  "kind": "webhook",
  "file": "api/v1alpha1/foo_webhook.go",
  "line": 34,
  "data": {
    "kind": "Foo",
    "type": "defaulting",
    "path": "/mutate-example-com-v1alpha1-foo",
    "has_auth_annotation": true
  }
}
```

**How to extract**:

- Methods named `Default()` → defaulting webhook
- Methods named `ValidateCreate`, `ValidateUpdate`, `ValidateDelete` →
  validating webhook
- Check `+kubebuilder:webhook` marker for `path=` and presence of
  `admissionReviewVersions=`
- `has_auth_annotation`: check if the webhook registration includes
  `sideEffects=None` (required for auth)

---

### scheme.go

**Rules fed**: `scheme-registration`

Find the `main.go` or `cmd/` entry point and extract scheme registrations.

Emit a fact per registration:

```json
{
  "rules": ["scheme-registration"],
  "kind": "scheme_registration",
  "file": "main.go",
  "line": 52,
  "data": {
    "package": "example.com/api/v1alpha1",
    "call": "AddToScheme"
  }
}
```

**How to extract**:

- Find `utilruntime.Must(...)` or `scheme.AddToScheme(...)` calls
- Extract the package path of the argument
- Cross-reference with CRD versions found in `api/` — any version in
  `api/` whose package is NOT in scheme registrations is a gap

---

## main.go

The CLI accepts one positional argument (repo path) and these flags:

```
--rules     comma-separated list of rules to extract for; default "all"
--format    output format: "json" (default) or "pretty"
--out       output file path; default stdout
```

Execution flow:

1. Load packages from `<repo-path>/...` using `go/packages` with
   `packages.NeedSyntax | packages.NeedTypes | packages.NeedFiles |
   packages.NeedCompiledGoFiles | packages.NeedImports`
2. Run all extractors, collect facts into `[]Fact`
3. Filter facts by requested rules if `--rules` is not "all"
4. Serialise to JSON and write to output

Handle load errors gracefully — if some packages fail to load (missing
deps, build errors), log warnings to stderr and continue with what loaded.
The tool must never exit non-zero just because the codebase has build
issues; partial extraction is better than no extraction.

---

## EXTRACTORS.md

Create this file alongside `go.mod`. It is the living contract between
rules and extractors. Update it every time a rule is added or an
extractor changes.

```markdown
# Extractor ↔ Rule Contract

This file documents which extractor fields feed which skill rules.
When a new rule is added to a skill SKILL.md, update this file and
add the corresponding extraction to the relevant extractor file.

| Rule ID                  | Skill                        | Extractor file(s)                          | Fields used                                                    | Added |
|--------------------------|------------------------------|--------------------------------------------|----------------------------------------------------------------|-------|
| rbac-coverage            | k8s.controller-analyze       | extractor/controllers.go                   | rbac_markers, owns, watches                                    | v0.1  |
| requeue-safety           | k8s.controller-analyze       | extractor/controllers.go                   | error_returns, requeue_ops                                     | v0.1  |
| finalizer-safety         | k8s.controller-analyze       | extractor/controllers.go                   | finalizer_ops, external_write_ops                              | v0.1  |
| status-conditions        | k8s.controller-analyze       | extractor/controllers.go                   | status_condition_sets, reconciles.kind                         | v0.1  |
| watch-owns-alignment     | k8s.controller-analyze       | extractor/controllers.go                   | owns, watches, rbac_markers                                    | v0.1  |
| crd-version-coverage     | k8s.controller-analyze       | extractor/crd_versions.go                  | kind, version, storage, hub, spoke                             | v0.1  |
| webhook-auth             | k8s.controller-analyze       | extractor/webhooks.go                      | type, has_auth_annotation                                      | v0.1  |
| scheme-registration      | k8s.controller-analyze       | extractor/scheme.go                        | package, call                                                  | v0.1  |

## How to add a new rule

1. Add the rule description to the relevant skill's SKILL.md under
   `## Validation Rules`, following the existing format.
2. Identify what facts the rule needs to reason about.
3. If an existing extractor already emits those facts, add the new
   rule ID to the `rules` array in that extractor's output.
4. If new facts are needed, add a new extractor file or extend an
   existing one.
5. Update this table with the new row.
6. Run `go test ./...` to verify nothing is broken.
```

---

## SKILL.md update (Phase 3)

After the tool is built, update `k8s.controller-analyze/SKILL.md` to
add a section before the main analysis steps:

```markdown
## Step 0 — Extract operator facts

Build and run the operator-analyzer to get structured facts about this
codebase. This grounds the analysis in concrete evidence rather than
inference from reading files.

### Build the tool (once per session)

```bash
TOOL_DIR="$(dirname $0)/../../tools/operator-analyzer"
TOOL_BIN="/tmp/operator-analyzer-$(basename $(pwd))"

if [ ! -f "$TOOL_BIN" ]; then
  echo "Building operator-analyzer..."
  (cd "$TOOL_DIR" && go build -o "$TOOL_BIN" .)
fi
```

### Run extraction

```bash
"$TOOL_BIN" . --format json --out /tmp/operator-facts.json
```

If the tool exits non-zero, note any stderr warnings but continue —
partial facts are still useful.

### Load facts into context

Read `/tmp/operator-facts.json`. The facts are tagged with the rule
they feed. For each rule in `## Validation Rules` below, find the
relevant facts and apply the rule's reasoning to them.

## Validation Rules

For each rule, the format is:
- **Needs**: which fact kinds to look for in the JSON
- **Checks**: what the LLM should reason about
- **Output**: how to report findings

### rbac-coverage
**Needs**: facts with kind `controller`, fields `rbac_markers`, `owns`, `watches`
**Checks**:
  - Every resource in `owns` must have a marker with `create,update,patch,delete` verbs
  - Every resource in `watches` must have a marker with at least `get,list,watch` verbs
  - Flag any resource present in owns/watches but absent from rbac_markers
  - Flag any rbac_marker for a resource not present in owns/watches (over-provisioned)
**Output**: list mismatches with file:line from the fact data

### requeue-safety
**Needs**: facts with kind `controller`, fields `error_returns`, `requeue_ops`
**Checks**:
  - Every `error_returns` entry with `has_requeue: false` is a potential lost event
  - Reconcilers with no `requeue_ops` at all may rely entirely on watch events —
    flag if the reconciler manages external resources (infer from `external_write_ops`)
  - A return of `Result{}, nil` without a prior status update is suspicious
**Output**: list error return lines that lack requeue, with context

### finalizer-safety
**Needs**: facts with kind `controller`, fields `finalizer_ops`, `external_write_ops`
**Checks**:
  - `AddFinalizer` line must be less than the lowest `external_write_ops` line
    in the same function scope
  - If `RemoveFinalizer` is present, check there is a corresponding deletion
    of external resources before it (look for Delete ops between the two)
  - A reconciler with `external_write_ops` but no `finalizer_ops` is a leak risk
**Output**: ordering violations with line numbers, missing finalizer warnings

### status-conditions
**Needs**: facts with kind `controller`, fields `status_condition_sets`, `reconciles.kind`
**Checks**:
  - A reconciler with no `status_condition_sets` provides no observability
  - Check for at minimum a `Ready` condition being set
  - If error paths exist (from `error_returns`) but no `Degraded`/`Failed`
    condition is set, the resource status won't reflect failures
**Output**: missing condition types, reconcilers with no conditions

### watch-owns-alignment
**Needs**: facts with kind `controller`, fields `owns`, `watches`
**Checks**:
  - Resources in `owns` are managed by this controller and should be watched
  - Cross-reference: any resource in `owns` not in `watches` won't trigger
    reconciliation on child resource changes
  - Flag `watches` entries that are very broad (e.g. watching a core type
    like ConfigMap without a predicate — infer from absence of filter hints)
**Output**: owns/watches misalignment with explanation

### crd-version-coverage
**Needs**: facts with kind `crd_version` and kind `controller`
**Checks**:
  - Every CRD version with `served: true` should have a corresponding
    controller reconciling it (match on kind + group)
  - If multiple versions exist and none has `hub: true`, conversion
    webhook may be missing
  - A version with `storage: false` and no `spoke: true` has no
    conversion path — flag if other versions exist
**Output**: unreconciled versions, missing conversion paths

### webhook-auth
**Needs**: facts with kind `webhook`
**Checks**:
  - Any webhook with `has_auth_annotation: false` is missing sideEffects=None
  - Flag validating webhooks that lack explicit fail-open/fail-closed policy
    (infer from absence of `failurePolicy` in marker)
**Output**: webhook registration gaps

### scheme-registration
**Needs**: facts with kind `scheme_registration` and kind `crd_version`
**Checks**:
  - Every CRD version package found in `api/` must appear in scheme_registration facts
  - A CRD version that is served but not registered will cause runtime panics
**Output**: unregistered API versions
```

---

## Testing

Add a `testdata/` directory under `operator-analyzer/` with a minimal
synthetic operator:

```
testdata/
  simple-operator/
    go.mod
    main.go
    api/
      v1alpha1/
        foo_types.go      ← one CRD, one version, storage=true
    controllers/
      foo_controller.go   ← one reconciler, rbac markers, finalizer, status condition
```

Write table-driven tests in `pkg/extractor/*_test.go` that load the
testdata operator and assert the extracted facts match expected JSON.
Run with `go test ./...`.

---

## Acceptance criteria

The tool is complete when:

1. `go build ./...` succeeds from `tools/operator-analyzer/`
2. `go test ./...` passes against the testdata operator
3. Running against the `opendatahub-operator` repo produces valid JSON
   with at least one fact per extractor (controllers, crd_versions,
   webhooks, scheme_registrations)
4. `EXTRACTORS.md` is populated with all v0.1 rules
5. The SKILL.md `## Step 0` section has been added and references the
   correct relative tool path
6. Running the updated skill against a test operator repo produces a
   report that cites specific file:line references from the extracted facts
