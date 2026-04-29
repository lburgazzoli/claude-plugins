# Context Builder

Build the shared context file (`context.md`) that serves as the sole source of truth for all persona assessments. This is split into three phases, each run by a separate step/agent to keep context windows focused:

- **Phase 1** (Step 001): Resolve repos + build raw inventories → writes `context-draft.md`
- **Phase 2** (Step 002): Verify and cross-validate CRDs → updates `context-draft.md` in place
- **Phase 3** (Step 003): Build derived data + finalize → renames to `context.md`

**References** — read before starting any phase:
- **`CLAUDE.md` (vault root)** — **Repository cloning and refs** (default tree unchanged; per-ref access via **separate clone** at `.context/repos/<repo>@<ref>`). **Override**: do NOT clean up component repo clones after the assessment — keep them in `.context/repos/` for reuse by future runs and persona subagents. If a clone already exists, update it (`git fetch && git pull --ff-only`) instead of re-cloning.
- `${CLAUDE_SKILL_DIR}/resources/constitution.md` — universal rules (read for the "never guess" and meta-operator architecture rules)

---

## Shared Rules

These rules apply to all three phases.

### Discrepancy Reporting

Whenever any phase finds that architecture-context docs contradict actual code in a component repo — for any resource type (CRDs, deployments, endpoints, webhooks, kustomize overlays, dependencies, TLS configuration, etc.) — **add the discrepancy to `{run_dir}/discrepancies.yaml`** (read the file, parse the array, append the entry, rewrite). Always use the repo data for the context and flag the architecture-context doc as needing correction.

Write discrepancies to `{run_dir}/discrepancies.yaml` (not markdown) as a single YAML array. Each entry is a list item. This format is machine-parsable and supports variable-length reference lists.

```yaml
- component: {component name}
  description: {brief description}
  claim: {what the source says}
  evidence: {what the actual code/data shows}
  resolution: {which data was used and why}
  refs:
    - {file}#L{line}
    - {file}#L{line}
```

**Reference format**: `{source}:{path}#L{line}` or `#L{start}-L{end}` for ranges. Examples:
- `architecture-context:architecture/rhoai-3.4/kserve.md#L47`
- `red-hat-data-services/kserve@rhoai-3.4:config/crd/full/serving.kserve.io_servingruntimes.yaml#L30`
- `architecture-context:architecture/rhoai-3.3/PLATFORM.md#L223`

**Rules**:
- `refs` MUST include every file consulted — both the source of the claim and the source of the evidence. The most authoritative source (repo CRD YAML) must be listed. Architecture docs that led to the discovery can also be listed, but the repo file is the primary evidence.
- Every ref MUST include the full file path and line number. Never write just a repo name.
- For red-flag discrepancies (scope change, version downgrade, etc.), at least one ref MUST point to the actual CRD YAML or code in the component repo — architecture docs cannot verify architecture docs.

**Example**:
```yaml
- component: KServe
  description: ServingRuntime API version in 3.4 doc is stale
  claim: ServingRuntime is v1beta1
  evidence: v1alpha1 only (served=true, storage=true)
  resolution: used evidence (v1alpha1) in context; architecture doc is stale
  refs:
    - architecture-context:architecture/rhoai-3.4/kserve.md#L47
    - red-hat-data-services/kserve@rhoai-3.4:config/crd/full/serving.kserve.io_servingruntimes.yaml#L30

- component: MLflow
  description: 3.3 PLATFORM.md reports wrong CRD scope
  claim: MLflow CRD scope is Namespaced
  evidence: Cluster scope in actual CRD YAML
  resolution: PLATFORM.md error; both versions have Cluster scope
  refs:
    - architecture-context:architecture/rhoai-3.3/PLATFORM.md#L223
    - architecture-context:architecture/rhoai-3.3/mlflow-operator.md#L36
    - red-hat-data-services/mlflow-operator@rhoai-3.3:config/crd/bases/mlflow.opendatahub.io_mlflows.yaml#L15
```

