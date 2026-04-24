---
name: k8s.controller-lifecycle
description: Assess a Kubernetes controller's operational lifecycle — leader election, graceful shutdown, webhook certificate provisioning, and CRD upgrade safety.
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

# Kubernetes Controller Lifecycle Assessment

Assess the operational lifecycle of a Kubernetes controller focusing on leader election correctness, graceful shutdown behavior, webhook certificate provisioning, and CRD upgrade safety.
This skill is primarily designed for Go controllers built with `controller-runtime` and kubebuilder patterns.

## References

Consult [upstream-conventions.md](${CLAUDE_SKILL_DIR}/../../references/k8s-controller/upstream-conventions.md) for upstream conventions.
Consult [analyzer-schema.md](${CLAUDE_SKILL_DIR}/../../references/k8s-controller/analyzer-schema.md) for the analyzer JSON input contract.
Consult [report-schema.md](${CLAUDE_SKILL_DIR}/../../references/k8s-controller/report-schema.md) for the canonical report model.
Consult [deterministic-execution.md](${CLAUDE_SKILL_DIR}/../../references/k8s-controller/deterministic-execution.md) for deterministic execution rules.

## Inputs

- `$ARGUMENTS` may contain:
  - scope text such as files, packages, controller names, or GitHub repositories
  - `--mode=deterministic`
  - `--mode=exploratory`
- Default mode is `--mode=deterministic`.
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source.
- If no controller implementation assets or manager configuration are found in the resolved scope, return `Not applicable`.

## Input Validation

