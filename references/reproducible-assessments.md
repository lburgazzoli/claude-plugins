# Reproducible Assessment Contract

Use this reference for validation-style skills that should behave deterministically by default.

## Default Mode

- Default mode is deterministic.
- Skills may offer `--mode=exploratory`, but exploratory behavior must be opt-in.
- Deterministic mode must not depend on subagent fan-out, completion order, or freeform "expand until satisfied" exploration.

## Recognized Flags

- All `k8s.controller-*` skills may accept:
  - `--mode=deterministic`
  - `--mode=exploratory`
- `k8s.controller-assessment` may additionally accept `--scope=<list>`.
- Any other `--flag` should stop the run and ask for confirmation.

## Scope Resolution

When no explicit scope is provided, deterministic mode must use a fixed directory order documented in the skill:

- Never default to `git diff`.
- Never widen scope opportunistically.
- If a documented directory does not exist, skip it and continue.
- If no relevant assets are found in the fixed scope set, return `Not applicable`.

When `$ARGUMENTS` is a GitHub repository URL or `owner/repo` reference:

- Clone or checkout the repository to a local path before assessment.
- All subsequent evidence collection operates on local files only.
- Normalize `scope` to `repo://owner/repo`.

When an explicit scope is provided:

- Use it as the primary scope.
- Do not add unrelated directories.
- Normalize the final scope to a URI-like string when possible.

## Evidence Sources

- All checks operate on the checked-out repository files.
- Do not execute code generators, tests, or external tools. Assess generated output by reading the committed artifacts.
- Plugin-local analysis tools (such as `k8s-controller-analyzer`) are permitted. These tools extract structured facts from the repository — they do not generate code, run tests, or modify the repository.
- Time-sensitive vendor guidance must come from version-pinned local references. If no local pinned reference exists, mark the check `Not verified`.

## Evidence Collection

- Build an evidence manifest: list all files in scope directories sorted alphabetically by full path. This is the reading order.
- Read every file in the manifest sequentially. Do not skip files, reorder reads, or revisit files.
- Separate evidence collection from analysis: read all files before evaluating any checklist items.
- Prefer repo-relative paths.
- Prefer exact line references when available.
- Do not use "search until enough evidence appears" behavior in deterministic mode.

## Checklist Disposition

After reading all evidence and before drafting findings, walk every checklist item in the skill's assessment areas in order. For each item, record one disposition:

- `finding`: evidence supports an issue
- `pass`: evidence shows compliance
- `not-observed`: no relevant evidence in scope

Every checklist item must receive a disposition. Do not skip items. This disposition pass is internal working and does not appear in the output.

## Finding Lifecycle

Use this sequence:

1. Build evidence manifest and read all files.
2. Walk every checklist item and record dispositions.
3. Draft findings only from items with `finding` disposition.
4. In deterministic mode, every finding must trace to a specific checklist item. Observations that do not correspond to any checklist item may appear in a `Notes` section but do not receive an ID, severity, or score impact.
5. Run one deterministic second pass over the drafted findings:
   - verify evidence still supports the claim
   - verify severity matches the decision table
   - remove contradictory highlights
6. Apply adjustments.
7. Sort findings.
8. Assign final IDs.
9. Compute scores.

The second pass may only keep, adjust, or dismiss findings. It must not introduce new findings.

## Area Names

Finding `area` values must use the exact heading string from the skill's assessment areas section. Do not paraphrase, abbreviate, or reformat area names.

## Confidence And Scoring

- A finding requires at least one `where` reference with a line range (`path#Lstart-Lend`) for `High` confidence.
- Findings with path-only `where` references are capped at `Medium` confidence.
- Findings with `confidence: Low` appear in the report but do not contribute to score deductions.

## Severity Decisions

- Severity must come from explicit criteria in the skill, not from freeform language such as "seems severe".
- If evidence supports two adjacent severities and the skill has no explicit tie-break rule, choose the lower severity.
- Anti-findings should be explicit and unconditional when their conditions match.

## Merge And Dedupe

Orchestrators should merge findings using a deterministic fingerprint:

- `area`
- normalized problem class: the checklist item `title` from the originating leaf skill, used verbatim without paraphrasing
- primary `where` path
- primary line range when available

Rules:

- If fingerprints match, merge.
- If fingerprints differ, keep findings separate.
- Do not dedupe on prose similarity alone.
- When merged severities differ, choose the higher severity.
- When merged severities tie, choose `primarySource` from a fixed priority list documented in the skill.

## Output Normalization

- Keep section order fixed.
- Sort findings and highlights using the shared schema rules.
- Keep score arithmetic explicit so the same finding set always yields the same score explanation.
- Render empty sections consistently.

## Prohibited Defaults In Deterministic Mode

- `git diff` as implicit scope
- parallel sub-skill execution
- validator subagents
- "expand only when needed"
- "when in doubt" tie-breaks without a concrete fallback rule
- executing code generators, tests, or external tools
