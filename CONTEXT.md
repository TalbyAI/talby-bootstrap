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
The explicit classification of a **Source** that determines how the CLI connects to it and resolves artifacts from it. In v1, only `file` and `git` are supported source types for approval and resolution.
_Avoid_: Inferred protocol, source guess

**Source Descriptor**:
The explicit definition published by a **Source** to declare which artifacts it provides and the metadata needed to resolve them.
_Avoid_: Folder inference, implicit structure

**Descriptor Schema Version**:
The explicit schema version declared by a **Source Descriptor** so the CLI can parse it predictably and evolve compatibility over time.
_Avoid_: Implicit format version, source type version

**Source Version**:
The version or snapshot identifier that represents a published state of a **Source** as a whole. When direct install omits an explicit source version, resolution defaults to the latest stable published source version; if the source type cannot provide a stable published version concept, the install must fail until a source version is specified explicitly. Once resolved, later sync operations keep that resolved source version until the user explicitly asks to move to a newer one.
_Avoid_: Artifact version, floating state

**Source Descriptor Filename**:
The canonical filename used by a **Source** to publish its **Source Descriptor**. The initial filename is `talby-source.yaml`.
_Avoid_: Generic source manifest name, implicit location

**Catalog**:
An index that lists available **Artifacts**, the **Sources** that provide them, and the versions available for them. A catalog may be a JSON file or an API that can be queried for indexed sources and artifacts. When configured locally, each catalog is assigned a unique local name used in catalog-qualified install references.
_Avoid_: Source, registry

**Direct Install Reference**:
The explicit typed source reference a user can provide to install an **Artifact** directly from its **Source** without relying on any **Catalog**. Its canonical form identifies the source as `{source-type}:{source-name}`; the artifact is selected separately, and any direct-install version pin applies to the **Source Version** rather than being embedded in the artifact selector. Ambiguity is evaluated only after normalization within the declared **Source Type**.
_Avoid_: Catalog entry, inferred lookup

**Catalog Install Reference**:
The catalog-qualified install reference a user can provide when resolving through a configured **Catalog** instead of naming a typed **Source** directly. Its canonical form identifies the configured catalog name and the catalog-local source name as `{catalog-name}/{catalog-source-name}`, with an optional artifact selector provided separately. Without an artifact selector, it installs the resolved **Source** as a whole using the same semantics as a direct source install.
_Avoid_: Direct source reference, implicit global lookup

**Channel**:
A higher-level distribution entry point that groups one or more **Catalogs** into a consumable feed, similar to a marketplace track.
_Avoid_: Catalog, source

**Manifest**:
A versioned file in a consumer repository that declares which **Artifacts** should be installed there, including enough source identity to re-resolve them stably even if the lockfile must be regenerated. The stable source identity is normative; any preserved original user-facing reference is optional metadata only.
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
A reproducible record of a **Resolution**, including the exact versions or immutable references selected for installation. The lockfile is intended to live in and be versioned with the consumer repository. If a manifest exists without a lockfile, the first successful install resolves from the manifest and creates the lockfile as part of that operation.
_Avoid_: Manifest, machine config

**Floating Source**:
A source-level installation mode where future artifacts added to a **Source** may be picked up automatically on later sync operations.
_Avoid_: Pinned source, snapshot

**Pinned Source**:
A source-level installation mode where the installed set of artifacts is limited to the resolved snapshot captured at the time of installation.
_Avoid_: Floating source, live source

**Trust Policy**:
The versioned security policy that controls which artifacts, artifact types, approved sources, or age constraints are allowed for a consumer repository. The manifest defines the allowlist of approved sources; approval of a source does not automatically approve every artifact type it can deliver. `file:` sources are allowed by default only when they point inside the current **Operation Root**; `git:` sources always require explicit approval in the manifest. Source types that can prove publication time may also be constrained by a minimum age rule. Local machine settings may harden this policy, but should not silently weaken it.
_Avoid_: Machine default, ad hoc flag

**Minimum Age Rule**:
A trust constraint that rejects a resolved installation target unless its exact resolved publication, tag, commit, or version is older than the configured minimum age.
_Avoid_: Source age, catalog age