**Schema**: see `${CLAUDE_SKILL_DIR}/resources/schemas/discrepancies.schema.yaml` for the formal field definitions. After writing `discrepancies.yaml`, validate it with: `yq '.[] | has("component") and has("description") and has("claim") and has("evidence") and has("resolution") and has("refs")' discrepancies.yaml` — all entries must return `true`.

### Deterministic Exhaustiveness

**The `context.md` this step writes is the sole source of truth for all persona assessments.** Personas do not perform their own discovery — they assess what this step provides.

Rules:
1. If this step omits a resource, no persona will ever evaluate it. There is no safety net.
2. Enumerate **every** resource from both source and target platform architectures — every CRD, deployment, endpoint, webhook, workload type, dependency operator, interaction edge, restart trigger, and storage-claim inventory.
3. Never apply heuristics to skip "low-risk" items. Unchanged resources get listed with `Change: none`.
4. Never summarize or aggregate. Each resource is its own row.
5. **Reproducible evidence, not bit-identical prose**: write `context.md` from the same pinned repo SHAs, same scope, and the same static sources.
6. **Keep tables compact**: each table row should be a single line with terse cell values. No prose in table cells.

### CRD Identity Rules

These rules apply whenever building or updating the CRD Inventory (Phases 1 and 2):

- A CRD is uniquely identified by **API group + kind**. Never match CRDs by kind alone. Never embed a single API version into the CRD identity — version belongs in the Versions columns.
- **Multi-version CRDs are normal.** Kubernetes CRDs can serve multiple API versions simultaneously via `spec.versions[]` (e.g., a CRD serving both `v1` and `v1alpha1`). When reading CRD tables from PLATFORM.md or architecture docs, extract **all listed API versions** for a given group+kind, not just the first one. If a doc lists separate entries for `feast.dev/v1/FeatureStore` and `feast.dev/v1alpha1/FeatureStore`, these are the same CRD serving multiple versions — merge them into one row with both versions listed.
- **Version format**: list all served versions comma-separated, with the storage version marked: e.g., `v1 [storage], v1alpha1`. If the doc does not indicate which is the storage version, mark as `(storage unknown)`.
- **Resolve conflicting CRD metadata**: precedence order: (1) the doc that references the actual CRD YAML file path wins; (2) the component architecture doc wins over PLATFORM.md; (3) PLATFORM.md is the fallback. When a CRD appears in multiple component docs for the same version with different versions, prefer the doc that owns the CRD definition files. Flag every discrepancy.
- **Cross-validate source version before reporting changes**: before marking any CRD as `versions-changed` or `scope-changed`, apply the precedence rule to the **source** version too — not just the target. If the source PLATFORM.md says one thing (e.g., Namespaced) but the source component doc says another (e.g., Cluster), the PLATFORM.md is wrong. Use the component doc value as the source baseline, then re-compare with the target. If source component doc and target component doc agree → the change status is `none` and the PLATFORM.md error is a discrepancy, not a real change.
- When computing the diff, match source and target CRDs by `group + kind`. A CRD that appears in a new API group in the target is an **added** CRD (and the old-group CRD is **removed**), not a "changed" CRD.

CRD change status values:
- `added` — CRD exists in target but not source
- `removed` — CRD exists in source but not target
- `scope-changed` — CRD scope changed (Namespaced↔Cluster)
- `versions-changed` — the set of served versions changed (version added or removed from served set)
- `storage-version-changed` — same served versions but storage designation moved (e.g., storage moved from `v1alpha1` to `v1`)
- `none` — no change

---

## Phase 1: Resolve Repos and Build Raw Inventories

**Inputs**: user-provided source and target versions
**Outputs**: `{run_dir}/context-draft.md`, `{run_dir}/discrepancies.yaml` (empty `[]`)

This phase is mechanical extraction — no judgment calls. Read docs and build tables.

### 1a. Clean up stale clones from previous runs

Before starting, check `.context/repos/` for stale version-specific clones (`odh-gitops@rhoai-*`) left by previous failed runs. Remove any that exist — they may contain outdated content. Prefer separate clones (`odh-gitops@rhoai-X.Y`) over worktrees since clones don't require explicit cleanup on failure.

