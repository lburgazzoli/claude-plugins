---
domain: networking
personas: [engineer, solution-architect, sre]
applies-when: a component adds or changes a MutatingWebhookConfiguration or ValidatingWebhookConfiguration
---

# Webhook Scope Validation

- When a component deploys a webhook that targets broad resource types (e.g., pods, services, deployments), **verify the actual webhook manifest** from the component repo before reporting scope claims. Clone the component repo at the target branch and read the MutatingWebhookConfiguration or ValidatingWebhookConfiguration YAML. Architecture docs alone are not sufficient evidence for webhook scope.
- Extract and report: `failurePolicy`, `namespaceSelector`, `objectSelector`, `rules[].scope`, and `matchPolicy`. These fields determine the actual blast radius — not the webhook endpoint path.
- A webhook endpoint like `/mutate--v1-pod` does NOT mean "intercepts all pods cluster-wide." The endpoint path is just a route on the webhook server. The `rules`, `namespaceSelector`, and `objectSelector` in the MutatingWebhookConfiguration determine which pods are actually sent to the webhook. Do not conflate endpoint path with scope.
- If `namespaceSelector` or `objectSelector` is present, the webhook is scoped — report the actual selector, not "cluster-wide."
- If neither selector is present and `rules[].scope` is `*` or `Cluster`, then the webhook genuinely has cluster-wide scope — report it as a finding with the verified evidence.
- Do not defer webhook scope verification to runtime when the webhook manifest is available in the component repo or kustomize overlay. The constitution rule "do not defer to post-upgrade validation what you can verify now" applies directly here.
- When the webhook uses self-managed TLS (not OpenShift service-ca or cert-manager), note the divergence but verify the actual certificate management mechanism from the operator code before flagging it as a risk.
