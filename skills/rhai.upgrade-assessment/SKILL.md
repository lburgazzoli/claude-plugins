---
name: rhai.upgrade-assessment
description: Multi-persona upgrade impact assessment for RHOAI version transitions. Spawns four independent clean-context agent reviewers (SRE, admin, engineer, architect) to assess upgrade risks. Usage - /rhai.upgrade-assessment --source <version> --target <version> [--dry-run] [--scope static|runtime]
user-invocable: true
allowed-tools: Read, Write, Glob, Grep, Agent, Bash, WebSearch, WebFetch
---

# RHOAI Upgrade Assessment

Run independent, isolated persona assessments against a RHOAI version transition. Each persona has a different domain lens. No persona sees another's output. Disagreements are preserved, not harmonized.

**References** — read before starting:
- **`CLAUDE.md` (vault root)** — **Repository cloning and refs**
- `${CLAUDE_SKILL_DIR}/resources/constitution.md` — universal rules for all persona sub-skills

## Input

`$ARGUMENTS` contains named flags. Both `--source` and `--target` are required.

**Required flags**:
- `--source <version>` — RHOAI version to upgrade from (e.g., `v2.25`, `2.25`)
- `--target <version>` — RHOAI version to upgrade to (e.g., `v3.3.2`, `3.3`)

**Optional flags**:
- `--dry-run` — show what would be done without spawning personas
- `--scope static|runtime` — which phase to run (default: `static`)
  - `static` — build context, spawn persona assessments, produce findings + Runtime Verification Checklists. No cluster access needed.
  - `runtime` — requires a previous static run. Reads the Runtime Verification Checklists from all persona outputs in the run directory, spawns a verification agent to run them against the live cluster, and updates the final report with confirmed/refuted findings.
- `--personas sre,engineer,...` — comma-separated list of personas to run (default: all four: `sre,admin,engineer,architect`). Use to run a subset, e.g. `--personas engineer,architect`.

Examples:
- `/rhai.upgrade-assessment --source 2.25 --target 3.3`
- `/rhai.upgrade-assessment --source 3.3 --target 3.4 --dry-run`
- `/rhai.upgrade-assessment --source 2.25 --target 3.3 --scope runtime`
- `/rhai.upgrade-assessment --source 2.25 --target 3.3 --personas engineer,sre`

If `--source` or `--target` is missing, print usage and stop.

## Step 1: Parse Input

Parse `$ARGUMENTS` using **strict named-flag parsing**. Only `--flag value` pairs are accepted — positional arguments are not supported.

Extract:
- `--source` — strip leading `v`, normalize to `major.minor` (e.g., `2.25`). **Required.**
- `--target` — strip leading `v`, normalize to `major.minor` (e.g., `3.3`). **Required.**
- `--dry-run` — boolean, default false
- `--scope` — `static` (default) or `runtime`
- `--personas` — comma-separated list (default: `sre,admin,engineer,architect`)

Validate:
- **Reject positional arguments**: any token in `$ARGUMENTS` that is not a `--flag` or the value immediately following a known flag is an error. Print usage and stop. Do not silently interpret bare values as source/target.
- Both `--source` and `--target` are required. If either is missing, print usage and stop.
- Source must be less than target (no downgrades). EA pre-release suffixes (e.g., `3.4-ea.1`) sort before their release version (`3.4`).

On any validation failure, print:
```
Usage: /rhai.upgrade-assessment --source <version> --target <version> [--dry-run] [--scope static|runtime] [--personas sre,admin,engineer,architect]

Error: <specific problem>
```
Then stop. Do not proceed to Step 2.

**If `--scope runtime`**: skip Steps 2-4. Instead:
1. Find the most recent static run directory matching this version pair in `.context/tmp/upgrade-assessments/{source}-to-{target}-*/`. If none exists, print an error: "no static assessment found — run without `--scope runtime` first."
2. Read all persona output files (`sre.md`, `admin.md`, `engineer.md`, `architect.md`) from that run directory.
3. Extract the **Runtime Verification Checklist** from each persona output.
4. Consolidate all checklist items into a single verification task list.
5. For each checklist item, run the verification against the live cluster using `oc`/`kubectl` (follow `tools.kubectl` patterns). Record the result: `PASS`, `FAIL`, or `SKIPPED` (with reason).
6. Write the verification results to `{run_dir}/runtime-verification.md`.
7. Update `{run_dir}/report.md` with the runtime verification results appended after the Coverage Gaps section.
8. Print a summary: how many items passed, failed, skipped, and any findings that changed severity based on runtime evidence.

