# Talby Bootstrap

Talby Bootstrap defines the language for a CLI that installs reusable repository artifacts from published Sources.

## Product 0.1 contract

The 0.1 release is the active contract in this document. It has one implemented command surface: `tbboot install`, including direct scalar `file:<locator>` references, `--artifact`, `--declare-only`, and targetless reconciliation. The implemented acquisition path is local `file:` Sources: in-root Sources are allowed by default, while external absolute Sources require explicit Manifest approval of their **Source Identity**. The only materialization step is whole-file `file`.

All six YAML documents use `schema_version: 1`, strict readers, deterministic writers, and these canonical filenames: `tbboot-source.yaml`, `tbboot-artifact.yaml`, `tbboot-artifacts.yaml`, `tbboot-artifacts.lock.yaml`, `tbboot-artifacts.managed.yaml`, and `tbboot-artifacts.recovery.yaml`. Persisted Source References are scalar `file:<locator>` or `git:<locator>` values; Git identities are stored and validated but not acquired in 0.1. There is no migration from earlier filenames or structured source objects.

Catalog, search, upgrade, Git acquisition, fragment/template/script/prompt steps, full rollback lifecycle, and persisted operation logs are deferred. The deferred design archive below is retained as context only and does not describe implemented 0.1 behavior.

## Language

**Artifact**:
The canonical installable unit managed by the CLI. An artifact is something resolved from a source and materialized into a target repository or folder.
_Avoid_: Pattern, snippet, module

**Artifact Name**:
The identifier used to refer to an **Artifact** within its own **Source**. An artifact name is only required to be unique inside that source.
_Avoid_: Global ID, path

**Artifact Descriptor**:
The explicit manifest published for an **Artifact** that declares its version, descriptive metadata, and materialization steps without requiring the CLI to infer artifact behavior from file contents.
_Avoid_: Inline metadata, inferred manifest

**Artifact Descriptor Filename**:
The canonical filename used by an **Artifact** to publish its **Artifact Descriptor**. The initial filename is `tbboot-artifact.yaml`.
_Avoid_: Generic artifact manifest name, implicit location

**Artifact Descriptor Location**:
The canonical location where an **Artifact Descriptor** is stored. It lives in the root folder of the individual artifact alongside the files owned by that artifact.
_Avoid_: Repository root, inferred location

**Source**:
A published origin that defines and can deliver one or more **Artifacts** through its **Source Descriptor** and artifact contents. A **Source** describes published content, not the consumer's acquisition semantics.
_Avoid_: Catalog

**Acquisition Channel**:
The consumer-side mechanism used to obtain a **Source** for resolution, trust evaluation, and locking. In 0.1, in-root `file:` Sources are allowed by default and external absolute `file:` Sources require explicit Manifest approval of their **Source Identity**; `git:` is a stored identity for a later acquisition implementation.
_Avoid_: Source kind, published transport, inferred protocol

**Source Type**:
The explicit classification of an **Acquisition Channel** used in consumer-facing references, **Manifest** entries, trust policy checks, and **Lockfile** resolution records. The persistence contract recognizes `file` and `git`; 0.1 acquires `file` only, with external absolute locators requiring explicit Manifest approval.
_Avoid_: Published source kind, inferred protocol, source guess

**Source Descriptor**:
The explicit definition published by a **Source** to declare which artifacts it provides and source-local publication metadata. Acquisition semantics such as **Source Type**, trust handling, and consumer resolution behavior do not live in the published descriptor; they live in the consumer-facing reference, **Manifest**, and **Lockfile**.
_Avoid_: Folder inference, implicit structure

**Descriptor Schema Version**:
The explicit schema version declared by a **Source Descriptor** so the CLI can parse it predictably and evolve compatibility over time.
_Avoid_: Implicit format version, source type version

**Source Version**:
The version or snapshot identifier that represents a resolved state of a **Source** as obtained through a specific **Acquisition Channel**. In 0.1, `file` resolution records a deterministic `sha256:` snapshot hash in the **Lockfile**; Git version storage is validated but acquisition and advancement are deferred.
_Avoid_: Artifact version, floating state

**Source Identity**:
The stable consumer-side identity used to distinguish one acquired source from another for declaration, trust, and locking. It consists of **Source Type** plus the normalized source locator, so identical published content acquired through different channels is still treated as different sources.
_Avoid_: Source Descriptor alone, content hash alone, artifact path