### 1b. Update repos

Best-effort pulls, continue on failure:

```bash
git -C .context/repos/architecture-context pull --ff-only 2>/dev/null || true
git -C .context/repos/odh-cli pull --ff-only 2>/dev/null || true
git -C .context/repos/odh-gitops pull --ff-only 2>/dev/null || true
git -C .context/repos/architecture-decision-records pull --ff-only 2>/dev/null || true
```

If a repo is missing, clone the **default** branch:
```bash
gh repo clone opendatahub-io/{repo} .context/repos/{repo} -- --depth 1 --single-branch
```

Record HEAD commit SHA of each default-clone repo for reproducibility. **odh-gitops** version-specific content uses **separate** `@<branch>` clones per **CLAUDE.md** — not `git checkout` on `.context/repos/odh-gitops`. RHOAI release branches (`rhoai-*`) live in the **midstream** repo (`red-hat-data-services/odh-gitops`), not upstream:
```bash
gh repo clone red-hat-data-services/odh-gitops .context/repos/odh-gitops@rhoai-{version} -- --depth 1 --single-branch --branch rhoai-{version}
```

### 1c. Resolve architecture directories

Map each version to `.context/repos/architecture-context/architecture/rhoai-{major}.{minor}/PLATFORM.md`.

If the directory doesn't exist:
1. Check for early-access directories (`rhoai-{major}.{minor}-ea.{N}`) — use the latest EA
2. If no EA exists, try OLM: `oc get packagemanifest rhods-operator -n openshift-marketplace -o json` (requires cluster access — skip if unavailable)
3. If resolution still fails, **do not block**: proceed in **reduced-confidence mode** — document the gap in `context.md` and continue. Only stop if the user's arguments are invalid (e.g. missing version).

### 1d. Component Diff

Read both PLATFORM.md files. Extract the component lists and compute:
- **Removed**: components in source but absent from target
- **Added**: components in target but absent from source
- **Changed**: components in both but with different versions, images, or CRD counts
- **OCP compatibility**: supported OCP versions for each, overlap window, floor change

For each component in the diff (removed, added, changed), include the path to its architecture doc in both versions:
```
- **{component}** — {brief description of change}
  - Source doc: `architecture/rhoai-{source}/{component}.md` (or "N/A" if added)
  - Target doc: `architecture/rhoai-{target}/{component}.md` (or "N/A" if removed)
```

### 1e. Static Resource Discovery

For **every component** in both source and target, read its architecture summary (e.g., `architecture/rhoai-{version}/kserve.md`). Each component doc has structured sections for CRDs, endpoints, deployments, webhooks.

Also read the odh-gitops manifests. For each of **source** and **target** RHOAI versions, if a branch exists matching the version (e.g. `rhoai-3.3`), ensure a **dedicated** tree and read `components/operators/`, `dependencies/operators/`, and `configurations/` from that path only. The gitops repo is authoritative for dependency operators. If no matching branch exists, fall back to architecture-context and note the gap. Record each gitops path used and its HEAD SHA in `context-draft.md`.

**PVC / storage (static)**: from architecture component docs and gitops manifests, list every **documented** PersistentVolumeClaim name pattern, VolumeClaimTemplate, or storage-related workload volume (per component, per version). One row per distinct pattern; `Change: none` when unchanged.

Build a unified resource map. For each resource discovered, record which file it was read from (repo, branch, path). This provenance feeds the **Source Map** section in `context-draft.md`.

**PLATFORM.md structural variance**: PLATFORM.md files are generated independently per version and may have different section names, table formats, and levels of detail. Do not assume identical structure across versions. Extract data semantically (e.g., find CRD tables by looking for columns like `Group`, `API Group`, `Kind`, `Scope` regardless of section heading). When the source and target PLATFORM.md use different table schemas, normalize to the output format rather than diffing tables structurally.

Then compute the diff for each resource category:

