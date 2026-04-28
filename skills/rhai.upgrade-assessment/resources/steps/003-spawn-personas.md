---
name: Spawn Personas
description: Spawn independent persona subagents via Agent tool
scope: [static]
state-status: personas_complete
---

# Spawn Personas

Invoke the selected personas (from `--personas`, default all four) in a **single message** via the Agent tool. Each persona runs as an independent subagent with a clean context — no access to the orchestrator's conversation history. This ensures each persona's assessment is independent and unbiased.

Model mapping:
- `sre`, `engineer`, `architect` → `model: "opus"`
- `admin` → `model: "sonnet"`

Invoke **all selected personas in a single message** so they run in parallel. Each agent is independent — no persona reads another's output, so there are no dependencies between them. Use one Agent tool call per persona, all in the same message:

```
Agent(
  description="RHOAI upgrade assessment — {persona} persona",
  model="{opus|sonnet}",
  prompt="You are the {persona} persona for an RHOAI upgrade assessment.

Read these files in this order — they contain your full instructions:
1. ${CLAUDE_SKILL_DIR}/resources/prompts/personas/{persona}.md — your role, checks, output format
2. ${CLAUDE_SKILL_DIR}/resources/constitution.md — universal rules
3. Applicable rules from ${CLAUDE_SKILL_DIR}/resources/rules/ (only those whose frontmatter `personas` list includes `{persona}`)
4. {run_dir}/context.md — the assessment context

Follow the instructions in your persona file. Write your output to {run_dir}/{persona}.md.
After completing your assessment and self-review, write structured metadata to {run_dir}/{persona}.yaml using the metadata CLI as described in the constitution.
Do not read or write any other files in the run directory."
)
```
