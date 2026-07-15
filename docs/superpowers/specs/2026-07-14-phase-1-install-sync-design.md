# Phase 1 install and sync design

## Status

Approved.

## Context

Phase 1 of the v1 roadmap is partially implemented. Explicit installation supports one artifact from a local `file:` Source, and bare `install` can replay one persisted declaration. The remaining phase must support realistic Manifests containing multiple artifact-scoped or source-scoped declarations while preserving predictable pinned resolution, drift protection, and non-interactive removal behavior.

Earlier designs established repository-state persistence, declare-only installation, whole-file materialization, and the first narrow reconcile loop. This design extends those foundations without replacing the existing install boundary.

## Goal

Complete phase 1 so bare `tbboot install` can reconcile multiple declarations and produce predictable no-op, apply, drift, ownership-conflict, and removal-required outcomes.

## Non-goals

This design does not add:

- Floating Source behavior;
- automatic managed-artifact removal;
- `upgrade` behavior;
- a real `git` Source;
- fragment, template, script, or prompt materialization;
- strong transactional rollback;
- a new reconciliation package or generic planner framework.

## Decisions

Phase 1 uses these product rules:

- Source-level declarations are pinned snapshots.
- `install <source>` always declares the whole Source, even when the Source currently contains one Artifact.
- Only `install <source> --artifact <name>` creates an artifact-level declaration.
- Source-level and artifact-level declarations for the same Source Identity are invalid together.
- Sync reproduces locked resolutions exactly and never upgrades them.
- Repeating plain explicit `install` for an identical declaration also reproduces its locked resolution and never upgrades it.
- Declarations without a locked resolution are resolved during their first successful Sync.
- A managed Artifact outside the final desired state causes a removal-required conflict before mutation.
- Sync performs no partial apply when preflight detects any conflict.
- Direct `file:` Source Identity is its Source Type plus its canonical locator. The published Source name is metadata, not consumer identity.
- Relative `file:` locators are interpreted against the Operation Root and persisted in normalized root-relative form. Approved external locators are persisted in canonical absolute form.
- Phase 1 does not expose `--source-version` because its only real Source, `file:`, cannot select historical versions. The Manifest reserves no requested-version field; a future version-selecting Source may introduce one with its own design.

## Architecture

`internal/install.Service` remains the command-independent orchestration boundary for explicit installation and Sync.

`internal/repositorystate` owns the persisted Manifest, Lockfile, and Materialization Record. It gains the new Lockfile shape, validation, normalization for programmatic updates, and flat lookup helpers derived from the persisted snapshot blocks.

Each managed Artifact entry records Source Identity, resolved Source version, Artifact name, exact Artifact version, and owned whole-file paths with digests. Artifact version is evidence for cross-checking managed state when locked state must be reconstructed; it is not part of the Managed Artifact identity key.

Like the Lockfile change below, this replaces the undeployed schema-version-1 Materialization Record shape without migration code.

`internal/install` owns:

- conversion of CLI intent into artifact-level or source-level declarations;
- expansion of declarations into final desired state;
- resolution of declarations not yet represented in the Lockfile;
- full-operation preflight;
- deterministic application and result construction.

Sync implementation may move from `service.go` to `sync.go` inside the same package. This is file organization, not a new abstraction.

`cmd/tbboot` remains limited to Cobra wiring, argument and flag parsing, request construction, and output rendering. It continues using `internal/app.Result`.

No `internal/reconcile` package, generic planner, executor interface, or new dependency is introduced. A separate reconciliation package should exist only when another real consumer, such as `upgrade`, demonstrates a shared boundary.

## Lockfile model

The Lockfile stores canonical blocks grouped by resolved Source snapshot:

```yaml
schema_version: 1
resolutions:
  - source:
      type: file
      locator: artifacts/tools
    resolved_version: abc123
    artifacts:
      - name: format
        version: 2.0.0
      - name: lint
        version: 1.0.0
```

A snapshot key is the combination of Source Identity and exact resolved Source version. An Artifact key is the combination of Source Identity and Artifact Name.

