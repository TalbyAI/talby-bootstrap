# Example library

This directory contains the shared example library for Talby Bootstrap.

- `scenarios/` holds broad end-to-end flows with fuller expected results.
- `atomic-cases/` holds focused examples that lock one main rule or contract.

## Required example files

Each example contains:

- `README.md` for the human explanation.
- `example.yaml` for machine-readable metadata.
- `consumer/` for the initial consumer repository state.
- `expected/` for the normative expected results.

Examples that acquire published content also contain `source/`.

## Metadata and verification

`example.yaml` is intentionally small and explicit. It identifies the example, represented argv commands, polarity, verification modes, and normative output paths. Verification may cover exact exit codes or exit-code classes, text, JSON, stderr, and consumer state. Metadata must agree with files present under `expected/`.

## Staging and execution

The runner stages each example into a temporary workspace, runs commands from the staged consumer root, and places a declared local `file:` Source under the alias used by the example. Tests use the existing in-process CLI path and closed stdin; they do not require a process-level binary runner or an interactive terminal.

## Execution status

Examples declare one status: `active`, `broken`, `skipped`, or `deprecated`. `active` must pass its verification. `broken` records known not-yet-implemented behavior, `skipped` is intentionally not executed, and `deprecated` is temporary documentation on the path to removal rather than a future executable target.

Keep the schema install-centered. Extend it only when another implemented command creates concrete fixture pressure.
