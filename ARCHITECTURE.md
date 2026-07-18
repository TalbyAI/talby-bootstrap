# Talby Bootstrap Architecture

This document is the entry point for the Talby Bootstrap 0.1 architecture. `CONTEXT.md` is the product contract; this file indexes the architectural decisions that turn that language into implementation constraints.

## Product 0.1 boundary

The 0.1 implementation publishes six strict schema-version-1 YAML documents with canonical `tbboot-` filenames. Consumer-facing and persisted Source References are scalar `file:<locator>` or `git:<locator>` values. The implemented acquisition path is local `file:` Sources: in-root Sources are allowed by default, while external absolute Sources require explicit Manifest approval of their Source Identity. Materialization uses whole-file `file` steps.

Git identity storage is present for the contract, but Git acquisition is deferred. Catalogs, search, upgrade, executable or prompt-driven steps, fragment/template rendering, durable rollback lifecycle, and persisted operation logs are outside 0.1 and must not be represented as successful CLI behavior.

## Architecture shape

Talby Bootstrap is a CLI named `tbboot` that reconciles reusable repository artifacts into a consumer repository or folder.

The architecture is organized around five stable responsibilities:

- **Command surface**: parses the implemented `install` command into explicit user intent; later command surfaces remain separate tickets.
- **Resolution**: turns a **Manifest** and install target into an exact **Resolution** recorded in a **Lockfile**.
- **Trust policy**: allows in-root local Sources by default and requires Manifest approval for external absolute local Source Identities before writes; broader source and step policy is deferred.
- **Materialization**: applies whole-file `file` steps and tracks the existing ownership record.
- **Operation reporting**: emits the current human and machine-readable install results.

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
- Full filesystem race protection, operation locking, prune, and rollback lifecycle.
- Rich version constraints, source/materialization caches, and persisted operation logs.