**CRD Inventory** — for every component, list every CRD with group, kind, scope, served versions, and change status. Apply CRD Identity Rules from the Shared Rules section. Leave Conversion and Schema Delta columns empty — they are populated in Phase 2.

| Component | CRD (group/kind) | Scope | Source Versions | Target Versions | Change | Conversion | Schema Delta |

**Deployment Inventory** — for every component, list every Deployment, DaemonSet, StatefulSet with type and change status.

**Endpoint Inventory** — for every component, list every service, route, and HTTP endpoint with port and change status.

**Webhook Inventory** — for every component, list every validating and mutating webhook with change status.

**Workload Type Inventory** — for every component, list every CRD kind that represents user-created workloads with change status.

**Dependency Operator Inventory** — for every dependency operator from odh-gitops, list channel, source version, target version, and change status.

### 1f. Write context-draft.md

The run directory already exists (created by the orchestrator before the step loop).

Write `context-draft.md` to the run directory with all data from Phase 1. The file must contain all sections listed in the context.md template below, **except** the following sections which are added in Phase 3:
- Cross-Component Interaction Map
- Potential Accidental Restart Triggers
- odh-cli Check Registry
- Persona Routing and Finding Ownership
- Reference Paths

Write an empty `discrepancies.yaml` (`[]`).

### context.md Template

The final `context.md` (after all three phases) must contain:

```markdown
# Upgrade Assessment Context

**Source version**: {source}
**Target version**: {target}
**Run ID**: {run_id}
**Date**: {today}
**Repos**: architecture-context@{sha}, odh-cli@{sha}, odh-gitops (source) @{path}@{sha}, odh-gitops (target) @{path}@{sha} (or `N/A` if no branch), ADRs@{sha}

## Source Platform
**Path**: {absolute path to source PLATFORM.md}
**OCP versions**: {list}
**Component count**: {N}

## Target Platform
**Path**: {absolute path to target PLATFORM.md}
**OCP versions**: {list}
**Component count**: {N}

## Component Diff

### Removed (source → target)
### Added (target only)
### Changed
### OCP Compatibility

## Source Map

### Component Docs
| Component | Source Doc | Target Doc |

### Dependency Operator Sources
| Operator | Source Path | Target Path |

### ADR References
| ADR | Path |

### odh-cli Check Sources
| Check ID | Path |

## Resource Discovery (Static)

### CRD Inventory
| Component | CRD (group/kind) | Scope | Source Versions | Target Versions | Change | Conversion | Schema Delta |

### Deployment Inventory
| Component | Deployment/DaemonSet/StatefulSet | Type | Source | Target | Change |

### Endpoint Inventory
| Component | Endpoint/Service/Route | Port | Source | Target | Change |

### Webhook Inventory
| Component | Webhook | Type (V/M) | Source | Target | Change |

### Workload Type Inventory
| Component | Workload Kind | Scope | Source | Target | Change |

### PVC and Storage (Static)
| Component | Claim / pattern / volume | Source | Target | Change |

### Dependency Operator Inventory
| Operator | Channel | Source Version | Target Version | Change | Breaking Changes | Behavioral Changes | Risk |

## Cross-Component Interaction Map
| Component A | Depends On (B) | Interaction Type | Source (explicit/inferred) | Impact if B Changes |

## Potential Accidental Restart Triggers
| Component | Managed Resource | Change Type | Restart Trigger? | Data-Plane? |

## odh-cli Check Registry

### Pre-Upgrade Checks (upgrade linter — run against source version)
### Post-Upgrade Checks (version lint — run against target version)

## Reference Paths
- Architecture Context: {absolute path}
- ODH CLI: {absolute path}
- ODH GitOps: {absolute path}
- ADRs: {absolute path}
- Constitution: {absolute path to ${CLAUDE_SKILL_DIR}/resources/constitution.md}
- Assessment Rules: {absolute path to ${CLAUDE_SKILL_DIR}/resources/rules/}

## Persona Routing and Finding Ownership

Each persona has primary ownership over specific finding categories. See the constitution's Finding Ownership Protocol for the full rules.

| Finding Category | Primary Owner | Cross-Ref Permitted |
|---|---|---|
| Pod restart / disruption windows | SRE | Engineer (necessity only) |
| Endpoint disruption / connection drops | SRE | Architect (integration pattern) |
| Controller downtime windows | SRE | (none) |
| Rollback risk / PDB / HPA resilience | SRE | (none) |
| CRD schema / API version changes | Engineer | SRE (if disruption), Architect (if integration) |
| Restart root-cause (necessary/avoidable) | Engineer | (SRE consumes classification) |
| Migration tooling (odh-cli) gaps | Engineer | (others cite check IDs only) |
| Cross-component API impact | Engineer | Architect (integration topology) |
| Auth architecture shift / Gateway API | Engineer | SRE (endpoint disruption), Architect (integration) |
| Component removal / addition topology | Architect | Engineer (CRD cleanup), SRE (blast radius), Admin (prerequisites) |
| Architecture / integration topology | Architect | Engineer (API-level), SRE (endpoint disruption) |
| Custom configuration risk | Architect | (none) |
| Resource quota feasibility | Architect | Admin (prerequisite checklist) |
| ADR alignment | Architect | Engineer (implementation) |
| OLM path, OCP compat, backup | Admin | (none) |
| Dependency operator management | Admin | Architect (integration risk) |
| Pre-upgrade checklist / downtime estimation | Admin | SRE (data-plane windows) |

Primary data sources per persona:
- **SRE**: Deployment Inventory, Endpoint Inventory, Workload Types, Restart Triggers
- **Admin**: Component Diff, Dependency Operator Inventory, OLM health
- **Engineer**: CRD Inventory, Webhook Inventory, Restart Triggers, Interaction Map, odh-cli Check Registry
- **Architect**: All inventories, Interaction Map, Dependency Operator Inventory

Personas produce full findings ONLY for their primary ownership categories. For cross-reference categories, they emit a one-line XREF. For unlisted categories, they skip entirely.
```

