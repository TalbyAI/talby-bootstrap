# Talby Bootstrap

Talby Bootstrap defines the language for a CLI that installs and maintains reusable repository artifacts from local or remote sources.

## Language

**Artifact**:
The canonical installable unit managed by the CLI. An artifact is something resolved from a source and materialized into a target repository or folder.
_Avoid_: Pattern, snippet, module

**Artifact Name**:
The identifier used to refer to an **Artifact** within its own **Source**. An artifact name is only required to be unique inside that source.
_Avoid_: Global ID, path

**Artifact Type**:
A category of **Artifact** that defines how it is interpreted and installed. Some artifact types are safe by default, while others require explicit enablement before installation.
_Avoid_: Kind, format

**Artifact Descriptor**:
The explicit manifest published for an **Artifact** that declares its version and descriptive metadata without requiring the CLI to inspect the artifact contents.
_Avoid_: Inline metadata, inferred manifest

**Artifact Descriptor Filename**:
The canonical filename used by an **Artifact** to publish its **Artifact Descriptor**. The initial filename is `talby-artifact.yaml`.
_Avoid_: Generic artifact manifest name, implicit location

**Artifact Descriptor Location**:
The canonical location where an **Artifact Descriptor** is stored. It lives in the root folder of the individual artifact alongside the files owned by that artifact.
_Avoid_: Repository root, inferred location

**Source**:
A concrete origin that can resolve and deliver one or more **Artifacts**. A source may be a local private folder, a repository on GitHub or another code host, or another retrievable content location that defines artifacts or artifact definitions.
_Avoid_: Catalog, channel

**Source Type**:
The explicit classification of a **Source** that determines how the CLI connects to it and resolves artifacts from it.
_Avoid_: Inferred protocol, source guess

**Source Descriptor**:
The explicit definition published by a **Source** to declare which artifacts it provides and the metadata needed to resolve them.
_Avoid_: Folder inference, implicit structure

**Descriptor Schema Version**:
The explicit schema version declared by a **Source Descriptor** so the CLI can parse it predictably and evolve compatibility over time.
_Avoid_: Implicit format version, source type version

**Source Version**:
The version or snapshot identifier that represents a published state of a **Source** as a whole.
_Avoid_: Artifact version, floating state

**Source Descriptor Filename**:
The canonical filename used by a **Source** to publish its **Source Descriptor**. The initial filename is `talby-source.yaml`.
_Avoid_: Generic source manifest name, implicit location

**Catalog**:
An index that lists available **Artifacts**, the **Sources** that provide them, and the versions available for them. A catalog may be a JSON file or an API that can be queried for indexed sources and artifacts.
_Avoid_: Source, registry

**Direct Install Reference**:
The explicit installation reference a user can provide to install an **Artifact** directly from its **Source** without relying on any **Catalog**.
_Avoid_: Catalog entry, inferred lookup

**Channel**:
A higher-level distribution entry point that groups one or more **Catalogs** into a consumable feed, similar to a marketplace track.
_Avoid_: Catalog, source

**Manifest**:
A versioned file in a consumer repository that declares which **Artifacts** should be installed there, optionally including where they are resolved from.
_Avoid_: Lockfile, machine config

**Manifest Format**:
The canonical serialization used to write a **Manifest**. The initial format is YAML for manual editing.
_Avoid_: Multi-format by default, implicit format

**Manifest Filename**:
The canonical filename used to store the versioned **Manifest** in a consumer repository. The initial filename is `talby-artifacts.yaml`.
_Avoid_: Generic config name, implicit location

**Manifest Location**:
The canonical location where a **Manifest** is stored. Consumer manifests live at the repository root.
_Avoid_: Hidden path, arbitrary location

**Lockfile Filename**:
The canonical filename used to store the versioned **Lockfile** in a consumer repository. The initial filename is `talby-artifacts.lock.yaml`.
_Avoid_: Generic lock name, implicit location

**Lockfile Location**:
The canonical location where a **Lockfile** is stored. It lives at the root of the consumer repository alongside the manifest.
_Avoid_: Hidden path, arbitrary location

