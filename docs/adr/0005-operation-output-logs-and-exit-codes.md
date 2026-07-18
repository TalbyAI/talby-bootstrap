# ADR-0005: Operation output and exit codes

## Status

Accepted for 0.1.

## Context

The first public CLI slice needs predictable install output for humans and automation without adding a persisted diagnostics subsystem.

## Decision

`install` writes concise human output to standard output on success and errors to standard error. The output identifies the operation, outcome, change kind, scalar Source Reference, Source Version when known, artifact, path, and ownership kind.

JSON output uses the shared envelope with `code`, `message`, `details`, and `warnings`. Provenance Source References are scalar strings in the JSON boundary. Success uses exit code `0`; operational or validation errors use `1`; user-action conflicts use `2`; trust or policy denials use `3`.

The 0.1 CLI persists no operation logs, replay data, operation identifiers, retention metadata, or verbosity configuration. Upgrade, catalog, and search output are deferred with those commands.

## Consequences

- Shells and CI can classify the current install result without parsing prose.
- Human output remains short and deterministic.
- Future commands can extend the envelope without changing the current install contract.
