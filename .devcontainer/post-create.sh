#!/usr/bin/env bash
# Copyright (c) Microsoft Corporation.
# SPDX-License-Identifier: MIT
#
# post-create.sh
# Installs Codex CLI and prepares persistent Codex state for the dev container.

set -euo pipefail

readonly CODEX_HOME_PATH="${CODEX_HOME:-/home/vscode/.codex}"

main() {
  local current_user
  local current_group

  current_user="$(id -un)"
  current_group="$(id -gn)"

  sudo mkdir -p "${CODEX_HOME_PATH}"
  sudo chown -R "${current_user}:${current_group}" "${CODEX_HOME_PATH}"

  npm install -g @openai/codex
}

main "$@"