The package provides two derived access patterns after loading:

- snapshot lookup by Source Identity and resolved Source version;
- Artifact lookup by Source Identity and Artifact Name.

These are access patterns, not additional persisted indexes. The YAML contains no artificial IDs or cross-list references.

### Lockfile invariants

- Every snapshot has a complete Source Identity and resolved version.
- Every snapshot contains at least one Artifact.
- Every Artifact has a name and exact version.
- An Artifact key appears in exactly one snapshot.
- Multiple snapshots may share a Source Identity when separate artifact-level declarations resolve against different Source versions.
- A source-level declaration with locked state resolves to exactly one snapshot for its Source Identity. Zero snapshots means first resolution is still required; more than one is invalid persisted state.
- A source-level snapshot contains exactly its pinned Artifact set. Sync verifies the resolved Source Identity, resolved Source version, Artifact names, and Artifact versions against that block.
- An artifact-level resolution verifies the resolved Source Identity, resolved Source version, Artifact name, and Artifact version against its locked entry.
- Loading rejects duplicate snapshot blocks and duplicate Artifact keys.
- Programmatic insertion merges Artifacts into an existing block with the same snapshot key.
- Writes sort snapshots by normalized Source Identity and resolved version, then sort Artifacts by name.

Because the schema has not been deployed, this design replaces the existing schema-version-1 Lockfile contract. It does not add migration code or preserve the earlier development-only shape.

## Manifest rules

Manifest scope records user intent; Lockfile grouping does not replace it.

The direct `file:` Source Identity change replaces the undeployed schema-version-1 Manifest identity shape without migration code.

- A source-level target has `scope: source` and no Artifact name.
- An artifact-level target has `scope: artifact` and requires an Artifact name.
- A Source Identity may have one source-level declaration or one or more artifact-level declarations, but not both scopes.
- Duplicate declaration targets are invalid on load and rejected during updates.
- For direct `file:` declarations, Source Identity contains the normalized locator. A preserved `Manifest.Input.Locator` is optional original-reference metadata; when present, it must normalize to the identity locator or the Manifest is invalid.
- `Manifest.Input` contains only the optional preserved locator. Unknown Manifest fields, including `Input.Version`, are invalid persisted state with exit code `1` rather than ignored extension points.
- Exact resolved Source and Artifact versions belong only in the Lockfile.

The `file:` generated local snapshot hash remains an exact resolved version used for later drift-safe replay, not a selectable historical version.

Changing requested intent is neither plain install nor Sync behavior. Later `upgrade` work will define how an existing declaration advances to another version.

## Explicit install behavior

Explicit installation follows this flow:

1. Parse the Source and optional `--artifact`.
2. Select declaration scope from syntax. Source contents do not influence scope.
3. Normalize the acquisition locator into Source Identity, load persisted state, and validate the prospective declaration, including mixed scope.
4. Evaluate Trust Policy before resolving Source content.
5. For a new declaration or one without lock state, resolve and pin the current Source snapshot. For an identical declaration with lock state, resolve and verify only that exact locked snapshot.
6. Select one Artifact for artifact scope or every Artifact in the resolved snapshot for source scope.
7. Run ownership and drift preflight for the selected installation.
8. Materialize supported steps.
9. Persist matching Manifest, Lockfile, and Materialization Record state.

An explicit request that would introduce mixed scopes is a user-action conflict with exit code `2`. Mixed scopes already present in a loaded Manifest are invalid persisted state with exit code `1`.

Repeating an identical explicit declaration never replaces compatible locked state with the Source's current version. If its exact locked resolution is unavailable, explicit install fails with exit code `1`. An attempt to change existing declaration input fails with exit code `2` and points to future `upgrade` behavior.

`--declare-only` resolves enough Source information to validate the reference, requested Artifact existence, and descriptor structure, then writes only the Manifest. It does not require selected Materialization Step types to be supported because it does not apply them, and it does not create a pinned snapshot. The snapshot is fixed by the first later successful Sync, which enforces materialization support and policy.