**Overwrite Policy**:
The rule that determines what happens when installation would replace existing files in the target repository or folder. By default, content already owned by the same **Managed Artifact** may be overwritten automatically when it still matches the prior **Materialization Record**; otherwise the policy may prompt, skip, or require an explicit override. Manifest settings, command-line flags, and Git state may refine that decision.
_Avoid_: Merge strategy, sync mode

**Sync**:
The reconciliation operation that brings the target repository or folder into alignment with the desired state declared in the **Manifest**.
_Avoid_: Top-level command, apply-only command

**Dry Run**:
A non-mutating execution of reconciliation that resolves the intended changes and reports them without modifying the target repository or folder.
_Avoid_: Apply, install

**JSON Output Envelope**:
The canonical machine-readable output shape for CLI commands when JSON output is requested. The initial envelope contains `code`, `message`, and `details`.
_Avoid_: Ad hoc JSON, command-specific envelope

**Exit Code**:
The canonical process result code returned by the CLI to classify overall command outcome for shells, CI, and automation.
_Avoid_: Message parsing, ad hoc status

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
The primary user-facing command for artifact management. The canonical command is `install`, with alias `i`; without arguments it runs **Sync**, and with one explicit target it declares and applies that target unless a declarative-only mode is requested. V1 does not accept multiple explicit install targets in a single command, and ambiguous target syntax fails until the user provides an unambiguous form.
_Avoid_: sync command, add command

**Upgrade Command**:
The dedicated user-facing command for advancing already-declared artifacts or sources to newer resolved versions. Without arguments, it attempts to upgrade the entire manifest. With an explicit target, it reuses the same unambiguous target forms as `install`: typed source, catalog-qualified source, or shorthand only when uniquely resolved. Source targets upgrade the whole declared source; artifact targets upgrade only the selected artifact. Targets not already declared in the manifest are rejected instead of being installed implicitly. If the manifest declares a whole source, upgrade must respect that scope and reject artifact-level upgrade requests inside that source. It supports the same **Dry Run** contract as other reconciliation flows: resolve and report without mutating manifest, lockfile, or files. For multi-target upgrades it applies targets in deterministic order, processing whole sources before individual artifacts and sorting each group lexicographically by normalized identity, then stops at the first mutating failure. Successful upgrade writes the new exact resolution to the **Lockfile** and leaves the **Manifest** unchanged unless the user's declared intent changes. By default it advances each eligible target to the latest stable published version allowed by the active trust policy. Versions skipped because of trust or age rules are reported in more verbose output, not in the default short summary. The canonical command name is `upgrade`; `install --upgrade` is an equivalent shortcut when the user wants install-style targeting with upgrade behavior.
_Avoid_: Sync alias, catalog refresh

**Operation Summary**:
The default short human-readable output shown after a successful install or sync. It uses one stable, concise shape across human-facing commands: a one-line result followed by effective changes only when present. For materialization commands it reports what sources or artifacts were reconciled, how many changes were applied, and the **Provenance Summary** for effective changes only. Additional detail lives behind **Verbosity Level** selection or the **Logs Command**.
_Avoid_: Full verbose log, success-only silence

**Operation Log**:
The more verbose record of what happened during an install or sync, intended for follow-up inspection after the default short summary. The same operation may be rendered later at different verbosity levels without re-running the materialization itself.
_Avoid_: Default output, transient debug spew

**Verbosity Level**:
The canonical named detail level used when rendering command output or replaying an **Operation Log**. V1 defines exactly three levels: `summary`, `normal`, and `verbose`, selected canonically with `--verbosity` and optionally via shorthand aliases such as `-v` and `-vv`.
_Avoid_: Ad hoc per-command levels, numeric-only guess

**Operation ID**:
The stable identifier assigned to a recorded CLI operation so it can be listed and inspected later.
_Avoid_: Ephemeral console line, implicit history position

**Logs Command**:
The lowercase `logs` command surface used to inspect recorded operations scoped to an **Operation Root**. Without additional arguments, it replays the most recent recorded operation for that root. Listing recorded operations is explicit via `logs ls` or `logs list`. Past operations may be inspected by **Operation ID** at a chosen verbosity level. When inspecting a recorded operation without an explicit verbosity override, it re-renders the log at the operation's original verbosity level. List output is sorted by descending date by default while allowing explicit sort flags.
_Avoid_: Install-only subcommand, debug flag

