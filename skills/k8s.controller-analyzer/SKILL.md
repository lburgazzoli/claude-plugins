---
name: k8s.controller-analyzer
description: Validate that the k8s-controller-analyzer tool and the k8s.controller-* assessment skills are in sync — every checklist item has a corresponding extractor, and every extractor maps to a checklist item.
---

# Controller Analyzer ↔ Skill Sync Validation

Validate that the `k8s-controller-analyzer` Go tool and the `k8s.controller-*` assessment skills are consistent. Every skill checklist item should have a corresponding analyzer fact kind and extractor, and every extractor rule ID should map to a checklist item in a skill.

## Inputs

- `$ARGUMENTS` is ignored. This skill always operates on the plugin repository itself.

## Source Files

Read all of the following files before performing any checks:

### Analyzer sources
- `references/analyzer-output-schema.md` — normative analyzer JSON input contract for skills
- `tools/k8s-controller-analyzer/EXTRACTORS.md` — the living contract between rules and extractors
- `tools/k8s-controller-analyzer/pkg/extractor/consts.go` — all rule ID and fact kind constants
- `tools/k8s-controller-analyzer/pkg/extractor/types.go` — all data types for extracted facts
- `tools/k8s-controller-analyzer/pkg/output/json.go` — analyzer report envelope and schema version

### Skill sources
- `skills/k8s.controller-architecture/SKILL.md` — architecture assessment checklist and fact-to-checklist mapping
- `skills/k8s.controller-api/SKILL.md` — API conventions assessment checklist and fact-to-checklist mapping
- `skills/k8s.controller-production-readiness/SKILL.md` — production readiness assessment checklist and fact-to-checklist mapping
- `skills/k8s.controller-assessment/SKILL.md` — orchestrator flow and analyzer fan-out contract

## Checks

Perform each check in order. For each check, report `pass`, `finding`, or `not-observed`.

### 1. Rule ID coverage

For every rule ID constant defined in `consts.go`:
- It MUST appear in at least one row of `EXTRACTORS.md`
- It MUST be referenced in the fact-to-checklist mapping table of at least one skill SKILL.md

**Finding**: a rule ID exists in code but is not documented in EXTRACTORS.md or not referenced by any skill.

### 2. Checklist item coverage

For every checklist item in each skill SKILL.md (e.g., 1a, 2b, 3c):
- It MUST appear in the skill's fact-to-checklist mapping table
- The fact kinds referenced in that mapping MUST exist as constants in `consts.go`
- The fact kinds MUST have corresponding data types in `types.go`

**Finding**: a checklist item has no fact mapping, or references a fact kind that doesn't exist in the analyzer.

### 3. Fact kind coverage

For every fact kind constant in `consts.go`:
- It MUST have a corresponding data type in `types.go`
- It MUST appear in at least one extractor's output (documented in EXTRACTORS.md)
- It MUST be referenced by at least one skill's fact-to-checklist mapping

**Finding**: a fact kind is defined but orphaned — not used by any extractor or skill.

### 4. EXTRACTORS.md consistency

For every row in EXTRACTORS.md:
- The rule ID MUST exist as a constant in `consts.go`
- The extractor file referenced MUST exist under `pkg/extractor/`
- The fields listed MUST exist as struct fields in the corresponding data type in `types.go`

**Finding**: EXTRACTORS.md references a rule, file, or field that doesn't exist.

### 5. Skill analyzer section consistency

For each leaf skill (architecture, api, production-readiness):
- The `## Static Analyzer` section MUST exist
- It MUST reference `references/analyzer-output-schema.md` as the analyzer input contract
- It MUST reference the correct `--skill` flag value
- The `## gopls Verification Protocol` section MUST exist
- The `## Deterministic Procedure` MUST reference the analyzer as step 2

**Finding**: a skill is missing the analyzer integration sections or references the wrong skill flag.

For the orchestrator skill (`k8s.controller-assessment`):
- It MUST reference `references/analyzer-output-schema.md` as the analyzer input contract
- It MUST state that the analyzer is run once before child skills
- It MUST state that child skills consume the already-loaded analyzer JSON

**Finding**: the orchestrator skill does not accurately describe the shared analyzer-input contract.

### 6. YAML extractor coverage

For each YAML fact kind (`rbac_manifest`, `crd_manifest`, `webhook_manifest`, `deployment_manifest`, `networkpolicy_manifest`):
- A corresponding `yaml_*.go` extractor file MUST exist under `pkg/extractor/`
- The fact kind MUST be referenced in the appropriate skill's fact-to-checklist mapping

**Finding**: a YAML extractor exists but no skill references it, or a skill references a YAML fact that has no extractor.

### 7. Manifest builder coverage

For each valid `--skill` value (`architecture`, `api`, `production-readiness`):
- The manifest builder in `manifest.go` MUST produce categories matching the skill's expected evidence categories
- Run: `<plugin-root>/scripts/k8s-controller-analyzer.sh tools/k8s-controller-analyzer/testdata/simple-operator --skill <skill> --format json`
- Verify the output has a `manifest` section with `count > 0` and a 12-character `hash`

**Finding**: manifest builder produces empty or malformed output for a valid skill.

### 8. Analyzer schema contract consistency

Use `references/analyzer-output-schema.md` as the canonical analyzer-input reference and verify:

- The documented report envelope matches `pkg/output/json.go` (`schema_version`, `repo_path`, `extracted_at`, optional `manifest`, `facts`)
- The fact envelope documents `rules`, `kind`, `file`, `line`, and `data`
- The contract states that `rules` is always an array in schema `v3`
- The controller payload documents nested `reconciles.{group,version,kind}`
- The RBAC guidance is manifest-first (`rbac_manifest` primary, `rbac_markers` secondary)

**Finding**: the shared analyzer schema reference is stale or contradicts the actual analyzer output contract.

## Output Format

```markdown
# Analyzer ↔ Skill Sync Report

## Summary

| Check | Status |
|-------|--------|
| Rule ID coverage | pass/finding |
| Checklist item coverage | pass/finding |
| Fact kind coverage | pass/finding |
| EXTRACTORS.md consistency | pass/finding |
| Skill analyzer sections | pass/finding |
| YAML extractor coverage | pass/finding |
| Manifest builder coverage | pass/finding |
| Analyzer schema contract consistency | pass/finding |

## Findings

For each finding:
- **Check**: which check failed
- **What**: what is missing or inconsistent
- **Where**: file paths and line references
- **Fix**: concrete action to resolve

## Recommendations

If any checks produce findings, list the recommended changes in priority order:
1. Missing extractors (add code)
2. Missing skill mappings (update SKILL.md)
3. Stale analyzer schema references (update `references/analyzer-output-schema.md` and affected skills)
4. Stale EXTRACTORS.md rows (update documentation)
5. Orphaned constants (remove dead code)
```
