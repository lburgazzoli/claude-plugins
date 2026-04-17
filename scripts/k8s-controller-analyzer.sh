#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ANALYZER_DIR="${REPO_ROOT}/tools/k8s-controller-analyzer"
CALLER_DIR="$(pwd)"

if [[ ! -d "${ANALYZER_DIR}" ]]; then
  echo "error: analyzer directory not found: ${ANALYZER_DIR}" >&2
  exit 1
fi

usage() {
  cat >&2 <<EOF
Usage: $0 [k8s-controller-analyzer args]

Examples:
  $0 ./path/to/repo -rules crd-version-coverage
  $0 -rules crd-version-coverage ./path/to/repo
EOF
}

args=("$@")
repo_arg_index=-1

# Find positional repo arg while skipping known option values.
i=0
while (( i < ${#args[@]} )); do
  token="${args[$i]}"

  case "${token}" in
    -rules|-format|-out|-skill)
      ((i += 2))
      continue
      ;;
    --rules|--format|--out|--skill)
      ((i += 2))
      continue
      ;;
    -strict-load)
      ((i += 1))
      continue
      ;;
    --rules=*|--format=*|--out=*|--skill=*)
      ((i += 1))
      continue
      ;;
    --strict-load|--strict-load=*)
      ((i += 1))
      continue
      ;;
    -*)
      ((i += 1))
      continue
      ;;
    *)
      repo_arg_index=$i
      break
      ;;
  esac
done

if (( repo_arg_index < 0 )); then
  # Default to this repository when no explicit target is provided.
  args=("${REPO_ROOT}" "${args[@]}")
else
  repo_arg="${args[$repo_arg_index]}"
  if [[ "${repo_arg}" != /* ]]; then
    args[$repo_arg_index]="${CALLER_DIR}/${repo_arg}"
  fi
fi

if (( ${#args[@]} == 0 )); then
  usage
  exit 1
fi

(
  cd "${ANALYZER_DIR}"
  go run ./cmd "${args[@]}"
)
