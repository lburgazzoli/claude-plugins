# Controller Exploration Contract

Use this reference for blind exploratory Kubernetes controller review. Unlike the
deterministic `k8s.controller-*` assessment skills, this contract is designed
for fresh-context investigation that starts from raw repository evidence rather
than analyzer output.

## Purpose

- Provide a clean exploratory path for controller review without analyzer bias
- Keep exploration useful and comparable to the existing review dimensions
- Preserve a clear boundary between exploratory output and deterministic scoring

## Blind-Start Rules

An exploratory controller agent following this contract must:

- start from repository files only
- not preload or consume `k8s-controller-analyzer` JSON
- not invoke `k8s.controller-assessment`
- not invoke analyzer-backed leaf skills as child reviews
- not treat prior deterministic findings as part of its runtime context

The agent may read the leaf skill files as policy references only:

- `skills/k8s.controller-architecture/SKILL.md`
- `skills/k8s.controller-api/SKILL.md`
- `skills/k8s.controller-production-readiness/SKILL.md`

When using those references, use them to borrow:

- dimension names
- area names
- checklist interpretation

Do not borrow:

- analyzer run steps
- analyzer-derived fact mappings
- deterministic scoring
- deterministic merge rules

## Inputs

- `$ARGUMENTS` may contain explicit scope text such as files, directories,
  controller names, packages, or GitHub repositories
- No flags are required for blind exploration. If unexpected `--flag` text is
  present, ask the user to confirm whether it is intentional

## Scope Resolution

When `$ARGUMENTS` contains a GitHub repository URL or `owner/repo` reference:

1. Clone or check out the repository locally first
2. Perform all exploration on local files only
3. Normalize `scope` to `repo://owner/repo` when possible

When explicit scope text is provided:

1. Use it as the primary scope
2. Prefer matching repository paths and symbols before widening outward
3. Do not add unrelated directories unless needed to understand dependencies or
   controller boundaries

When no explicit scope is provided:

1. Start at the repository root
2. Use the fixed discovery order below
3. Within each discovery bucket, inspect paths in alphabetical repo-relative
   order

## Fixed Discovery Order

Use this repo-local discovery order to establish clear initial context before
branching into deeper exploration:

1. Controller implementation paths
   - `controllers/`
   - `internal/controller/`
   - `pkg/controller/`
   - `operator/`
   - `cmd/`
   - `main.go`
   - files matching `*_controller.go`
2. API type and CRD paths
   - `api/`
   - `apis/`
   - `pkg/apis/`
   - `config/crd/`
   - `bundle/manifests/`
   - files matching `*_types.go`
3. Webhook paths
   - `webhooks/`
   - `internal/webhook/`
   - files matching `*_webhook.go`
4. Deployment and RBAC manifest paths
   - `config/manager/`
   - `config/rbac/`
   - `config/default/`
   - `deploy/`
   - `manifests/`
   - `bundle/`
5. Test paths
   - `test/`
   - `tests/`
   - `e2e/`
   - files matching `*_test.go`

If a listed path does not exist, skip it and continue. If no relevant assets are
found after the full discovery order, return `Not applicable`.

## Evidence Collection

Exploratory review is intentionally more flexible than deterministic assessment,
but it still needs clear guardrails:

- inspect code and manifests directly
- prefer repository evidence over general framework assumptions
- follow adjacent code paths when they materially affect the observation
- revisit files when needed; exploratory review is not bound to one-pass reading
- keep evidence repo-relative and line-specific when possible
- call out uncertainty explicitly instead of flattening it into confident claims

## Mapping Rules

Map each exploratory observation to one of these dimensions:

- `architecture`
- `api`
- `production-readiness`

When the match is strong enough, also map the observation to:

- the exact area heading used by the corresponding leaf skill
- the exact checklist item ID when the evidence clearly fits one checklist item

If the fit is only partial:

- keep the dimension
- omit the checklist item ID
- explain the uncertainty in the finding text or notes

## Output Contract

Exploratory output is not the same as the deterministic validation schema.
Render a Markdown report with these sections:

## Summary

- 2-3 sentences
- state that the review is exploratory and unscored

## Scope

- resolved scope string
- blind-start note confirming no analyzer input was used

## Exploratory Findings

For each finding include:

- `Title`
- `Dimension`: `architecture`, `api`, or `production-readiness`
- `Area`: exact leaf-skill area heading when available
- `Checklist item`: optional
- `Importance`: `High`, `Medium`, or `Low` as follow-up priority only, not
  canonical severity
- `Where`: repo-relative evidence locations
- `Confidence`: `High`, `Medium`, or `Low`
- `Why`
- `Suggested follow-up` or `Fix`
- `Validation status`: always state that the finding is exploratory and not
  analyzer-validated

## Open Questions

- unanswered questions that block stronger conclusions

## Evidence Trails

- short investigation trails such as "followed status writes from Reconcile into
  helper methods" or "traced webhook path from marker to implementation"

## Notes

- optional extra context that does not rise to a finding

## Separation From Deterministic Assessment

Blind exploration must remain separate from deterministic assessment:

- do not assign canonical scores
- do not recompute deterministic severity counts
- do not merge exploratory findings into deterministic findings automatically
- do not claim analyzer-backed validation

If the user later wants deterministic confirmation, that should be a separate
follow-up run with fresh context rather than a continuation of the same
exploratory session.
