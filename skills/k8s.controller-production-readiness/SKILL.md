---
name: k8s.controller-production-readiness
description: Evaluate a Kubernetes controller's production readiness by reviewing test coverage, observability (events, logs, metrics), and operational maturity.
---

# Kubernetes Controller Production Readiness

Evaluate a Kubernetes controller's production readiness focusing on test coverage and observability.
This skill is primarily designed for Go controllers built with `controller-runtime`/kubebuilder patterns.

## Inputs

- If `$ARGUMENTS` is provided, treat it as scope (files, package, controller name, assessment focus, or GitHub repository).
- GitHub repository inputs may be full URLs (for example, `https://github.com/org/repo`) or shorthand (`org/repo`).
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source and do not apply local `git diff` defaults.
- If no arguments are provided, assess current repository changes from git diff.
- If there are no changes, start with controller packages and test directories first; expand to the full codebase only when needed for evidence.
- If the project has no controller/operator implementation assets in scope (for example, no controller reconciler code, no relevant tests, and no operator deployment/runtime manifests), skip this skill and report `Not applicable`.
- `--detail` includes a full breakdown of each finding (Why, Fix, metadata) after the summary tables. Without this flag, only the summary tables are produced.

## References

Consult [k8s-upstream.md](../../references/k8s-upstream.md) for the authoritative source of conventions and high-quality reference implementations.

## Assessment Areas

### 1. Test Coverage

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

### 2. Observability (Events, Logs, Metrics)

**Events:**
- Event recording is not strictly mandatory — do not flag its absence as a hard requirement, as excessive events become noisy
- Spot cases where events would aid debugging: significant decisions, state transitions, and error conditions are better surfaced as events than buried in logs, since events are visible via `kubectl describe`
- When events are used: `Normal` type for successful operations, `Warning` for failures; reasons are CamelCase, messages are human-readable
- Does not emit events on every reconciliation (only meaningful transitions)

**Logging:**
- Uses structured logging (`log.FromContext(ctx)` from controller-runtime, or `klog.InfoS`/`klog.ErrorS`)
- No unstructured logging (`klog.Infof`, `fmt.Printf`, `log.Printf`)
- Key names use lowerCamelCase (`podName`, `namespace`, not `pod_name`)
- Prefer consistent message style within the project (concise and stable wording) to reduce log noise and improve searchability — do not flag capitalization or wording choices that are internally consistent
- Uses `klog.KObj()` / `klog.KRef()` for Kubernetes object references in log values
- Appropriate verbosity levels:
  - V(0): Critical errors, startup info
  - V(1): Configuration, expected repeated errors
  - V(2): Default operational level — state changes, reconciliation events
  - V(4): Debug-level detail
- Libraries that log-and-return-error or log internally instead of returning errors are a code smell — flag as a warning, not a hard rule, since some cases (e.g., logging additional context before returning) may be intentional

**Metrics:**
- Exposes custom metrics for controller-specific business logic if applicable
- Uses prometheus client conventions (snake_case metric names, proper label cardinality)
- Includes reconciliation duration and error rate metrics where appropriate

### 3. Deployment Hardening

Recommend configuring [kube-linter](https://github.com/stackrox/kube-linter) (or equivalent policy/lint tooling) if not already in use. Verify the following checks are enabled and passing:

- **Resource Management**: CPU and memory requests and limits are defined for the controller manager deployment to prevent OOMKills or node exhaustion
- **Security Context**: Controller pod follows Restricted Pod Security Standards (`runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`)
- **Health Probes**: Liveness (`/healthz`) and readiness (`/readyz`) probes are configured in the deployment manifest and correctly implemented in `main.go`

### 4. OpenShift TLS Configuration Compliance

> Only applicable when the controller/operator targets OpenShift. Skip this section if the controller is platform-agnostic or targets vanilla Kubernetes only.

OpenShift requires all components to dynamically inherit TLS settings from the platform's central configuration rather than hardcoding them. This is a release blocker as of OCP 4.23/5.0 and is driven by Post-Quantum Cryptography (PQC) readiness — PQC-resilient algorithms (ML-KEM key encapsulation) are available only in TLS 1.3+, so components must follow the cluster-wide TLS profile to enable platform-wide PQC adoption in one pass.

- **No hardcoded TLS configuration**: No local or hardcoded TLS protocol versions, cipher suites, or curves in the codebase or deployment manifests
- **Central TLS source**: The component fetches its TLS server settings from one of the three platform configuration sources:
  - **API Server configuration** — default for most components
  - **Kubelet configuration** — for components running on nodes
  - **Ingress configuration** — for components serving ingress traffic
- **Explicit TLS profile respect**: The component explicitly applies all TLS profile settings; it must not rely on Go `crypto/tls` defaults
- **Custom TLS profiles**: The component correctly handles custom TLS profiles defined by customers, not just the named presets (Old, Intermediate, Modern)
- **Managed CRs**: TLS settings must apply to all CRs that the operator manages, not just the operator process itself
- **Server-side scope**: This requirement applies to TLS server settings only; client TLS settings are not in scope
- **Opt-out with documentation**: If the component has a legitimate reason to deviate from the cluster-wide TLS default, it must expose its own configuration knob that defaults to the cluster-wide config, and the deviation must be clearly documented

Severity mapping:
- Hardcoded TLS config or ignoring central TLS source: `must` (Critical)
- Not handling custom TLS profiles: `should` (Major)
- Missing documentation for opt-out deviations: `contextual` (Minor)

## Assessment Procedure

Use this repeatable workflow:

1. Determine scope from `$ARGUMENTS`, git diff, or targeted controller/test packages.
   - If `$ARGUMENTS` points to a GitHub repository, prioritize `controllers/`, test directories, and `main.go` as initial evidence sources.
2. If no controller/operator implementation assets are present in scope, stop and return `Not applicable`.
3. Collect evidence first (specific files and call sites), then classify issues by impact.
4. Mark each finding as `must`, `should`, or `contextual` based on production risk.
5. Map internal labels to report severities:
   - `must` -> `Critical`
   - `should` -> `Major`
   - `contextual` -> `Minor`
6. If runtime validation is unavailable (for example, tests cannot be executed or lint/policy tools are not runnable), perform static evidence review and mark those checks as `Not verified` with reduced confidence.
7. Generate output with severity, concrete fix, confidence, and any unverified assumptions.

## Scoring

1. Start at 100.
2. For each finding, subtract points based on severity:
   - Critical: **-20** per finding
   - Major: **-10** per finding
   - Minor: **-3** per finding
3. Floor score at 0.

## Output Format

Produce the assessment in this format. All sections are always included unless noted otherwise.

### Summary

2-3 sentences describing the overall production readiness of the controller.

Score table:

| Metric | Value |
|--------|-------|
| **Score** | 0-100 (or `Not applicable`) |
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
- Evidence rule: include at least one concrete file path and line reference for every Critical and Major finding.

### Findings (only with `--detail`)

This section is only included when the `--detail` flag is passed.

For each finding (numbered to match the summary table), produce:

#### _N_. _Finding title_

| | |
|---|---|
| **Severity** | Critical / Major / Minor |
| **Area** | Assessment area name |
| **Where** | File and line reference |
| **Confidence** | High / Medium / Low |
| **Not verified** | Any assumptions or runtime checks not validated (or `—`) |

**Why**: Explanation of why this is an issue.

**Fix**: Concrete suggested change.

---

### Positive Highlights

Things the implementation does well, patterns worth preserving or replicating.
