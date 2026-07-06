#!/usr/bin/env bash
# Copyright (c) Microsoft Corporation.
# SPDX-License-Identifier: MIT
#
# toolchain-audit.sh
# Captures and compares the devcontainer toolchain surface.

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  .devcontainer/toolchain-audit.sh snapshot [output_file]
  .devcontainer/toolchain-audit.sh compare <baseline_file> [current_file]

Commands:
  snapshot   Capture the current toolchain state to stdout or a file.
  compare    Compare a baseline snapshot with the current state or another file.
EOF
}

command_path_or_missing() {
  local command_name="$1"

  if command -v "${command_name}" >/dev/null 2>&1; then
    command -v "${command_name}"
  else
    printf 'missing\n'
  fi
}

emit_line() {
  local key="$1"
  local value="$2"

  printf '%s=%s\n' "${key}" "${value}"
}

normalize_snapshot() {
  sed \
    -e '/^meta.timestamp_utc=/d' \
    -e '/^meta.hostname=/d' \
    -e 's/(cached)/<cached>/g' \
    -E -e 's/[[:space:]][0-9]+\.[0-9]+s/<duration>/g'
}

run_capture() {
  local label="$1"
  shift

  local output
  local status=0

  if output="$("$@" 2>&1)"; then
    status=0
  else
    status=$?
  fi

  output="$(printf '%s' "${output}" | tr '\n' '|' | tr '\r' ' ' )"

  emit_line "check.${label}.status" "${status}"
  emit_line "check.${label}.output" "${output}"
}

write_snapshot() {
  emit_line "meta.timestamp_utc" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  emit_line "meta.hostname" "$(hostname)"
  emit_line "meta.cwd" "$PWD"

  emit_line "tool.go.path" "$(command_path_or_missing go)"
  emit_line "tool.node.path" "$(command_path_or_missing node)"
  emit_line "tool.npm.path" "$(command_path_or_missing npm)"
  emit_line "tool.gh.path" "$(command_path_or_missing gh)"
  emit_line "tool.rg.path" "$(command_path_or_missing rg)"
  emit_line "tool.docker.path" "$(command_path_or_missing docker)"
  emit_line "tool.pwsh.path" "$(command_path_or_missing pwsh)"
  emit_line "tool.pwsh_exe.path" "$(command_path_or_missing pwsh.exe)"
  emit_line "tool.just.path" "$(command_path_or_missing just)"

  run_capture "go_version" go version
  run_capture "node_version" node --version
  run_capture "npm_version" npm --version
  run_capture "gh_version" gh --version
  run_capture "rg_version" rg --version
  run_capture "docker_version" docker --version
  run_capture "pwsh_version" pwsh --version
  run_capture "just_version" just --version

  run_capture "go_env" go env GOVERSION GOOS GOARCH
  run_capture "just_list" just --list
  run_capture "just_check_go" just check-go
  run_capture "post_command_help" bash .devcontainer/post-command.sh unsupported
}

snapshot() {
  local output_file="${1:-}"

  if [[ -n "${output_file}" ]]; then
    write_snapshot > "${output_file}"
    printf 'Snapshot written to %s\n' "${output_file}"
  else
    write_snapshot
  fi
}

compare() {
  local baseline_file="$1"
  local current_file="${2:-}"
  local temp_file=""

  [[ -f "${baseline_file}" ]] || {
    printf 'ERROR: Baseline file not found: %s\n' "${baseline_file}" >&2
    exit 1
  }

  if [[ -z "${current_file}" ]]; then
    temp_file="$(mktemp)"
    trap 'rm -f "${temp_file}"' RETURN
    write_snapshot > "${temp_file}"
    current_file="${temp_file}"
  fi

  [[ -f "${current_file}" ]] || {
    printf 'ERROR: Current file not found: %s\n' "${current_file}" >&2
    exit 1
  }

  if diff -u --label baseline --label current \
    <(normalize_snapshot < "${baseline_file}") \
    <(normalize_snapshot < "${current_file}"); then
    printf 'Snapshots match.\n'
  else
    printf 'Snapshots differ.\n' >&2
    exit 1
  fi

}

main() {
  local command_name="${1:-}"

  case "${command_name}" in
    snapshot)
      shift
      snapshot "$@"
      ;;
    compare)
      shift
      [[ $# -ge 1 ]] || {
        usage
        exit 1
      }
      compare "$@"
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
