---
domain: operations
personas: [admin]
applies-when: the target version has dependency operators listed in the Dependency Operator Inventory
---

# Dependency Operator Findings

- Only report a dependency operator as a finding if something about it changes in this transition. A dependency that was already required in the source version and remains required with the same version, channel, and configuration in the target version is a standing prerequisite, not an upgrade finding.
- Report a dependency operator finding only if: (1) it is newly required in the target version and was not required in the source, (2) its required version or channel changed, or (3) it has a new interaction with a changed component that alters how it must be configured.
- "Verify it is installed and healthy" is a generic operational checklist item applicable to every upgrade. Do not elevate it to a numbered finding unless the upgrade introduces a specific reason it might not be installed or healthy.
- New conditional dependencies (required only when an opt-in component is enabled) are finding-worthy because the admin must act before enabling the component. State the activation condition explicitly.
