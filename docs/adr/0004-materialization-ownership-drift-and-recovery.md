# ADR-0004: Materialization Ownership, Drift, and Recovery

## Status

Accepted for v1.

## Context

Materialization writes files and fragments into repositories that users may edit manually. The CLI must avoid silent data loss and make ownership auditable.

## Decision

Each **Managed Artifact** has a **Materialization Record** tracking whole files written by that artifact.

Ownership is exclusive. If two **Managed Artifacts** claim the same canonical whole-file path, materialization fails with an **Ownership Conflict**.

**Whole-File Drift** is detected by comparing current content and path topology against the prior **Materialization Record**.

By default:

- preflight accumulates all detectable drift and ownership conflicts, then blocks the entire mutation without writing; **Dry Run** reports the same preflight conflicts but does not claim to detect mutation-time races;
- removal requires explicit `--prune` on targetless `tbboot install`, operates only on final desired state resolved from the complete **Manifest**, and never derives removal from a targeted install subset;
- `--prune` removes an undesired **Managed Artifact** only when every owned whole file still matches its **Materialization Record**;
- whole-file content owned by the same **Managed Artifact** may be overwritten automatically only when it still matches the prior **Materialization Record**.

An identical unmanaged regular file at an unclaimed canonical path may be adopted without rewriting it. **Whole-File Drift** includes changed content, absence, symlinks, non-regular entries, and changed parent topology; permission-only differences are outside v1 drift, while permissions that prevent inspection are operational errors.

After successful preflight, mutation uses deterministic order: write desired files, remove pruned files, then persist repository-state files using prior in-memory backups. Adoption does not rewrite an identical file; it only changes the resulting **Materialization Record**.

Rollback is verified best-effort for controlled in-process failures. It preserves prior bytes, modes, and absence in memory for affected materialized and repository-state files, restores paths in reverse order, and attempts every restoration even after one fails. Verification requires safe parent topology, expected kind, content or absence, and mode where the platform supports it. It records directories created by the operation and removes them in reverse order using non-recursive removal only while they remain empty. It does not guarantee recovery from process termination, power loss, or filesystem failure; durable backup and journaling are outside v1.

If rollback cannot be completed and verified, the operation writes **Recovery State** without following symlinks and atomically replaces the reserved root path `talby-artifacts.recovery.yaml`, using mode `0600` where supported. The record contains a schema version, stable `rollback_incomplete` error code, sanitized summary, and canonical root-relative path observations; owner is optional for repository-state files. Each restoration is `restored` only after verification, `restore_failed` when the restoration operation fails, or `verification_failed` when the operation succeeds but verification does not. It never stores raw errors or prior contents.

Failure to persist **Recovery State** retains exit code `1`, emits a prominent warning, and sets `recovery_state_persisted: false` in JSON because future blocking cannot be guaranteed. An incomplete rollback that persists the record sets that field to `true`; the field is omitted when rollback succeeds or is unnecessary. Later mutations against the same repository root remain blocked until manual repair matches every recorded prior observation; unrelated global configuration or external catalog state is not necessarily blocked. A later mutating run verifies the repair, exits `2` and preserves the record on mismatch, or atomically removes the record before normal preflight; failure to remove it exits `1` before mutation. **Dry Run** may inspect but never clear Recovery State.

## Consequences

- User edits are not overwritten silently.
- Non-interactive behavior is deterministic: drift and unapproved removal use exit code `2` rather than prompting.
- Managed changes can be traced back to artifact, source, source version, and ownership kind.
- V1 does not promise transactional filesystem semantics it cannot guarantee.
- Removal is based on final resolved managed artifacts, not prior manifest declaration shape.