**Operation Retention Policy**:
The rule that limits how many recorded operations and how much history the local operation log keeps in the **User Configuration Directory**.
_Avoid_: Unlimited history, session-only guess

**Operation Root**:
The root path used to scope recorded operations for `logs`. If the current location belongs to a Git repository, the operation root is the repository root; otherwise it is the current local folder.
_Avoid_: Session cwd only, global implicit scope

**Operation Root Key**:
The stable storage key derived from a normalized **Operation Root** to store recorded operations under the **User Configuration Directory**. The root path itself is preserved as readable metadata alongside the stored logs.
_Avoid_: Raw long path, ad hoc folder name

**Main Artifact**:
The canonical artifact of a **Source** used as the shorthand install target when a user installs by bare name through configured **Catalogs**.
_Avoid_: Default guess, arbitrary first artifact

**Declare-Only Install**:
An **Install Command** mode enabled by `--declare-only` that updates only the **Manifest** without materializing artifacts, updating the **Lockfile**, or touching cache state. This remains true for direct source installs and catalog-qualified installs.
_Avoid_: Partial install, preview install

**Catalog Refresh**:
The catalog maintenance operation that refreshes cached catalog metadata and indexes without changing the **Manifest**, **Lockfile**, or installed artifacts. Without explicit catalog arguments, it refreshes all configured catalogs.
_Avoid_: Upgrade check, install refresh

**Catalog Cache**:
The cached catalog metadata and indexes stored in the **User Configuration Directory** for discovery and resolution support.
_Avoid_: Installed artifacts, lockfile state

**Catalog Add**:
The catalog maintenance operation that registers a catalog in active configuration and performs an initial fetch so the catalog is usable immediately. Its canonical command shape is `catalog add <catalog-reference> --name <local-name>`, while omitting `--name` is allowed only when the catalog provides a non-conflicting default local name. If the chosen local name conflicts with an existing configured catalog, the add fails and reports the conflicting catalog so a different name can be chosen. If the initial fetch fails, the catalog is not registered.
_Avoid_: Config-only add, deferred activation

**Catalog Remove**:
The catalog maintenance operation that removes a catalog from active configuration and deletes its associated **Catalog Cache** so its indexes and artifacts no longer appear from that catalog. Its canonical target is the configured local catalog name: `catalog remove <local-name>`.
_Avoid_: Soft disable, stale cache retention

**Catalog List**:
The catalog maintenance operation that shows configured catalogs together with minimal operational status. Its default output includes `local-name`, `catalog-reference`, `cache-status`, `last-refresh`, and `last-refresh-result`.
_Avoid_: Config-only listing, verbose diagnostics dump

**Search Command**:
The top-level command that queries all configured catalog caches and returns matching **Sources** as the primary result unit. Artifact information is shown as summary or expanded detail according to output flags, using **Artifact Name** and indexable **Artifact Descriptor** metadata such as description, type, and tags. In non-interactive CLI use it requires an explicit query.
_Avoid_: catalog admin command, full repository search

**Managed Artifact**:
An **Artifact** whose installed state is tracked by the CLI as part of the repository's managed configuration.
_Avoid_: Untracked file, incidental content

**Removal Policy**:
The rule that determines how **Sync** handles a **Managed Artifact** that is no longer present in the final resolved desired state. Removal is decided per managed artifact after full resolution, not by the prior textual shape of the **Manifest** declaration.
_Avoid_: Garbage collection, overwrite policy

**Materialization Record**:
The tracked record of what a **Managed Artifact** wrote into the target repository or folder.
_Avoid_: Best-effort guess, inferred state

**Provenance Summary**:
The minimal user-visible identity shown for a managed change: artifact, source, source version, and whether the change owns a whole file or a fragment.
_Avoid_: Opaque change, path-only report

**Ownership Conflict**:
The explicit conflict state where two **Managed Artifacts** would claim the same whole file or overlapping fragment region. Ownership is exclusive, so this conflict must be detected before writing.
_Avoid_: Shared ownership, last-write-wins

