# ADR-0001: CLI Surfaces and Command Model

## Status

Accepted for 0.1.

## Context

The first public slice needs one predictable command surface for installing artifacts. Broader maintenance and discovery surfaces are deferred until their persistence and acquisition contracts exist.

## Decision

The canonical executable is `tbboot`.

0.1 exposes the implemented `install` surface only. `upgrade`, `search`, and `catalog` are future command surfaces and are not successful placeholders.

`tbboot install` without arguments runs **Sync**. `tbboot install <target>` declares and applies one explicit target. 0.1 accepts only one explicit install target per command.

Explicit source installs use:

```text
tbboot install <source-ref> [--artifact <artifact-name>]
```

The direct source form is `tbboot install <source-ref> [--artifact <artifact-name>]`, where the current implementation acquires an in-root `file:` Source and accepts whole-file `file` steps. Invalid or unsupported references fail before mutation.

`--declare-only` updates only the **Manifest** and does not materialize artifacts, update the **Lockfile**, or touch cache state.

## Consequences

- Install behavior is predictable enough for CI and scripts.
- Catalog discovery, search, and upgrade remain deferred rather than claiming implemented behavior.
- Git identity is stored by the persistence contract but acquisition is deferred.
- Multi-target install and broader filesystem safety remain later tickets.
- **Sync** remains an internal operation name, not a top-level command.
