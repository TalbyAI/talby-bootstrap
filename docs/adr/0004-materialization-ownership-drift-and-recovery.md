# ADR-0004: Materialization Ownership, Drift, and Recovery

## Status

Accepted for v1.

## Context

Materialization writes files and fragments into repositories that users may edit manually. The CLI must avoid silent data loss and make ownership auditable.

## Decision

Each **Managed Artifact** has a **Materialization Record** tracking whole files and bounded **Fragments** written by that artifact.

Ownership is exclusive. If two **Managed Artifacts** claim the same whole file or overlapping fragment region, materialization fails with an **Ownership Conflict**.

Visible **Fragment Boundaries** are the default mechanism for managed fragments. **Fragment Drift** and **Whole-File Drift** are detected by comparing current content against the prior **Materialization Record**.

By default:

- drift prompts before update or removal;
- removal prompts before deleting a **Managed Artifact** no longer present in final desired state;
- whole-file content owned by the same **Managed Artifact** may be overwritten automatically only when it still matches the prior **Materialization Record**.

Rollback is verified best-effort only. If rollback cannot be completed and verified, the operation records **Recovery State**.

## Consequences

- User edits are not overwritten silently.
- Managed changes can be traced back to artifact, source, source version, and ownership kind.
- V1 does not promise transactional filesystem semantics it cannot guarantee.
- Removal is based on final resolved managed artifacts, not prior manifest declaration shape.
