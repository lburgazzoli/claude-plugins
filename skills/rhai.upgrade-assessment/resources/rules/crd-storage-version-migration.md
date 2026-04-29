---
domain: crd
personas: [engineer]
applies-when: a CRD's API version or storage version changes between source and target versions, or architecture docs report a version difference
---

# CRD Version Migration

## Version change verification

- When the architecture docs report different CRD versions between source and target (e.g., v1 in source, v2beta1 in target), verify the claim by reading the actual CRD YAML in the component repo at both branches before reporting it as a finding. Architecture docs may lag the code. Clone the component repo at the correct branch (`--branch rhoai-{version}`) and read `config/crd/bases/*.yaml`.
- If only one branch is available, verify the available one — a single contradiction is enough to flag the doc as unreliable for that CRD.
- When both docs report the same version, no repo-level verification is needed.
- A finding based solely on an architecture doc version number without CRD file verification is an unverified claim, not a confirmed finding. Do not rate it above LOW until verified.

## Storage version migration severity

- A storage version change means the API server will store new and updated objects in the new version's schema. Existing objects in etcd remain in the old version's schema until individually rewritten. This is a standard Kubernetes CRD lifecycle event, not inherently a risk.
- **Severity depends on the conversion implementation, not the fact of the change.** Before assigning severity, verify four things in the component repo at the target branch:
  1. Does a conversion webhook exist with real conversion logic (hub/spoke pattern or explicit ConvertTo/ConvertFrom functions)?
  2. Is the conversion lossless (all source fields map to target fields, no data dropped)?
  3. Is there test coverage for the conversion round-trip?
  4. Is the webhook deployment ordering correct (webhook ready before resources that trigger conversion)?
- If all four are true, the storage version change is a well-engineered migration — rate as LOW. The residual risk is only the transient webhook unavailability during controller replacement, which is covered by `webhook-transient-unavailability.md`.
- If any of the four are false, rate based on the specific gap and CRD ownership. Lossy or missing conversion on user-created CRDs (e.g., InferenceService, TrainJob) is BLOCKING — user data is at risk with no automatic recovery. Lossy or missing conversion on controller-managed CRDs is HIGH — data is recreated by reconciliation, but transient inconsistency may cause errors. Missing tests is MEDIUM. Incorrect ordering is MEDIUM.

## Alpha and beta API version changes

- A storage version change within the alpha lifecycle (e.g., v1alpha1 to v1alpha2) is the alpha API contract working as designed. Consumers of alpha APIs accept that breaking changes may occur between minor versions. Do not rate an alpha-to-alpha storage version change higher than you would rate the same change on a stable API — rate it lower, because the breaking-change expectation is already priced in.
- The same principle applies to beta-to-beta changes (e.g., v2beta1 to v2beta2), though with slightly less tolerance since beta implies broader adoption.

## Conversion-preserved ghost fields

- When a field is removed from the new version's Go struct but preserved via annotation during conversion (for round-trip fidelity), check whether any controller in the target version reads that annotation. If no controller consumes it, the field is semantically dead — setting it via the old API version has no practical effect.
- Report ghost fields as LOW (misleading API surface), not as a data-loss or migration risk.

## Pre-upgrade inventory checks as severity drivers

- If the recommendation is "check whether CRs exist, and proceed with the upgrade either way," the check is informational. Do not use "no automated pre-upgrade validation" as a severity driver when the conversion is fully implemented — the conversion webhook IS the validation.
- Reserve severity for cases where the pre-upgrade check would change the decision (e.g., "if CRs exist, you must run a manual migration script before upgrading" or "if CRs exist, the upgrade will fail because no conversion mechanism exists").

## storedVersions cleanup

- After all objects are rewritten to the new storage version, the old version should be removed from `status.storedVersions`. This is standard Kubernetes CRD lifecycle maintenance. The conversion webhook continues to serve both versions indefinitely regardless of storedVersions state.
- Do not rate storedVersions cleanup as a finding. Mention it in the Runtime Verification Checklist if relevant.
