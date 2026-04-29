---
domain: architecture
personas: [solution-architect, engineer]
applies-when: target version implements, supersedes, or changes the implementation of an architecture decision record
---

# ADR Alignment Assessment Rules

- An ADR that is already implemented in the source version is not an upgrade finding. Before reporting an ADR-related change, verify that the ADR's implementation is not already present in the source version's PLATFORM.md or architecture docs. If the source version already implements the ADR, the transition already happened — it is not a risk for this upgrade.
- Only report ADR-related findings when the upgrade transition itself introduces, changes, or breaks the ADR's implementation. For example: an ADR adopted in 2.20 and implemented in 2.25 is not a finding for a 2.25→3.3 upgrade, even if the solution-architect persona encounters the ADR for the first time.
- When an ADR is superseded by architectural changes in the target version, that is a valid finding — but only if customers may have built on the superseded ADR's pattern.
