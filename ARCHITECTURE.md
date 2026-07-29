# Talby Bootstrap Architecture

This document is the entry point for the Talby Bootstrap 0.1 architecture. `CONTEXT.md` is the product contract; this file indexes the architectural decisions that turn that language into implementation constraints.

## Product 0.1 boundary

The 0.1 implementation publishes six strict schema-version-1 YAML documents with canonical `tbboot-` filenames. Consumer-facing and persisted Source References are scalar `file:<locator>` or `git:<locator>` values. The implemented acquisition path is local `file:` Sources: in-root Sources are allowed by default, while external absolute Sources require explicit Manifest approval of their Source Identity. Materialization uses whole-file `file` steps.

Git identity storage is present for the contract, but Git acquisition is deferred. Catalogs, search, upgrade, executable or prompt-driven steps, fragment/template rendering, durable backups, process-crash recovery, crash-recoverable lock takeover, and persisted operation logs are outside 0.1 and must not be represented as successful CLI behavior.

## Architecture shape

Talby Bootstrap is a CLI named `tbboot` that reconciles reusable repository artifacts into a consumer repository or folder.

The architecture is organized around five stable responsibilities:

- **Command surface**: parses the implemented `install` command into explicit user intent; later command surfaces remain separate tickets.
- **Resolution**: turns a **Manifest** and install target into an exact **Resolution** recorded in a **Lockfile**.
- **Trust policy**: allows in-root local Sources by default and requires Manifest approval for external absolute local Source Identities before writes; broader source and step policy is deferred.
- **Materialization**: applies whole-file `file` steps and tracks the existing ownership record.
- **Operation reporting**: emits the current human and machine-readable install results.

## Persistence and filesystem safety

The six persisted YAML documents use integer `schema_version: 1`, strict readers, deterministic writers, and canonical `tbboot-` filenames. Consumer-facing and persisted Source References are scalar `file:<locator>` or `git:<locator>` values. The 0.1 implementation stores and validates Git identities but acquires local `file:` Sources only; it does not migrate earlier filenames or structured source objects.

Mutating operations resolve one canonical Operation Root, acquire an exclusive root-scoped operation lock before reading state or resolving sources, and revalidate root, parent, source, and target identity before writes. Source and target paths must remain confined to their allowed roots and must reject symlinks, reparse points, special files, unsafe topology, and identity-changing races. File replacement uses a same-directory temporary file and atomic rename; existing modes are preserved and new files use `0644`.

Each non-Dry Run Install or Sync operation records prior bytes, permission bits, absence, and parent topology in an in-memory mutation journal immediately before every materialization, Manifest, Lockfile, or Materialization Record mutation. Controlled failures roll journal entries back in reverse order, attempt every restoration, remove operation-created directories, and re-observe each target. Verified final state determines rollback success even when a restoration action reported an error.

If the acquired Operation Root is moved and replaced, loss of its identity stops rollback and Recovery State persistence without touching the replacement. This is an operational safe stop, not successful rollback.

When rollback remains unverified, the operation writes sanitized Recovery State. The write is accepted only after rooted observation confirms a regular file with mode `0600`, strict reload reproduces the intended value, and topology revalidation detects no change. Later Install and Sync operations inspect Recovery State before normal state loading or Source resolution; mismatches block as user-action conflicts, matching non-Dry Run operations clear and verify its absence, and Dry Run never clears it.

Dry Run follows the same read, resolve, and preflight path without acquiring the mutation lock or writing Manifest, Lockfile, Materialization Record, Recovery State, or target files. Planned changes are reported as `planned`; unchanged operations remain `no_op`.

## Decision index

- [ADR-0001: CLI surfaces and command model](docs/adr/0001-cli-surfaces-and-command-model.md)
- [ADR-0002: Source resolution, versioning, and locking](docs/adr/0002-source-resolution-versioning-and-locking.md)
- [ADR-0003: Trust policy and risky materialization](docs/adr/0003-trust-policy-and-risky-materialization.md)
- [ADR-0004: Materialization ownership, drift, and recovery](docs/adr/0004-materialization-ownership-drift-and-recovery.md)
- [ADR-0005: Operation output and exit codes](docs/adr/0005-operation-output-logs-and-exit-codes.md)

## Non-functional requirements

- Reproducibility: resolved versions are recorded in a versioned **Lockfile**.
- Safety: in-root local Sources are allowed by default; external absolute local Sources require explicit approval, and materialization uses whole-file steps.
- Auditability: managed changes report provenance in immediate operation output.
- Predictability: scalar Source References and strict YAML fail before mutation when invalid.
- Minimal 0.1 scope: YAML is the only manifest format; only `file:` Sources are acquired.

## Deferred beyond 0.1

- Git acquisition and other Source Types.
- Catalog browsing, search, upgrade, and catalog maintenance commands.
- Fragment, template, script, and prompt materialization.
- Crash-recoverable operation locks, durable backups, and process-crash recovery.
- Rich version constraints, source/materialization caches, and persisted operation logs.
