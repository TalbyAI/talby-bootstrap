# Ubiquitous Language

`CONTEXT.md` is the source of truth for Talby Bootstrap v1 domain language. This document is the compact glossary used by architecture, ADRs, and implementation work.

## Artifact publication

| Term                          | Definition                                                                                                         | Aliases to avoid                       |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------ | -------------------------------------- |
| **Artifact**                  | The canonical installable unit resolved from a source and materialized into a target repository or folder.         | Pattern, snippet, module               |
| **Artifact Name**             | The source-local identifier for an **Artifact**.                                                                   | Global ID, path                        |
| **Artifact Descriptor**       | The manifest published by an **Artifact** that declares version, metadata, and materialization steps.              | Inline metadata, inferred manifest     |
| **Materialization Step**      | One declared action in an **Artifact Descriptor** that writes, updates, renders, or executes installation content. | Artifact type, hidden install behavior |
| **Materialization Step Type** | The category of a **Materialization Step**: `file`, `fragment`, `template`, `script`, or `prompt` in v1.           | Artifact kind, source type             |

## Source and discovery

- **Source**: A published origin that defines and can deliver one or more **Artifacts**.
  Avoid: Catalog
- **Acquisition Channel**: The consumer-side mechanism used to obtain a **Source** for resolution, trust, and locking; v1 supports `file` and `git`.
  Avoid: Published transport, source kind
- **Source Type**: The explicit classification of an **Acquisition Channel** used in references, manifests, and lockfiles.
  Avoid: Inferred protocol, published source kind
- **Source Descriptor**: The manifest published by a **Source** that declares provided artifacts and source-local publication metadata, but not acquisition semantics such as **Source Type**.
  Avoid: Folder inference, implicit structure
- **Source Version**: The version or snapshot identifier for a resolved state of a **Source** as obtained through a specific **Acquisition Channel**.
  Avoid: Artifact version, floating state
- **Source Identity**: The stable consumer-side identity of an acquired source, including **Source Type** plus its source locator.
  Avoid: Descriptor-only identity, content-only identity
- **Catalog**: An index listing available **Artifacts**, the **Sources** that provide them, and available versions.
  Avoid: Source, registry

## Consumer repository state

| Term                       | Definition                                                                                    | Aliases to avoid                   |
| -------------------------- | --------------------------------------------------------------------------------------------- | ---------------------------------- |
| **Manifest**               | The versioned file that declares desired installed artifacts and stable **Source Identity**.  | Lockfile, machine config           |
| **Resolution**             | The exact resolved artifact versions and origins derived from a **Manifest**.                 | Intent, manifest                   |
| **Lockfile**               | The versioned record of a **Resolution** for reproducible installation.                       | Manifest, machine config           |
| **Managed Artifact**       | An installed **Artifact** tracked by the CLI as part of repository-managed configuration.     | Untracked file, incidental content |
| **Materialization Record** | The tracked record of what a **Managed Artifact** wrote into the target repository or folder. | Best-effort guess, inferred state  |

## Policy and safety

- **Trust Policy**: The versioned security policy controlling approved **Source Identity** values, step types, and age constraints.
  Avoid: Machine default, ad hoc flag
- **Minimum Age Rule**: A trust constraint rejecting resolved targets that are newer than the configured age threshold.
  Avoid: Source age, catalog age
- **Overwrite Policy**: The rule for replacing existing files during materialization.
  Avoid: Merge strategy, sync mode
- **Removal Policy**: The rule for handling a **Managed Artifact** no longer present in final desired state.
  Avoid: Garbage collection, overwrite policy
- **Ownership Conflict**: A conflict where two **Managed Artifacts** claim the same whole file or overlapping fragment region.
  Avoid: Shared ownership, last-write-wins
- **Recovery State**: The failure state recorded when materialization stops after partial writes and rollback cannot be verified.
  Avoid: Silent partial failure, implicit dirty state

## Command and reporting

| Term                  | Definition                                                                                                                               | Aliases to avoid                         |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| **CLI Command Name**  | The executable name `tbboot`.                                                                                                            | talby, tboot                             |
| **Install Command**   | The primary command for declaring, reconciling, and materializing artifacts.                                                             | sync command, add command                |
| **Sync**              | The reconciliation operation that aligns actual state with the **Manifest** and the recorded **Lockfile** / derived **Resolution**.      | Top-level command, apply-only command    |
| **Upgrade Command**   | The command that advances already-declared artifacts or sources to newer resolved versions.                                              | Sync alias, catalog refresh              |
| **Search Command**    | The command that queries configured catalog caches and returns matching **Sources**.                                                     | catalog admin command, repository search |
| **Logs Command**      | The command that inspects recorded operations for an **Operation Root**.                                                                 | Install-only subcommand, debug flag      |
| **Operation Summary** | The default concise human-readable output after install or sync.                                                                         | Full verbose log, success-only silence   |
| **Operation Log**     | The replayable record of what happened during an operation.                                                                              | Default output, transient debug spew     |
| **Exit Code**         | The process result code classifying command outcome for shells, CI, and automation.                                                      | Message parsing, ad hoc status           |

## Relationships

- A **Source** provides one or more **Artifacts**.
- A **Source Descriptor** does not declare **Source Type**.
- An **Acquisition Channel** determines **Source Type**.
- An **Artifact Name** is unique only within its **Source**.
- The stable identity of an **Artifact** is **Source** plus **Artifact Name**.
- The stable identity of an acquired source is its **Source Identity**, not its published descriptor alone.
- A **Manifest** declares intent; a **Resolution** makes it exact; a **Lockfile** records it.
- A **Catalog** indexes **Sources** but is not source of truth for artifacts.
- A **Managed Artifact** owns whole files or bounded **Fragments** through its **Materialization Record**.
- **Sync** reconciles the target repository against the **Manifest** and **Lockfile**.
- **Trust Policy** is checked against **Source Identity** before materialization writes.

## Example dialogue

> **Dev:** "If a user runs `tbboot install foo`, is `foo` globally unique?"
>
> **Domain expert:** "No. `foo` is an **Artifact Name**. It is only stable after resolution to a **Source** plus that artifact name."
>
> **Dev:** "So does the **Catalog** decide what gets installed?"
>
> **Domain expert:** "No. A **Catalog** is only an index. The **Source** and **Artifact Descriptor** define the artifact, and the **Lockfile** records the exact **Resolution**."
>
> **Dev:** "If `file:./artifacts` and `git:https://example.com/artifacts.git` publish the same content, are they the same source?"
>
> **Domain expert:** "No. They may publish the same content, but they have different **Source Identity** because they come through different **Acquisition Channels**."
>
> **Dev:** "Can sync remove files silently if an artifact disappears?"
>
> **Domain expert:** "No. The **Removal Policy** prompts before removing a **Managed Artifact** by default, and drift is checked through the **Materialization Record**."

## Flagged ambiguities

- "source" and "catalog" are distinct: **Source** delivers, while **Catalog** indexes.
- "published source" and "acquired source identity" are distinct: the **Source Descriptor** publishes content, while **Source Identity** is consumer-side and includes **Source Type**.
- "manifest" and "lockfile" are distinct: **Manifest** declares intent, **Lockfile** records exact resolution.
- "install" and "sync" are distinct: **Install Command** is the user-facing entrypoint, **Sync** is the underlying reconciliation operation.
- "artifact version" and **Source Version** are distinct: artifact versions live in **Artifact Descriptors**, while **Source Version** identifies a source snapshot.
