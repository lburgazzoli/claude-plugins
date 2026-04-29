---
domain: observability
personas: [engineer, solution-architect, sre]
applies-when: component changes metrics port, adds/removes kube-rbac-proxy sidecar, or modifies ServiceMonitor configuration
---

# Metrics & Monitoring Assessment Rules

- When a component changes its metrics port or adds a kube-rbac-proxy sidecar, the rhods-operator deploys the component's ServiceMonitor via kustomize overlays. Check the target version's architecture doc to confirm the ServiceMonitor is updated to match the new port and auth scheme.
- Do not flag a metrics port change as a finding unless you verify that the ServiceMonitor in the target version's kustomize overlay does NOT match the new port. If the overlay correctly references the new port, the operator handles the migration automatically — no user action required.
- When verifying, read the component's architecture doc for the target version (e.g., `architecture/rhoai-3.3/odh-model-controller.md`) and check the Prometheus metrics endpoint table and any ServiceMonitor references.
