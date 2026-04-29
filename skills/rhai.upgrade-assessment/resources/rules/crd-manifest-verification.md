---
domain: crd
personas: [engineer, solution-architect]
applies-when: architecture docs report a CRD as added, removed, or version-changed between source and target versions
---

# CRD Manifest Verification

- CRD lifecycle claims from architecture docs require manifest verification. When the architecture docs report a CRD as added, removed, or version-changed between source and target, and that claim would produce a MEDIUM or above finding, verify it against the actual deployment chain before reporting.
- Trace from controller code to rendered manifests: (1) identify the overlay path the controller selects for the target platform (e.g., overlaysSourcePaths in the rhods-operator component controller), (2) read that overlay's kustomization.yaml to trace its resource includes, (3) if the include chain is ambiguous or involves patches that may add or remove resources, run `kustomize build` on the overlay and extract the relevant CRD with `yq 'select(.kind == "CustomResourceDefinition" and .metadata.name == "...")'`.
- A CRD absent from the architecture doc is not evidence of removal — it may be an omission.
- Files present in `prefetched-manifests/` are not evidence of deployment — they may be unreferenced. Only the rendered kustomize output for the active overlay determines what is actually applied to the cluster.
- When verification reveals a discrepancy between the architecture doc and the actual manifests, record it in `discrepancies.yaml` before proceeding — this prevents downstream personas from building findings on incorrect data.