**Source Descriptor Filename**:
The canonical filename used by a **Source** to publish its **Source Descriptor**. The initial filename is `tbboot-source.yaml`.
_Avoid_: Generic source manifest name, implicit location

**Catalog**:
An index that lists available **Artifacts**, the **Sources** that provide them, and the versions available for them. A catalog may be a JSON file or an API that can be queried for indexed sources and artifacts. When configured locally, each catalog is assigned a unique local name used in catalog-qualified install references.
_Avoid_: Source, registry

**Catalog Reference**:
A resolvable location or identifier used to register a **Catalog** before it has a configured local name.
_Avoid_: Local catalog name, catalog source

**Direct Install Reference**:
The scalar typed source reference a user can provide to install an **Artifact** directly from its **Source** without relying on any **Catalog**. Its canonical form is `{source-type}:{locator}`; the artifact is selected separately.
_Avoid_: Catalog entry, inferred lookup

**Catalog Install Reference**:
The catalog-qualified install reference a user can provide when resolving through a configured **Catalog** instead of naming a typed **Source** directly. Its canonical form identifies the configured catalog name and the catalog-local source name as `{catalog-name}/{catalog-source-name}`, with an optional artifact selector provided separately. Without an artifact selector, it installs the resolved **Source** as a whole using the same semantics as a direct source install.
_Avoid_: Direct source reference, implicit global lookup

**Manifest**:
A versioned file in a consumer repository that declares which **Artifacts** should be installed there, including enough **Source Identity** to re-resolve them stably even if the lockfile must be regenerated. The stable source identity is normative; any preserved original user-facing reference is optional metadata only.
_Avoid_: Lockfile, machine config

**Manifest Format**:
The canonical serialization used to write a **Manifest**. The initial format is YAML for manual editing.
_Avoid_: Multi-format by default, implicit format

**Manifest Filename**:
The canonical filename used to store the versioned **Manifest** in a consumer repository. The initial filename is `tbboot-artifacts.yaml`.
_Avoid_: Generic config name, implicit location

**Manifest Location**:
The canonical location where a **Manifest** is stored. Consumer manifests live at the repository root.
_Avoid_: Hidden path, arbitrary location

**Lockfile Filename**:
The canonical filename used to store the versioned **Lockfile** in a consumer repository. The initial filename is `tbboot-artifacts.lock.yaml`.
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
A deferred source-level installation mode where future artifacts added to a **Source** may be picked up automatically on later sync operations.
_Avoid_: Pinned source, snapshot

**Pinned Source**:
A deferred source-level installation mode where the installed set of artifacts is limited to the resolved snapshot captured at the time of installation.
_Avoid_: Floating source, live source

**Trust Policy**:
The versioned security policy for approved **Source Identity** values. In 0.1, in-root `file:` Sources are allowed by default, while external absolute `file:` Sources require explicit approval; Git is not acquired and risky materialization steps are not executed. Broader approval and age rules are deferred.
_Avoid_: Machine default, ad hoc flag

**Minimum Age Rule**:
A deferred trust constraint that may reject a resolved installation target unless its exact resolved publication, tag, commit, or version is older than the configured minimum age.
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
The canonical machine-readable output shape for CLI commands when JSON output is requested. It always contains `code`, `message`, `details`, and `warnings`; empty details and warnings use `{}` and `[]` rather than being omitted or set to null. Operation details always contain `operation`, `outcome`, and `dry_run`; other fields are present only when known and relevant, and absent optional fields are omitted rather than set to null.
_Avoid_: Ad hoc JSON, command-specific envelope

**Exit Code**:
The canonical process result code returned by the CLI to classify overall command outcome for shells, CI, and automation. The 0.1 CLI uses `0` for success, `1` for operational or validation errors, `2` for user-action conflicts, and `3` for trust or policy denial.
_Avoid_: Message parsing, ad hoc status

**CLI Command Name**:
The canonical executable name used to invoke Talby Bootstrap from the command line. The initial command name is `tbboot`.
_Avoid_: talby, tboot

**Distribution Package**:
A platform-specific archive that distributes the `tbboot` executable. It is release material for the CLI, not an **Artifact** managed by the CLI.
_Avoid_: Release artifact, binary artifact