**If `--scope static`** (default): proceed with Steps 2-6 below.

## State Persistence

This skill is long-running (context build + parallel agents + verification + synthesis). To survive context compression, persist state at each step boundary:

```bash
# Initialize after creating the run directory (end of Step 2)
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py init {run_dir} \
    --source {source} --target {target} --scope {scope} --personas {personas}

# Update at each step transition
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} --step {N} --status {status}

# Read back if context is lost
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py read {run_dir}
```

If you lose track of the run directory after context compression, find it with `ls -td .context/tmp/upgrade-assessments/*/ | head -1`, then recover state with `python3 ${CLAUDE_SKILL_DIR}/scripts/state.py read {run_dir}`.

## Step 2: Build Context

Follow the instructions in `${CLAUDE_SKILL_DIR}/resources/prompts/context-builder.md` to build the shared context. This will:
- Resolve repos, clone version-specific branches
- Read architecture docs, cross-validate CRDs against actual component code
- Read odh-cli checks with version gate analysis
- Write `context.md` and `discrepancies.yaml` to the run directory

After the context build completes:
1. Verify `context.md` exists in the run directory (`.context/tmp/upgrade-assessments/{run_id}/`).
2. Initialize state persistence:
   ```bash
   python3 ${CLAUDE_SKILL_DIR}/scripts/state.py init {run_dir} \
       --source {source} --target {target} --scope {scope} --personas {personas}
   ```

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} --step 3 --status context_complete
```

## Step 3: Handle --dry-run

If `--dry-run` is set:
- Print the `context.md` content to the user
- Print which personas would run
- **Stop. Do not spawn personas.**

## Step 4: Spawn Personas

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} --step 4 --status spawning_personas
```

Invoke the selected personas (from `--personas`, default all four) in a **single message** via the Agent tool. Each persona runs as an independent subagent with a clean context — no access to the orchestrator's conversation history. This ensures each persona's assessment is independent and unbiased.

Model mapping:
- `sre`, `engineer`, `architect` → `model: "opus"`
- `admin` → `model: "sonnet"`

Invoke **all selected personas in a single message** so they run in parallel. Each agent is independent — no persona reads another's output, so there are no dependencies between them. Use one Agent tool call per persona, all in the same message:

```
Agent(
  description="RHOAI upgrade assessment — {persona} persona",
  model="{opus|sonnet}",
  prompt="You are the {persona} persona for an RHOAI upgrade assessment.

Read these files in this order — they contain your full instructions:
1. ${CLAUDE_SKILL_DIR}/resources/prompts/personas/{persona}.md — your role, checks, output format
2. ${CLAUDE_SKILL_DIR}/resources/constitution.md — universal rules
3. Applicable rules from ${CLAUDE_SKILL_DIR}/resources/rules/ (only those whose frontmatter `personas` list includes `{persona}`)
4. {run_dir}/context.md — the assessment context

Follow the instructions in your persona file. Write your output to {run_dir}/{persona}.md.
After completing your assessment and self-review, write structured metadata to {run_dir}/{persona}.yaml using the metadata CLI as described in the constitution.
Do not read or write any other files in the run directory."
)
```

## Step 5: Verify Unverified Claims

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} --step 5 --status verifying_claims
```

After all personas return, run the metadata CLI to find which personas have unverified claims:

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/metadata.py unverified {run_dir}
```

If any persona reports unverified claims, read the `## Unverified Claims` section from that persona's `.md` file to get the claim details. For each unverified claim, spawn a **focused verification subagent** via the Agent tool. The subagent's instructions are in `${CLAUDE_SKILL_DIR}/resources/prompts/verification-agent.md`.