An identical normalized declaration already present in the Manifest makes `--declare-only` an immediate `no_op`: it does not resolve the Source or write state. Only a new declaration performs Source validation.

## Sync resolution and expansion

Sync loads and validates the Manifest, Lockfile when present, and Materialization Record when present.

A missing Manifest is an operational error with exit code `1`; it is not interpreted as empty desired state. Only a present, structurally valid Manifest with no declarations receives the empty-Manifest behavior below.

For each declaration:

- if a compatible locked resolution exists, Sync uses it exactly;
- if no locked resolution exists, Sync resolves the declaration using its canonical Source Identity;
- if persisted locked state is structurally incompatible with its declaration, Sync returns a validation error instead of repairing or re-resolving it.

Compatibility is verified against newly resolved content, not inferred from lookup alone. Artifact scope requires the same Source Identity, resolved Source version, Artifact name, and Artifact version. Source scope additionally requires the exact complete Artifact name-and-version set from its single snapshot. Any mismatch is an invalid or unavailable exact resolution; Sync does not repair the Lockfile.

Artifact-level declarations contribute one exact Artifact each. A source-level declaration contributes every Artifact in its one pinned snapshot. Sync unions these contributions by Artifact key and sorts them by normalized Source Identity and Artifact Name.

New Artifacts later published by a pinned Source are not added during Sync. They require future upgrade behavior. Conversely, a declaration created by `--declare-only` has no pinned set yet, so its first successful Sync captures the Source contents resolved at that time.

Lockfile Artifacts not justified by the final Manifest are outside desired state. If they have no managed output, a successful Sync prunes that derived stale lock state. Pruning is an effective change and produces `applied`, not `no_op`. If a corresponding managed Artifact exists, Sync reports removal required and preserves all state.

An empty but valid Manifest is a valid desired state. With no relevant Lockfile or Materialization Record entries it produces `no_op` with Artifact count zero. Stale lock-only state is pruned as `applied`; any Managed Artifact produces `removal_required`.

When a desired Artifact has an existing Materialization Record, its recorded resolved Source version and Artifact version must match the locked resolution, and its canonical managed-path set must exactly match the desired file-step target set. A version or partial path-set mismatch is invalid persisted state with exit code `1`. A completely absent Artifact record instead follows the unowned-target rules during preflight.

If the Lockfile is absent while a Materialization Record exists, Sync may reconstruct resolutions only when newly resolved Source version, Artifact version, and canonical path set exactly match the record and desired descriptor. A match writes `resolution_locked`; any mismatch is invalid persisted state with exit code `1`. Existing managed state is never reinterpreted against a new version.

## Preflight and application

Before any mutation, Sync evaluates the complete desired set for:

- Trust Policy admission;
- whole-file ownership conflicts, including collisions among desired Artifacts;
- whole-file drift;
- managed Artifacts absent from desired state;
- valid, unique target paths and other checks possible before writing.

Checks occur in this order:

1. validate persisted structure;
2. evaluate Trust Policy using normalized Source Identity and locator;
3. resolve admitted Sources and verify exact locked state;
4. evaluate ownership, drift, and removal conflicts.

Persisted-state and resolution errors use exit code `1`. Trust denials use exit code `3` and accumulate all denied Sources. Ownership, drift, and removal conflicts use exit code `2`. Source content is not resolved before Trust admission.

Validation and Source-resolution errors stop at the first failure in deterministic declaration order; Sync does not continue reading later Sources or consumer paths. Aggregation is limited to Trust denials and user-action conflicts.

Every canonical whole-file target path must occur exactly once in the complete desired set, including steps belonging to the same Artifact. Duplicate targets inside one Artifact are validation errors; collisions between Artifacts are ownership conflicts. Preflight compares canonical paths using platform filesystem semantics rather than raw descriptor strings.

The repository state files `talby-artifacts.yaml`, `talby-artifacts.lock.yaml`, and `talby-artifacts.managed.yaml` are reserved targets and cannot be materialized by an Artifact.

