---
domain: security
personas: [engineer, architect, sre]
applies-when: component uses TLS certificates for webhooks, conversion webhooks, or service endpoints
---

# TLS Certificate Provisioning Rules

RHOAI runs on OpenShift and uses **OpenShift service-ca** for TLS certificate provisioning — not cert-manager. Upstream KServe, Kubeflow, and other projects use cert-manager in their default configurations, but the RHOAI/ODH kustomize overlay (`config/overlays/odh/`) patches this out:

1. `patches/remove-cert-manager.yaml` — deletes cert-manager Certificate and Issuer resources
2. `patches/openshift-ca-patch.yaml` — replaces `cert-manager.io/inject-ca-from` annotations with `service.beta.openshift.io/inject-cabundle: "true"` on CRDs, ValidatingWebhookConfigurations, and MutatingWebhookConfigurations
3. `patches/openshift-serving-cert-*-patch.yaml` — adds `service.beta.openshift.io/serving-cert-secret-name` annotations to webhook Services

When assessing TLS or certificate dependencies:
- Do not flag cert-manager as a dependency for webhook TLS or conversion webhook CA unless you verify the ODH overlay does NOT patch it out
- OpenShift service-ca is built into the platform and does not require a separate operator installation
- cert-manager IS a dependency for other purposes (e.g., DSPO RayCluster mTLS) — but verify each use case against the actual overlay
- The `dependencies.certmanager.installed` odh-cli check covers cert-manager presence for cases where it is genuinely required
