---
name: Runtime Verification
description: Run cluster verification against static assessment findings
scope: [runtime]
state-status: runtime_verification_complete
---

# Runtime Verification

Requires a previous static run. Operates on the most recent static run directory for this version pair.

1. Find the most recent static run directory matching this version pair in `.context/tmp/upgrade-assessments/{source}-to-{target}-*/`. If none exists, print an error: "no static assessment found — run without `--scope runtime` first." and stop.
2. Read all persona output files (`sre.md`, `admin.md`, `engineer.md`, `solution-architect.md`) from that run directory.
3. Extract the **Runtime Verification Checklist** from each persona output.
4. Consolidate all checklist items into a single verification task list.
5. For each checklist item, run the verification against the live cluster using `oc`/`kubectl` (follow `tools.kubectl` patterns). Record the result: `PASS`, `FAIL`, or `SKIPPED` (with reason).
6. Write the verification results to `{run_dir}/runtime-verification.md`.
7. Update `{run_dir}/report.md` with the runtime verification results appended after the Coverage Gaps section.
8. Print a summary: how many items passed, failed, skipped, and any findings that changed severity based on runtime evidence.
