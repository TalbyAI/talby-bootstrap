# Ubiquitous Language

`CONTEXT.md` is the closed source for Talby Bootstrap v1 domain language. This document is the compact glossary used by architecture, ADRs, and implementation work.

## Artifact publication

| Term                          | Definition                                                                                                         | Aliases to avoid                       |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------ | -------------------------------------- |
| **Artifact**                  | The canonical installable unit resolved from a source and materialized into a target repository or folder.         | Pattern, snippet, module               |
| **Artifact Name**             | The source-local identifier for an **Artifact**.                                                                   | Global ID, path                        |
| **Artifact Descriptor**       | The manifest published by an **Artifact** that declares version, metadata, and materialization steps.              | Inline metadata, inferred manifest     |
| **Materialization Step**      | One declared action in an **Artifact Descriptor** that writes, updates, renders, or executes installation content. | Artifact type, hidden install behavior |
| **Materialization Step Type** | The category of a **Materialization Step**: `file`, `fragment`, `template`, `script`, or `prompt` in v1.           | Artifact kind, source type             |

## Source and discovery

| Term                  | Definition                                                                                                      | Aliases to avoid                        |
| --------------------- | --------------------------------------------------------------------------------------------------------------- | --------------------------------------- |
| **Source**            | A concrete origin that can resolve and deliver one or more **Artifacts**.                                       | Catalog                                 |
| **Source Type**       | The explicit classification that determines how the CLI connects to a **Source**; v1 supports `file` and `git`. | Inferred protocol, source guess         |
| **Source Descriptor** | The manifest published by a **Source** that declares provided artifacts and resolution metadata.                | Folder inference, implicit structure    |
| **Source Version**    | The version or snapshot identifier for a resolved state of a **Source** as a whole.                             | Artifact version, floating state        |
| **Catalog**           | An index listing available **Artifacts**, the **Sources** that provide them, and available versions.            | Source, registry                        |

## Consumer repository state

| Term                       | Definition                                                                                    | Aliases to avoid                   |
| -------------------------- | --------------------------------------------------------------------------------------------- | ---------------------------------- |
| **Manifest**               | The versioned file that declares desired installed artifacts and stable source identity.      | Lockfile, machine config           |
| **Resolution**             | The exact resolved artifact versions and origins derived from a **Manifest**.                 | Intent, manifest                   |
| **Lockfile**               | The versioned record of a **Resolution** for reproducible installation.                       | Manifest, machine config           |
| **Managed Artifact**       | An installed **Artifact** tracked by the CLI as part of repository-managed configuration.     | Untracked file, incidental content |
| **Materialization Record** | The tracked record of what a **Managed Artifact** wrote into the target repository or folder. | Best-effort guess, inferred state  |

## Policy and safety

| Term                   | Definition                                                                                                  | Aliases to avoid                             |
| ---------------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| **Trust Policy**       | The versioned security policy controlling approved sources, step types, and age constraints.                | Machine default, ad hoc flag                 |
| **Minimum Age Rule**   | A trust constraint rejecting resolved targets that are newer than the configured age threshold.             | Source age, catalog age                      |
| **Overwrite Policy**   | The rule for replacing existing files during materialization.                                               | Merge strategy, sync mode                    |
| **Removal Policy**     | The rule for handling a **Managed Artifact** no longer present in final desired state.                      | Garbage collection, overwrite policy         |
| **Ownership Conflict** | A conflict where two **Managed Artifacts** claim the same whole file or overlapping fragment region.        | Shared ownership, last-write-wins            |
| **Recovery State**     | The failure state recorded when materialization stops after partial writes and rollback cannot be verified. | Silent partial failure, implicit dirty state |

## Command and reporting

| Term                  | Definition                                                                                  | Aliases to avoid                         |
| --------------------- | ------------------------------------------------------------------------------------------- | ---------------------------------------- |
| **CLI Command Name**  | The executable name `tbboot`.                                                               | talby, tboot                             |
| **Install Command**   | The primary command for declaring, reconciling, and materializing artifacts.                | sync command, add command                |
| **Sync**              | The reconciliation operation that aligns actual state with the **Manifest**.                | Top-level command, apply-only command    |
| **Upgrade Command**   | The command that advances already-declared artifacts or sources to newer resolved versions. | Sync alias, catalog refresh              |
| **Search Command**    | The command that queries configured catalog caches and returns matching **Sources**.        | catalog admin command, repository search |
| **Logs Command**      | The command that inspects recorded operations for an **Operation Root**.                    | Install-only subcommand, debug flag      |
| **Operation Summary** | The default concise human-readable output after install or sync.                            | Full verbose log, success-only silence   |
| **Operation Log**     | The replayable record of what happened during an operation.                                 | Default output, transient debug spew     |
| **Exit Code**         | The process result code classifying command outcome for shells, CI, and automation.         | Message parsing, ad hoc status           |

## Relationships

- A **Source** provides one or more **Artifacts**.
- An **Artifact Name** is unique only within its **Source**.
- The stable identity of an **Artifact** is **Source** plus **Artifact Name**.
- A **Manifest** declares intent; a **Resolution** makes it exact; a **Lockfile** records it.
- A **Catalog** indexes **Sources** but is not source of truth for artifacts.
- A **Managed Artifact** owns whole files or bounded **Fragments** through its **Materialization Record**.
- **Sync** reconciles the target repository against the **Manifest** and **Lockfile**.
- **Trust Policy** is checked before materialization writes.

## Example dialogue

> **Dev:** "If a user runs `tbboot install foo`, is `foo` globally unique?"
>
> **Domain expert:** "No. `foo` is an **Artifact Name**. It is only stable after resolution to a **Source** plus that artifact name."
>
> **Dev:** "So does the **Catalog** decide what gets installed?"
>
> **Domain expert:** "No. A **Catalog** is only an index. The **Source** and **Artifact Descriptor** define the artifact, and the **Lockfile** records the exact **Resolution**."
>
> **Dev:** "Can sync remove files silently if an artifact disappears?"
>
> **Domain expert:** "No. The **Removal Policy** prompts before removing a **Managed Artifact** by default, and drift is checked through the **Materialization Record**."

## Flagged ambiguities

- "source" and "catalog" are distinct: **Source** delivers, while **Catalog** indexes.
- "manifest" and "lockfile" are distinct: **Manifest** declares intent, **Lockfile** records exact resolution.
- "install" and "sync" are distinct: **Install Command** is the user-facing entrypoint, **Sync** is the underlying reconciliation operation.
- "artifact version" and **Source Version** are distinct: artifact versions live in **Artifact Descriptors**, while **Source Version** identifies a source snapshot.