**Release Candidate**:
The exact commit on `main` proposed for publication as a product release. Its acceptance evidence, distribution packages, and release metadata are all tied to that commit.
_Avoid_: Latest main, build candidate

**Acceptance Evidence**:
The executable test, validation result, or explicitly recorded review that demonstrates a retained 0.1 product contract for a Release Candidate.
_Avoid_: Best-effort check, informal verification

**Acceptance Gate**:
The blocking condition a Release Candidate must satisfy before publication, based on its required Acceptance Evidence.
_Avoid_: Advisory check, post-release cleanup

**Acceptance Manifest**:
The machine-readable record mapping a Release Candidate to its required Acceptance Evidence and gate results.
_Avoid_: Test log, informal checklist

**Release Owner**:
The named person accountable for recording candidate acceptance and any explicitly permitted cosmetic or documentation exception.
_Avoid_: Anonymous approver, automatic approval

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
The implemented user-facing command for artifact management. The canonical command is `install`, with alias `i`; without arguments it runs **Sync**, and with one scalar direct Source Reference it declares and applies that target unless `--declare-only` is requested.
_Avoid_: sync command, add command

**Upgrade Command**:
The deferred command intended to advance already-declared artifacts or sources to newer resolved versions. It is not available in 0.1.
_Avoid_: Sync alias, catalog refresh

**Operation Summary**:
The default short human-readable output shown after the implemented `install` or **Sync** operation. Success is written to standard output and failure to standard error.
_Avoid_: Full verbose log, success-only silence

**Operation Root**:
The consumer root used to resolve relative Source References and scope repository state and materialization. If the current location belongs to a Git repository, the Operation Root is the repository root; otherwise it is the current local folder.
_Avoid_: Session cwd only, global implicit scope

**Declare-Only Install**:
An **Install Command** mode enabled by `--declare-only` that updates only the **Manifest** without materializing artifacts or updating the **Lockfile**. In 0.1 it applies to direct scalar Source References.
_Avoid_: Partial install, preview install

**Catalog Refresh**:
The deferred catalog maintenance operation that refreshes cached catalog metadata and indexes without changing the **Manifest**, **Lockfile**, or installed artifacts.
_Avoid_: Upgrade check, install refresh

**Catalog Cache**:
The deferred cached catalog metadata and indexes stored in the **User Configuration Directory** for discovery and resolution support.
_Avoid_: Installed artifacts, lockfile state

**Catalog Add**:
The deferred catalog maintenance operation that registers a catalog in active configuration and performs an initial fetch.
_Avoid_: Config-only add, deferred activation

**Catalog Remove**:
The deferred catalog maintenance operation that removes a catalog from active configuration and deletes its associated **Catalog Cache**.
_Avoid_: Soft disable, stale cache retention

**Catalog List**:
The deferred catalog maintenance operation that shows configured catalogs together with operational status.
_Avoid_: Config-only listing, verbose diagnostics dump

**Search Command**:
The deferred top-level command that queries configured catalog caches and returns matching **Sources**.
_Avoid_: catalog admin command, full repository search

**Managed Artifact**:
An **Artifact** whose installed state is tracked by the CLI as part of the repository's managed configuration.
_Avoid_: Untracked file, incidental content

**Materialization Step**:
One declared action in an **Artifact Descriptor** that writes, updates, renders, or executes part of an artifact installation.
_Avoid_: Artifact type, hidden install behavior

**Materialization Step Type**:
The category of a **Materialization Step** that defines how that step is interpreted. Product 0.1 accepts only whole-file `file`; fragment, template, script, and prompt are deferred.
_Avoid_: Artifact kind, source type

**Removal Policy**:
The deferred rule that determines how **Sync** handles a **Managed Artifact** that is no longer present in the final desired state. Prune is not implemented in 0.1.
_Avoid_: Garbage collection, overwrite policy

**Materialization Record**:
The tracked record of what a **Managed Artifact** wrote into the target repository or folder.
_Avoid_: Best-effort guess, inferred state

**Provenance Summary**:
The minimal user-visible identity shown for an effective or planned managed change: canonical Source Reference, Source Version, Artifact Name when applicable, canonical root-relative path when applicable, and ownership kind when applicable.
_Avoid_: Opaque change, path-only report

**Ownership Conflict**:
The explicit conflict state where two **Managed Artifacts** would claim the same whole file. Fragment overlap is deferred; ownership is exclusive.
_Avoid_: Shared ownership, last-write-wins