- The only recognized flags are `--mode=deterministic` and `--mode=exploratory`.
- If `$ARGUMENTS` contains any other `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.

## Static Analyzer

In deterministic mode, build and run the static analyzer to extract structured facts from Go code and YAML manifests. This is the single evidence source.

Treat [analyzer-schema.md](${CLAUDE_SKILL_DIR}/../../references/k8s-controller/analyzer-schema.md) as the normative schema for the analyzer JSON envelope and fact payloads.

### Run

Use the `analyze_controller` MCP tool with `repo_path` set to the repository root and `skill` set to `lifecycle`.

If the orchestrator (`k8s.controller-assessment`) has already run the analyzer and the JSON is loaded in context, skip the run step.

### Load and use

Load the full JSON output into context. The output includes:

- A `manifest` section with `count`, `hash` (MANIFEST_HASH), and categorized file `entries` — include `manifest.hash` verbatim in the report for auditability
- A `facts` array with all extracted evidence

If `manifest.count` is 0, return `Not applicable`.

The analyzer provides facts relevant to this skill:

- `manager_config` — leader election settings, signal handler detection, graceful shutdown timeout
- `deployment_manifest` — deployment configuration
- `webhook_manifest` — webhook presence (implies certificate lifecycle concern)
- `cert_provisioning` — detected certificate provisioning signals (cert-manager, OpenShift service-ca, CertDir)
- `crd_manifest` — multi-version serving, conversion strategy, deprecated versions

### Fact-to-checklist mapping

| Analyzer Facts | Checklist Items |
|----------------|-----------------|
| `manager_config.leader_election` + `manager_config.leader_election_id` + `manager_config.leader_election_resource_lock` | 1a-c (Leader election) |
| `manager_config.leader_election_release_on_cancel` | 1d (Leader release) |
| `manager_config.has_signal_handler` | 2a (Signal handling) |
| `manager_config.graceful_shutdown_timeout` | 2b (Shutdown timeout) |
| `webhook_manifest` presence + `cert_provisioning` presence/absence | 3a (Webhook certificate) |
| `crd_manifest.has_multiple_served` + `crd_manifest.conversion_strategy` | 4a (Multi-version conversion) |
| `crd_manifest.versions[].deprecated` + `crd_manifest.versions[].served` | 4b (Deprecated migration) |
| `crd_manifest.has_multiple_served` + `crd_manifest.served_version_count` | 4c (Storage version migration) |

## gopls Verification Protocol

Use gopls MCP tools to verify and enrich the static analyzer's structural findings. These verification steps are **mandatory** in deterministic mode.

If the `gopls-lsp` MCP server is unavailable, use the built-in `LSP` tool with gopls as a fallback:

| MCP tool | LSP equivalent |
|----------|---------------|
| `go_file_context` | `documentSymbol` on the file, then `hover` on specific symbols |
| `go_symbol_references` | `findReferences` at the symbol's position |
| `go_search` | `workspaceSymbol` with the symbol name |
| `go_package_api` | `documentSymbol` on the package's Go files |

### Leader election verification (checklist 1a-d)

If `manager_config` facts exist:

1. Call `go_file_context` on the main/cmd file containing `ctrl.NewManager()` to confirm leader election options
2. If `leader_election` is false or absent, check whether the controller is designed for single-replica operation (DaemonSet pattern, node-scoped controller)

### Context propagation verification (checklist 2a)

1. Call `go_file_context` on the main file to verify `ctrl.SetupSignalHandler()` or equivalent is used
2. For controllers with external API calls in `api_calls`, use `go_file_context` on the controller file to check whether context is passed to external calls

## Assessment Areas

Use area names exactly as written in the section headings below.

### 1. Leader Election and High Availability

- **1a. Leader election is enabled**
  - title: "Leader election not enabled"
  - finding: manager does not enable leader election, risking split-brain when multiple replicas run (`Critical`)
  - pass: `LeaderElection: true` is set in manager options, OR the controller is explicitly designed for single-replica or DaemonSet operation
  - not-observed: no manager configuration in scope

- **1b. Leader election ID is set**
  - title: "Leader election ID not set"
  - finding: leader election is enabled but `LeaderElectionID` is empty, using a default that may conflict with other controllers (`Major`)
  - pass: `LeaderElectionID` is set to a unique value
  - not-observed: leader election is not enabled

- **1c. Leader election uses Lease resource lock**
  - title: "Leader election uses deprecated resource lock"
  - finding: leader election uses deprecated ConfigMap or Endpoints lock type instead of Lease (`Major`)
  - pass: `LeaderElectionResourceLock` is "leases" or unset (controller-runtime defaults to "leases")
  - not-observed: leader election is not enabled

- **1d. Leader lease is released on cancel**
  - title: "Leader lease not released on cancel"
  - finding: `LeaderElectionReleaseOnCancel` is not set to true, causing the full lease duration to expire before failover during graceful shutdown (`Minor`)
  - pass: `LeaderElectionReleaseOnCancel` is true
  - not-observed: leader election is not enabled

### 2. Graceful Shutdown

- **2a. Signal handler is configured**
  - title: "Signal handler not configured"
  - finding: main/cmd does not use `ctrl.SetupSignalHandler()` or equivalent, preventing graceful shutdown on SIGTERM (`Major`)
  - pass: signal handler is set up and passed to manager.Start()
  - not-observed: no main/cmd files in scope

- **2b. Graceful shutdown timeout is configured**
  - title: "No explicit graceful shutdown timeout"
  - finding: no `GracefulShutdownTimeout` is set in manager options, relying on the default which may not be appropriate for controllers with long-running operations (`Minor`)
  - pass: `GracefulShutdownTimeout` is explicitly configured, OR the controller has no long-running operations
  - not-observed: no manager configuration in scope

### 3. Webhook Certificate Lifecycle

- **3a. Webhook certificate provisioning is visible**
  - title: "Webhook certificate provisioning not visible"
  - finding: admission webhooks are configured but no certificate provisioning mechanism is visible (no cert-manager annotations, no controller-runtime cert directory configuration, no OLM annotations) (`Major`)
  - pass: certificate provisioning is visible through cert-manager annotations, controller-runtime webhook.Server CertDir, or equivalent mechanism
  - not-observed: no admission webhooks in scope
  - evidence: if `webhook_manifest` facts exist, check for `cert_provisioning` facts. If any `cert_provisioning` fact exists (any mechanism) → pass. If none → finding.

### 4. CRD Upgrade Safety

- **4a. Multi-version CRDs have a conversion mechanism**
  - title: "Multi-version CRD served without conversion"
  - finding: CRD serves multiple versions but uses `None` conversion strategy, meaning the API server performs no field mapping between versions and will silently drop or zero fields that differ between versions (`Critical`)
  - pass: conversion strategy is `Webhook` or only one version is served, OR all served versions have identical schemas
  - not-observed: no multi-version CRDs in scope
  - evidence: use `crd_manifest.has_multiple_served` and `crd_manifest.conversion_strategy`
  - > **Ownership**: this skill owns the operational/upgrade angle (silent data loss). API skill 2c owns the design angle (strategy vs. schema diff). When both fire on the same CRD, the orchestrator merges with api as primarySource.

- **4b. Deprecated versions have a migration path**
  - title: "Deprecated CRD version served without migration guidance"
  - finding: a CRD version is marked deprecated but is still served, and no migration documentation or storage version migration tooling is visible in the repository (`Major`)
  - pass: deprecated versions have visible migration documentation, OR the deprecated version is no longer served
  - not-observed: no deprecated versions in scope
  - evidence: use `crd_manifest.versions[].deprecated` and `crd_manifest.versions[].served`
  - > **Deterministic mode**: assess only the coarse signal from `crd_manifest` facts (deprecated + still served). Migration documentation search requires exploratory mode with grep through docs, README, and changelog.

- **4c. Storage version change is coordinated**
  - title: "Storage version change without migration awareness"
  - finding: a CRD has multiple versions with one marked as storage, but neither the controller code nor the repository shows awareness of stored version migration (no references to `storedVersions`, storage migration, or version migration tooling) (`Major`)
  - pass: single-version CRD, OR the controller or repository contains storage migration awareness
  - not-observed: no multi-version CRDs in scope
  - > **Deterministic mode**: assess only the coarse signal from `crd_manifest` facts (`has_multiple_served`). Storage version migration awareness (storedVersions search, migration tooling) requires exploratory mode with grep.

## Anti-Findings

Do not emit findings for:

- Single-replica controllers that explicitly document they do not need leader election
- Controllers using DaemonSet deployment pattern (inherently single-instance per node)
- `GracefulShutdownTimeout` absence when the controller has no external API calls or long-running operations
- Single-version CRDs for any upgrade lifecycle items

## Deterministic Procedure

Run the assessment in this exact order:

1. Resolve scope from explicit scope text or `$ARGUMENTS`.
2. Run the `analyze_controller` MCP tool with `skill=lifecycle`. Load the full JSON output into context.
3. If `manifest.count` is 0, return `Not applicable`.
4. Run the gopls verification protocol for checklist areas 1 and 2 as specified above.
5. Walk every checklist item (1a through 4c) in order. For each item, record a disposition: `finding`, `pass`, or `not-observed` using the criteria defined above, the analyzer facts, and gopls verification results. Do not skip items.
6. Draft findings only from items with `finding` disposition. Every finding must trace to a specific checklist item ID (e.g., "1a"). Observations outside the checklist may appear in a `Notes` section but do not receive an ID, severity, or score impact.
7. Apply anti-finding rules to dismiss any violations.
8. Run one deterministic leaf validation pass in the same session:
   - dismiss findings unsupported by cited evidence
   - dismiss any anti-finding violations
   - lower severity only when the written criteria support the lower level
   - remove or reword highlights that contradict findings
9. Sort findings using the shared schema rules. Assign final IDs after sorting.
10. Compute score and severity counts.

Use area names exactly as written in the section headings above.

In deterministic mode do not:

- default to `git diff`
- expand scope beyond what the evidence manifest discovered
- launch subagents
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

Produce the assessment using the canonical model from [report-schema.md](${CLAUDE_SKILL_DIR}/../../references/k8s-controller/report-schema.md).

Output conventions:

- `scope` should use a URI-like string such as `repo://org/repo`, `dir://cmd`, or `controller://MyReconciler`
- `where` should use repo-relative GitHub-style location strings
- Sort findings by severity, area, `where`, and title before assigning final IDs

### Evidence Manifest

Paste the `manifest` section from the analyzer JSON output, including `count`, `hash`, and the categorized file entries. Do not edit, reformat, or summarize the output. This section makes the report auditable and verifiable.

### Summary

Write 2-3 sentences describing lifecycle and operational safety.

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
- **Checklist item**: the item ID (e.g., 1a)
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

#### Upgrade ordering highlight

If both `webhook_manifest` and `crd_manifest` facts exist, include this highlight:
> "This controller serves webhooks and CRDs. Upgrade ordering matters: deploy CRDs before webhook configs, and webhook configs before the controller Deployment, to avoid rejected requests during rollout."

This is informational — no score impact.

### Notes

Optional. Observations outside the checklist that may be useful but do not receive IDs, severity, or score impact.