**Recovery State**:
The explicit failure state recorded when a materialization operation stops after partial writes and the CLI cannot fully restore the prior state with certainty.
_Avoid_: Silent partial failure, implicit dirty state

**Fragment**:
A bounded section inserted by an **Artifact** inside an existing file instead of creating or replacing the whole file.
_Avoid_: Whole file, anonymous patch

**Fragment Boundary**:
The explicit start and end markers that identify a **Fragment** so it can be updated or removed safely later. These markers are visible in the target file so generated sections are easy to audit and avoid manual edits.
_Avoid_: Fuzzy match, heuristic location

**Fragment Drift**:
The state where the content inside a managed **Fragment** no longer matches what the CLI previously materialized.
_Avoid_: Expected update, normal sync

**Whole-File Drift**:
The state where a whole file owned by a **Managed Artifact** no longer matches the prior **Materialization Record**.
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
- A **Trust Policy** defines an explicit allowlist of approved **Sources**
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
- A bare `install <name>` resolves against configured **Catalogs** by matching the **Main Artifact**
- A bare `install <name>` may proceed only when the matching catalog entries resolve to one unique source identity, or all hits collapse to the same source and source version
- A command may render results through the **JSON Output Envelope** when machine-readable output is requested
- An **Exit Code** classifies command outcome as success, operational error, user-action conflict, or trust/policy denial
- A successful install or sync shows an **Operation Summary** by default
- A successful install or sync may be inspected later through an **Operation Log**
- A **Verbosity Level** controls how much detail a command or replayed operation shows
- A recorded operation has one **Operation ID**
- An **Operation Root** scopes recorded operations for `logs`
- An **Operation Root Key** stores recorded operations for a normalized **Operation Root**
- `logs` without additional arguments replays the most recent recorded operation for the current **Operation Root**
- `logs ls` and `logs list` explicitly list recorded operations with their command type, date, and verbosity level
- The **Logs Command** may re-render a recorded **Operation Log** for an **Operation ID** at a requested verbosity level
- The **Logs Command** uses the operation's original verbosity level by default when inspecting a recorded **Operation ID**
- The **Logs Command** lists operations in descending date order by default, with flags to choose a different sort order
- The **Operation Retention Policy** keeps at most 100 recorded operations and no more than one week of history by default
- The **Catalog Management Surface** contains catalog configuration and **Catalog Refresh**
- The **User Configuration Directory** stores **Catalog Cache** and other user-scoped configuration
- The **User Configuration Directory** contains a `catalogs` subdirectory for **Catalog Cache**
- The **Install Command** without arguments runs **Sync**
- The **Install Command** may use a **Direct Install Reference** without any configured **Catalog**
- The **Install Command** may use a **Catalog Install Reference** when resolving through a configured **Catalog**
- The **Search Command** returns **Sources** as the primary result unit
- A configured **Catalog** has one unique local name used for catalog-qualified install references
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
- A managed change is reported to the user with a **Provenance Summary**
- An **Ownership Conflict** exists when two **Managed Artifacts** would claim the same whole file or overlapping fragment region
- A failed materialization may enter **Recovery State** when verified rollback is incomplete
- A **Fragment** is delimited by exactly two **Fragment Boundaries**
- Visible **Fragment Boundaries** are the default mechanism for managed fragment insertion
- **Fragment Drift** is detected by comparing the current fragment contents against the prior **Materialization Record**
- **Whole-File Drift** is detected by comparing the current whole-file contents against the prior **Materialization Record**
- The default reaction to **Fragment Drift** is to prompt before updating or removing the fragment
- The default reaction to **Whole-File Drift** is to prompt before updating or removing the whole file
- The default **Removal Policy** is to prompt before removing a **Managed Artifact** no longer declared in the **Manifest**
- A whole file already owned by the same **Managed Artifact** may be overwritten automatically when it still matches the prior **Materialization Record**

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
- direct install versioning could split authority between source and artifact selectors — resolved: a **Direct Install Reference** identifies a typed source, and direct-install version pinning applies to the **Source Version** only
- typed direct references could still resolve inconsistently — resolved: source-name normalization and ambiguity checks happen within the declared **Source Type**, and the artifact selector must resolve exactly one **Artifact Name**
- manifest declaration shape could accidentally control removal — resolved: **Removal Policy** evaluates the final resolved **Managed Artifacts**, so switching between artifact-level and source-level declarations does not remove artifacts that remain desired
- new repositories could trust remote content too broadly by default — resolved: the **Manifest** starts with an explicit allowlist of approved **Sources**, and remote source types remain denied until approved
- temporal trust rules could be scoped too loosely — resolved: a **Minimum Age Rule** may be required for source types that can prove publication time, such as Git tags or releases
- source approval could implicitly allow dangerous artifact behavior — resolved: source trust and artifact-type trust are separate; risky artifact types require explicit allowlisting and first-install confirmation
- safe managed upgrades could become noisy and interactive — resolved: whole-file content already owned by the same **Managed Artifact** is overwritten automatically when it has no drift against the prior **Materialization Record**
- rollback guarantees could over-promise atomicity — resolved: v1 only guarantees verified best-effort rollback, and unrecoverable partial failure enters explicit **Recovery State**
- related artifacts could create ambiguous file ownership — resolved: ownership is exclusive, and any whole-file or overlapping-fragment collision is an explicit **Ownership Conflict**
- managed changes could be too opaque to audit — resolved: every managed change reports a **Provenance Summary** with artifact, source, source version, and ownership kind
- source-type sprawl could outpace the trust model — resolved: v1 supports only `file` and `git` as approvable **Source Types**
- direct and catalog-qualified installs could collapse into one ambiguous syntax — resolved: typed-source installs use **Direct Install Reference**, while catalog-qualified installs use **Catalog Install Reference**
- catalog-qualified source installs could diverge semantically from direct source installs — resolved: a **Catalog** is only an index of sources, so `{catalog-name}/{catalog-source-name}` without an artifact selector installs the same whole **Source** a direct reference would resolve
- bare install shorthand could become too magical — resolved: bare `install <name>` searches configured **Catalogs** for a matching **Main Artifact** source and proceeds only when the result collapses to one source identity and source version
- CLI output could fragment across human and automation use cases — resolved: v1 uses one **JSON Output Envelope** with `code`, `message`, and `details` when JSON output is requested
- error handling could become too granular or too vague for automation — resolved: v1 uses four **Exit Codes** only: `0` success, `1` operational or validation error, `2` user-action conflict, `3` trust or policy denial
- whole-file drift could behave differently from fragment drift — resolved: whole-file managed content also prompts before update or removal when it no longer matches the prior **Materialization Record**
- search results could fight the install mental model — resolved: **Search Command** returns **Sources** first, with artifact detail level controlled by output flags
- catalog-qualified references could drift if local catalog names are unstable — resolved: each configured **Catalog** has a unique local name, and **Catalog Add** fails on local-name conflicts while reporting the conflicting catalog
- default install output could be too noisy or too thin — resolved: success output defaults to a short **Operation Summary**, with deeper follow-up inspection available through an **Operation Log**
- follow-up inspection could be tied too narrowly to install syntax — resolved: recorded operations are inspected through a separate **Logs Command** keyed by **Operation ID**, while commands may also choose verbosity at execution time
- operation history could grow without bounds or disappear too quickly — resolved: the default **Operation Retention Policy** keeps at most 100 operations and no more than one week of history
- operation history listing could feel arbitrary — resolved: **Logs Command** sorts by descending date by default, with explicit sort flags for other orders
- follow-up inspection could lose the original user perspective — resolved: `logs <operation-id>` defaults to the operation's original verbosity level, with flags to re-render at a different level
- logs scope could become ambiguous across repositories and folders — resolved: logs are scoped by **Operation Root**, using the Git repository root when present and the current folder otherwise
- the default `logs` action could fight the likely user intent — resolved: bare `logs` replays the most recent operation, while listing history is explicit via `logs ls` or `logs list`
- operation-log storage paths could become too long or brittle — resolved: each **Operation Root** is stored under a stable **Operation Root Key** derived from the normalized root path, while preserving the readable path as metadata
- output detail could drift across commands — resolved: v1 uses exactly three **Verbosity Levels** named `summary`, `normal`, and `verbose`
- verbosity selection could fragment across commands — resolved: commands use canonical `--verbosity <level>` with optional shorthand aliases like `-v` and `-vv`
- catalog naming could stay too implicit at creation time — resolved: `catalog add <catalog-reference> --name <local-name>` is the canonical form, with default-name fallback only when non-conflicting
- catalog status output could be too thin to operate safely — resolved: `catalog list` defaults to `local-name`, `catalog-reference`, `cache-status`, `last-refresh`, and `last-refresh-result`
- catalog refresh scope could diverge from global search/install resolution — resolved: `catalog refresh` without arguments refreshes all configured catalogs
- catalog removal could target the wrong identity layer — resolved: `catalog remove` targets the configured local catalog name, not the remote catalog reference
- search could blur into accidental catalog dumping — resolved: non-interactive `search` requires an explicit query, leaving queryless exploration to a future interactive mode
- declare-only behavior could drift between install modes — resolved: `--declare-only` updates only the **Manifest** for direct, catalog-qualified, and shorthand installs alike
- multi-target install could overcomplicate prompts, conflicts, and rollback — resolved: v1 allows only one explicit install target per command, while bare `install` remains the **Sync** entrypoint
- install target parsing could become precedence-driven and surprising — resolved: ambiguous target syntax fails and requires an explicit unambiguous install form
- supported source types could still have unsafe default trust rules — resolved: `file:` is allowed by default only inside the current **Operation Root**, while `git:` always requires explicit manifest approval
- direct install defaults could undermine reproducibility — resolved: omitting `--source-version` selects the latest stable published **Source Version**, and fails when no stable published version concept exists
- unpinned source declarations could silently float on later syncs — resolved: default version selection is only for the first resolution, and later syncs keep the previously resolved **Source Version** until the user explicitly changes it
- the underlying **Sync** operation could be mistaken for a top-level CLI command — resolved: users trigger reconciliation through bare `install`, while **Sync** remains the underlying operation name
- upgrade behavior could get conflated with install or catalog refresh — resolved: advancing to a newer resolved version uses a dedicated `upgrade` command
- upgrade scope could be unclear by default — resolved: bare `upgrade` attempts to advance the entire manifest
- upgrade selection could be under-specified — resolved: `upgrade` advances to the latest stable published version still allowed by trust and age rules
- blocked newer versions could blur together with normal success output — resolved: policy-blocked upgrade candidates do not alter the default short summary, but are reported in more verbose output and logs
- upgrade target syntax could drift away from install syntax — resolved: `upgrade` reuses the same unambiguous target forms as `install`, with source targets upgrading a whole source and artifact targets upgrading only that artifact
- upgrade could silently become install for undeclared targets — resolved: `upgrade` rejects targets not already declared in the **Manifest**
- whole-source declarations could be undermined by artifact-level upgrades — resolved: when a source is declared as a whole, `upgrade` rejects narrower artifact-level targets inside that source
- upgrade could become the only mutating command without a safe preview — resolved: `upgrade` supports the same non-mutating **Dry Run** contract as other reconciliation flows
- global upgrade failure handling could become inconsistent or over-optimistic — resolved: multi-target `upgrade` runs in deterministic order and stops at the first mutating failure
- deterministic upgrade ordering could still be underspecified — resolved: global `upgrade` processes whole sources before individual artifacts, with lexicographic ordering by normalized identity inside each group
- upgrade output could blur intent and resolution — resolved: successful `upgrade` writes the new exact version only to the **Lockfile**, not to the **Manifest**, unless declared intent changes
- first-time manifest reconciliation could leave reproducibility unbootstrapped — resolved: `install` without an existing **Lockfile** resolves from the **Manifest** and creates the lockfile during that operation
- shorthand or catalog-based declarations could become unstable after lockfile loss — resolved: the **Manifest** stores enough source identity to re-resolve declared targets stably without depending on fresh search ambiguity
- manifest entries could end up with two competing truths — resolved: only stable source identity is normative in the **Manifest**, while any original user-facing reference is optional metadata only

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
    - **Direct Install Reference** uses typed-source syntax `{source-type}:{source-name}`
    - catalog-qualified install syntax uses `{catalog-name}/{catalog-source-name}`
    - catalog-qualified source installs and direct source installs share the same whole-source semantics
    - bare `install <name>` resolves through configured catalogs by matching the **Main Artifact** and requires a unique collapsed source result
    - **Search Command** returns sources as the primary result unit, with artifact detail controlled by flags
    - each configured catalog has a unique local name, and conflicting local names make **Catalog Add** fail explicitly
    - successful install and sync operations show a short **Operation Summary** by default, with deeper follow-up inspection via **Logs Command** and **Operation ID**
    - recorded operation history is retained locally for at most 100 operations and one week
    - **Logs Command** sorts operations by descending date by default, with flags for alternate ordering
    - the canonical command name is lowercase `logs`, and inspecting an operation defaults to its original verbosity level
    - logs are scoped by an **Operation Root**, and bare `logs` replays the most recent operation while `logs ls|list` lists history
    - each operation root is stored under a stable **Operation Root Key** derived from the normalized root path, with readable path metadata preserved
    - v1 defines exactly three verbosity levels: `summary`, `normal`, and `verbose`
    - verbosity is selected canonically with `--verbosity`, with optional shorthand aliases such as `-v` and `-vv`
    - `catalog add <catalog-reference> --name <local-name>` is the canonical add form, with default-name fallback only when non-conflicting
    - `catalog list` default output is `local-name`, `catalog-reference`, `cache-status`, `last-refresh`, and `last-refresh-result`
    - `catalog refresh` without arguments refreshes all configured catalogs
    - `catalog remove` canonically targets the configured local catalog name
    - non-interactive `search` requires an explicit query
    - `--declare-only` updates only the **Manifest** across direct, catalog-qualified, and shorthand installs
    - v1 `install` accepts only one explicit target per command
    - ambiguous install target syntax fails until the user provides an explicit unambiguous form
    - reconciliation is triggered by bare `install`; **Sync** is the underlying operation, not a separate top-level command
    - version advancement uses a dedicated `upgrade` command
    - bare `upgrade` attempts to advance the entire manifest
    - explicit `upgrade` targets reuse the same unambiguous forms as `install`
    - `upgrade` supports the same **Dry Run** contract as other reconciliation flows
    - direct and catalog-qualified explicit installs use one canonical shape: `tbboot install <source-ref> [--artifact <artifact-name>] [--source-version <version>]`
    - omitting `--artifact` installs the whole **Source**, while `--artifact` selects one **Artifact** inside that source
    - explicit upgrades use one canonical shape: `tbboot upgrade <source-ref> [--artifact <artifact-name>]`
    - omitting `--artifact` upgrades the declared whole **Source**, while `--artifact` narrows the explicit source target to one declared **Artifact**
    - `upgrade` remains the canonical version-advancement command, and `install --upgrade` is a shortcut for the same upgrade behavior on the selected install target
    - canonical v1 command shapes outside install and upgrade are:
      - `tbboot search <query>`
      - `tbboot catalog add <catalog-reference> [--name <local-name>]`
      - `tbboot catalog list`
      - `tbboot catalog refresh [<local-name>...]`
      - `tbboot catalog remove <local-name>`
      - `tbboot logs [<operation-id>]`
      - `tbboot logs list`
    - default human-facing output uses one short stable shape, with extra detail available only through `--verbosity` or `logs`
  - Still open:
    - shorthand `install <name>`

