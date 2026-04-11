---
name: k8s.controller-production-readiness
description: Evaluate a Kubernetes controller's production readiness by reviewing test coverage, observability (events, logs, metrics), and operational maturity.
---

# Kubernetes Controller Production Readiness Assessment

Evaluate a Kubernetes controller's production readiness focusing on test coverage and observability.
This skill is primarily designed for Go controllers built with `controller-runtime`/kubebuilder patterns.

## Inputs

- If `$ARGUMENTS` is provided, treat it as scope (files, package, controller name, assessment focus, or GitHub repository).
- GitHub repository inputs may be full URLs (for example, `https://github.com/org/repo`) or shorthand (`org/repo`).
- If `$ARGUMENTS` is a GitHub repository, use that repository as the primary scope source and do not apply local `git diff` defaults.
- If no arguments are provided, assess current repository changes from git diff.
- If there are no changes, start with controller packages and test directories first; expand to the full codebase only when needed for evidence.
- If the project has no controller/operator implementation assets in scope (for example, no controller reconciler code, no relevant tests, and no operator deployment/runtime manifests), skip this skill and report `Not applicable`.
- `--details` includes a full breakdown of each finding (Why, Fix, metadata) after the summary tables. Without this flag, only the summary tables are produced.

## Input Validation

The only recognized flag is `--details`. If `$ARGUMENTS` contains any unrecognized `--<flag>`, stop before running the assessment and ask the user to confirm whether the flag is intentional or a typo.
When this skill is invoked by `/k8s.controller-assessment`, it should receive only scope text and optional `--details` (orchestration flags such as `--scope` are not valid here).

## References

Consult [k8s-upstream.md](../../references/k8s-upstream.md) for the authoritative source of conventions and high-quality reference implementations.
Consult [validation-output-schema.md](../../references/validation-output-schema.md) for the canonical finding, evidence, highlight, and validation model used by validation-style skills.

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

