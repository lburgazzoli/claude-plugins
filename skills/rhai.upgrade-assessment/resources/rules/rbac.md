---
domain: security
personas: [admin, engineer, solution-architect]
applies-when: component adds/removes CRDs, controllers change, or auth architecture shifts
---

# RBAC Assessment Rules

- Controllers ship their own RBAC (ClusterRole, Role, RoleBinding) via kustomize manifests. Do not flag RBAC gaps for new components unless you find evidence that the shipped RBAC is insufficient.
- Only report RBAC impact when there is **concrete evidence** of a consumer that needs updated permissions — e.g., a dashboard component that reads a new CRD kind but its ClusterRole doesn't include it, or a monitoring tool with a hardcoded allowlist.
- New CRDs added by new opt-in components do not automatically require RBAC changes for existing consumers. The component is opt-in via DSC configuration; its RBAC is deployed when enabled.
- When a CRD is removed, check whether aggregated ClusterRoles (e.g., `training-edit`, `training-view`) reference the removed resource. Stale aggregated role rules are a real finding.
- When auth architecture changes (e.g., oauth-proxy to kube-auth-proxy), assess whether existing RBAC policies (RoleBindings, ClusterRoleBindings) still grant correct access through the new auth path. This is a real finding.
