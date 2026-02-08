---
name: lb:code-review
description: Review code changes for quality, security, and maintainability. Use with optional scope in $ARGUMENTS.
---

# /lb:code-review

Perform a focused code review.

Inputs:
- If `$ARGUMENTS` is provided, treat it as scope (files, module, PR notes, or review focus).
- If no arguments are provided, review current repository changes from git diff.

When reviewing:
1. Inspect changed files first.
2. Prioritize correctness, security, and maintainability.
3. Highlight highest-impact issues first.

Review checklist:
- Code is simple and readable
- Functions and variables are well named
- No duplicated code
- Error handling is appropriate
- Input validation is present where needed
- No exposed secrets or credentials
- Test coverage is sufficient for changed behavior
- Performance concerns are identified

Output format:
- Critical issues (must fix)
- Warnings (should fix)
- Suggestions (nice to improve)

For each issue, include a concrete fix recommendation.
