# Ponytail non-interactive Codex install check

Date: 2026-07-07

## Question

Can Ponytail be installed for Codex non-interactively, such as from a devcontainer post-command?

## Sources

- Official install page: <https://ponytail.dev/#install>
- Official repository: <https://github.com/DietrichGebert/ponytail>
- Local Codex CLI help:
  - `codex plugin --help`
  - `codex plugin marketplace add --help`
  - `codex plugin add --help`

## Findings

- The official Ponytail install page documents a Codex CLI command to add the marketplace:
  - `codex plugin marketplace add DietrichGebert/ponytail`
- Codex CLI in this environment supports non-interactive plugin installation commands:
  - `codex plugin marketplace add <source>`
  - `codex plugin add <plugin@marketplace>`
- A real installation test succeeded when `CODEX_HOME` pointed at a writable directory:
  - `env CODEX_HOME=/tmp/codex-ponytail-test codex plugin marketplace add DietrichGebert/ponytail --json`
  - `env CODEX_HOME=/tmp/codex-ponytail-test codex plugin add ponytail@ponytail --json`
- The installed plugin reported:
  - plugin id: `ponytail@ponytail`
  - version: `4.8.4`
  - status after install: `installed, enabled`
- The plugin manifest in the installed artifact declares Codex skills and lifecycle hooks via `.codex-plugin/plugin.json`.

## Environment-specific blockers observed

- Using the default `~/.codex` failed in this session because the home-backed Codex directory was read-only.
- The first sandboxed attempt also failed on network resolution to `github.com`.
- Those failures were environmental, not a limitation of Ponytail or the Codex plugin mechanism.

## Conclusion

Yes. Ponytail can be installed non-interactively for Codex, and the Codex CLI supports the necessary commands for automating it from a devcontainer post-command.

For automation to work reliably, the container needs:

- a writable `CODEX_HOME` or writable `~/.codex`
- network access to GitHub during installation
- a `codex` binary available in the container

## Suggested automation shape

```bash
codex plugin marketplace add DietrichGebert/ponytail
codex plugin add ponytail@ponytail
```

If the container's default home is not writable or not persisted, set `CODEX_HOME` to a writable persisted path before running those commands.
