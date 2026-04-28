---
name: dev.git
description: >
  Use when exploring git history, tracing changes, analyzing branches, or
  composing non-trivial git queries. Triggers on: "who changed", "when was
  this introduced", "what changed in", "git log", "git blame", "find the
  commit", "git history", "show me the diff", "what's in this branch",
  branch comparisons, or any time multiple sequential git calls could be
  replaced by one composed command.
allowed-tools:
  - Bash
  - Read
  - Grep
  - Glob
---

# Git Composition Skill

## Core principle: compose, don't iterate

Each git invocation should answer the question. Issuing `git log`, then `git show`, then `git diff` as three separate calls is the anti-pattern this skill exists to prevent.

```bash
# BAD — three round trips
git log --oneline -20
git show abc123
git diff abc123~1..abc123 -- pkg/foo/

# GOOD — one composed query
git log --follow -p --stat -- pkg/foo/bar.go
```

## Always-on rules

Apply these regardless of task.

1. **Scope bounds** — never run `git log` without `-n`, `--since`, a path, or a ref range. Unbounded log in large repos floods context.
2. **`--follow` for file history** — always use `--follow` when tracing a file. It handles renames transparently.
3. **`--stat` for context** — include `--stat` or `--name-status` to see which files changed without needing a second command.
4. **`git grep` over shell grep** — for tracked files, `git grep` is faster, respects `.gitignore`, and works on any revision.
5. **`--format=` for machine output** — when output will be parsed, use `--format=` to produce exactly the fields needed. Avoid parsing default pretty output.
6. **Three-dot for branch comparison** — `git diff A...B` diffs against the fork point, not the tip of A. Use two-dot `A..B` only when you specifically want "commits reachable from B but not A".
7. **`--first-parent` for mainline** — in merge-heavy repos, `--first-parent` shows only mainline merge commits, cutting through feature-branch noise.
8. **`git -C` for other directories** — use `git -C <dir> <cmd>` instead of `cd <dir> && git <cmd>` to avoid directory changes and subshells.

## Intent dispatch

Match user intent to a single composed command.

| Intent | Command |
|--------|---------|
| When was this code introduced? | `git log -S'string' --follow -p -- path` |
| When was this regex pattern added/changed? | `git log -G'regex' --follow -p -- path` |
| Who last changed this function? | `git blame -L :funcname -- path` |
| Who last changed these lines? | `git blame -L start,end -- path` |
| What changed between branches? | `git diff A...B --stat` |
| Commits on A but not B? | `git log B..A --oneline` |
| How did this file evolve? | `git log --follow --stat -p -n 20 -- path` |
| Commits touching a directory? | `git log --oneline --stat -n 30 -- dir/` |
| Commits on a specific lineage? | `git log --ancestry-path ancestor..descendant --oneline` |
| Find added/deleted/modified files between refs? | `git diff --diff-filter=D --name-only A..B` |
| Unpicked commits between branches? | `git cherry -v upstream branch` |
| Contributor summary? | `git shortlog -sn --no-merges ref-range` |
| Automated regression hunt? | `git bisect start bad good && git bisect run script` |
| Search tracked file content? | `git grep -n 'pattern'` |
| Search content at a specific revision? | `git grep -n 'pattern' rev` |
| Find a deleted file? | `git log --diff-filter=D --oneline -- '**/filename'` |
| Branches containing a commit? | `git branch --all --contains sha` |
| When was a file first added? | `git log --follow --diff-filter=A --oneline -- path` |

## Anti-patterns

| Instead of | Do this |
|------------|---------|
| `git log` (unbounded) | Add `-n`, `--since`, a path, or a ref range |
| `git log -- file` without `--follow` | `git log --follow -- file` |
| `git log` then `git show` then `git diff` | `git log -p --stat` or `git log -S'...' -p` |
| `grep -r pattern .` on tracked files | `git grep -n 'pattern'` |
| `git diff A..B` for branch comparison | `git diff A...B` (three-dot, fork-point aware) |
| `git blame file` (entire file) | `git blame -L start,end -- file` or `-L :funcname` |
| `git log --all --oneline \| grep msg` | `git log --all --oneline --grep='msg'` |
| `git log -S'x'` without path or file-type scope | `git log -S'x' -- '*.go'` or `-- path/` |
| `cd <dir> && git rev-parse HEAD` | `git -C <dir> rev-parse HEAD` |

## Composition recipes

Patterns that combine multiple concepts into a single pass.

```bash
# Pickaxe scoped to file type — find who introduced a symbol in Go files
git log -S'FunctionName' --follow -p -- '*.go'

# Blame skipping bulk-format commits (when .git-blame-ignore-revs exists)
git blame --ignore-revs-file .git-blame-ignore-revs -L :MyFunc -- path/to/file.go

# Fork-point diff with summary and context control
git diff main...feature -U5 --stat -- pkg/

# All deleted files in a directory
git log --oneline --diff-filter=D --name-only -- dir/

# Time-bounded + message-filtered log
git log --since=2024-01-01 --until=2024-06-30 --grep='fix' --oneline

# Mainline history only (skip merged feature-branch commits)
git log --first-parent --oneline -n 30

# Count commits between two refs
git rev-list --count origin/main..HEAD

# Commits that touched a symbol, across all branches
git log -S'SymbolName' --all --oneline --source
```
