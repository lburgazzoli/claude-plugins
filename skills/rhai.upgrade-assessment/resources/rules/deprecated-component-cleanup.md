---
domain: crd
personas: [admin, engineer, solution-architect, sre]
applies-when: component CRDs are removed in the target version for components that were deprecated or had their workloads removed in a prior version
---

# Deprecated Component CRD Cleanup Rules

- When a component was deprecated or its workloads removed in a prior version, the removal of its leftover CRDs in the current upgrade is housekeeping — not a risk. Do not produce a numbered finding for it.
- Detect this case by checking: (1) the component had no active workloads or controller in the source version (read-only, stub, or absent from DSC), and (2) the CRDs are controller-managed (owned by rhods-operator or its sub-controllers, not user-created).
- These CRD removals may still appear in the CRD Change Matrix with change type `removed` and `Breaking? No` — that is correct. They must not be elevated to numbered findings with MEDIUM or higher severity.
- If an odh-cli check exists for the removal but is gated to the wrong version transition (e.g., gated to 2.x->3.x when the CRD is cleaned up in 3.3->3.4), note the gating mismatch in the Migration Code Gap Analysis table but do not treat the CRD removal itself as a finding.
- The only exception: if the deprecated component's CRDs could still hold user-created custom resources (not controller-managed instances), flag the removal as a finding with appropriate severity based on data-loss risk.