- **modelo de resolución/versionado** — partially resolved
  - Resolved:
    - stable identity is **Source + Artifact Name**
    - **Catalog** is only an index, not a source of truth
    - duplicate catalog hits are only ambiguous when they resolve to different **Sources**
    - direct installation without any catalog must be supported
    - direct-install version pinning applies to **Source Version** only
    - source-qualified direct installs normalize and check ambiguity only within the declared **Source Type**
    - omitting `--source-version` selects the latest stable published **Source Version**, or fails if the source type has no stable published version concept
    - later syncs keep the previously resolved **Source Version** until the user explicitly requests a newer one
    - moving to a newer resolved version uses a dedicated `upgrade` command
    - bare `upgrade` targets the entire manifest by default
    - `upgrade` advances to the latest stable published version allowed by policy by default
    - source targets upgrade a whole source, while artifact targets upgrade only the selected artifact
    - `upgrade` rejects targets that are not already declared in the **Manifest**
    - artifact-level upgrade is rejected when the manifest declares that source as a whole
    - `upgrade` can preview selected version moves through **Dry Run** without mutating state
    - global `upgrade` processes whole sources before individual artifacts, ordered lexicographically by normalized identity
    - successful `upgrade` writes the new exact resolution to the **Lockfile** and leaves the **Manifest** unchanged by default
    - first successful `install` creates the **Lockfile** when only a **Manifest** exists
    - the **Manifest** stores enough source identity to re-resolve stably if the **Lockfile** is lost or regenerated
    - only stable source identity is normative in the **Manifest**; original user-facing references are optional metadata only
    - v1 adds no extra version-selection controls beyond direct-install `--source-version` and default upgrade-to-latest-stable-allowed behavior
  - Still open:
    - none for v1