**Resolution**:
The concrete result of resolving the desired state declared in a **Manifest** into exact artifact versions and origins.
_Avoid_: Intent, manifest

**Lockfile**:
A reproducible record of a **Resolution**, including the exact versions or immutable references selected for installation. The lockfile is intended to live in and be versioned with the consumer repository.
_Avoid_: Manifest, machine config

**Floating Source**:
A source-level installation mode where future artifacts added to a **Source** may be picked up automatically on later sync operations.
_Avoid_: Pinned source, snapshot

**Pinned Source**:
A source-level installation mode where the installed set of artifacts is limited to the resolved snapshot captured at the time of installation.
_Avoid_: Floating source, live source

**Trust Policy**:
The versioned security policy that controls which artifacts, artifact types, sources, or age constraints are allowed for a consumer repository. Local machine settings may harden this policy, but should not silently weaken it.
_Avoid_: Machine default, ad hoc flag

**Minimum Age Rule**:
A trust constraint that rejects a resolved installation target unless its exact resolved publication, tag, commit, or version is older than the configured minimum age.
_Avoid_: Source age, catalog age

**Overwrite Policy**:
The rule that determines what happens when installation would replace existing files in the target repository or folder. It may use manifest settings, command-line flags, and Git state to decide whether to prompt, skip, or overwrite.
_Avoid_: Merge strategy, sync mode

**Sync**:
The reconciliation operation that brings the target repository or folder into alignment with the desired state declared in the **Manifest**.
_Avoid_: Top-level command, apply-only command

**Dry Run**:
A non-mutating execution of reconciliation that resolves the intended changes and reports them without modifying the target repository or folder.
_Avoid_: Apply, install

**CLI Command Name**:
The canonical executable name used to invoke Talby Bootstrap from the command line. The initial command name is `tbboot`.
_Avoid_: talby, tboot

**Artifact Management Surface**:
The user-facing command surface for declaring, reconciling, and materializing **Artifacts** in a target repository or folder.
_Avoid_: Sync API, artifact internals

**Catalog Management Surface**:
The user-facing command surface for configuring and maintaining the **Catalogs** that Talby Bootstrap consults for artifact discovery.
_Avoid_: Search UI, artifact installation surface

**User Configuration Directory**:
The home-directory folder `.tbboot` that stores user-scoped Talby Bootstrap configuration and cache state outside any consumer repository.
_Avoid_: Repository state, hidden temp cache

**Install Command**:
The primary user-facing command for artifact management. The canonical command is `install`, with alias `i`; without arguments it runs **Sync**, and with artifact arguments it declares and applies them unless a declarative-only mode is requested.
_Avoid_: sync command, add command

**Declare-Only Install**:
An **Install Command** mode enabled by `--declare-only` that updates only the **Manifest** without materializing artifacts, updating the **Lockfile**, or touching cache state.
_Avoid_: Partial install, preview install

**Catalog Refresh**:
The catalog maintenance operation that refreshes cached catalog metadata and indexes without changing the **Manifest**, **Lockfile**, or installed artifacts.
_Avoid_: Upgrade check, install refresh

**Catalog Cache**:
The cached catalog metadata and indexes stored in the **User Configuration Directory** for discovery and resolution support.
_Avoid_: Installed artifacts, lockfile state

**Catalog Add**:
The catalog maintenance operation that registers a catalog in active configuration and performs an initial fetch so the catalog is usable immediately. If that initial fetch fails, the catalog is not registered.
_Avoid_: Config-only add, deferred activation

**Catalog Remove**:
The catalog maintenance operation that removes a catalog from active configuration and deletes its associated **Catalog Cache** so its indexes and artifacts no longer appear from that catalog.
_Avoid_: Soft disable, stale cache retention

**Catalog List**:
The catalog maintenance operation that shows configured catalogs together with minimal operational status such as cache presence, last successful refresh time, and whether the last refresh attempt failed.
_Avoid_: Config-only listing, verbose diagnostics dump

