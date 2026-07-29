# Rollback and Recovery State design

## Goal

Protect `Install` and `Sync` materialization and repository-state writes from controlled in-process failures. Restore every completed mutation when possible, and persist sanitized Recovery State when any restoration cannot be verified.

Crash recovery and durable backups remain outside product 0.1.

## Mutation boundary

A non-Dry Run operation acquires the existing root operation lock before inspecting or changing Recovery State. One in-memory mutation journal covers:

- whole-file materialization creates, replacements, and removals;
- Manifest writes;
- Lockfile writes;
- Materialization Record writes; and
- directories created while writing those files.

Immediately before each mutation, the journal records an entry containing the target's prior bytes, permission bits, absence, and existing parent topology. The current entry therefore participates in rollback even when its mutation returns an error after making a partial change. Backups remain in memory and are never written to Recovery State.

The operation applies mutations in its existing deterministic order. It does not infer after failure whether a mutation changed filesystem state.

## Rollback

Any controlled mutation failure starts rollback. Rollback processes journal entries in reverse order and attempts every restoration even after an earlier restoration fails.

Restoration preserves prior regular-file bytes and permission bits. A previously absent target becomes absent again. Directories created by the operation are removed in reverse depth order when empty. Existing directories are never removed.

Each restoration is re-observed and verified against its captured prior state. A successful rollback returns the original operation error and leaves no Recovery State. Rollback failures are joined only for internal error handling; raw error text is not persisted.

## Incomplete rollback

If any target cannot be restored or verified, the operation writes `tbboot-artifacts.recovery.yaml` atomically with:

- schema version 1;
- code `rollback_incomplete`;
- one fixed, sanitized, single-line summary;
- canonical root-relative observations for every failed restoration or verification;
- expected absence, or expected digest and permission mode for a regular file; and
- optional canonical ownership metadata when the affected target belongs to a managed Artifact.

Created parent directories that could not be removed use an expected-absent observation. Observations contain neither prior contents nor raw errors. Recovery State uses the existing deterministic schema writer and mode `0600`.

If Recovery State itself cannot be written and verified, the operation returns the original failure joined with the recovery-write failure. It does not claim that recovery was recorded.

## Later operations

After acquiring the operation lock and validating root identity, every later `Install` or `Sync` checks for Recovery State before loading normal repository state, resolving Sources, or running preflight.

When no Recovery State exists, the operation continues normally. When it exists, every observation is compared with current state using canonical path handling:

- expected-absent paths must be absent;
- expected-file paths must be regular files with the recorded digest and permission mode; and
- unsafe or changed parent topology is a mismatch.

A mismatch returns a typed user-action conflict with stable code `rollback_incomplete` and performs no mutation or Source resolution.

A non-Dry Run operation whose observations all match atomically removes Recovery State, verifies its absence, then starts normal state loading and preflight. Failure to remove or verify removal leaves the operation blocked.

Dry Run performs the same inspection. It never removes Recovery State. It continues to normal resolution and preflight only when all observations match; otherwise it returns the same typed conflict.

## Reporting

The install service exposes a typed recovery conflict carrying sanitized observations. Human output identifies `rollback_incomplete` and the affected canonical paths. JSON output retains the shared numeric `code` for the user-action-conflict exit class and adds `recovery_code: "rollback_incomplete"` plus structured observations under `details`. Neither output includes prior contents, raw errors, or absolute paths.

The initial operation failure remains an operational error even when it creates Recovery State. A subsequent operation blocked by that state is a user-action conflict.

## Implementation shape

The transaction remains an internal implementation detail of `internal/install`. It reuses `internal/materialize` observation, digest, atomic replacement, removal, and path-confinement behavior. A small internal mutation hook permits deterministic controlled failures in tests; it is neither exported nor designed as an extension point.

Repository-state persistence gains only the minimum removal operation required to clear Recovery State atomically. No general transaction interface, durable journal, or new dependency is introduced.

## Testing seams

Tests exercise public `Service.Install` and `Service.Sync` behavior. Focused cases prove:

- successful rollback restores prior bytes, modes, absence, and created-directory topology;
- rollback runs in reverse order and attempts every restoration;
- state-file failure rolls back earlier target and state writes;
- incomplete restoration writes deterministic sanitized Recovery State;
- a mismatching Recovery State blocks before Source resolution and mutation;
- a repaired Recovery State is verified and cleared before a real operation;
- Dry Run inspects but never clears Recovery State; and
- human and JSON errors expose stable `rollback_incomplete` reporting without raw errors or prior contents.

Existing repository-state schema tests remain the seam for strict Recovery State serialization and validation.

## Canonical documentation

Implementation updates:

- `CONTEXT.md` to mark in-process rollback, blocking, verification, and clearing as implemented;
- `ARCHITECTURE.md` to describe the in-memory transaction and its crash-recovery boundary; and
- ADR-0004 to replace obsolete deferral language with the implemented lifecycle.

Specific obsolete claims are searched across canonical documents. Durable backups, process-crash recovery, and crash-recoverable lock takeover remain explicitly deferred.