- **reglas de sync/overwrite/remove** — partially resolved
  - Resolved:
    - **Sync** remains the underlying reconciliation operation
    - **Overwrite Policy** exists and may inspect Git state
    - **Removal Policy** defaults to prompting before removing a **Managed Artifact**
    - **Catalog Remove** deletes associated **Catalog Cache**
    - removal is decided per final resolved **Managed Artifact**, not by the prior manifest declaration shape
    - whole-file content owned by the same **Managed Artifact** is overwritten automatically when it has no drift
  - Still open:
    - rollback/recovery behavior on partial materialization failure

- **trust policy y defaults de seguridad** — still open
- **trust policy y defaults de seguridad** — resolved for v1 scope
  - Resolved:
    - base **Trust Policy** lives in the versioned **Manifest**
    - **Minimum Age Rule** is evaluated against the exact **Resolution**
    - new repos start with an explicit allowlist of approved sources in the **Manifest**
    - remote source types are denied until explicitly approved
    - source types that support publication-time checks may be gated by a minimum age rule
    - source approval does not automatically approve risky artifact types
    - risky artifact types require explicit allowlisting and first-install confirmation
    - only `file` and `git` are supported approvable source types in v1
    - `file:` is allowed by default only inside the current **Operation Root**
    - `git:` always requires explicit manifest approval