**Recovery State**:
The explicit failure state recorded atomically when verified best-effort rollback cannot restore every affected path to its prior observed state. **Dry Run** may report it but never clears it.
_Avoid_: Silent partial failure, implicit dirty state

**Recovery State Filename**:
The canonical filename used to store **Recovery State** at the consumer repository root. The initial filename is `tbboot-artifacts.recovery.yaml`.
_Avoid_: Recovery log, temporary backup filename

**Fragment**:
A bounded section inserted by an **Artifact** inside an existing file instead of creating or replacing the whole file.
_Avoid_: Whole file, anonymous patch

**Fragment Boundary**:
The deferred explicit start and end markers that identify a **Fragment** so it can be updated or removed safely later.
_Avoid_: Fuzzy match, heuristic location

**Fragment Drift**:
The deferred state where the content inside a managed **Fragment** no longer matches what the CLI previously materialized.
_Avoid_: Expected update, normal sync

**Whole-File Drift**:
The state where a whole file owned by a **Managed Artifact**, or its required parent topology, no longer matches the prior **Materialization Record**. Changed content, absence, symlinks, and non-regular entries are drift; permission-only differences are not.
_Avoid_: Expected update, normal sync

## Relationships

- An **Artifact** comes from exactly one source location
- An **Artifact** is materialized into a target repository or folder
- An **Artifact** publishes exactly one **Artifact Descriptor**
- An **Artifact Descriptor** has exactly one canonical **Artifact Descriptor Filename**
- An **Artifact Descriptor** lives in the root folder of its individual **Artifact**
- An **Artifact Descriptor** declares one or more **Materialization Steps**
- A **Materialization Step** has exactly one **Materialization Step Type**
- Product 0.1 accepts only the whole-file `file` **Materialization Step Type**
- A **Source** provides one or more **Artifacts**
- A **Source** does not declare its **Source Type** in the published descriptor
- An **Acquisition Channel** has exactly one **Source Type**
- A **Source** publishes exactly one **Source Descriptor**
- A **Source Descriptor** has exactly one canonical **Source Descriptor Filename**
- A **Source Descriptor** lives at the source repository root
- A **Source Descriptor** declares exactly one **Descriptor Schema Version**
- A **Source** may publish one or more **Source Versions**
- A **Source Identity** includes **Source Type** plus the consumer-side source locator
- An **Artifact Name** is unique only within its **Source**
- The stable identity of an **Artifact** is the combination of its **Source** and **Artifact Name**
- An **Artifact Descriptor** includes the artifact version and descriptive metadata
- The version of an **Artifact** is defined only in its **Artifact Descriptor**
- An **Artifact** may be published as part of a specific **Source Version**
- A **Catalog** references one or more **Sources** and the **Artifacts** they provide
- A **Catalog Reference** identifies the catalog being registered by **Catalog Add**
- A **Catalog** is an index for discovery, not the source of truth for an **Artifact**
- A **Manifest** belongs to one consumer repository
- A **Manifest** has exactly one canonical **Manifest Format**
- A **Manifest** has exactly one canonical **Manifest Filename**
- A **Manifest** lives at the consumer repository root
- A **Manifest** declares one or more desired **Artifacts**
- A **Manifest** may also declare an entire **Source Identity**, which implies all of its resolved **Artifacts**
- Whole-**Source** install in 0.1 is pinned to its recorded snapshot
- A **Manifest** defines the base **Trust Policy** for its consumer repository
- A **Trust Policy** defines an explicit allowlist of approved **Source Identity** values
- A **Manifest** may define an **Overwrite Policy**
- Local machine settings may harden the **Trust Policy** or require more confirmation
- A **Manifest** expresses installation intent, not exact resolved versions
- A **Resolution** is derived from a **Manifest**
- **Minimum Age Rule** evaluation is deferred beyond 0.1
- A **Lockfile** persists a **Resolution** for reproducible installation
- A **Lockfile** has exactly one canonical **Lockfile Filename**
- A **Lockfile** lives at the consumer repository root
- A whole-**Source** resolution in the **Lockfile** includes the exact resolved **Artifacts** and their versions
- An **Overwrite Policy** may inspect Git state before replacing files
- The **Artifact Management Surface** is centered on the **Install Command**
- A direct scalar `install file:<locator>` reference is resolved without a catalog
- A command may render results through the **JSON Output Envelope** when machine-readable output is requested
- An **Exit Code** classifies command outcome as success, operational error, user-action conflict, or trust/policy denial
- Install and Sync show an **Operation Summary** immediately
- A successful operation writes its result to standard output; a failed operation writes its result to standard error
- Human warnings use standard error, while JSON warnings remain inside the single **JSON Output Envelope**
- A **Provenance Summary** accompanies effective install changes
- Catalog management and **Catalog Cache** are deferred beyond 0.1
- The **Install Command** without arguments runs **Sync**
- The **Install Command** may use a **Direct Install Reference** without any configured **Catalog**
- **Catalog Install Reference** and **Search Command** are deferred beyond 0.1
- The **Install Command** with artifact arguments declares and applies those **Artifacts** by default
- The **Install Command** accepts a scalar **Direct Install Reference** without any configured **Catalog**
- **Declare-Only Install** updates only the **Manifest**
- Catalog commands and **Search Command** are deferred beyond 0.1
- **Sync** reconciles actual state against the **Manifest** and its **Resolution**
- **Dry Run** is deferred beyond 0.1
- The canonical **CLI Command Name** for this tool is `tbboot`
- A **Managed Artifact** is eligible for whole-file drift detection by **Sync**
- A **Managed Artifact** is materialized by applying its declared whole-file `file` **Materialization Steps**
- A **Materialization Record** belongs to a **Managed Artifact**
- A **Materialization Record** tracks whole files in 0.1; **Fragments** are deferred
- A managed change is reported to the user with a **Provenance Summary**
- An **Ownership Conflict** exists when two **Managed Artifacts** would claim the same whole file; fragment overlap is deferred
- **Recovery State** has a persisted schema contract; runtime creation and rollback lifecycle are deferred
- **Recovery State** has exactly one canonical **Recovery State Filename**
- **Recovery State** stores canonical root-relative paths and sanitized failure metadata, not prior file contents or raw errors
- Recovery blocking and manual-repair verification are deferred beyond 0.1
- Fragment boundaries and **Fragment Drift** are deferred beyond 0.1
- **Whole-File Drift** is detected by comparing the current whole-file contents against the prior **Materialization Record**
- **Whole-File Drift** is accumulated during preflight and blocks the entire mutation with a user-action conflict
- Prune and broader **Removal Policy** behavior are deferred beyond 0.1
- A whole file already owned by the same **Managed Artifact** may be overwritten automatically when it still matches the prior **Materialization Record**

