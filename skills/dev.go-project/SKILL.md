---
name: dev.go-project
description: Set up a Go project with a standard Makefile. Use when the user asks to create a Go project Makefile, set up Go development tooling, or bootstrap a Go project structure.
user-invocable: true
allowed-tools:
  - Read
  - Glob
  - Grep
  - Bash
  - Write
---

# Go Project Setup

Generate a standard Makefile for a Go project, using `go run` for all tool invocations to avoid downloading binaries.

## Input

`$ARGUMENTS` is optional. It may contain project-specific customization requests.

## Steps

1. Verify the current directory is a Go project by checking for `go.mod`. If not found, stop and inform the user.
2. Check if a `Makefile` already exists. If it does, show the user what will change and ask before overwriting.
3. Read the template from `${CLAUDE_SKILL_DIR}/assets/Makefile.tmpl`.
4. Resolve tool versions: look up the latest stable release for each tool (`golangci-lint`, `govulncheck`) and replace the placeholder versions in the template. Use `go list -m -versions <module>` or check the tool's release page to find the current stable version.
5. Read the golangci-lint config from `${CLAUDE_SKILL_DIR}/assets/.golangci.yml` and write it to the project root as `.golangci.yml`. If one already exists, ask before overwriting.
6. Write the Makefile to the project root.
7. If `$ARGUMENTS` mentions additional targets or customizations, add them following the same conventions (`.PHONY`, `##` comment for help, `go run` for tools, version variable at the top).
8. Run `make help` to confirm the Makefile works and show available targets to the user.

## Conventions

- **`go run` for all tools**: never install binaries into `bin/` or use `go install`. Invoke tools via `go run <module>@<version>`.
- **Pin versions**: never use `@latest`. Always resolve the current stable version at Makefile creation time and pin it in a `_VERSION` variable at the top of the Makefile (e.g., `GOLANGCI_LINT_VERSION ?= v2.1.6`).
- **Tool variables**: define a Makefile variable for each tool command (e.g., `GOLANGCI_LINT = go run ...@$(GOLANGCI_LINT_VERSION)`) and use the variable in targets, never the raw `go run` invocation.
- **Test separation**: unit tests use `-short` flag (tests should call `t.Skip` when `testing.Short()` is true for long-running tests). E2E tests use the `e2e` build tag.
- **Race detector**: enabled by default with `-race` on all test targets.
- **No test caching**: `-count=1` disables Go's test result caching.
- **Self-documenting**: every target has a `## Description` comment. `make help` is the default target.
- **Grouped targets**: use `##@ Section Name` comments to group related targets in help output.

## Makefile-First Principle

Once a Makefile is generated, **always use `make` targets instead of direct tool invocations**:
- `make lint` — not `golangci-lint run` or `go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run`
- `make test` — not `go test ./...`
- `make vulncheck` — not `govulncheck ./...`
- `make deps` — not `go mod tidy`
- `make fmt` — not `go fmt ./...`

This ensures consistent flags, options, and tool versions across all invocations.

## Target Reference

| Target | Description |
|--------|-------------|
| `help` | Display available targets (default) |
| `deps` | Tidy `go.mod` and `go.sum` |
| `fmt` | Format source code |
| `test` | Run all tests with race detector |
| `test/unit` | Run unit tests only (`-short` flag) |
| `test/e2e` | Run e2e tests (`-tags e2e` build tag) |
| `lint` | Run `golangci-lint` via `go run` |
| `lint/fix` | Run `golangci-lint` with `--fix` via `go run` |
| `vulncheck` | Run `govulncheck` via `go run` |
