#!/usr/bin/env bash
# Copyright (c) Microsoft Corporation.
# SPDX-License-Identifier: MIT
#
# post-command.sh
# Prepares Codex state and sanitizes host-derived Git config for the dev container.

set -euo pipefail

readonly CODEX_HOME_PATH="${CODEX_HOME:-/home/vscode/.codex}"

err() {
  printf "ERROR: %s\n" "$1" >&2
  exit 1
}

prune_invalid_safe_directories() {
  local gitconfig_path="${HOME}/.gitconfig"
  local temp_path

  [[ -f "${gitconfig_path}" ]] || return 0

  temp_path="$(mktemp)"

  awk '
    BEGIN { in_safe = 0 }
    { sub(/\r$/, "", $0) }
    /^\[.*\]$/ {
      in_safe = ($0 == "[safe]")
      print
      next
    }
    in_safe && $0 ~ /^[[:space:]]*directory = [A-Za-z]:\// { next }
    { print }
  ' "${gitconfig_path}" > "${temp_path}"

  if cmp -s "${gitconfig_path}" "${temp_path}"; then
    rm -f "${temp_path}"
    return 0
  fi

  mv "${temp_path}" "${gitconfig_path}"
}

prepare_codex_home() {
  local current_user="$1"
  local current_group="$2"

  sudo mkdir -p "${CODEX_HOME_PATH}"
  sudo chown -R "${current_user}:${current_group}" "${CODEX_HOME_PATH}"
}

install_codex_cli() {
  if npm list -g @openai/codex --depth=0 >/dev/null 2>&1; then
    return 0
  fi

  npm install -g @openai/codex
}

main() {
  local current_user
  local current_group
  local mode="${1:-create}"

  current_user="$(id -un)"
  current_group="$(id -gn)"

  case "${mode}" in
    create)
      prune_invalid_safe_directories
      prepare_codex_home "${current_user}" "${current_group}"
      install_codex_cli
      ;;
    start)
      prune_invalid_safe_directories
      ;;
    *)
      err "Unsupported mode: ${mode}"
      ;;
  esac
}

main "$@"