## Canonical examples

`tbboot install file:./artifacts`
: Resolves the in-root local Source, records its scalar Source Reference and snapshot in the canonical state files, and materializes all declared artifacts.

`tbboot install file:./artifacts --artifact editorconfig`
: Resolves and materializes one named artifact from the in-root Source.

`tbboot install file:./artifacts --declare-only`
: Records only the scalar declaration in `tbboot-artifacts.yaml`; it does not write a Lockfile or materialize files.

`tbboot install file:./artifacts --artifact editorconfig`
: Resolves an in-root Source and records its local snapshot hash in the **Lockfile**.

`tbboot install git:https://example.com/example/artifacts.git`
: Is stored as a valid Source Reference by the persistence contract but fails in 0.1 because Git acquisition is deferred.

## Example dialogue

> **Dev:** "What command do I run for Talby Bootstrap without consuming the broader Talby name?"
> **Domain expert:** "Use the canonical **CLI Command Name** `tbboot`, and keep `Talby` reserved for the wider tool suite."
>
> **Dev:** "If I run `tbboot install foo`, does that only declare `foo` or also apply it?"
> **Domain expert:** "The **Install Command** declares and applies by default; use **Declare-Only Install** when you want to update only the **Manifest**."

## Design history (not active 0.1 contract)

The remaining notes preserve earlier product exploration. They are not implementation promises for 0.1; future tickets must promote a capability into the active contract before code or fixtures rely on it.

