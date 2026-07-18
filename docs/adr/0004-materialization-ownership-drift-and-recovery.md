# ADR-0004: Materialization Ownership, Drift, and Recovery

## Status

Accepted for 0.1.

## Context

The 0.1 materializer writes whole files into repositories that users may edit manually. The CLI must avoid silent ownership conflicts and preserve enough provenance for later safety work.

## Decision

Each **Managed Artifact** has a **Materialization Record** containing its canonical root-relative files, `sha256:` digests, Source Reference, Source Version, and artifact version. Ownership is exclusive: two managed artifacts may not claim the same whole-file path.

The current preflight detects changed content, missing paths, symlinks, non-regular entries, and unsafe parent topology before writes. A file already owned by the same artifact may be rewritten only when it still matches the prior record. Materialization accepts whole-file `file` steps only; fragments and other step types are deferred.

**Recovery State** has a strict schema-version-1 persistence format at `tbboot-artifacts.recovery.yaml`. It stores only a fixed error code, sanitized summary, canonical root-relative observations, expected state, safe digests, modes, and optional ownership metadata. It never stores raw errors or prior contents. Runtime rollback, recovery blocking, durable backup, prune, and cross-process filesystem safety are deferred to later tickets.

## Consequences

- User edits are not overwritten silently when they create drift or ownership conflicts.
- Managed changes can be traced to their artifact, source, source version, and owned paths.
- 0.1 does not promise transactional filesystem behavior that the current implementation does not provide.
- The persisted Recovery State contract can evolve independently from the later rollback lifecycle.
