# ADR-0004: Materialization Ownership, Drift, and Recovery

## Status

Accepted for 0.1.

## Context

The 0.1 materializer writes whole files into repositories that users may edit manually. The CLI must avoid silent ownership conflicts, require explicit pruning for removal, and preserve enough provenance for later safety work.

## Decision

Each **Managed Artifact** has a **Materialization Record** containing its canonical root-relative files, `sha256:` digests, Source Reference, Source Version, and artifact version. Ownership is exclusive: two managed artifacts may not claim the same whole-file path.

The current preflight detects changed content, missing paths, symlinks, non-regular entries, and unsafe parent topology before writes. A file already owned by the same artifact may be rewritten only when it still matches the prior record. Materialization accepts whole-file `file` steps only; fragments and other step types are deferred.

Mutating operations use one canonical Operation Root and an exclusive root-scoped operation lock. The lock is acquired before repository-state reads or Source resolution. Preflight rejects unsafe source or target topology, including symlinks, reparse points, special files, root escapes, target collisions, and changed filesystem identity. Root, existing parents, opened directories, and targets are revalidated immediately before replacement and state persistence.

Whole-file replacement uses a same-directory temporary file followed by atomic rename. Existing file permission bits are preserved and new files use `0644`. Dry Run performs the same read, resolve, and preflight work without acquiring the mutation lock or writing state or target files; it reports planned changes and never clears Recovery State.

**Recovery State** has a strict schema-version-1 persistence format at `tbboot-artifacts.recovery.yaml`. It stores only a fixed error code, sanitized summary, canonical root-relative observations, expected state, safe digests, modes, and optional ownership metadata. It never stores raw errors or prior contents. Targetless `tbboot install --prune` removes only unchanged managed whole files after complete preflight.

Each non-Dry Run operation journals prior target state in memory immediately before every materialization, Manifest, Lockfile, or Materialization Record mutation. Controlled mutation failure rolls entries back in reverse order, attempts and re-observes every restoration, verifies prior bytes, permission bits, absence, and created-directory cleanup, and records sanitized Recovery State when final state remains unverified. Verified final state counts as restored even when the restoration action reported an error. Recovery State itself is accepted as recorded only after rooted observation confirms a regular file with mode `0600`, strict reload reproduces the intended values, and revalidation detects no change during verification.

Later Install and Sync operations inspect Recovery State before normal state loading or Source resolution. Mismatches and changes during the current inspect/revalidate window are user-action conflicts; Recovery State does not claim to detect safe parent-directory replacement between processes because it stores no directory identities. Matching non-Dry Run operations clear and verify its absence, while Dry Run never clears it. Clear failures remain sanitized operational errors. Human output reports only the fixed recovery code and canonical paths; JSON uses a private command DTO with nested `expected` data; neither output emits the persisted summary. Durable backups, process-crash recovery, and crash-recoverable lock takeover remain deferred.

## Consequences

- User edits are not overwritten silently when they create drift or ownership conflicts.
- Managed changes can be traced to their artifact, source, source version, and owned paths.
- Controlled in-process failures receive verified best-effort rollback without promising process-crash recovery.
- Unverified restoration becomes explicit, sanitized Recovery State that blocks later work until repaired.