```
Agent(
  description="Verify: {brief claim description}",
  prompt="Read ${CLAUDE_SKILL_DIR}/resources/prompts/verification-agent.md for your instructions, then verify:

claim: {the claim text}
context: {why verification is needed}
check_path: {the repo/path the persona provided}"
)
```

Spawn all verification subagents in a single message (parallel). After they return, update each persona's finding:
- **CONFIRMED**: promote from Unverified Claims to a numbered finding with HIGH confidence
- **REFUTED**: drop the claim entirely and note it in the report as "claim refuted by verification"
- **INCONCLUSIVE**: keep in Runtime Verification Checklist (requires cluster access)

Write verification results to `{run_dir}/verifications.md`.

**Update metadata after verification**: for each persona whose findings changed (confirmed claims added, refuted claims removed), re-run `metadata.py write` to update the `.yaml` sidecar with the corrected findings list and `--unverified_claims 0`. This ensures `synthesize.py` in Step 6 uses accurate data.

If the metadata CLI reports `none`, skip this step.

## Step 6: Synthesize

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} --step 6 --status synthesizing
```

### Phase A: Pre-compute report data

Verify that `context.md` still exists in the run directory. If it was deleted or overwritten, print an error and stop.

Run the synthesis script to pre-compute tables, aggregates, and cross-persona analysis from the structured metadata:

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/synthesize.py {run_dir} \
    --source {source} --target {target} \
    --personas {personas} --strict
```

This reads each persona's `.yaml` sidecar file and writes `{run_dir}/synthesis.yaml` with:
- Pre-rendered verdict table and persona assessments table
- Aggregate finding counts across all personas
- Blocking findings list
- Corroboration matches (XREFs matched to owning persona's findings)
- Disagreement detections (XREF severity differs from owner's finding severity)
- Missing personas list

If the script reports errors (missing `.yaml` files, validation failures), read the affected persona's `.md` file, extract the data, write the `.yaml` file via `metadata.py write`, then re-run synthesis.

### Phase B: Write report

Spawn a synthesis agent with a clean context to write the final report:

```
Agent(
  description="Synthesize upgrade assessment report",
  model="opus",
  prompt="You are writing the final synthesis report for an RHOAI upgrade assessment.

Read these files:
1. {run_dir}/context.md — read only the header section (up to ## Component Diff) for repo SHAs and platform metadata
2. {run_dir}/synthesis.yaml — pre-computed tables, aggregates, corroborations, disagreements
3. Each persona's .md file in {run_dir}/ — prose findings with evidence

Write {run_dir}/report.md with this structure:

# Upgrade Assessment Report: RHOAI {source} -> {target}

**Date**: {date from synthesis.yaml}
**Scope**: {scope}
**Method**: Independent clean-context agent personas, each isolated.
**Repos**: {copy the Repos line from context.md header}

## Executive Summary
3-5 sentences: overall risk, key blockers, recommended path. Use the aggregate
counts and blocking_findings from synthesis.yaml to ground the summary.

## Verdict Table
Copy the verdict_table from synthesis.yaml verbatim.

**Overall recommendation**: synthesize the persona verdicts without averaging
or harmonizing. State the recommendation and the key factors driving it.

## Blocking Prerequisites
Use blocking_findings from synthesis.yaml. For each, read the corresponding
persona's .md file to add the mitigation details and evidence.

---

## Persona Assessments
Copy the persona_table from synthesis.yaml verbatim.

---

## Cross-Persona Corroboration
Use corroborations from synthesis.yaml. For each match, read the relevant
persona .md files for detail. Note any XREF that adds a concern the owner
did not cover.

## Disagreements
Use disagreements from synthesis.yaml. Read both persona .md files to present
both views. Preserve both — do not pick a winner.

## Unclaimed Findings
Read persona .md files for findings outside ownership boundaries.

## Coverage Gaps
List missing_personas from synthesis.yaml. Read persona .md files for domains
inadequately covered.

Preserve disagreements. The user decides. Unclaimed findings signal ownership
taxonomy gaps to fix."
)
```

## Step 7: Report

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} --step 7 --status complete
```

Print a short summary to the user:
- The verdict table
- Top 5 cross-cutting findings
- Path to the full report file