**Search Command**:
The top-level command that queries all configured catalog caches using **Artifact Name** and indexable **Artifact Descriptor** metadata such as description, type, tags, and source.
_Avoid_: catalog admin command, full repository search

**Managed Artifact**:
An **Artifact** whose installed state is tracked by the CLI as part of the repository's managed configuration.
_Avoid_: Untracked file, incidental content

**Removal Policy**:
The rule that determines how **Sync** handles a **Managed Artifact** that is no longer declared in the **Manifest**.
_Avoid_: Garbage collection, overwrite policy

**Materialization Record**:
The tracked record of what a **Managed Artifact** wrote into the target repository or folder.
_Avoid_: Best-effort guess, inferred state

**Fragment**:
A bounded section inserted by an **Artifact** inside an existing file instead of creating or replacing the whole file.
_Avoid_: Whole file, anonymous patch

**Fragment Boundary**:
The explicit start and end markers that identify a **Fragment** so it can be updated or removed safely later. These markers are visible in the target file so generated sections are easy to audit and avoid manual edits.
_Avoid_: Fuzzy match, heuristic location

**Fragment Drift**:
The state where the content inside a managed **Fragment** no longer matches what the CLI previously materialized.
_Avoid_: Expected update, normal sync

## Relationships

- An **Artifact** comes from exactly one source location
- An **Artifact** is materialized into a target repository or folder
- An **Artifact** has exactly one **Artifact Type**
- An **Artifact** publishes exactly one **Artifact Descriptor**
- An **Artifact Descriptor** has exactly one canonical **Artifact Descriptor Filename**
- An **Artifact Descriptor** lives in the root folder of its individual **Artifact**
- An **Artifact Type** may require explicit enablement before its **Artifacts** can be installed
- A **Source** provides one or more **Artifacts**
- A **Source** has exactly one **Source Type**
- A **Source** publishes exactly one **Source Descriptor**
- A **Source Descriptor** has exactly one canonical **Source Descriptor Filename**
- A **Source Descriptor** lives at the source repository root
- A **Source Descriptor** declares exactly one **Descriptor Schema Version**
- A **Source** may publish one or more **Source Versions**
- An **Artifact Name** is unique only within its **Source**
- The stable identity of an **Artifact** is the combination of its **Source** and **Artifact Name**
- An **Artifact Descriptor** includes the artifact version and descriptive metadata
- The version of an **Artifact** is defined only in its **Artifact Descriptor**
- An **Artifact** may be published as part of a specific **Source Version**
- A **Catalog** references one or more **Sources** and the **Artifacts** they provide
- A **Catalog** is an index for discovery, not the source of truth for an **Artifact**
- A **Channel** groups one or more **Catalogs**
- A **Manifest** belongs to one consumer repository
- A **Manifest** has exactly one canonical **Manifest Format**
- A **Manifest** has exactly one canonical **Manifest Filename**
- A **Manifest** lives at the consumer repository root
- A **Manifest** declares one or more desired **Artifacts**
- A **Manifest** may also declare an entire **Source**, which implies all of its **Artifacts**
- A whole-**Source** install defaults to **Pinned Source** behavior
- **Floating Source** behavior must be enabled explicitly
- A **Manifest** defines the base **Trust Policy** for its consumer repository
- A **Manifest** may define an **Overwrite Policy**
- Local machine settings may harden the **Trust Policy** or require more confirmation
- A **Manifest** expresses installation intent, not exact resolved versions
- A **Resolution** is derived from a **Manifest**
- A **Minimum Age Rule** is evaluated against the exact **Resolution**
- A **Lockfile** persists a **Resolution** for reproducible installation
- A **Lockfile** has exactly one canonical **Lockfile Filename**
- A **Lockfile** lives at the consumer repository root
- A whole-**Source** resolution in the **Lockfile** includes the exact resolved **Artifacts** and their versions
- An **Overwrite Policy** may inspect Git state before replacing files
- The **Artifact Management Surface** is centered on the **Install Command**
- The **Catalog Management Surface** contains catalog configuration and **Catalog Refresh**
- The **User Configuration Directory** stores **Catalog Cache** and other user-scoped configuration
- The **User Configuration Directory** contains a `catalogs` subdirectory for **Catalog Cache**
- The **Install Command** without arguments runs **Sync**
- The **Install Command** with artifact arguments declares and applies those **Artifacts** by default
- The **Install Command** may also accept a **Direct Install Reference** without any configured **Catalog**
- **Declare-Only Install** updates only the **Manifest**
- **Catalog Add** registers a catalog and performs an initial fetch
- **Catalog Refresh** updates cached catalog metadata and indexes only
- **Catalog List** shows configured catalogs with minimal operational status
- **Catalog Remove** deletes the removed catalog's associated **Catalog Cache**
- The **Search Command** queries catalog indexes without changing installed state
- **Sync** reconciles actual state against the **Manifest** and its **Resolution**
- **Dry Run** executes reconciliation without mutating files
- The canonical **CLI Command Name** for this tool is `tbboot`
- A **Managed Artifact** is eligible for drift detection and controlled removal by **Sync**
- A **Materialization Record** belongs to a **Managed Artifact**
- A **Materialization Record** tracks whole files and any inserted **Fragments**
- A **Fragment** is delimited by exactly two **Fragment Boundaries**
- Visible **Fragment Boundaries** are the default mechanism for managed fragment insertion
- **Fragment Drift** is detected by comparing the current fragment contents against the prior **Materialization Record**
- The default reaction to **Fragment Drift** is to prompt before updating or removing the fragment
- The default **Removal Policy** is to prompt before removing a **Managed Artifact** no longer declared in the **Manifest**