Print the run directory path and a summary:
- Run ID
- Source and target versions
- Component counts (source, target, added, removed, changed)
- Path to `context-draft.md` and `discrepancies.yaml`

---

## Phase 2: Verify and Cross-Validate CRDs

**Inputs**: `{run_dir}/context-draft.md`, `{run_dir}/discrepancies.yaml`
**Outputs**: updated `{run_dir}/context-draft.md` (CRD Inventory gains Conversion and Schema Delta columns), updated `{run_dir}/discrepancies.yaml`

This is where judgment happens. Start with a clean context focused on "here are the CRDs to check."

### Anomaly Detection — Smell Test

Scan the CRD Inventory in `context-draft.md` for changes that look anomalous. The following patterns are **red flags** that require mandatory verification against the actual component repo before being included in context.md. Do not include them as confirmed changes until verified.

**Red flags** (verify before reporting):
- **Scope change** (Namespaced↔Cluster) — CRD scope changes are extremely rare and breaking at the Kubernetes level. Almost always a doc error rather than a real change. Verify against the actual CRD YAML in the component repo for both source and target versions.
- **Served version dropped** — a version that was served in source is no longer served in target. This is potentially breaking for consumers of that version. Verify against the repo. **Do not confuse with multi-version CRDs**: if source serves `v1` and target serves `v1, v1alpha1`, that's a version *added* (not a red flag). If both source and target serve `v1, v1alpha1`, the change status is `none`. Only flag when a previously-served version disappears from the target.
- **CRD removal for actively-used kinds** — if a CRD kind that users create (InferenceService, Notebook, etc.) shows as removed, verify it's not a PLATFORM.md omission. Controller-managed CRDs can be removed (they get recreated), but user-facing CRDs should never silently disappear.
- **API group migration** — a kind moving from one API group to another (e.g., serving.kserve.io → inference.networking.k8s.io) is unusual. Verify the old-group CRD is actually gone and the new-group CRD exists in the component repo.
- **Incomplete version data** — when a doc lists only one version for a CRD but another doc or PLATFORM.md lists additional versions for the same group+kind, the single-version doc may be incomplete. Apply precedence rules to resolve, and verify against the repo if inconsistency persists.

