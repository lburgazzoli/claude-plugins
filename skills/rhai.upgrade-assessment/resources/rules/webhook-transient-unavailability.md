---
domain: networking
personas: [engineer, solution-architect, sre]
applies-when: a component adds, changes, or replaces a webhook (mutating, validating, or conversion) during the upgrade
---

# Webhook Transient Unavailability

- During a controller pod replacement (rolling update, scale-down/up, or reconciliation-driven redeployment), the webhook-serving pod is temporarily unavailable. Kubernetes admission requests or CRD conversion requests targeting that webhook will be rejected until the new pod passes its readiness probe. This applies equally to mutating admission webhooks, validating admission webhooks, and conversion webhooks on CRDs.
- Transient rejection during controller replacement is expected Kubernetes behavior and does NOT constitute a finding on its own. This can happen at any time — node drain, OOM kill, rolling update — it is not unique to upgrades. Do not rate a webhook's temporary unavailability as HIGH or MEDIUM severity solely because API requests may fail during the replacement window.
- Before assessing severity, verify the webhook's configuration is correct by applying the checks in `webhook-scope-validation.md` (scope, selectors, failurePolicy) and `tls-certificate-provisioning.md` (TLS mechanism). A correctly configured webhook auto-recovers; a misconfigured one does not.
- **Correct configuration** means all of the following are true: (1) the webhook's Service reference points to a real Service backed by the controller Deployment, (2) TLS is properly managed (OpenShift service-ca annotation or operator-managed cert rotation), and (3) the webhook-serving Deployment has a readiness probe.
- **Severity calibration** — regardless of `failurePolicy`:
  - Configuration correct → transient unavailability is not a finding. The webhook auto-recovers once the new pod passes its readiness probe. At most, note the expected recovery window as informational context (LOW), not as a risk.
  - Configuration incorrect or permanently broken (wrong Service reference, broken TLS, missing readiness probe, webhook never registered) → HIGH or BLOCKING depending on blast radius. The webhook will NOT auto-recover; manual intervention is required.
- `failurePolicy` determines what happens during the transient window (`Fail` = requests rejected; `Ignore` = requests bypass webhook), but does NOT change whether the transient window itself is a problem. Both are expected and self-resolving.
- For conversion webhooks on CRDs: transient unavailability means API requests for the CRD's converted versions will fail briefly during controller replacement. If the CRD has only one served version (no conversion needed), this is not a concern. If the CRD serves multiple versions and uses `spec.conversion.strategy: Webhook`, verify that the conversion webhook configuration is correct and apply the severity calibration above.
- When the SRE persona estimates webhook availability windows (disruption duration), the window is bounded by the controller Deployment's rolling update strategy and readiness probe timing. State the bound, do not speculate beyond it.
- Do not conflate webhook transient unavailability with webhook scope problems. A webhook that intercepts resources too broadly (scope finding from `webhook-scope-validation.md`) is a separate finding from the webhook's transient unavailability during pod replacement. The transient unavailability itself is expected behavior, not an additional finding.
