---
name: dev.go-project
description: >
  Use when working in a Go project that has a Makefile. Triggers on: build,
  test, lint, format, vulnerability check, or any development task in a Go
  project directory. Reads the Makefile to understand available targets and
  uses them for all interactions.
allowed-tools:
  - Read
  - Glob
  - Grep
  - Bash
---

# Go Project — Makefile-First Development

When working in a Go project, always use the project's Makefile as the primary interface for build, test, lint, and other development tasks.

## First Contact

On first interaction with a Go project:

1. Check for a `Makefile` in the project root.
2. If found, read it and understand the available targets, tool versions, and conventions.
3. Run `make help` (if the target exists) to get a summary of available targets.

## Rules

1. **Always use `make` targets** instead of direct tool invocations. Examples:
   - `make lint` — not `golangci-lint run`
   - `make test` — not `go test ./...`
   - `make fmt` — not `go fmt ./...`

2. **If the Makefile lacks a target for the task**, fall back to direct commands but inform the user that the Makefile doesn't cover this case. Suggest adding a target if the task is likely to recur.

3. **If no Makefile exists**, use direct Go commands but suggest running `/dev.go-project-new` to bootstrap standard tooling.

4. **Respect the Makefile's tool versions**. Don't override pinned versions with `@latest` or different versions.

5. **Read the Makefile before guessing**. Different projects have different target names and conventions. Don't assume `make test` exists — check first.
