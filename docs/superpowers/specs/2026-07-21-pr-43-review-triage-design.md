# PR 43 review triage design

## Goal

Resolve the one open review finding on PR 43 by making the remediation plan use
the CLI operation name consistently.

## Current state

The CLI exposes targetless reconciliation as `tbboot install`; that command
delegates to the internal `Service.Sync` operation. The remediation plan calls
the regression “targetless Sync” but its executable example correctly invokes
targetless `install`.

The existing Go test, `TestSyncJSONIncludesUnsafeTopologyConflict`, already
exercises the correct CLI path. No production behavior or test logic needs
changing.

## Decision

Use the minimal documentation-only approach:

- Change the plan wording to targetless `install`, with an optional note that it
  exercises the Sync reconciliation path.
- Keep the `TestSync...` test name because it describes the underlying service
  operation and matches existing repository terminology.
- Do not add a `sync` CLI command.
- Do not resolve the GitHub thread until the wording change is validated.

## Scope and validation

Modify only `docs/superpowers/plans/2026-07-21-pr-review-remediation.md`.
Run `just check-md` and `git diff --check HEAD` after the edit.
