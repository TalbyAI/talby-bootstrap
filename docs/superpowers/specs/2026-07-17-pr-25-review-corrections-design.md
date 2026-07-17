# PR 25 review corrections design

## Goal

Resolve the open PR 25 findings and the review-found documentation contradictions while keeping the v1 command surface internally consistent.

## Decisions

- V1 exposes only `tbboot upgrade`; `install --upgrade` is not supported.
- Upgrading a `git:` Source selects the latest stable published Source Version allowed by policy.
- Upgrading a `file:` Source re-reads its current local path and records the resulting snapshot hash.
- An unchanged `file:` snapshot is a no-op.
- Source-scoped upgrade updates every declared Artifact in that Source snapshot. Artifact-scoped upgrade updates only the selected Artifact and leaves sibling Artifacts on their existing snapshots.
- Upgrade leaves the Manifest unchanged and updates only the affected Lockfile resolutions.
- Only the ADR-0005 title is changed to sentence case; older ADR title cleanup is outside this PR.
- V1 has immediate diagnostics only: no `logs` command, persisted Operation Logs, replay, operation identifiers, retention policy, or verbosity levels.

## Documentation changes

Update `CONTEXT.md` to remove the remaining `install --upgrade` shortcut claim, define the `git:` and `file:` upgrade behavior, and reflect that behavior in the resolved decision lists.

Update ADR-0002 so its Source versioning decision distinguishes published `git:` versions from local `file:` snapshot hashes.

Update the ADR-0005 heading to sentence case.

Update ADR-0001, `ARCHITECTURE.md`, and `UBIQUITOUS_LANGUAGE.md` so the active canonical documentation no longer promises logs, replay, or verbosity. Keep auditability through immediate provenance and deterministic operation output.

Update the remaining version-selection summary in `CONTEXT.md` so it distinguishes `git:` latest-stable resolution from `file:` re-snapshot behavior.

No Go behavior changes are part of this correction.

## Validation

- Run `just check-md`.
- Search for `install --upgrade` and confirm no remaining text claims v1 support.
- Inspect the changed upgrade statements for agreement between `CONTEXT.md` and ADR-0002.
- Search active canonical documentation for logs, replay, and verbosity promises; none may remain.
