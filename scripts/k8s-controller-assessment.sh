#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <controller-repo-path> [extra-args...]"
  echo ""
  echo "Examples:"
  echo "  $0 .tmp/repos/mlflow-operator"
  echo "  $0 .tmp/repos/mlflow-operator --scope=architecture"
  echo "  $0 .tmp/repos/mlflow-operator --mode=exploratory"
  exit 1
fi

CONTROLLER_PATH="$(cd "$1" && pwd)"
shift

ALLOWED_TOOLS="Read,Grep,Glob,Bash,Agent,Skill"
ALLOWED_TOOLS="${ALLOWED_TOOLS},mcp__k8s-controller-analyzer__analyze_controller"
ALLOWED_TOOLS="${ALLOWED_TOOLS},mcp__gopls__go_file_context"
ALLOWED_TOOLS="${ALLOWED_TOOLS},mcp__gopls__go_symbol_references"
ALLOWED_TOOLS="${ALLOWED_TOOLS},mcp__gopls__go_search"
ALLOWED_TOOLS="${ALLOWED_TOOLS},mcp__gopls__go_package_api"

PROMPT="/k8s.controller-assessment ${CONTROLLER_PATH}"
if [[ $# -gt 0 ]]; then
  PROMPT="${PROMPT} $*"
fi

cd "${REPO_ROOT}"

echo "${PROMPT}" | claude -p \
  --plugin-dir . \
  --allowedTools "${ALLOWED_TOOLS}"
