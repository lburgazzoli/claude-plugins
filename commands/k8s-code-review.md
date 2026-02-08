---
name: lb:k8s-code-review
description: Review Kubernetes controller code for reconciliation, RBAC, status, finalizers, and API conventions.
---

# /lb:k8s-code-review

Perform a Kubernetes controller focused code review.

Inputs:
- If `$ARGUMENTS` is provided, treat it as scope (files, package, controller name, or review focus).
- If no arguments are provided, review current repository changes from git diff.

Assess the following areas:
1. Reconciliation idempotency and state handling
2. Error handling and requeue strategy
3. Resource management and API efficiency
4. RBAC least privilege and security
5. Status/conditions patterns and observed generation usage
6. Finalizers, cleanup logic, and owner references
7. Test coverage for edge cases and reconciliation paths
8. Observability (events, logs, metrics)
9. Performance and cache usage
10. Kubernetes API conventions compliance

Output format:
1. Summary (2-3 sentences)
2. Critical issues (must fix)
3. Major issues (should fix)
4. Minor issues (nice to improve)
5. Positive highlights

For each issue, provide a concrete suggested change.
