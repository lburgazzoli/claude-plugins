#!/usr/bin/env bash
# reload-marketplace.sh
# Hard-cleans a Claude Code local marketplace and reinstalls all its plugins.
#
# Usage:
#   ./reload-marketplace.sh <marketplace-name> [plugin1 plugin2 ...]
#
# Examples:
#   ./reload-marketplace.sh lburgazzoli lb
#   ./reload-marketplace.sh lburgazzoli          # auto-discovers plugins from installed_plugins.json
#
# If no plugins are given, the script reads them from installed_plugins.json.

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <marketplace-name> [plugin1 plugin2 ...]"
  exit 1
fi

MARKETPLACE_NAME="$1"
shift
PLUGINS=("$@")

PLUGINS_DIR="${HOME}/.claude/plugins"
INSTALLED_JSON="${PLUGINS_DIR}/installed_plugins.json"
KNOWN_JSON="${PLUGINS_DIR}/known_marketplaces.json"

info()    { echo "  → $*"; }
success() { echo "  ✓ $*"; }
warn()    { echo "  ⚠ $*"; }

require() {
  for cmd in "$@"; do
    command -v "$cmd" &>/dev/null || { echo "Error: '$cmd' not found in PATH."; exit 1; }
  done
}

require claude jq

# ── resolve marketplace source from known_marketplaces.json ───────────────────
resolve_source() {
  if [[ ! -f "$KNOWN_JSON" ]]; then
    echo "Error: ${KNOWN_JSON} not found — marketplace '${MARKETPLACE_NAME}' is not registered."
    exit 1
  fi

  local source_type
  source_type=$(jq -r --arg m "$MARKETPLACE_NAME" '.[$m].source.source // empty' "$KNOWN_JSON")

  if [[ -z "$source_type" ]]; then
    echo "Error: marketplace '${MARKETPLACE_NAME}' not found in known_marketplaces.json."
    exit 1
  fi

  case "$source_type" in
    directory)
      MARKETPLACE_SOURCE=$(jq -r --arg m "$MARKETPLACE_NAME" '.[$m].source.path' "$KNOWN_JSON")
      ;;
    github)
      MARKETPLACE_SOURCE=$(jq -r --arg m "$MARKETPLACE_NAME" '.[$m].source.repo' "$KNOWN_JSON")
      ;;
    *)
      MARKETPLACE_SOURCE=$(jq -r --arg m "$MARKETPLACE_NAME" '.[$m].source.repo // .[$m].source.path // .[$m].source.url' "$KNOWN_JSON")
      ;;
  esac

  info "Source: ${MARKETPLACE_SOURCE} (${source_type})"
}

# ── auto-discover installed plugins for this marketplace ──────────────────────
discover_plugins() {
  if [[ ! -f "$INSTALLED_JSON" ]]; then
    warn "No installed_plugins.json — pass plugins explicitly."
    return
  fi

  mapfile -t PLUGINS < <(
    jq -r --arg m "$MARKETPLACE_NAME" \
      '.plugins | to_entries[] | select(.key | endswith("@" + $m)) | .key | split("@")[0]' \
      "$INSTALLED_JSON"
  )

  if [[ ${#PLUGINS[@]} -gt 0 ]]; then
    info "Auto-discovered plugins: ${PLUGINS[*]}"
  else
    warn "No plugins found for marketplace '${MARKETPLACE_NAME}'."
  fi
}

# ── uninstall plugins ─────────────────────────────────────────────────────────
uninstall_plugins() {
  for plugin in "${PLUGINS[@]}"; do
    info "Uninstalling ${plugin}…"
    claude plugin uninstall "${plugin}@${MARKETPLACE_NAME}" 2>/dev/null || true
  done
}

# ── cleanup cache and registry entries ────────────────────────────────────────
cleanup_cache() {
  info "Removing plugin cache…"
  rm -rf "${PLUGINS_DIR}/cache/${MARKETPLACE_NAME}"

  info "Removing marketplace clone…"
  rm -rf "${PLUGINS_DIR}/marketplaces/${MARKETPLACE_NAME}"

  # Prune installed_plugins.json
  if [[ -f "$INSTALLED_JSON" ]]; then
    info "Pruning installed_plugins.json…"
    local tmp
    tmp="$(mktemp)"
    jq --arg m "$MARKETPLACE_NAME" \
      '.plugins |= with_entries(select(.key | endswith("@" + $m) | not))' \
      "$INSTALLED_JSON" > "$tmp" && mv "$tmp" "$INSTALLED_JSON"
  fi

  success "Cache cleaned."
}

# ── remove + re-add marketplace ───────────────────────────────────────────────
reinstall_marketplace() {
  info "Removing marketplace '${MARKETPLACE_NAME}'…"
  claude plugin marketplace remove "$MARKETPLACE_NAME" 2>/dev/null || true

  info "Re-adding marketplace from '${MARKETPLACE_SOURCE}'…"
  claude plugin marketplace add "$MARKETPLACE_SOURCE"
  success "Marketplace added."
}

# ── install plugins ───────────────────────────────────────────────────────────
install_plugins() {
  if [[ ${#PLUGINS[@]} -eq 0 ]]; then
    warn "No plugins to install."
    return
  fi
  for plugin in "${PLUGINS[@]}"; do
    info "Installing ${plugin}@${MARKETPLACE_NAME}…"
    claude plugin install "${plugin}@${MARKETPLACE_NAME}"
    success "${plugin} installed."
  done
}

# ── main ──────────────────────────────────────────────────────────────────────
echo ""
echo "━━━ reload-marketplace: ${MARKETPLACE_NAME} ━━━"
echo ""

resolve_source

if [[ ${#PLUGINS[@]} -eq 0 ]]; then
  discover_plugins
fi

echo "[ 1/4 ] Uninstalling plugins…"
uninstall_plugins

echo "[ 2/4 ] Cleaning cache…"
cleanup_cache

echo "[ 3/4 ] Re-adding marketplace…"
reinstall_marketplace

echo "[ 4/4 ] Installing plugins…"
install_plugins

echo ""
echo "━━━ Done. Restart Claude Code to pick up the changes. ━━━"
echo ""