- "pattern", "snippet", and "module" were used for the same concept — resolved: use **Artifact** as the canonical term
- "source" and "catalog" could be conflated — resolved: **Source** delivers, while **Catalog** indexes
- "manifest" and "lockfile" could be conflated — resolved: **Manifest** declares intent, **Lockfile** stores exact resolution
- artifact identity could be ambiguous across sources — resolved: an **Artifact Name** is source-local, and stable identity is **Source + Artifact Name**
- whole-source install behavior could be unsafe by default — resolved: default to **Pinned Source** and require explicit opt-in for **Floating Source**
- trust configuration could drift outside the repo — resolved: base **Trust Policy** lives in the versioned **Manifest**
- temporal trust checks could be applied at the wrong level — resolved: **Minimum Age Rule** is validated against the exact **Resolution**
- overwrite handling could be too blunt — resolved: use an **Overwrite Policy** that can consult Git state plus manifest and CLI overrides
- planning and applying could be split unnecessarily — resolved: the **Install Command** is the primary user-facing command, and **Sync** remains the underlying reconciliation operation
- source acquisition semantics could become implicit and fragile — resolved: every consumer-facing source reference declares an explicit **Source Type**, while the published **Source Descriptor** stays transport-agnostic
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
- local cache could conflate catalog indexes with source materialization — resolved: v1 includes **Catalog Cache** for catalog metadata and indexes, but excludes source and materialization caches until a real performance need is measured
- manifest discovery could be ambiguous — resolved: the canonical **Manifest Filename** is `tbboot-artifacts.yaml`
- lockfile discovery could be ambiguous — resolved: the canonical **Lockfile Filename** is `tbboot-artifacts.lock.yaml`
- source descriptor discovery could be ambiguous — resolved: the canonical **Source Descriptor Filename** is `tbboot-source.yaml`
- artifact descriptor discovery could be ambiguous — resolved: the canonical **Artifact Descriptor Filename** is `tbboot-artifact.yaml`
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
- source approval could implicitly allow dangerous artifact behavior — resolved: source trust and materialization-step trust are separate; risky **Materialization Step Types** require explicit allowlisting and first-install confirmation
- safe managed upgrades could become noisy and interactive — resolved: whole-file content already owned by the same **Managed Artifact** is overwritten automatically when it has no drift against the prior **Materialization Record**
- rollback guarantees could over-promise atomicity — resolved: v1 only guarantees verified best-effort rollback, and unrecoverable partial failure enters explicit **Recovery State**
- related artifacts could create ambiguous file ownership — resolved: ownership is exclusive, and any whole-file or overlapping-fragment collision is an explicit **Ownership Conflict**
- managed changes could be too opaque to audit — resolved: every managed change reports a **Provenance Summary** with artifact, source, source version, and ownership kind
- source-type sprawl could outpace the trust model — resolved: v1 supports only `file` and `git` as approvable **Source Types**
- direct and catalog-qualified installs could collapse into one ambiguous syntax — resolved: typed-source installs use **Direct Install Reference**, while catalog-qualified installs use **Catalog Install Reference**
- catalog-qualified source installs could diverge semantically from direct source installs — resolved: a **Catalog** is only an index of sources, so `{catalog-name}/{catalog-source-name}` without an artifact selector installs the same whole **Source** a direct reference would resolve
- CLI output could fragment across human and automation use cases — resolved: v1 uses one **JSON Output Envelope** with always-present `code`, `message`, `details`, and `warnings` when JSON output is requested
- error handling could become too granular or too vague for automation — resolved: v1 uses four **Exit Codes** only: `0` success, `1` operational or validation error, `2` user-action conflict, `3` trust or policy denial
- whole-file drift could behave differently from fragment drift — resolved: whole-file managed content also prompts before update or removal when it no longer matches the prior **Materialization Record**
- search results could fight the install mental model — resolved: **Search Command** returns **Sources** first, with artifact detail level controlled by output flags
- catalog-qualified references could drift if local catalog names are unstable — resolved: each configured **Catalog** has a unique local name, and **Catalog Add** fails on local-name conflicts while reporting the conflicting catalog
- default output could be too noisy or too thin — resolved: install, Sync, and upgrade emit one short **Operation Summary** followed only by effective or planned changes
- persisted diagnostics could add unsupported complexity — resolved: v1 has immediate output only, with no persisted logs, replay, operation identifiers, retention, or verbosity levels
- catalog naming could stay too implicit at creation time — resolved: `catalog add <catalog-reference> --name <local-name>` is the canonical form, with default-name fallback only when non-conflicting
- catalog status output could be too thin to operate safely — resolved: `catalog list` defaults to `local-name`, `catalog-reference`, `cache-status`, `last-refresh`, and `last-refresh-result`
- catalog refresh scope could diverge from global search/install resolution — resolved: `catalog refresh` without arguments refreshes all configured catalogs
- catalog removal could target the wrong identity layer — resolved: `catalog remove` targets the configured local catalog name, not the remote catalog reference
- search could blur into accidental catalog dumping — resolved: non-interactive `search` requires an explicit query, leaving queryless exploration to a future interactive mode
- declare-only behavior could drift between install modes — resolved: `--declare-only` updates only the **Manifest** for direct and catalog-qualified installs alike
- multi-target install could overcomplicate prompts, conflicts, and rollback — resolved: v1 allows only one explicit install target per command, while bare `install` remains the **Sync** entrypoint
- install target parsing could become precedence-driven and surprising — resolved: ambiguous target syntax fails and requires an explicit unambiguous install form
- supported source types could still have unsafe default trust rules — resolved: `file:` is allowed by default only inside the current **Operation Root**, while `git:` always requires explicit manifest approval
- direct install defaults could undermine reproducibility — resolved: omitting `--source-version` selects the latest stable published **Source Version** for `git`, while `file` records a local snapshot hash in the **Lockfile**
- unpinned source declarations could silently float on later syncs — resolved: default version selection is only for the first resolution, and later syncs keep the previously resolved **Source Version** until the user explicitly changes it
- the underlying **Sync** operation could be mistaken for a top-level CLI command — resolved: users trigger reconciliation through bare `install`, while **Sync** remains the underlying operation name
- upgrade behavior could get conflated with install or catalog refresh — resolved: advancing to a newer resolved version uses a dedicated `upgrade` command
- upgrade scope could be unclear by default — resolved: bare `upgrade` attempts to advance the entire manifest
- upgrade selection could be under-specified — resolved: `upgrade` advances `git:` targets to the latest stable published Source Version allowed by trust and age rules, while `file:` targets record a new snapshot hash from the current local path
- skipped version detail could overwhelm normal success output — resolved: routine skipped versions are omitted from v1 diagnostics
- upgrade target syntax could drift away from install syntax — resolved: `upgrade` reuses the same unambiguous target forms as `install`, with source targets upgrading a whole source and artifact targets upgrading only that artifact
- upgrade could silently become install for undeclared targets — resolved: `upgrade` rejects targets not already declared in the **Manifest**
- whole-source declarations could be undermined by artifact-level upgrades — resolved: when a source is declared as a whole, `upgrade` rejects narrower artifact-level targets inside that source
- upgrade could become the only mutating command without a safe preview — resolved: `upgrade` supports the same non-mutating **Dry Run** contract as other reconciliation flows
- global upgrade failure handling could become inconsistent or over-optimistic — resolved: multi-target `upgrade` runs in deterministic order and stops at the first mutating failure
- deterministic upgrade ordering could still be underspecified — resolved: global `upgrade` processes whole sources before individual artifacts, with lexicographic ordering by normalized identity inside each group
- upgrade output could blur intent and resolution — resolved: successful `upgrade` writes the new exact version only to the **Lockfile**, not to the **Manifest**, unless declared intent changes
- first-time manifest reconciliation could leave reproducibility unbootstrapped — resolved: `install` without an existing **Lockfile** resolves from the **Manifest** and creates the lockfile during that operation
- catalog-based declarations could become unstable after lockfile loss — resolved: the **Manifest** stores enough source identity to re-resolve declared targets stably without depending on fresh search ambiguity
- manifest entries could end up with two competing truths — resolved: only stable source identity is normative in the **Manifest**, while any original user-facing reference is optional metadata only

