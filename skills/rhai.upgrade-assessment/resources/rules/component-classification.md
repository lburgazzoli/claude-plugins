---
domain: architecture
personas: [solution-architect]
applies-when: the target version introduces new entries in the Component Inventory or Deployment Inventory
---

# Component Classification

- Distinguish three categories when assessing new entries in the Component Inventory or Deployment Inventory. Do not aggregate them into a single finding.
  - **DSC component**: has its own toggle in the DSC spec. Enabling it is an explicit admin decision. Examples: Spark Operator, KServe, ModelRegistry.
  - **Sub-feature of an existing component**: activates when its parent DSC component is enabled, not independently. Its deployment is a consequence of the parent toggle. Examples: EvalHub (managed by TrustyAI Operator), Batch Gateway (deployed by rhods-operator when KServe parent is enabled), llm-d KV-Cache (routing library under KServe).
  - **Internal refactoring of an existing component**: adds pods but does not represent new functionality. A controller split, webhook extraction, or sidecar addition to an already-running component. Examples: llmisvc-controller-manager (split from kserve-controller-manager), gateway-discovery-server (added to odh-model-controller).
- New DSC components are informational (opt-in, no pre-existing state to migrate). They are not upgrade risks unless they introduce a dependency conflict with existing components or modify shared resources during installation. Per the constitution's adversarial posture: "New components added in the target version are not upgrade risks by default."
- Sub-features belong in the parent component's assessment, not in a separate "platform footprint" finding. Their resource impact is part of the parent component's resource budget.
- Internal refactoring belongs in the owning component's finding (e.g., the KServe topology split). Do not double-count a deployment that is already assessed in another finding.
- Libraries, component libraries, and ephemeral task pods (e.g., KFP component library, LM Evaluation Harness batch jobs) are not persistent deployments. Do not count them toward deployment footprint or namespace quota impact.
- When assessing namespace quota impact, count only net-new persistent deployments that would run on a cluster that does not enable any new opt-in components. Internal refactoring adds pods to already-enabled components; new opt-in components only add pods when explicitly enabled by the admin.