- **ownership/provenance de archivos y fragments** — resolved for v1 scope
  - Resolved:
    - **Materialization Record** tracks what a **Managed Artifact** wrote
    - **Fragment Boundary** is visible by default
    - **Fragment Drift** is detected and prompts before update or removal
    - ownership is exclusive, and overlapping whole-file or fragment claims fail as **Ownership Conflict**
    - managed changes report artifact, source, source version, and ownership kind as minimal provenance

- **manejo de errores, drift y rollback** — partially resolved
  - Resolved:
    - successful operations keep exit code `0` even with warnings
    - stale catalog cache warns and continues by default
    - `--refresh` forces catalog refresh before install when desired
    - **Catalog Add** is atomic on initial fetch
    - rollback is best-effort only when prior state can be restored and verified from pre-write backups
    - unrecoverable partial failure enters explicit **Recovery State**
    - machine-readable command output uses one **JSON Output Envelope** with `code`, `message`, and `details`
    - v1 exit codes are `0` success, `1` operational or validation error, `2` user-action conflict, and `3` trust or policy denial
    - whole-file drift prompts before update or removal
    - policy-blocked upgrade candidates do not change exit code `0` and are reported only in verbose output or logs
    - multi-target `upgrade` stops at the first mutating failure after processing targets in deterministic order
  - Still open:
    - whether any additional rollback guarantees beyond verified best-effort are worth defining in v1

### Immediate open questions

- shorthand `install <name>`
- whether rollback semantics need any stronger guarantees than verified best-effort for v1

### Session checkpoint

- This interview closed a large first-pass v1 model for install, catalog management, trust policy, logs, lockfile/bootstrap, and upgrade semantics.
- The next session should avoid reopening resolved glossary decisions unless a contradiction appears in code or examples.
- The highest-value continuation is to turn the remaining open items into concrete command examples and output samples.

### Deferred future topics

- interactive terminal mode for browsing catalogs, searching sources, and inspecting logs without composing full commands
- additional source types beyond `file` and `git`
- richer version selection controls such as channels, ranges, or upgrade policies beyond "latest stable allowed"