A consumer target cannot overlap any file read to calculate an active `file:` Source snapshot, including Source descriptors, Artifact descriptors, and step inputs. This prevents installation from changing the Source bytes represented by its own locked version.

Path containment is canonical, not merely lexical. Relative `file:` locators resolve from the Operation Root. Source resolution verifies that Artifact directories remain inside the Source root and that descriptors and step inputs remain inside their corresponding Source or Artifact roots after resolving symlinks. For consumer targets, any existing symlink path component is rejected and the canonical target remains inside the Operation Root.

A whole-file target may be absent or an existing regular file. Directories, symlinks, sockets, FIFOs, devices, and other special filesystem entries are rejected during preflight before their contents are read. A previously managed path that is now absent, a symlink, or non-regular is drift; the same occupied path without prior ownership is an ownership conflict.

An existing unowned target with different content is an ownership conflict and is never overwritten automatically. Identical content may be adopted safely by writing its Materialization Record; adoption is an effective change and produces `applied`.

A recorded managed file that is missing is Whole-File Drift. Sync does not recreate a manually removed managed file without later explicit user-action behavior.

A Managed Artifact outside desired state always produces `removal_required`. Preflight also reports every detectable drift for that Artifact, including modified, missing, symlinked, or non-regular recorded paths, so later removal work receives the complete risk state.

Resolution and preflight capture whole-file source bytes in memory. Application writes those captured bytes instead of reopening Source paths. Immediately before each write, application rechecks that the consumer target still has the state observed during preflight; a change aborts as drift. This closes Source and target time-of-check/time-of-use gaps without introducing a generic planner framework.

Each consumer file write uses a temporary file in the target directory followed by rename, matching the existing repository-state persistence pattern. This provides atomic replacement per file without claiming a transaction across the complete operation.

New consumer files use mode `0644`. Replacing an existing managed regular file preserves its current mode. Phase 1 drift compares content only; declarative file modes and permission drift remain future work.

An Artifact descriptor must contain at least one Materialization Step. Phase 1 materializes only `file` steps. Resolution may parse and hash unsupported steps in unselected Artifacts: an artifact-scoped install is not blocked by unrelated Artifacts, while a source-scoped materialization fails if any selected Artifact contains an unsupported step.

A Source Descriptor must list at least one Artifact. An empty Source is a validation error and never produces an empty snapshot block.

Preflight accumulates all detectable ownership, drift, and removal conflicts rather than stopping after the first. Any conflict prevents Manifest, Lockfile, Materialization Record, and consumer-file writes.

Accumulated conflicts are sorted by kind, normalized Source Identity, Artifact Name, and canonical path before result construction. Human and JSON output therefore remain stable across runs.

After successful preflight, Sync materializes Artifacts in deterministic order and persists newly resolved Lockfile entries and the resulting Materialization Record. A no-op performs no persistence writes.

Operational failures during application retain the existing best-effort rollback behavior. Strong transactionality and verified recovery remain phase 3 work; phase 1 does not claim that arbitrary filesystem or persistence failures can never leave partial state.

## Results and errors

Human and JSON output continue to use the shared operation result model.

Successful outcomes use exit code `0`:

- `no_op` reports the reconciled Artifact count and omits the `changes` field;
- `applied` reports the reconciled Artifact count and effective changes only, including lock pruning and ownership adoption when those are the only changes.

Success details use one typed `changes` list with the minimum kinds `file_created`, `file_updated`, `ownership_adopted`, `resolution_locked`, and `lock_pruned`. Each entry includes Source and Artifact provenance plus a path when applicable. `resolution_locked` appears once per newly pinned declaration, with Artifact omitted for source scope. Unchanged files never appear in this list. Human output renders from the same effective-change model.

Failures use existing exit-code classes:

- exit `1` for invalid persisted state, an unavailable exact resolution, IO failure, or other operational or validation error;
- exit `2` for mixed-scope requests, ownership conflicts, drift, or removal-required outcomes;
- exit `3` for Trust Policy denial.

