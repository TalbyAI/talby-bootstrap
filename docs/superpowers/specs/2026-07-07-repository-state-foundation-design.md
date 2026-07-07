# Repository state foundation design

## Status

Proposed.

## Context

The next implementation tranche in the roadmap starts with the persistence foundation for repository state. The repository already has:

- a command-independent `install` service that resolves a `Source` and selects an `Artifact`;
- accepted ADRs defining the distinction between declared state and resolved state;
- example fixtures that already expect `talby-artifacts.yaml` and `talby-artifacts.lock.yaml` to exist as durable repository files.

The first slice should create the durable state model without prematurely implementing the full operational semantics of `install --declare-only`, trust enforcement, file materialization, or sync behavior.

## Goal

Create one internal module that owns the repository persistence model for:

- the **Manifest** at `talby-artifacts.yaml`;
- the **Lockfile** at `talby-artifacts.lock.yaml`.

This slice should make those files real domain concepts with stable YAML contracts, explicit validation, and testable read/write APIs. It should not yet make `install` write repository state as a user-visible behavior.

## Non-goals

This design deliberately excludes:

- user-visible `install` behavior changes;
- `--declare-only` writes from the CLI;
- trust-policy enforcement;
- materialization records or file writes into the consumer repository;
- drift detection;
- `git` source implementation work;
- sync or upgrade semantics.

## Decision

Implement a single `internal/repositorystate` module that owns only persisted repository state.

`internal/repositorystate` will:

- define domain types for `Manifest` and `Lockfile`;
- validate persisted state shapes;
- read and write YAML files for repository state;
- provide small domain helpers for inserting or replacing declarations and resolutions.

`internal/repositorystate` will not:

- resolve sources;
- select artifacts;
- know about `source.ResolvedSource`;
- enforce trust rules;
- materialize files.

That orchestration remains in `internal/install`.

## Module boundary

The new module is a persistence boundary, not an install orchestration boundary.

Responsibilities inside the module:

- repository-state domain types;
- YAML serialization and deserialization;
- stable file paths for the manifest and lockfile;
- validation of required fields and duplicate-state rules;
- pure state update helpers such as upsert operations.

Responsibilities outside the module:

- converting an install resolution into repository-state mutations;
- deciding when files should be written;
- operation-root and trust checks;
- materialization ownership and drift logic.

This keeps the repository-state contract stable even if source resolution evolves for `git`, sync, or trust policy work.

## Data model

The module should keep declared intent separate from exact resolved state.

### Manifest

The **Manifest** stores desired state. For this slice it contains declarations keyed by stable source identity, with support for both whole-source and single-artifact intent. PR 1 should keep install integration limited to single-artifact declarations, but the persisted schema should already represent both target scopes so later sync and upgrade slices do not need a schema rewrite.

Conceptual shape:

```go
type Manifest struct {
    TrustPolicy  TrustPolicy
    Declarations []Declaration
}

type Declaration struct {
    Source SourceIdentity
    Target DeclarationTarget
    Input  *SourceInput
}

type DeclarationTarget struct {
    Scope    DeclarationScope
    Artifact string
}

type DeclarationScope string

const (
    DeclarationScopeArtifact DeclarationScope = "artifact"
    DeclarationScopeSource   DeclarationScope = "source"
)

type TrustPolicy struct {
    ApprovedSources []SourceIdentity
}
```

`SourceIdentity` is the durable source identifier:

```go
type SourceIdentity struct {
    Type string
    Name string
}
```

`SourceInput` is optional metadata that preserves user-facing input without making it normative:

```go
type SourceInput struct {
    Locator string
    Version string
}
```

Rules:

- `SourceIdentity` is the normative source reference in the manifest.
- `DeclarationScopeArtifact` targets one artifact and requires `Artifact`.
- `DeclarationScopeSource` declares the whole source and forbids `Artifact`.
- `Locator` is not treated as durable identity.
- The manifest does not persist an exact resolved source version.
- The manifest includes minimal trust-policy state in schema version 1, but PR 1 does not enforce trust-policy decisions yet.
- `ApprovedSources` is real persisted state, not a placeholder field, even though later slices will enforce it more fully.

### Lockfile

The **Lockfile** stores exact resolved state that can be replayed later.

Conceptual shape:

```go
type Lockfile struct {
    Resolutions []Resolution
}

type Resolution struct {
    Source          SourceIdentity
    ResolvedVersion string
    Artifact        ArtifactResolution
}

type ArtifactResolution struct {
    Name    string
    Version string
}
```

Rules:

- the lockfile persists exact resolved source version state;
- for `file:` sources, that exact source version is the resolved local snapshot marker;
- artifact version is persisted as exact resolved state;
- the lockfile does not yet store materialization ownership or drift data.

## Relationship to ADRs

This design follows ADR-0002 directly:

- stable source identity belongs in the manifest;
- exact resolved source version belongs in the lockfile;
- artifact identity is source plus artifact name;
- user-facing original input is optional metadata only.

It intentionally does not implement ADR-0004 behavior yet. Materialization and ownership data remain out of scope for this slice.

## YAML contract

The module should write:

- `talby-artifacts.yaml` for the manifest;
- `talby-artifacts.lock.yaml` for the lockfile.

The exact field names should optimize for clarity and extension safety rather than mirroring in-memory Go types mechanically.

