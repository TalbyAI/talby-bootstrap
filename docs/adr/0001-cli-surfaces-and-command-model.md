# ADR-0001: CLI Surfaces and Command Model

## Status

Accepted for v1.

## Context

Talby Bootstrap needs one predictable command surface for users installing artifacts and a separate maintenance surface for catalogs. Ambiguous command targets would make automation unsafe and make troubleshooting harder.

## Decision

The canonical executable is `tbboot`.

V1 has two user-facing surfaces:

- **Artifact Management Surface** centered on `install`, `upgrade`, and `search`.
- **Catalog Management Surface** centered on `catalog add`, `catalog list`, `catalog refresh`, and `catalog remove`.

`tbboot install` without arguments runs **Sync**. `tbboot install <target>` declares and applies one explicit target. V1 accepts only one explicit install target per command.

Explicit source installs use:

```text
tbboot install <source-ref> [--artifact <artifact-name>] [--source-version <version>]
```

Explicit upgrades use:

```text
tbboot upgrade <source-ref> [--artifact <artifact-name>]
```

Ambiguous install syntax fails until the user provides an explicit form.

`--declare-only` updates only the **Manifest** and does not materialize artifacts, update the **Lockfile**, or touch cache state.

## Consequences

- Install behavior is predictable enough for CI and scripts.
- Catalog discovery stays separate from artifact reconciliation.
- Multi-target install is deferred to avoid prompt, conflict, and rollback complexity in v1.
- **Sync** remains an internal operation name, not a top-level command.
