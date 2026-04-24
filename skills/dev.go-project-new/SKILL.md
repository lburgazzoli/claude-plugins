---
name: dev.go-project-new
description: >
  Bootstrap a new Go project with a standard Makefile, golangci-lint config, and
  gitignore. Use when the user asks to create a new Go project, initialize a Go
  module, or scaffold Go project structure.
user-invocable: true
allowed-tools:
  - Read
  - Glob
  - Grep
  - Bash
  - Write
---

# New Go Project Setup

Bootstrap a new Go project with standard tooling: `go.mod`, Makefile, `.golangci.yml`, and `.gitignore`.

## Input

`$ARGUMENTS` must contain the Go module path (e.g., `github.com/user/project`). Additional customization requests are optional.

If `$ARGUMENTS` is empty, ask the user for the module path before proceeding.

## Steps

1. Determine the project directory. If the current directory is empty or the user specified a name, use that. Otherwise, confirm with the user.
2. Run `go mod init <module-path>` using the module path from `$ARGUMENTS`.
3. Read the Makefile template from `${CLAUDE_SKILL_DIR}/assets/Makefile.tmpl`.
4. Resolve tool versions: look up the latest stable release for each tool (`golangci-lint`, `govulncheck`) and replace the placeholder versions in the template. Use `go list -m -versions <module>` or check the tool's release page to find the current stable version.
5. Write the Makefile to the project root. If one already exists, ask before overwriting.
6. Read `${CLAUDE_SKILL_DIR}/assets/.golangci.yml` and write it to the project root. If one already exists, ask before overwriting.
7. Read `${CLAUDE_SKILL_DIR}/assets/.gitignore` and write it to the project root. If one already exists, ask before overwriting.
8. If `$ARGUMENTS` mentions additional targets or customizations, add them following the Makefile conventions (`.PHONY`, `##` comment for help, `go run` for tools, version variable at the top).
9. Run `make help` to confirm the Makefile works and show available targets to the user.

## Makefile Conventions

- **`go run` for all tools**: never install binaries into `bin/` or use `go install`. Invoke tools via `go run <module>@<version>`.
- **Pin versions**: never use `@latest`. Always resolve the current stable version at Makefile creation time and pin it in a `_VERSION` variable at the top of the Makefile (e.g., `GOLANGCI_LINT_VERSION ?= v2.1.6`).
- **Tool variables**: define a Makefile variable for each tool command (e.g., `GOLANGCI_LINT = go run ...@$(GOLANGCI_LINT_VERSION)`) and use the variable in targets, never the raw `go run` invocation.
- **Test separation**: unit tests use `-short` flag. E2E tests use the `e2e` build tag.
- **Race detector**: enabled by default with `-race` on all test targets.
- **No test caching**: `-count=1` disables Go's test result caching.
- **Self-documenting**: every target has a `## Description` comment. `make help` is the default target.
- **Grouped targets**: use `##@ Section Name` comments to group related targets in help output.