JSON retains `code`, `message`, `details`, and `warnings`. Sync details include the operation, overall outcome, Artifact count where known, effective changes on success, and typed conflicts on user-action failure. Each conflict identifies its kind and relevant Source, Artifact, and paths. When several user-action conflict kinds occur together, the overall outcome is `conflict`; individual entries retain `ownership`, `drift`, or `removal_required` kinds.

Human success output is one stable summary line followed only by effective changes. Human conflict output identifies the cause and relevant provenance. Planned changes are never reported as applied.

## Testing strategy

Focused `internal/repositorystate` tests cover:

- new Lockfile YAML round trips;
- Materialization Record round trips with exact Artifact version;
- deterministic ordering;
- programmatic snapshot merging;
- rejection of empty blocks, duplicate snapshots, duplicate Artifact keys, and missing fields;
- flat snapshot and Artifact lookups.

Focused `internal/install` tests cover:

- syntax-based declaration scope, including a one-Artifact Source;
- source-level pinned snapshot capture;
- mixed-scope rejection;
- direct `file:` identity and locator normalization;
- rejection of mismatched identity and preserved input locator;
- rejection of unknown persisted fields, including `Manifest.Input.Version`;
- first Sync resolution of declarations without Lockfile entries;
- exact replay of existing resolutions;
- exact replay without implicit upgrade during repeated explicit install;
- rejection of mismatched Source Identity, Source version, Artifact version, and source-scoped Artifact set;
- ignoring newly published Artifacts during pinned Sync;
- multiple declarations and deterministic ordering;
- duplicate desired target detection, including duplicates within one Artifact;
- reserved state-file target rejection;
- rejection of targets overlapping active Source inputs;
- canonical Source and target containment through symlinks;
- rejection of non-regular existing targets;
- unowned-file conflict and identical-file adoption;
- missing managed files as drift;
- combined removal-required and drift reporting;
- Materialization Record version and path-set consistency;
- safe Lockfile reconstruction with existing managed state;
- captured Source bytes and target revalidation before write;
- atomic per-file replacement;
- new-file and preserved-update mode behavior;
- unsupported steps in selected versus unselected Artifacts;
- declare-only acceptance of structurally valid unsupported steps;
- idempotent declare-only no-op without Source access;
- rejection of empty Sources and Artifacts;
- stale Lockfile pruning as an applied change;
- empty-Manifest no-op, pruning, and removal-required outcomes;
- missing-Manifest operational error;
- whole-operation preflight with no writes on conflict;
- deterministic ordering of accumulated conflicts;
- deterministic fail-fast validation and resolution errors;
- typed effective changes for files, adoption, new resolutions, and lock pruning;
- no-op, applied, drift, ownership, removal-required, validation, and trust results.

CLI tests cover exit codes, stdout and stderr placement, JSON envelopes, and outcome details. At least one real-path test uses a temporary repository, multiple declarations, materialized files, and a repeated Sync.

No new test framework, generic fixture layer, or duplicated per-function suite is needed.

## Alternatives rejected

### One complete Lockfile resolution per Artifact

This is operationally simple but repeats Source data and can represent inconsistent Source versions for what should be one pinned snapshot.

### One nested resolution per Source Identity

This represents whole-Source snapshots clearly but cannot naturally represent separate artifact-level declarations resolved from different versions of the same Source.

### Separate flat Source and Artifact lists with references

This avoids duplication but introduces persisted IDs, referential integrity, orphan cleanup, and joins. Snapshot blocks preserve the same normalization without stored references.

### New reconciliation package or generic planner

These create new boundaries before a second consumer exists. Phase 1 extends the existing install service and leaves extraction for demonstrated reuse.

## Completion criteria

Phase 1 is complete when bare `tbboot install` reconciles realistic Manifests containing multiple artifact-level or source-level declarations and produces predictable no-op, apply, drift, ownership-conflict, and removal-required outcomes without mutating state after a failed preflight.