## Interview state archive

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
    - **Search Command** returns sources as the primary result unit, with artifact detail controlled by flags
    - each configured catalog has a unique local name, and conflicting local names make **Catalog Add** fail explicitly
    - install, Sync, and upgrade show one immediate **Operation Summary** followed only by effective or planned changes
    - v1 has no persisted operation logs, replay, operation identifiers, retention, or verbosity levels
    - `catalog add <catalog-reference> --name <local-name>` is the canonical add form, with default-name fallback only when non-conflicting
    - `catalog list` default output is `local-name`, `catalog-reference`, `cache-status`, `last-refresh`, and `last-refresh-result`
    - `catalog refresh` without arguments refreshes all configured catalogs
    - `catalog remove` canonically targets the configured local catalog name
    - non-interactive `search` requires an explicit query
    - `--declare-only` updates only the **Manifest** across direct and catalog-qualified installs
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
    - `upgrade` is the only v1 version-advancement command; `install --upgrade` is not supported
    - canonical v1 command shapes outside install and upgrade are:
      - `tbboot search <query>`
      - `tbboot catalog add <catalog-reference> [--name <local-name>]`
      - `tbboot catalog list`
      - `tbboot catalog refresh [<local-name>...]`
      - `tbboot catalog remove <local-name>`
    - default human-facing output uses one short stable shape with no verbosity controls
  - Still open:
    - none for v1

