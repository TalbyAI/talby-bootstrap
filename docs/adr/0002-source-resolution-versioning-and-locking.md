# ADR-0002: Source Resolution, Versioning, and Locking

## Status

Accepted for 0.1.

## Context

The first public slice installs directly from a published local Source. Reproducibility requires exact resolved state, while user intent should remain editable and stable across acquisition channels that are added later.

## Decision

The stable identity of an **Artifact** is **Source** plus **Artifact Name**. A **Catalog** is only an index and is not the source of truth.

A **Manifest** stores desired state and enough stable **Source Identity** to re-resolve declared targets. Persisted Source References are scalar `file:<locator>` or `git:<locator>` values; the internal model remains structured. Any original user-facing reference is optional metadata only.

A **Resolution** records exact artifact versions and origins. A **Lockfile** persists the **Resolution** and lives beside the **Manifest** in the consumer repository.

For 0.1, local `file:` Sources are acquired. In-root Sources are allowed by default, while external absolute Sources require explicit **Manifest** approval of their **Source Identity**. The Source Version is a deterministic `sha256:` snapshot hash recorded in the Lockfile. Git identities and versions have canonical validation/storage rules, but Git acquisition is deferred.

## Consequences

- Reproducibility is versioned with the repository.
- No catalog or cache is consulted by the 0.1 direct install path.
- Two acquisition channels that publish the same content are still distinct when they produce different **Source Identity** values.
- Upgrade behavior and rich version ranges are deferred until there is a measured need.