## Example dialogue

> **Dev:** "What command do I run for Talby Bootstrap without consuming the broader Talby name?"
> **Domain expert:** "Use the canonical **CLI Command Name** `tbboot`, and keep `Talby` reserved for the wider tool suite."
>
> **Dev:** "If I run `tbboot install foo`, does that only declare `foo` or also apply it?"
> **Domain expert:** "The **Install Command** declares and applies by default; use **Declare-Only Install** when you want to update only the **Manifest**."
>
> **Dev:** "If I remove a catalog, why do its results disappear from search immediately?"
> **Domain expert:** "Because **Catalog Remove** also deletes that catalog's **Catalog Cache** from the **User Configuration Directory**."
>
> **Dev:** "Where does user-scoped cache live?"
> **Domain expert:** "In the **User Configuration Directory** `.tbboot`, with `catalogs` for **Catalog Cache**."
>
> **Dev:** "After `catalog add`, do I need a separate refresh before search works?"
> **Domain expert:** "No. **Catalog Add** performs an initial fetch so the catalog is usable immediately."
>
> **Dev:** "What if the initial fetch fails during `catalog add`?"
> **Domain expert:** "Then **Catalog Add** fails atomically and the catalog is not registered."
>
> **Dev:** "What does `catalog list` show besides the configured names?"
> **Domain expert:** "It shows each configured catalog with minimal operational status, including cache presence and refresh outcome."
>
> **Dev:** "Does `search` hit the network before answering?"
> **Domain expert:** "No. The **Search Command** uses the local cache from all configured catalogs unless you refresh explicitly."
>
> **Dev:** "If two catalogs both list the same artifact, do I get an ambiguity error?"
> **Domain expert:** "Only if they resolve to different **Sources**. A **Catalog** is just an index; if both entries point to the same **Source**, installation may proceed."

## Flagged ambiguities

