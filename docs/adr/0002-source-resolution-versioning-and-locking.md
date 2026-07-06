# ADR-0002: Source Resolution, Versioning, and Locking

## Status

Accepted for v1.

## Context

Artifacts can be discovered through catalogs or installed directly from a source. Reproducibility requires exact resolved state, while user intent should remain editable and stable.

## Decision

The stable identity of an **Artifact** is **Source** plus **Artifact Name**. A **Catalog** is only an index and is not the source of truth.

A **Manifest** stores desired state and enough stable source identity to re-resolve declared targets. Any original user-facing reference is optional metadata only.

A **Resolution** records exact artifact versions and origins. A **Lockfile** persists the **Resolution** and lives beside the **Manifest** in the consumer repository.

Direct install version pinning applies to **Source Version** only. If no source version is provided:

- `git` resolves to the latest stable published **Source Version** allowed by policy.
- `file` records a local snapshot hash in the **Lockfile**.

Later syncs keep the previously resolved **Source Version** until the user explicitly upgrades. `upgrade` advances already-declared sources or artifacts to the latest stable version allowed by policy and writes the new exact version to the **Lockfile**.

## Consequences

- Reproducibility is versioned with the repository.
- Catalog loss or cache refresh does not redefine installed artifacts.
- Artifact-level upgrade is rejected when the manifest declares the whole source.
- Rich version ranges are deferred until there is a measured need.