### CRD Version Compatibility Verification

For every CRD in the CRD Inventory with change status `versions-changed`, `storage-version-changed`, or `scope-changed`, and for any CRD where the architecture doc and PLATFORM.md disagree on versions or scope, verify the actual CRD definition from the component repo.

1. **Clone the component repo** at the correct branch. Read the architecture doc's metadata section (top of file) for the exact repository URL and branch. Use those coordinates — do not guess branch names. Clone following **CLAUDE.md → Repository cloning and refs**.

   When architecture-context data disagrees with what you find in the actual repo, **the repo wins**. Flag the discrepancy.

   **Read the actual kustomize overlay used on OpenShift**: the rhods-operator deploys components from prefetched manifests. The overlay path is defined in the rhods-operator's Go controller code — not in the component repo. Controller code structure varies per component (there is no fixed naming convention). To find the correct overlay:
   1. Clone the rhods-operator at the target branch (repo and branch from the rhods-operator architecture doc metadata)
   2. Search `internal/controller/components/{component}/` for Go files containing manifest path constants or kustomize overlay references (e.g., `grep -r 'Path\|overlay\|manifests' internal/controller/components/{component}/`)
   3. Clone the component repo and read the overlay at the path found in step 2

   The overlay patches upstream configurations for the RHOAI distribution (e.g., replacing cert-manager with OpenShift service-ca, removing upstream-only resources). The overlay is authoritative.

2. **Extract all served versions**: read the CRD YAML and extract the full `spec.versions[]` array. For each entry, record `name`, `served` (true/false), and `storage` (true/false). The served versions in the CRD YAML are authoritative — architecture docs may list only a subset. If the doc listed one version but the CRD YAML shows multiple served versions, update the inventory to include all served versions and file a discrepancy (doc is incomplete).

3. **Check conversion strategy**: read the CRD YAML and extract `spec.conversion.strategy`.
   - `Webhook` — verify the referenced webhook Service and caBundle/cert-manager annotation exist. Record: `conversion: webhook, service: {name}`.
   - `None` (or absent) — Kubernetes round-trips between versions. If the CRD serves multiple versions with `conversion: None`, all versions share the same internal representation. Record: `conversion: none (round-trip)`.

4. **Diff OpenAPI schemas**: for each pair of served versions within the same CRD, and across source/target for the same version, extract `spec.versions[].schema.openAPIV3Schema` and compare the top-level `.spec` properties:
   - **Identical schemas** → Record: `schema: identical`.
   - **Additive only** → Record: `schema: additive — {list of new fields}`.
   - **Breaking** → Record: `schema: breaking — {list of breaking changes}`.

5. **Compare version sets between source and target**: diff the complete served version sets, not just a single version string:
   - Versions added to served set (present in target, absent in source)
   - Versions removed from served set (present in source, absent in target)
   - Storage version changes (same versions but `storage: true` moved to a different version)

6. **Update the CRD Inventory** in `context-draft.md` with the verified data: fill in the `Conversion` and `Schema Delta` columns.

7. **Report discrepancies**: if the repo contradicts the architecture-context doc, record per **Discrepancy Reporting** above.

8. **Keep component repo clones** for reuse by persona subagents. Do not remove them. Record cloned repo paths in the Source Map. If a clone already exists, update it (`git fetch && git checkout <branch> && git pull --ff-only`) instead of re-cloning.

If the component repo cannot be cloned (private, missing branch), mark both columns as `unverified`.

**Verification procedure for red flags** (mandatory — cannot be resolved by reading architecture docs alone):
1. Clone the component repo at both source and target branches (use architecture doc metadata for repo URL and branch)
2. Read the actual CRD YAML files and compare — this is the only authoritative source
3. If using kustomize overlays, render with `kustomize build | yq` (or python fallback) to see the final output
4. Record the verification result in the CRD Inventory notes and in `discrepancies.yaml`