Recommend policy or lint tooling such as [kube-linter](https://github.com/stackrox/kube-linter) when the project ships deployment manifests. Treat the following checks as operational-readiness and deployment-hardening signals; severity should depend on the target environment and delivery model rather than being treated as direct proof of controller correctness

- **Resource Management**: CPU and memory requests and limits are defined for the controller manager deployment to prevent OOMKills or node exhaustion
- **Security Context**: Controller pods should generally trend toward Restricted Pod Security Standards (`runAsNonRoot: true`, `allowPrivilegeEscalation: false`). Treat `readOnlyRootFilesystem: true` as a strong recommendation when the image and runtime layout support it
- **Health Probes**: Liveness (`/healthz`) and readiness (`/readyz`) probes are configured in the deployment manifest and correctly implemented in `main.go`
- **Network Policies**: If the controller exposes network endpoints (metrics, webhooks, health probes) or communicates with external services, assess whether `NetworkPolicy` resources are expected for the target environment. Their absence is a stronger finding in hardened or multi-tenant clusters than in permissive internal environments

### 4. OpenShift TLS Configuration Compliance

> Only applicable when the controller/operator targets OpenShift. Skip this section if the controller is platform-agnostic or targets vanilla Kubernetes only.

For OpenShift-targeted operators, verify current platform guidance before treating TLS configuration as a hard requirement. Favor findings that are anchored in current OpenShift documentation over version-specific or time-sensitive assumptions. If the exact OpenShift requirement cannot be confirmed from the references available in scope, mark the check as `Not verified` and do not describe it as a release blocker.

- **No hardcoded TLS configuration**: No local or hardcoded server-side TLS protocol versions, cipher suites, or curves in the codebase or deployment manifests unless the deviation is intentional, configurable, and documented
- **Central TLS source**: When current OpenShift guidance requires platform-managed TLS, the component fetches its TLS server settings from the appropriate central configuration source instead of relying on local defaults:
  - **API Server configuration** — default for most components
  - **Kubelet configuration** — for components running on nodes
  - **Ingress configuration** — for components serving ingress traffic
- **Explicit TLS profile respect**: When the component is expected to honor the platform TLS profile, it explicitly applies the relevant settings instead of silently relying on Go `crypto/tls` defaults
- **Custom TLS profiles**: The component correctly handles customer-defined TLS profiles where platform-managed TLS is part of the product contract, not just the named presets (Old, Intermediate, Modern)
- **Managed operands**: When the operator configures server TLS for managed operands or exposes TLS-related CR fields, verify those settings inherit from the intended cluster or operator-level source; do not assume every managed CR must expose its own TLS knobs
- **Server-side scope**: This section applies to TLS server settings exposed by the operator, its webhooks, or managed operands. Client TLS settings are not in scope
- **Opt-out with documentation**: If the component has a legitimate reason to deviate from the cluster-wide TLS default, it should expose its own configuration knob that defaults to the cluster-wide config, and the deviation should be clearly documented

Severity mapping:
- Hardcoded TLS config or ignoring a required central TLS source when current OpenShift guidance clearly applies: `must` (Critical)
- Not handling custom TLS profiles where platform-managed TLS is required: `should` (Major)
- Missing documentation for an intentional opt-out deviation: `contextual` (Minor)

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
7. **Adversarial validation**: Launch a clean-context validator subagent with the draft findings, draft highlights, and scope. See [Leaf Validator Subagent](#leaf-validator-subagent) for the validator brief and isolation rules.
8. Apply validation results: update severities, remove dismissed findings, adjust or remove highlights per validator verdicts. Validation **always runs** and its results are applied before scoring regardless of flags. The detailed Validation output section is only included in the report when `--details` is passed.
9. Generate output with severity, concrete fix, confidence, and any unverified assumptions.

## Leaf Validator Subagent

After the primary analysis produces draft findings and draft highlights, launch a **separate subagent** to adversarially validate the results. The validator operates with a clean context — it does not receive the primary reviewer's reasoning or intermediate notes.

### Isolation rules

- Run in a separate subagent
- Receive only: the scope (from `$ARGUMENTS` or resolved defaults), draft findings, and draft highlights
- Re-read code independently from the evidence locations in each finding's `where` field
- Do not rely on the primary reviewer's internal reasoning

### Validator brief

> **Role**: You are a skeptical reviewer. Your job is to challenge each finding from a production readiness assessment and determine whether it actually impacts runtime behavior, correctness, or operational safety. You also verify that positive highlights do not contradict the findings.
>
> **Inputs you receive**:
> - The draft findings list (each following the canonical finding model from `validation-output-schema.md`)
> - The draft positive highlights list (each with: `id`, `description`)
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
> Do **not** produce new findings. Your role is to validate, not to review.

### Applying validation results

1. Update each finding's severity to the `validatedSeverity`.
2. Remove any finding with `validatedSeverity: dismissed`.
3. Remove any highlight with `verdict: remove`.
4. Replace any highlight with `verdict: reword` with the validator's `suggestedRewording`.
5. Keep downgrade and dismissal rationale for the `validation` results. Do not append validator provenance into the finding's `notVerified` field.
6. Recompute severity counts using the validated severities.

## Scoring

1. Start at 100.
2. For each finding, subtract points based on severity:
   - Critical: **-20** per finding
   - Major: **-10** per finding
   - Minor: **-3** per finding
3. Floor score at 0.

Interpretation:

- **90-100**: Production-ready with minor polish
- **75-89**: Solid baseline, a few important gaps
- **50-74**: Significant issues to address before production
- **<50**: High operational risk; major redesign/fixes recommended

## Output Format

Produce the assessment in this format. All sections are always included unless noted otherwise.
Use the canonical report, finding, highlight, and validation model from [validation-output-schema.md](../../references/validation-output-schema.md).

Output conventions:

- `scope` should follow the shared URI-like form when expressed structurally (for example, `diff://working-tree`, `repo://org/repo`, `controller://MyReconciler`)
- `where` should use repo-relative GitHub-style location string(s) (for example, `controllers/myresource_controller.go#L118-L146`)
- Use the shared `notVerified` concept consistently; render it in Markdown as `Not verified`

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

### Positive Highlights

Things the implementation does well, patterns worth preserving or replicating.