- "pattern", "snippet", and "module" were used for the same concept — resolved: use **Artifact** as the canonical term
- "source", "catalog", and "channel" could be conflated — resolved: **Source** delivers, **Catalog** indexes, **Channel** groups catalogs
- "manifest" and "lockfile" could be conflated — resolved: **Manifest** declares intent, **Lockfile** stores exact resolution
- artifact identity could be ambiguous across sources — resolved: an **Artifact Name** is source-local, and stable identity is **Source + Artifact Name**
- whole-source install behavior could be unsafe by default — resolved: default to **Pinned Source** and require explicit opt-in for **Floating Source**
- trust configuration could drift outside the repo — resolved: base **Trust Policy** lives in the versioned **Manifest**
- temporal trust checks could be applied at the wrong level — resolved: **Minimum Age Rule** is validated against the exact **Resolution**
- overwrite handling could be too blunt — resolved: use an **Overwrite Policy** that can consult Git state plus manifest and CLI overrides
- planning and applying could be split unnecessarily — resolved: the **Install Command** is the primary user-facing command, and **Sync** remains the underlying reconciliation operation
- source behavior could become implicit and fragile — resolved: every **Source** declares an explicit **Source Type**
- manifest format could expand too early — resolved: the initial canonical **Manifest Format** is YAML only
- reproducibility could be undermined by local-only state — resolved: the **Lockfile** lives in and is versioned with the repository
- source discovery could become ambiguous — resolved: each **Source** publishes an explicit **Source Descriptor**
- descriptor evolution could become brittle — resolved: each **Source Descriptor** declares an explicit **Descriptor Schema Version**
- artifact metadata could require content inspection — resolved: each **Artifact** publishes an explicit **Artifact Descriptor** with version and indexable metadata
- source snapshots could be lost if only artifacts are versioned — resolved: keep both **Source Version** and per-**Artifact** versioning
- artifact version authority could become duplicated — resolved: artifact version is defined only in the **Artifact Descriptor**
- whole-source locking could be under-specified — resolved: lock the **Source Version** plus the exact resolved **Artifacts** and their versions
- removal behavior could be destructive by default — resolved: prompt before removing a **Managed Artifact** no longer declared in the **Manifest**
- managed state could be tracked too coarsely — resolved: store a **Materialization Record** for whole files and bounded **Fragments**
- fragment removal could become heuristic and unsafe — resolved: default to visible **Fragment Boundaries** for managed fragments
- manual edits inside managed fragments could be lost silently — resolved: detect **Fragment Drift** and prompt before update or removal
- local cache could be premature complexity — resolved: exclude local cache/state from the initial specification until a real performance need is measured
- manifest discovery could be ambiguous — resolved: the canonical **Manifest Filename** is `talby-artifacts.yaml`
- lockfile discovery could be ambiguous — resolved: the canonical **Lockfile Filename** is `talby-artifacts.lock.yaml`
- source descriptor discovery could be ambiguous — resolved: the canonical **Source Descriptor Filename** is `talby-source.yaml`
- artifact descriptor discovery could be ambiguous — resolved: the canonical **Artifact Descriptor Filename** is `talby-artifact.yaml`
- descriptor placement could be confused across repository and artifact scopes — resolved: consumer/source manifests live at repo root; artifact descriptors live in each artifact folder
- lockfile placement could drift from the consumer manifest — resolved: the **Lockfile** lives at the consumer repository root beside the manifest
- the CLI binary name could consume the broader suite brand — resolved: use `tbboot` for this tool and reserve `Talby` for the wider suite
- "install" and "sync" could be conflated — resolved: **Install Command** is the primary user-facing entrypoint, while **Sync** is the underlying reconciliation operation
- "declare" could imply partial installation — resolved: **Declare-Only Install** updates only the **Manifest**
- catalog administration and artifact discovery could blur together — resolved: **Catalog Management Surface** owns maintenance commands, while **Search Command** stays top-level for discovery
- "refresh" could be confused with upgrade detection — resolved: **Catalog Refresh** only updates cached catalog metadata and indexes
- removing a catalog could leave stale discovery data behind — resolved: **Catalog Remove** also deletes that catalog's associated **Catalog Cache**
- user-scoped cache could be confused with repository-managed state — resolved: the **User Configuration Directory** stores cache and user config outside the consumer repository
- the home directory name could diverge from the CLI binary — resolved: use `.tbboot` to align with `tbboot`
- source cache could be premature complexity — resolved: the initial **User Configuration Directory** keeps only `catalogs` for **Catalog Cache**
- adding a catalog could leave it unusable until another command runs — resolved: **Catalog Add** performs an initial fetch immediately
- catalog add could leave half-configured state behind — resolved: **Catalog Add** is atomic and does not register the catalog when the initial fetch fails
- catalog list could be too shallow to diagnose issues — resolved: **Catalog List** includes minimal operational status with each configured catalog
- search freshness could be confused with online lookup — resolved: the **Search Command** uses local cache from all configured catalogs by default
- catalogs could be mistaken for the source of truth — resolved: a **Catalog** is only an index; the real identity of an **Artifact** is its **Source + Artifact Name**
- installation could appear to require catalogs — resolved: the **Install Command** may use a **Direct Install Reference** without any configured **Catalog**