Constraints for the first version:

- deterministic field ordering;
- stable list ordering after writes;
- explicit top-level `schema_version: 1` in both files;
- no placeholder fields for future materialization data.

The YAML DTO shape may differ from the domain types if that improves validation or keeps the persisted schema explicit.

## API shape

The module should expose explicit load and write operations rather than a generic repository object.

Conceptual API:

```go
type Store interface {
    LoadManifest(ctx context.Context, root string) (Manifest, error)
    WriteManifest(ctx context.Context, root string, manifest Manifest) error
    LoadLockfile(ctx context.Context, root string) (Lockfile, error)
    WriteLockfile(ctx context.Context, root string, lockfile Lockfile) error
}
```

Load operations should return one package-defined typed error for state-file failures:

```go
type StateFileError struct {
    File StateFile
    Kind StateFileErrorKind
    Err  error
}

type StateFile string

const (
    StateFileManifest StateFile = "manifest"
    StateFileLockfile StateFile = "lockfile"
)

type StateFileErrorKind string

const (
    StateFileErrorNotFound      StateFileErrorKind = "not_found"
    StateFileErrorInvalidFormat StateFileErrorKind = "invalid_format"
)
```

Semantics:

- `not_found` means the expected file does not exist yet and the caller may choose bootstrap behavior;
- `invalid_format` means the file exists but is empty, malformed YAML, fails schema-version checks, or fails repository-state validation;
- other filesystem failures such as permission errors remain ordinary operational errors.

Pure domain helpers should remain separate from filesystem IO:

```go
func (m Manifest) UpsertDeclaration(decl Declaration) (Manifest, ChangeKind)
func (l Lockfile) UpsertResolution(res Resolution) (Lockfile, ChangeKind)
func ValidateManifest(m Manifest) error
func ValidateLockfile(l Lockfile) error
```

`ChangeKind` should distinguish:

- inserted;
- replaced;
- unchanged.

This supports later install result reporting without forcing reporting concerns into the persistence module now.

## Integration with install

This slice should prepare, but not yet activate, install persistence behavior.

The intended boundary is:

1. `internal/install` resolves the source and selects the artifact;
2. `internal/install` maps that result into `repositorystate.Declaration` and `repositorystate.Resolution`;
3. later slices decide when those values are written.

For this PR, mapping helpers may be introduced near `internal/install` if needed, but `internal/repositorystate` should not import `internal/source`.

This avoids freezing persistence types around the current `file`-only resolution shape.

## Validation rules

The first slice should enforce only the validation rules required to keep persisted state coherent.

Manifest validation:

- source type is required;
- source name is required;
- declaration scope is required;
- artifact-scoped declarations require artifact name;
- source-scoped declarations must not include artifact name;
- duplicate declarations for the same source identity plus target scope plus artifact name are rejected or normalized deterministically.

Lockfile validation:

- source type is required;
- source name is required;
- resolved source version is required;
- artifact name is required;
- artifact version is required;
- duplicate resolutions for the same source identity plus artifact name are rejected or normalized deterministically.

The implementation should pick one duplicate strategy and make it explicit in tests. The preferred behavior for this slice is deterministic replacement through `Upsert...` helpers and rejection only when loading invalid on-disk files.

## Load semantics

The first slice should make bootstrap and corruption cases explicit.

- Missing `talby-artifacts.yaml` should load as `StateFileError{File: StateFileManifest, Kind: StateFileErrorNotFound}`.
- Missing `talby-artifacts.lock.yaml` should load as `StateFileError{File: StateFileLockfile, Kind: StateFileErrorNotFound}`.
- Present-but-empty files are invalid format, not equivalent to missing files.
- YAML parse failures, unsupported schema versions, and validation failures should surface as `StateFileError` with `Kind: StateFileErrorInvalidFormat`.
- Later slices such as `--declare-only` and bare `install` should rely on this distinction rather than inferring bootstrap state from empty domain values.

## Testing strategy

The slice should be validated primarily through direct tests on the new module.

Required tests:

- manifest YAML round-trip tests;
- lockfile YAML round-trip tests;
- validation tests for missing required fields;
- validation tests for artifact-scoped and source-scoped manifest declarations;
- upsert tests for insert, replace, and unchanged cases;
- duplicate on-disk state rejection tests where applicable;
- load tests for `not_found` versus `invalid_format`.

One cross-module contract test should also prove that a resolved `file:` source can be represented as:

- a stable manifest declaration using source identity plus artifact name;
- a reproducible lockfile resolution using the exact resolved source version and artifact version.

That mapping test should live outside `internal/repositorystate` because the module must remain decoupled from source resolution internals.

## Expected repository impact

This PR should add:

- `internal/repositorystate/` implementation files;
- focused tests for domain rules and YAML persistence;
- any minimal install-adjacent mapping helpers needed to prepare later slices.

This PR should not change:

- CLI output;
- example normative outputs;
- install exit-code behavior;
- trust or materialization flows.

## Follow-on slice

Once this foundation exists, the next slice should implement real `install --declare-only` behavior by:

- resolving and selecting an artifact through `internal/install`;
- mapping that result to a manifest declaration;
- writing only `talby-artifacts.yaml`;
- leaving the lockfile and consumer files untouched.