- **modelo de resolución/versionado** — partially resolved
  - Resolved:
    - stable identity is **Source + Artifact Name**
    - **Catalog** is only an index, not a source of truth
    - duplicate catalog hits are only ambiguous when they resolve to different **Sources**
    - direct installation without any catalog must be supported
    - direct-install version pinning applies to **Source Version** only
    - source-qualified direct installs normalize and check ambiguity only within the declared **Source Type**
    - omitting `--source-version` selects the latest stable published **Source Version** for `git`, while `file` records a local snapshot hash in the **Lockfile**
    - later syncs keep the previously resolved **Source Version** until the user explicitly requests a newer one
    - moving to a newer resolved version uses a dedicated `upgrade` command
    - bare `upgrade` targets the entire manifest by default
    - explicit `upgrade` accepts only one target in v1
    - `upgrade` advances `git:` targets to the latest stable published Source Version allowed by policy
    - `upgrade` re-reads each targeted `file:` Source from its current local path and records the resulting snapshot hash; an unchanged snapshot is a no-op
    - Source targets upgrade every declared Artifact in the Source snapshot, while Artifact targets upgrade only the selected Artifact and leave sibling Artifacts on their existing snapshots
    - `upgrade` rejects targets that are not already declared in the **Manifest**
    - artifact-level upgrade is rejected when the manifest declares that source as a whole
    - `upgrade` can preview selected version moves through **Dry Run** without mutating state
    - global `upgrade` processes whole sources before individual artifacts, ordered lexicographically by normalized identity
    - successful `upgrade` writes the new exact resolution to the **Lockfile** and leaves the **Manifest** unchanged by default
    - first successful `install` creates the **Lockfile** when only a **Manifest** exists
    - the **Manifest** stores enough source identity to re-resolve stably if the **Lockfile** is lost or regenerated
    - only stable source identity is normative in the **Manifest**; original user-facing references are optional metadata only
    - v1 adds no extra version-selection controls beyond direct-install `--source-version`; `git:` upgrade selects the latest stable published Source Version allowed by policy, while `file:` upgrade records a fresh local snapshot hash
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
    - rollback on partial materialization failure is verified best-effort only
  - Still open:
    - none for v1

- **trust policy y defaults de seguridad** — resolved for v1 scope
  - Resolved:
    - base **Trust Policy** lives in the versioned **Manifest**
    - **Minimum Age Rule** is evaluated against the exact **Resolution**
    - new repos start with an explicit allowlist of approved sources in the **Manifest**
    - remote source types are denied until explicitly approved
    - source types that support publication-time checks may be gated by a minimum age rule
    - source approval does not automatically approve risky materialization step types
    - risky materialization step types require explicit allowlisting and first-install confirmation
    - v1 materialization step types are `file`, `fragment`, `template`, `script`, and `prompt`
    - `script` and `prompt` are risky materialization step types
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
    - required prompts fail with exit code `2` in non-interactive execution or JSON output
    - whole-file drift prompts before update or removal
    - routine skipped-version detail is omitted from v1 diagnostics
    - bare `upgrade` stops at the first mutating failure after processing targets in deterministic order
  - Still open:
    - none for v1

### Immediate open questions

- none for v1

### Session checkpoint

- This interview closed a large first-pass v1 model for install, catalog management, trust policy, lockfile/bootstrap, and upgrade semantics.
- The next session should avoid reopening resolved glossary decisions unless a contradiction appears in code or examples.
- The highest-value continuation is to turn the remaining open items into concrete command examples and output samples.

### Deferred future topics

- interactive terminal mode for browsing catalogs and searching sources without composing full commands
- additional source types beyond `file` and `git`
- richer version selection controls such as ranges or upgrade policies beyond "latest stable allowed"