## Interview state

This section preserves the current specification interview state by explicit user request so the sequence of decisions and remaining open areas is not lost.

### Original pending areas

- superficie real del CLI
- modelo de resolución/versionado
- reglas de sync/overwrite/remove
- trust policy y defaults de seguridad
- ownership/provenance de archivos y fragments
- manejo de errores, drift y rollback

### Current status by area

- **superficie real del CLI** — partially resolved
  - Resolved:
    - two user-facing surfaces: **Artifact Management Surface** and **Catalog Management Surface**
    - primary command is **Install Command** `install` with alias `i`
    - `install` without arguments runs **Sync**
    - `install <artifact>` declares and applies by default
    - `--declare-only` updates only the **Manifest**
    - catalog commands include **Catalog Add**, **Catalog List**, **Catalog Refresh**, and **Catalog Remove**
    - **Search Command** is top-level
  - Still open:
    - canonical syntax of **Direct Install Reference**
    - full command shapes, arguments, and output contracts

- **modelo de resolución/versionado** — partially resolved
  - Resolved:
    - stable identity is **Source + Artifact Name**
    - **Catalog** is only an index, not a source of truth
    - duplicate catalog hits are only ambiguous when they resolve to different **Sources**
    - direct installation without any catalog must be supported
  - Still open:
    - canonical direct-reference syntax
    - precedence and normalization rules for source-qualified installs
    - exact version selection/update semantics beyond current glossary terms

- **reglas de sync/overwrite/remove** — partially resolved
  - Resolved:
    - **Sync** remains the underlying reconciliation operation
    - **Overwrite Policy** exists and may inspect Git state
    - **Removal Policy** defaults to prompting before removing a **Managed Artifact**
    - **Catalog Remove** deletes associated **Catalog Cache**
  - Still open:
    - exact overwrite decision matrix
    - rollback/recovery behavior on partial materialization failure
    - remove semantics for artifacts vs sources declared in the **Manifest**

- **trust policy y defaults de seguridad** — still open
  - Resolved:
    - base **Trust Policy** lives in the versioned **Manifest**
    - **Minimum Age Rule** is evaluated against the exact **Resolution**
  - Still open:
    - default trust posture for new repos
    - source-type allow/deny defaults
    - confirmation rules for risky artifact types or unsigned/untrusted sources

- **ownership/provenance de archivos y fragments** — partially resolved
  - Resolved:
    - **Materialization Record** tracks what a **Managed Artifact** wrote
    - **Fragment Boundary** is visible by default
    - **Fragment Drift** is detected and prompts before update or removal
  - Still open:
    - provenance display/reporting to users
    - ownership behavior when multiple artifacts target related files
    - exact conflict rules for overlapping or nested fragments

- **manejo de errores, drift y rollback** — partially resolved
  - Resolved:
    - successful operations keep exit code `0` even with warnings
    - stale catalog cache warns and continues by default
    - `--refresh` forces catalog refresh before install when desired
    - **Catalog Add** is atomic on initial fetch
  - Still open:
    - non-zero exit code taxonomy
    - rollback guarantees for failed installs after partial writes
    - machine-readable warning/error reporting
    - drift handling for non-fragment whole-file mutations

### Immediate open questions

- canonical syntax for **Direct Install Reference**
- ambiguity rules once direct source-qualified install syntax exists
- concrete overwrite/remove matrix
- trust-policy defaults for v1
- rollback model after partial write failure
