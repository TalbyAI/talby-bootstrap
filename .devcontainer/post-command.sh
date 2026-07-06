#!/usr/bin/env bash
# Copyright (c) Microsoft Corporation.
# SPDX-License-Identifier: MIT
#
# post-command.sh
# Prepares Codex state and sanitizes host-derived Git config for the dev container.

set -euo pipefail

readonly AGENTS_HOME_PATH="${AGENTS_HOME:-${HOME}/.agents}"
readonly CODEX_HOME_PATH="${CODEX_HOME:-/home/vscode/.codex}"
readonly GH_CONFIG_DIR_PATH="${GH_CONFIG_DIR:-${HOME}/.config/gh}"
readonly BASH_HISTORY_DIR_PATH="${BASH_HISTORY_DIR:-${HOME}/.local/share/bash-history}"
readonly BASH_HISTORY_FILE_PATH="${BASH_HISTORY_FILE:-${BASH_HISTORY_DIR_PATH}/history}"

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

prepare_agents_home() {
  local current_user="$1"
  local current_group="$2"

  sudo mkdir -p "${AGENTS_HOME_PATH}"
  sudo chown -R "${current_user}:${current_group}" "${AGENTS_HOME_PATH}"
}

prepare_gh_config_home() {
  local current_user="$1"
  local current_group="$2"

  sudo mkdir -p "${GH_CONFIG_DIR_PATH}"
  sudo chown -R "${current_user}:${current_group}" "${GH_CONFIG_DIR_PATH}"
}

configure_bash_history() {
  local current_user="$1"
  local current_group="$2"
  local bashrc_path="${HOME}/.bashrc"
  local marker_start="# talby-bootstrap bash history start"
  local marker_end="# talby-bootstrap bash history end"

  sudo mkdir -p "${BASH_HISTORY_DIR_PATH}"
  sudo touch "${BASH_HISTORY_FILE_PATH}"
  sudo chown -R "${current_user}:${current_group}" "${BASH_HISTORY_DIR_PATH}"

  if grep -Fq "${marker_start}" "${bashrc_path}"; then
    return 0
  fi

  cat <<EOF >> "${bashrc_path}"

${marker_start}
export HISTFILE="${BASH_HISTORY_FILE_PATH}"
export HISTSIZE=10000
export HISTFILESIZE=20000
shopt -s histappend

__talby_history_sync() {
    history -a
    history -n
}

if [[ ";${PROMPT_COMMAND:-};" != *";__talby_history_sync;"* ]]; then
    PROMPT_COMMAND="__talby_history_sync${PROMPT_COMMAND:+; ${PROMPT_COMMAND}}"
fi
${marker_end}
EOF
}

configure_shell_aliases() {
  local bashrc_path="${HOME}/.bashrc"
  local marker_start="# talby-bootstrap aliases start"
  local marker_end="# talby-bootstrap aliases end"

  if grep -Fq "${marker_start}" "${bashrc_path}"; then
    return 0
  fi

  cat <<EOF >> "${bashrc_path}"

${marker_start}
alias cy='codex --yolo'
${marker_end}
EOF
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
      prepare_agents_home "${current_user}" "${current_group}"
      prepare_codex_home "${current_user}" "${current_group}"
      prepare_gh_config_home "${current_user}" "${current_group}"
      configure_bash_history "${current_user}" "${current_group}"
      configure_shell_aliases
      install_codex_cli
      ;;
    start)
      prune_invalid_safe_directories
      prepare_agents_home "${current_user}" "${current_group}"
      prepare_gh_config_home "${current_user}" "${current_group}"
      configure_bash_history "${current_user}" "${current_group}"
      configure_shell_aliases
      ;;
    *)
      err "Unsupported mode: ${mode}"
      ;;
  esac
}

main "$@"