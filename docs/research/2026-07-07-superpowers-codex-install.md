# Superpowers non-interactive Codex install check

Date: 2026-07-07

## Question

Can Superpowers be installed for Codex non-interactively after a devcontainer rebuild on a host that has not already executed Codex plugin setup for that user home?

## Sources

- Official repository: <https://github.com/obra/superpowers>
- Local Codex CLI help:
  - `codex plugin --help`
  - `codex plugin add --help`
  - `codex plugin marketplace --help`

## Findings

- The Superpowers repository documents Codex installation via the interactive plugin UI, not via a documented non-interactive Codex CLI command.
- Codex CLI does support non-interactive plugin installation in general with:
  - `codex plugin add <plugin@marketplace>`
- In the existing user home on this machine, `superpowers@openai-curated` is already installed and enabled.
- In a fresh writable `CODEX_HOME` with copied authentication state but no existing plugin state:
  - `codex login status` succeeds
  - `codex plugin marketplace list` reports `No plugin marketplaces in scope.`
  - `codex plugin list` reports `No marketplace plugins found.`
  - `codex plugin add superpowers@openai-curated --json` fails with:
    - `plugin 'superpowers' was not found in marketplace 'openai-curated'`

## Conclusion

For a truly fresh Codex home, authentication alone does not make the official `openai-curated` marketplace available to the CLI. Based on this check, a rebuild on a host that has never initialized Codex plugin state for that user cannot currently rely on just:

```bash
codex plugin add superpowers@openai-curated
```

Something else must seed the official marketplace first. In this environment, that seeding appears to have happened outside the tested CLI flow.

## Practical implication

If your devcontainer post-command must work on a first-run host with a brand-new Codex home, the "official plugin only, no manual marketplace setup" path is not yet proven by CLI alone.

If the base image or bootstrap step already pre-seeds the official Codex marketplace for the target user, then the one-line install command is viable:

```bash
codex plugin add superpowers@openai-curated
```

## Direct repository installation

- A direct non-interactive installation path from the Superpowers repository did succeed in a fresh authenticated `CODEX_HOME`.
- Commands used:

```bash
codex plugin marketplace add https://github.com/obra/superpowers
codex plugin add superpowers@superpowers-dev
```

- The repository was registered by Codex as marketplace `superpowers-dev`.
- Installing from that marketplace succeeded with:
  - plugin id: `superpowers@superpowers-dev`
  - version: `6.1.1`
  - final status: `installed, enabled`

## Practical takeaway

If you want a first-run-safe devcontainer automation path without depending on the official marketplace already being seeded, installing Superpowers from its GitHub repository is viable.
