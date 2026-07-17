# ADR-0002: Source Resolution, Versioning, and Locking

## Status

Accepted for v1.

## Context

Artifacts can be discovered through catalogs or installed directly from a published source. Reproducibility requires exact resolved state, while user intent should remain editable and stable across different acquisition channels.

## Decision

The stable identity of an **Artifact** is **Source** plus **Artifact Name**. A **Catalog** is only an index and is not the source of truth.

A **Manifest** stores desired state and enough stable **Source Identity** to re-resolve declared targets. Any original user-facing reference is optional metadata only.

A **Resolution** records exact artifact versions and origins. A **Lockfile** persists the **Resolution** and lives beside the **Manifest** in the consumer repository.

Direct install version pinning applies to **Source Version** only. If no source version is provided:

- `git` resolves to the latest stable published **Source Version** allowed by policy for that acquired **Source Identity**.
- `file` records a local snapshot hash in the **Lockfile** for that acquired **Source Identity**.

Later syncs keep the previously resolved **Source Version** until the user explicitly upgrades. For `git:` targets, `upgrade` advances already-declared Sources or Artifacts to the latest stable published **Source Version** allowed by policy. For `file:` targets, it re-reads the current local path and records the resulting snapshot hash; an unchanged snapshot is a no-op. Source scope updates every declared Artifact in the Source snapshot, while Artifact scope updates only the selected Artifact and leaves sibling Artifacts on their existing snapshots. Successful upgrade writes only the affected exact resolutions to the **Lockfile** and leaves the **Manifest** unchanged.

## Consequences

- Reproducibility is versioned with the repository.
- Catalog loss or cache refresh does not redefine installed artifacts.
- Two acquisition channels that publish the same content are still distinct when they produce different **Source Identity** values.
- Artifact-level upgrade is rejected when the manifest declares the whole source.
- Rich version ranges are deferred until there is a measured need.
