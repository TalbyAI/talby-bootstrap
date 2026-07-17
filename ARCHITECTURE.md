# Talby Bootstrap Architecture

This document is the entry point for Talby Bootstrap v1 architecture. `CONTEXT.md` is the closed domain source for v1; this file indexes the architectural decisions that turn that language into implementation constraints.

## Architecture shape

Talby Bootstrap is a CLI named `tbboot` that reconciles reusable repository artifacts into a consumer repository or folder.

The architecture is organized around five stable responsibilities:

- **Command surface**: parses artifact, catalog, search, and upgrade commands into explicit user intent.
- **Resolution**: turns a **Manifest** and install target into an exact **Resolution** recorded in a **Lockfile**.
- **Trust policy**: rejects unapproved sources, immature versions, and risky materialization steps before writes happen.
- **Materialization**: applies declared **Materialization Steps**, tracks ownership, detects drift, and records recovery state when rollback cannot be verified.
- **Operation reporting**: emits short human summaries, machine-readable JSON, and exit codes.

## Decision index

- [ADR-0001: CLI surfaces and command model](docs/adr/0001-cli-surfaces-and-command-model.md)
- [ADR-0002: Source resolution, versioning, and locking](docs/adr/0002-source-resolution-versioning-and-locking.md)
- [ADR-0003: Trust policy and risky materialization](docs/adr/0003-trust-policy-and-risky-materialization.md)
- [ADR-0004: Materialization ownership, drift, and recovery](docs/adr/0004-materialization-ownership-drift-and-recovery.md)
- [ADR-0005: Operation output and exit codes](docs/adr/0005-operation-output-logs-and-exit-codes.md)

## Non-functional requirements

- Reproducibility: resolved versions are recorded in a versioned **Lockfile**.
- Safety: remote sources and risky step types are denied until explicitly approved.
- Auditability: managed changes report provenance in immediate operation output.
- Predictability: ambiguous install targets fail instead of using precedence rules.
- Minimal v1 scope: only `file` and `git` source types are supported, and YAML is the only manifest format.

## Out of scope for v1

- Interactive catalog browsing.
- Source types beyond `file` and `git`.
- Rich version constraints or upgrade policies beyond latest stable allowed.
- Source or materialization caches beyond catalog metadata.