**The `evidence_ref` for any red-flag discrepancy MUST point to the actual repo file** (e.g., `red-hat-data-services/mlflow-operator@rhoai-3.3:config/crd/bases/mlflow.opendatahub.io_mlflows.yaml#L15`), never to another architecture-context doc. Architecture docs cannot verify architecture docs — only code can. If you resolve a red flag without cloning the repo, the discrepancy is unverified and must be flagged as such.

**Always record the verification result in `discrepancies.yaml`** — whether the change is real or a doc error:
- **Doc error**: correct the inventory to `none` and file a discrepancy with the repo file as `evidence_ref`.
- **Real change confirmed by code**: include it in the inventory with `verified against {repo}@{branch}:{path}#L{line}` and file a discrepancy entry documenting the verification (claim = what the doc said, evidence = what the code confirmed, resolution = confirmed as real change).

### Dependency Operator Disruption Analysis

For each dependency operator with a version change:
1. Extract versions from odh-gitops Subscription specs for source and target
2. Changelog lookup (web search, upstream repo, architecture-context)
3. Classify disruption: API changes, behavioral changes, CRD schema changes, known issues
4. When changelog cannot be retrieved, mark as `Risk: unknown — changelog not available`

Update the Dependency Operator Inventory in `context-draft.md` with Breaking Changes, Behavioral Changes, and Risk columns.

Print a summary: CRDs verified, discrepancies found, red flags resolved.

---

## Phase 3: Build Derived Data and Finalize context.md

**Inputs**: verified `{run_dir}/context-draft.md`, `{run_dir}/discrepancies.yaml`
**Outputs**: `{run_dir}/context.md` (renamed from `context-draft.md`)

This phase consumes verified inventories and produces derived analysis.

### Cross-Component Interaction Map

Build an interaction map capturing how components depend on each other.

**Explicit interactions** (from architecture-context when documented):
- CRD consumers, service dependencies, shared infrastructure, webhook interactions

**Inferred interactions**:
- CRD cross-reference, namespace overlap, API group consumption, common dependency convergence

Each interaction is tagged as `explicit` or `inferred`.

### Accidental Data-Plane Restart Detection

For each component present in both source and target, identify pod template changes that would trigger rolling restarts. Only include data-plane components (inference runtimes, training jobs, notebooks, pipeline steps, guardrails pods serving traffic). Control-plane operator/controller pod restarts are expected during upgrades and should not be listed.

### odh-cli Check Registry

Read `.context/repos/odh-cli/pkg/lint/checks/` to enumerate all registered checks.

- **Pre-upgrade (upgrade linter)**: **primary focus** — checks that validate the source version state before upgrading.
- **Post-upgrade (version lint)**: informational — include for completeness but do not treat missing coverage as a gap.

**Version gate analysis**: for each pre-upgrade check, determine whether it is gated behind a version predicate (e.g., `IsUpgradeFrom2xTo3x()`) that would prevent it from firing for the current source→target transition. Record the gate.

| Category | Subdirectories |
|----------|---------------|
| components | dashboard, datasciencepipelines, kserve, kueue, modelmesh, ray, trainingoperator |
| platform | datasciencecluster, dscinitialization |
| workloads | datasciencepipelines, guardrails, kserve, kueue, llamastack, notebook, ray, trainingoperator |
| dependencies | certmanager, openshift, servicemesh |
| kueue | discovery |

For each check directory, read the Go files to extract check IDs, descriptions, mode, and version gate.

### Finalize context.md

Add the following sections to `context-draft.md`:
- Cross-Component Interaction Map
- Potential Accidental Restart Triggers
- odh-cli Check Registry (Pre-Upgrade and Post-Upgrade)
- Reference Paths
- Persona Routing and Finding Ownership

Rename `context-draft.md` to `context.md`.

Print the final summary:
- Run ID
- Source and target versions
- Component counts (source, target, added, removed, changed)
- Number of CRDs with changes
- Number of discrepancies found
- Path to `context.md` and `discrepancies.yaml`
