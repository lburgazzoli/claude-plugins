# Assessment Rules

Domain-specific rules that upgrade assessment personas load before starting their analysis. Rules override general constitution guidance when more specific.

## Frontmatter Schema

```yaml
---
domain: <string>           # Topic area: security, architecture, observability, crd, networking, storage, auth, scheduling
personas: [<string>, ...]  # Which personas must load this rule: sre, admin, engineer, architect
applies-when: <string>     # Natural-language condition describing when this rule is relevant to an upgrade transition
---
```

### Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `domain` | yes | string | Topic area. Used for grouping and discoverability. |
| `personas` | yes | list of strings | Persona identifiers that must load this rule. A persona skips rule files where its identifier is not listed. |
| `applies-when` | yes | string | Describes the upgrade conditions under which this rule applies (e.g., "component adds/removes CRDs"). Personas evaluate this against the current `input.md` context. |

## Convention

- One rule file per domain concern (e.g., `rbac.md`, `crd-versioning.md`, `restart-triggers.md`)
- Filenames are kebab-case, matching the topic
- Rule content is a flat list of bullet points — no nested sections, no examples longer than one line
- Rules state what to do and what not to do, grounded in evidence requirements
