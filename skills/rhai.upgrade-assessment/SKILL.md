---
name: rhai.upgrade-assessment
description: Multi-persona upgrade impact assessment for RHOAI version transitions. Spawns four independent clean-context agent reviewers (admin, engineer, solution-architect, SRE) to assess upgrade risks. Usage - /rhai.upgrade-assessment --source <version> --target <version> [--dry-run] [--scope static|runtime]
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
  - `runtime` — **not yet implemented; deferred for later.** When implemented: requires a previous static run, reads Runtime Verification Checklists from persona outputs, runs them against a live cluster, and updates the report with confirmed/refuted findings.
- `--personas sre,engineer,...` — comma-separated list of personas to run (default: all four: `admin,engineer,solution-architect,sre`). Use to run a subset, e.g. `--personas engineer,solution-architect`.

Examples:
- `/rhai.upgrade-assessment --source 2.25 --target 3.3`
- `/rhai.upgrade-assessment --source 3.3 --target 3.4 --dry-run`
- `/rhai.upgrade-assessment --source 2.25 --target 3.3 --scope runtime`
- `/rhai.upgrade-assessment --source 2.25 --target 3.3 --personas engineer,sre`

If `--source` or `--target` is missing, print usage and stop.

## Parse Input

Parse `$ARGUMENTS` using **strict named-flag parsing**. Only `--flag value` pairs are accepted — positional arguments are not supported.

Extract:
- `--source` — strip leading `v`, normalize to `major.minor` (e.g., `2.25`). **Required.**
- `--target` — strip leading `v`, normalize to `major.minor` (e.g., `3.3`). **Required.**
- `--dry-run` — boolean, default false
- `--scope` — `static` (default) or `runtime`
- `--personas` — comma-separated list (default: `admin,engineer,solution-architect,sre`)

Validate:
- **Reject positional arguments**: any token in `$ARGUMENTS` that is not a `--flag` or the value immediately following a known flag is an error. Print usage and stop. Do not silently interpret bare values as source/target.
- Both `--source` and `--target` are required. If either is missing, print usage and stop.
- If `--scope runtime` is passed, print an error that runtime scope is not yet implemented and stop.
- Source must be less than target (no downgrades). EA pre-release suffixes (e.g., `3.4-ea.1`) sort before their release version (`3.4`).

On any validation failure, print:
```
Usage: /rhai.upgrade-assessment --source <version> --target <version> [--dry-run] [--scope static|runtime] [--personas admin,engineer,solution-architect,sre]

Error: <specific problem>
```
Then stop. Do not proceed.

## State Persistence

This skill is long-running (context build + parallel agents + verification + synthesis). To survive context compression, persist state at each step boundary:

```bash
# Initialize after creating the run directory (first step)
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py init {run_dir} \
    --source {source} --target {target} --scope {scope} --personas {personas}

# Update at each step transition
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} --step {N} --status {status}

# Read back if context is lost
python3 ${CLAUDE_SKILL_DIR}/scripts/state.py read {run_dir}
```

If you lose track of the run directory after context compression, find it with `ls -td .context/tmp/upgrade-assessments/*/ | head -1`, then recover state with `python3 ${CLAUDE_SKILL_DIR}/scripts/state.py read {run_dir}`.

## Execution Flow

After parsing input:

1. Set `scope` to `runtime` if `--scope runtime`, otherwise `static`.
2. Generate run ID: `{source}-to-{target}-{YYYYMMDD-HHMMss}`
3. Create the run directory: `.context/tmp/upgrade-assessments/{run_id}/`
4. Initialize state:
   ```bash
   python3 ${CLAUDE_SKILL_DIR}/scripts/state.py init {run_dir} \
       --source {source} --target {target} --scope {scope} --personas {personas}
   ```
5. Build a flags string from the parsed input: if `--dry-run` is set, `flags=dry-run`; otherwise `flags=` (empty).
6. **Step loop** — repeat until done:
   ```bash
   python3 ${CLAUDE_SKILL_DIR}/scripts/steps.py next {run_dir} --flags {flags}
   ```
   - If output is `done` → stop the loop. This happens when all steps are complete, **or** when a completed step's stop condition matches the current flags (e.g., dry-run step with `--dry-run` flag).
   - If the command exits with an error → a required artifact is missing. Print the error and stop.
   - Otherwise, parse the output: `{step_number} {step_file}`
   - Mark the step as started:
     ```bash
     python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} \
         --step {step_number} --status running
     ```
   - Read `${CLAUDE_SKILL_DIR}/{step_file}` and follow its instructions.
   - After completing the step, read the `state-status` from the step file's YAML frontmatter and update state:
     ```bash
     python3 ${CLAUDE_SKILL_DIR}/scripts/state.py set {run_dir} \
         --step {step_number} --status {state-status}
     ```
   - Continue to the next iteration.
