# Phase 1 install and sync implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete phase 1 so explicit install and bare `tbboot install` safely reconcile multiple source-scoped or artifact-scoped declarations against exact locked `file:` Source snapshots.

**Architecture:** Keep `internal/install.Service` as the orchestration boundary. Replace the undeployed repository-state schemas in place, make `internal/source/file` return canonical paths and captured bytes, keep concrete whole-file inspection and atomic writes in `internal/materialize`, and perform desired-state expansion plus full-operation preflight in `internal/install/sync.go`.

**Tech Stack:** Go 1.26.4, Cobra 1.10.2, `gopkg.in/yaml.v3` 3.0.1, Go standard library, `just`.

## Global constraints

- Source-level declarations are pinned snapshots.
- `install <source>` always declares the whole Source, even when the Source currently contains one Artifact.
- Only `install <source> --artifact <name>` creates an artifact-level declaration.
- Source-level and artifact-level declarations for the same Source Identity are invalid together.
- Sync reproduces locked resolutions exactly and never upgrades them.
- Repeating plain explicit `install` for an identical declaration reproduces its locked resolution and never upgrades it.
- Declarations without a locked resolution are resolved during their first successful Sync.
- A managed Artifact outside the final desired state causes a removal-required conflict before mutation.
- Sync performs no partial apply when preflight detects any conflict.
- Direct `file:` Source Identity is its Source Type plus its canonical locator; published Source name is metadata only.
- Relative `file:` locators are interpreted against the Operation Root and persisted in normalized root-relative form. Approved external locators are persisted in canonical absolute form.
- Phase 1 does not support Source version input; add it when a version-selectable Source Type exists.
- Phase 1 materializes only `file` steps. It adds no `git` Source, upgrade behavior, automatic removal, rollback transaction, generic planner, reconciliation package, or dependency.
- State schema version remains exactly `1`; old development-only schema shapes receive no migration path.
- Tasks 1-5 form one compile migration and one reviewer gate. Their focused tests are checkpoints; do not create an intermediate commit while repository consumers still use a replaced API. Task 5 ends the migration with `go test -race ./... -count=1` before its commit-approval pause.
- Before every plan commit, stop and obtain explicit user approval at that moment. Never treat earlier approval as permission.

---

## File map

- Modify `internal/repositorystate/model.go`: canonical Source Identity, grouped Lockfile snapshots, exact managed Artifact versions, result-independent lookup keys.
- Modify `internal/repositorystate/manifest.go`: mixed-scope, duplicate, locator, and requested-input invariants.
- Modify `internal/repositorystate/lockfile.go`: grouped-snapshot validation, lookup, merge, and prune helpers.
- Modify `internal/repositorystate/materialization_record.go`: Artifact identity and exact-version validation plus lookup helpers.
- Modify `internal/repositorystate/store.go`: replacement schema DTOs and deterministic YAML mapping.
- Modify repository-state tests: replacement schema round trips, ordering, lookup, merge, and invalid-state cases.
- Modify `internal/source/model.go`: captured file bytes and canonical active Source input paths.
- Modify `internal/source/file/source.go`: canonical containment, non-selectable version rejection, empty descriptor rejection, unsupported-step parsing, and captured input bytes.
- Modify `internal/source/file/source_test.go`: real-path and descriptor behavior.
- Modify `internal/materialize/service.go`: canonical target observation, target revalidation, and atomic write.
- Modify `internal/materialize/service_test.go`: file-kind, symlink, mode, revalidation, and atomic-write behavior.
- Modify `internal/install/repository_state.go`: conversions between Source results and new state types.
- Modify `internal/install/service.go`: explicit install and declare-only flow; shared public result/error types.
- Create `internal/install/sync.go`: declaration expansion, exact lock verification, complete preflight, deterministic apply, pruning, and persistence.
- Modify `internal/install/service_test.go` and create `internal/install/sync_test.go`: focused unit and real-path coverage.
- Modify `cmd/tbboot/install.go` and `cmd/tbboot/root.go`: shared success/conflict rendering and exit classification.
- Modify `cmd/tbboot/root_test.go` and `cmd/tbboot/examples_e2e_test.go`: CLI envelopes, streams, exit codes, and multi-declaration replay.

No new package or interface is introduced. Existing `source.Registry`, `repositorystate.Store`, and `install.Service` remain the only seams needed by tests.

### Task 1: Replace repository-state schemas and invariants

**Files:**

- Modify: `internal/repositorystate/model.go`
- Modify: `internal/repositorystate/manifest.go`
- Modify: `internal/repositorystate/lockfile.go`
- Modify: `internal/repositorystate/materialization_record.go`
- Modify: `internal/repositorystate/store.go`
- Modify: `internal/repositorystate/manifest_test.go`
- Modify: `internal/repositorystate/lockfile_test.go`
- Modify: `internal/repositorystate/materialization_record_test.go`
- Modify: `internal/repositorystate/store_test.go`

**Interfaces:**

- Consumes: operation root passed to Store load/write methods.
- Produces: `SourceIdentity{Type, Locator}`, `ArtifactKey`, `SnapshotKey`, grouped `Resolution.Artifacts`, `ManagedArtifactRecord.ArtifactVersion`, `NormalizeSourceIdentity`, `AcquisitionLocator`, `Lockfile.Snapshot`, `Lockfile.Artifact`, `Lockfile.UpsertResolution`, and `MaterializationRecord.Artifact`.

- [ ] **Step 1: Replace repository-state model types and write failing model tests**

Replace state-domain declarations in `internal/repositorystate/model.go` with these exact shapes; retain existing `ChangeKind`, `StateFile`, and `StateFileError` declarations below them.

```go
type Manifest struct {
 TrustPolicy  TrustPolicy
 Declarations []Declaration
}

type TrustPolicy struct {
 ApprovedSources []SourceIdentity
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

type SourceIdentity struct {
 Type    string `yaml:"type" json:"type"`
 Locator string `yaml:"locator" json:"locator"`
}

const SourceTypeFile = "file"

type SourceInput struct {
 Locator string
}

type ArtifactKey struct {
 Source SourceIdentity
 Name   string
}

type SnapshotKey struct {
 Source          SourceIdentity
 ResolvedVersion string
}

type Lockfile struct {
 Resolutions []Resolution
}

type Resolution struct {
 Source          SourceIdentity
 ResolvedVersion string
 Artifacts       []ArtifactResolution
}

type ArtifactResolution struct {
 Name    string
 Version string
}

type MaterializationRecord struct {
 Artifacts []ManagedArtifactRecord
}

type ManagedArtifactRecord struct {
 Source          SourceIdentity
 ResolvedVersion string
 Artifact        string
 ArtifactVersion string
 Files           []ManagedFileRecord
}

type ManagedFileRecord struct {
 Path   string
 Digest string
}
```

Replace old tests with focused tests named below. Each constructs literal values using the exact types above and asserts exact errors or returned values; do not add a shared fixture layer.

```go
func TestValidateManifestRejectsDuplicateAndMixedScopes(t *testing.T)
func TestNormalizeSourceIdentityStoresRootRelativeAndExternalAbsoluteLocators(t *testing.T)
func TestValidateManifestRejectsMismatchedPreservedLocator(t *testing.T)
func TestValidateLockfileRejectsEmptyAndDuplicateSnapshotState(t *testing.T)
func TestLockfileLookupsAndSnapshotMerge(t *testing.T)
func TestValidateMaterializationRecordRequiresExactVersionsAndUniquePaths(t *testing.T)
func TestMaterializationRecordArtifactLookupUsesSourceAndArtifactName(t *testing.T)
```

For example, mixed scope must use this assertion:

```go
err := ValidateManifest(root, Manifest{Declarations: []Declaration{
 {Source: SourceIdentity{Type: "file", Locator: "sources/tools"}, Target: DeclarationTarget{Scope: DeclarationScopeSource}},
 {Source: SourceIdentity{Type: "file", Locator: "sources/tools"}, Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "lint"}},
}})
if err == nil || !strings.Contains(err.Error(), "mixes source and artifact scopes") {
 t.Fatalf("ValidateManifest() error = %v, want mixed-scope error", err)
}
```

- [ ] **Step 2: Run repository-state unit tests and confirm schema failures**

Run: `go test ./internal/repositorystate -count=1`

Expected: FAIL with compile errors referencing removed `SourceIdentity.Name`, singular `Resolution.Artifact`, and `ManagedArtifactKey`.

- [ ] **Step 3: Implement canonical Manifest validation and update semantics**

Replace `internal/repositorystate/manifest.go` with code exposing these signatures:

```go
func NormalizeSourceIdentity(root string, source SourceIdentity) (SourceIdentity, error)
func AcquisitionLocator(root string, source SourceIdentity) (string, error)
func ValidateManifest(root string, manifest Manifest) error
func (manifest Manifest) AddDeclaration(root string, declaration Declaration) (Manifest, ChangeKind, error)
func DeclarationKey(declaration Declaration) string
func SourceIdentityKey(source SourceIdentity) string
```

`NormalizeSourceIdentity` must use `filepath.Abs`, `filepath.Clean`, `filepath.Rel`, and `filepath.ToSlash`. Interpret relative locators against Operation Root, never process CWD. For an in-root `file:` locator, return a cleaned relative locator; for an external locator, return the cleaned absolute locator. Reject empty Type/Locator and unsupported Source Types. `AcquisitionLocator` returns the canonical absolute filesystem path represented by a normalized identity, joining relative locators to Operation Root and rechecking normalization. `ValidateManifest` must normalize every approved Source and declaration Source, require persisted values already equal their normalized values, enforce target shape, reject duplicate targets, reject mixed scope by `SourceIdentityKey`, and require any non-empty preserved `Input.Locator` to normalize to the declaration Source. `AddDeclaration` must normalize its argument, return `ChangeKindUnchanged` only for `reflect.DeepEqual`, reject changed input for an existing target, reject mixed scope, append otherwise, and call `ValidateManifest` before returning.

Use these stable keys; they avoid artificial persisted IDs:

```go
func SourceIdentityKey(source SourceIdentity) string {
 return source.Type + "\x00" + filepath.ToSlash(source.Locator)
}

func DeclarationKey(declaration Declaration) string {
 return SourceIdentityKey(declaration.Source) + "\x00" + string(declaration.Target.Scope) + "\x00" + declaration.Target.Artifact
}
```

- [ ] **Step 4: Implement grouped Lockfile and Materialization Record helpers**

`internal/repositorystate/lockfile.go` must provide:

```go
func ValidateLockfile(lockfile Lockfile) error
func (lockfile Lockfile) Snapshot(key SnapshotKey) (Resolution, bool)
func (lockfile Lockfile) Artifact(key ArtifactKey) (Resolution, ArtifactResolution, bool)
func (lockfile Lockfile) UpsertResolution(resolution Resolution) (Lockfile, ChangeKind, error)
func (lockfile Lockfile) KeepArtifacts(keys map[ArtifactKey]struct{}) (Lockfile, []ArtifactKey)
```

Validation requires complete snapshot fields, at least one Artifact per snapshot, complete Artifact fields, unique snapshot keys, and each Artifact key in exactly one snapshot. `UpsertResolution` sorts copied Artifacts by name, merges missing Artifacts into an equal snapshot key, rejects an Artifact key already belonging to another snapshot, sorts snapshots by Source Identity then resolved version, and never mutates its receiver. `KeepArtifacts` removes unjustified Artifacts, drops empty snapshots, returns removed keys in the same deterministic Artifact-key order, and never treats pruning as automatic managed removal.

`internal/repositorystate/materialization_record.go` must provide:

```go
func ValidateMaterializationRecord(record MaterializationRecord) error
func (record MaterializationRecord) Artifact(key ArtifactKey) (ManagedArtifactRecord, bool)
func UpsertManagedArtifact(record MaterializationRecord, next ManagedArtifactRecord) MaterializationRecord
func ManagedArtifactKey(record ManagedArtifactRecord) ArtifactKey
```

Validation requires Source, resolved Source version, Artifact name, exact Artifact version, at least one owned file, canonical non-empty paths, lowercase SHA-256 digests, unique Artifact keys, and globally unique paths. Upsert replaces by `ArtifactKey`, then sorts Artifacts and each Artifact's files.

- [ ] **Step 5: Replace YAML DTOs and round-trip mappings**

Use this exact persisted shape in `internal/repositorystate/store.go`:

```go
type lockfileResolutionDTO struct {
 Source          SourceIdentity         `yaml:"source"`
 ResolvedVersion string                 `yaml:"resolved_version"`
 Artifacts       []lockfileArtifactDTO `yaml:"artifacts"`
}

type materializationArtifactRecordDTO struct {
 Source          SourceIdentity                   `yaml:"source"`
 ResolvedVersion string                           `yaml:"resolved_version"`
 Artifact        string                           `yaml:"artifact"`
 ArtifactVersion string                           `yaml:"artifact_version"`
 Files           []materializationManagedFileDTO `yaml:"files"`
}
```

Change manifest load/write calls to `ValidateManifest(root, manifest)`. Map grouped Artifact slices without flattening, map managed exact Artifact versions, and sort with exported key helpers. Keep `writeFileAtomically` unchanged. Add round-trip assertions against exact YAML containing `source.type`, `source.locator`, grouped `artifacts`, and `artifact_version`; delete assertions for old `source.name`, singular `artifact`, and nested `key`.

- [ ] **Step 6: Run formatting and repository-state tests**

Run: `gofmt -w internal/repositorystate/*.go`

Run: `go test ./internal/repositorystate -count=1`

Expected: PASS.

### Task 2: Harden `file:` Source resolution and capture immutable inputs

**Files:**

- Modify: `internal/source/model.go`
- Modify: `internal/source/file/source.go`
- Modify: `internal/source/file/source_test.go`

**Interfaces:**

- Consumes: `source.ResolveRequest{Ref source.Ref}` where `Ref.Locator` is canonical and `Ref.Version` is an optional requested version.
- Produces: `ResolvedSource.Identity` as published metadata, `ResolvedSource.InputPaths []string`, and `MaterializationStep.SourceBytes []byte`; all paths are canonical absolute paths.

- [ ] **Step 1: Write failing Source behavior tests**

Add these independent tests to `internal/source/file/source_test.go`:

```go
func TestResolveRejectsRequestedVersion(t *testing.T)
func TestResolveRejectsEmptySourceArtifactList(t *testing.T)
func TestResolveRejectsArtifactWithoutSteps(t *testing.T)
func TestResolveParsesUnsupportedStepWithoutRejectingSource(t *testing.T)
func TestResolveCapturesFileBytesAndEveryInputPath(t *testing.T)
func TestResolveRejectsArtifactSymlinkEscapingSourceRoot(t *testing.T)
func TestResolveRejectsStepInputSymlinkEscapingArtifactRoot(t *testing.T)
func TestResolveRejectsSourceDescriptorSymlinkEscapingSourceRoot(t *testing.T)
func TestResolveRejectsArtifactDescriptorSymlinkEscapingArtifactRoot(t *testing.T)
```

Captured-byte assertion:

```go
got, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Type: "file", Locator: root}})
if err != nil {
 t.Fatalf("Resolve() error = %v", err)
}
if string(got.Artifacts[0].Steps[0].SourceBytes) != "captured\n" {
 t.Fatalf("SourceBytes = %q, want captured bytes", got.Artifacts[0].Steps[0].SourceBytes)
}
if len(got.InputPaths) != 3 {
 t.Fatalf("InputPaths count = %d, want source descriptor, artifact descriptor, and step input", len(got.InputPaths))
}
```

- [ ] **Step 2: Run focused Source tests and confirm failure**

Run: `go test ./internal/source/file -count=1`

Expected: FAIL because unsupported steps are rejected, empty descriptors pass, symlink containment is lexical, and captured fields do not exist.

- [ ] **Step 3: Extend resolved Source types**

Change only these types in `internal/source/model.go`:

```go
type MaterializationStep struct {
 Type       string
 TargetPath string
 SourceBytes []byte
}

type ResolvedSource struct {
 Identity   Identity
 Artifacts  []ArtifactDescriptor
 InputPaths []string
}
```

Remove both obsolete `SourcePath` fields. Keep `Source`, `Registry`, `Capabilities`, `Ref`, and `Identity` APIs unchanged.

- [ ] **Step 4: Replace lexical resolution with canonical containment**

Implement these helpers in `internal/source/file/source.go`:

```go
func canonicalExistingDir(path string) (string, error) {
 abs, err := filepath.Abs(path)
 if err != nil {
  return "", err
 }
 return filepath.EvalSymlinks(abs)
}

func canonicalContained(root string, path string, message string) (string, error) {
 canonical, err := filepath.EvalSymlinks(path)
 if err != nil {
  return "", err
 }
 rel, err := filepath.Rel(root, canonical)
 if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
  return "", errors.New(message)
 }
 return canonical, nil
}
```

At Resolve entry, reject `req.Ref.Version != ""` with `file source does not support requested versions`. Canonicalize Source root before reading. Resolve the Source descriptor with `canonicalContained(sourceRoot, filepath.Join(sourceRoot, sourceDescriptorName), "source descriptor must stay within source root")`. Canonicalize every Artifact directory against Source root, resolve each Artifact descriptor with `canonicalContained(artifactDir, filepath.Join(artifactDir, artifactDescriptorName), "artifact descriptor must stay within artifact directory")`, and canonicalize every step input against its Artifact directory. Read descriptors only through those canonical paths. Append canonical Source descriptor, Artifact descriptor, and file-step input paths to `InputPaths`. Read file-step bytes once, store them in `SourceBytes`, and hash those same bytes. Sort neither published Artifact order nor steps; descriptor order remains hash input.

Change descriptor checks to:

```go
if len(descriptor.Artifacts) == 0 {
 return fmt.Errorf("source must contain at least one artifact")
}
```

and:

```go
if len(descriptor.Steps) == 0 {
 return fmt.Errorf("artifact must contain at least one materialization step")
}
```

For unsupported step types, preserve `Type` and `TargetPath` in `MaterializationStep`; do not read a Source payload and do not reject until a selected Artifact is preflighted for materialization. Continue validating `path` for every step and `source` for `file` steps.

- [ ] **Step 5: Run Source tests and full Go tests to expose downstream compile work**

Run: `gofmt -w internal/source/model.go internal/source/file/source.go internal/source/file/source_test.go`

Run: `go test ./internal/source/file -count=1`

Expected: PASS.

Run: `go test ./... -run '^$'`

Expected: repository-state consumers may still fail to compile; record exact failures for the Tasks 4-5 compile migration rather than adding compatibility fields.

### Task 3: Make whole-file IO preflightable and race-aware

**Files:**

- Modify: `internal/materialize/service.go`
- Modify: `internal/materialize/service_test.go`

**Interfaces:**

- Consumes: Operation Root, descriptor target path, captured desired bytes.
- Produces: `Observe(root, path) (Observation, error)`, `SameObservation`, and `Write(Observation, []byte) error`.

- [ ] **Step 1: Replace apply-centric tests with concrete IO tests**

Add focused tests with these exact names:

```go
func TestObserveCanonicalizesAbsentAndRegularTargets(t *testing.T)
func TestObserveRejectsExistingSymlinkPathComponent(t *testing.T)
func TestObserveReportsFinalSymlinkAndNonRegularTargets(t *testing.T)
func TestWriteCreatesWithMode0644(t *testing.T)
func TestWritePreservesExistingMode(t *testing.T)
func TestWriteRejectsChangedTargetSinceObservation(t *testing.T)
func TestWriteReplacesThroughTemporaryFileInTargetDirectory(t *testing.T)
```

Race check assertion:

```go
observed, err := Observe(root, "README.md")
if err != nil {
 t.Fatalf("Observe() error = %v", err)
}
if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed"), 0o644); err != nil {
 t.Fatal(err)
}
err = Write(observed, []byte("desired"))
var changed ChangedSincePreflightError
if !errors.As(err, &changed) {
 t.Fatalf("Write() error = %T %v, want ChangedSincePreflightError", err, err)
}
```

- [ ] **Step 2: Run materialization tests and confirm missing APIs**

Run: `go test ./internal/materialize -count=1`

Expected: FAIL because `Observation`, `Observe`, `Write`, and `ChangedSincePreflightError` do not exist.

- [ ] **Step 3: Replace `internal/materialize/service.go` public API**

Use these exact declarations:

```go
type EntryKind string

const (
 EntryAbsent  EntryKind = "absent"
 EntryRegular EntryKind = "regular"
 EntrySymlink EntryKind = "symlink"
 EntryOther   EntryKind = "other"
)

type Observation struct {
 Root         string
 Path         string
 AbsolutePath string
 Kind         EntryKind
 Mode         os.FileMode
 Digest       string
}

type ChangedSincePreflightError struct{ Path string }

func (err ChangedSincePreflightError) Error() string {
 return fmt.Sprintf("target %q changed after preflight", err.Path)
}

func Observe(root string, target string) (Observation, error)
func PathKey(absolutePath string) string
func SameObservation(left Observation, right Observation) bool
func Write(observed Observation, content []byte) error
func Digest(content []byte) string
```

`Observe` must clean a relative target, reject absolute and escaping paths, canonicalize Operation Root, walk every existing parent with `os.Lstat`, reject any symlink or non-directory parent, and classify final path with `os.Lstat`. Read bytes only for a regular file. Store canonical Root and slash-normalized root-relative Path. `PathKey` cleans an absolute path and applies `strings.ToLower` on Windows so equality follows platform path semantics. `SameObservation` compares Root, Path, AbsolutePath, Kind, Mode permission bits, and Digest. `Write` calls `Observe(observed.Root, observed.Path)` immediately before mutation, returns `ChangedSincePreflightError` if observations differ, creates missing parents, writes a temp file in target directory, applies `0644` for a new target or preserves current permission bits for a regular target, closes, and renames. Reuse the existing repository-state atomic-write sequence; do not expose an executor interface.

- [ ] **Step 4: Run materialization tests**

Run: `gofmt -w internal/materialize/service.go internal/materialize/service_test.go`

Run: `go test ./internal/materialize -count=1`

Expected: PASS.

### Task 4: Implement declaration scope and exact explicit-install replay

**Files:**

- Modify: `internal/install/repository_state.go`
- Modify: `internal/install/service.go`
- Modify: `internal/install/repository_state_test.go`
- Modify: `internal/install/service_test.go`

**Interfaces:**

- Consumes: new repository-state and captured Source APIs from Tasks 1-3.
- Produces: shared `install.Result`, typed changes/conflicts, syntax-selected declaration scope, normalized identity, declare-only no-op, and exact-lock explicit replay.

- [ ] **Step 1: Define result and error contracts with failing tests**

Replace old result declarations in `internal/install/service.go` with:

```go
type Outcome string

const (
 OutcomeNoOp   Outcome = "no_op"
 OutcomeApplied Outcome = "applied"
 OutcomeConflict Outcome = "conflict"
)

type ChangeKind string

const (
 ChangeDeclarationAdded ChangeKind = "declaration_added"
 ChangeFileCreated       ChangeKind = "file_created"
 ChangeFileUpdated       ChangeKind = "file_updated"
 ChangeOwnershipAdopted  ChangeKind = "ownership_adopted"
 ChangeResolutionLocked  ChangeKind = "resolution_locked"
 ChangeLockPruned        ChangeKind = "lock_pruned"
)

type Change struct {
 Kind     ChangeKind                     `json:"kind"`
 Source   repositorystate.SourceIdentity `json:"source"`
 Artifact string                         `json:"artifact,omitempty"`
 Path     string                         `json:"path,omitempty"`
}

type ConflictKind string

const (
 ConflictIntent         ConflictKind = "intent"
 ConflictOwnership      ConflictKind = "ownership"
 ConflictDrift          ConflictKind = "drift"
 ConflictRemovalRequired ConflictKind = "removal_required"
)

type Conflict struct {
 Kind     ConflictKind                   `json:"kind"`
 Source   repositorystate.SourceIdentity `json:"source"`
 Artifact string                         `json:"artifact,omitempty"`
 Paths    []string                       `json:"paths,omitempty"`
}

type Result struct {
 Operation     string     `json:"operation"`
 Outcome       Outcome    `json:"outcome"`
 ArtifactCount int        `json:"artifact_count"`
 Changes       []Change   `json:"changes,omitempty"`
 Conflicts     []Conflict `json:"conflicts,omitempty"`
}

type UserActionError struct{ Result Result }
type TrustPolicyError struct{ Denied []repositorystate.SourceIdentity }
```

`ConflictIntent` represents an explicit request that mixes declaration scopes or changes an existing declaration input. It carries normalized Source identity and the requested Artifact when artifact-scoped; it has no paths. `UserActionError.Error` returns a stable summary based on conflict count. `TrustPolicyError.Error` lists sorted denied Source identities. Add tests for JSON omission of empty changes, deterministic error text, source-level selection for a one-Artifact Source, artifact-level selection only with `--artifact`, typed mixed-scope and changed-input rejection, direct-file locator identity, declare-only unsupported-step acceptance, declare-only identical no-op without Resolve, repeated explicit install using locked resolution, and explicit install preserving other locked and managed declarations.

Use these exact regression-test names for the new intent and state-preservation behavior:

```go
func TestInstallMixedScopeReturnsTypedIntentConflict(t *testing.T)
func TestInstallChangedInputReturnsTypedIntentConflict(t *testing.T)
func TestInstallPreservesOtherLockedAndManagedDeclarations(t *testing.T)
```

- [ ] **Step 2: Run focused install tests and confirm failures**

Run: `go test ./internal/install -run 'Test(InstallScope|InstallIdentity|DeclareOnly|RepeatedExplicit)' -count=1`

Expected: FAIL because current install always chooses artifact scope, resolves before no-op detection, and uses published Source name as identity.

- [ ] **Step 3: Replace state conversion helpers**

Use these signatures in `internal/install/repository_state.go`:

```go
func declarationFor(request Request, identity repositorystate.SourceIdentity) repositorystate.Declaration
func resolutionFor(identity repositorystate.SourceIdentity, resolved source.ResolvedSource, artifacts []source.ArtifactDescriptor) repositorystate.Resolution
func managedRecordFor(identity repositorystate.SourceIdentity, resolved source.ResolvedSource, artifact source.ArtifactDescriptor, files []repositorystate.ManagedFileRecord) repositorystate.ManagedArtifactRecord
```

`declarationFor` selects source scope when `request.Artifact == ""`, otherwise artifact scope using the requested name. Preserve `Input.Locator` only when it differs textually from normalized identity Locator. `resolutionFor` includes all selected exact Artifact versions in one snapshot. `managedRecordFor` records resolved Source version and exact Artifact version while identity remains Source plus Artifact name.

- [ ] **Step 4: Implement explicit install in terms of shared reconcile preparation**

Keep request types:

```go
type Request struct {
 Root        string
 Source      source.Ref
 Artifact    string
 DeclareOnly bool
}

type SyncRequest struct{ Root string }
```

Implement `Install` in this exact order:

1. Validate Root, Source Type, Locator, and reject CLI `Source.Version`.
2. Normalize Source Identity from Type plus locator before Source resolution, then use `AcquisitionLocator` for filesystem access.
3. Load Manifest or empty, construct syntax-scoped declaration, call `AddDeclaration`, and reject changed or mixed intent as `UserActionError` with exit-class conflict data.
4. If identical `--declare-only`, return `no_op` without registry lookup or Source access.
5. Evaluate trust against normalized identity before registry lookup.
6. Lookup Source and resolve once using the canonical acquisition locator. Check published metadata only for display; never replace consumer identity.
7. For `--declare-only`, verify requested Artifact existence and descriptor structure, write only Manifest, and return `applied` with `declaration_added`.
8. Load Lockfile and Materialization Record as empty when absent. If compatible lock exists, verify current resolved identity/version/Artifact set exactly and use it. If none exists, build one snapshot for selected scope.
9. Call `preflightFiles` and `applyPrepared` from Task 5 with only this declaration's desired set. `preflightFiles` checks selected targets against the complete Materialization Record for ownership and drift but does not classify unselected managed Artifacts as removals and does not prune unselected Lockfile entries. Removal detection and stale-lock pruning belong only to `prepareSyncUndesired`, which explicit install never calls.
10. Upsert only this declaration's resolution and managed records into the existing Lockfile and Materialization Record; preserve every unrelated entry byte-for-byte at the domain-model level. Put the prospective Manifest in `nextManifest` and call `service.persistPrepared(ctx, request.Root, "install", prepared, &nextManifest)`. Preserve current best-effort cleanup for newly created files if state persistence fails.

Do not keep `selectArtifact`'s implicit one-Artifact behavior. Replace it with:

```go
func selectedArtifacts(resolved source.ResolvedSource, target repositorystate.DeclarationTarget) ([]source.ArtifactDescriptor, error)
```

Source scope returns every Artifact and errors only on an empty Source. Artifact scope returns exactly the named Artifact.

- [ ] **Step 5: Run explicit-install tests**

Run: `gofmt -w internal/install/repository_state.go internal/install/service.go internal/install/*_test.go`

Run: `go test ./internal/install -run 'TestInstall|TestDeclareOnly|TestRepeatedExplicit' -count=1`

Expected: PASS after Task 5 shared functions exist. If executing sequentially, add Task 4 tests now, leave implementation compile completion in Task 5, and do not add temporary compatibility APIs.

### Task 5: Implement multi-declaration Sync, full preflight, and deterministic apply

**Files:**

- Create: `internal/install/sync.go`
- Create: `internal/install/sync_test.go`
- Modify: `internal/install/service.go`
- Modify: `internal/install/service_test.go`

**Interfaces:**

- Consumes: Manifest declarations, grouped Lockfile, managed records, Source registry, concrete materialization observations.
- Produces: `Service.Sync`, plus package-private `prepare`, `apply`, and exact verification used by explicit install.

- [ ] **Step 1: Write focused Sync tests before implementation**

Create `internal/install/sync_test.go` with local fake Source/Store helpers only where real files cannot prove ordering or call counts. Add independent tests named:

```go
func TestSyncMissingManifestIsOperationalError(t *testing.T)
func TestSyncEmptyManifestNoOp(t *testing.T)
func TestSyncFirstResolutionLocksDeclaration(t *testing.T)
func TestSyncReplaysExactArtifactResolution(t *testing.T)
func TestSyncReplaysExactSourceArtifactSetWithoutAddingPublishedArtifacts(t *testing.T)
func TestSyncRejectsIdentitySourceVersionArtifactVersionAndSetMismatch(t *testing.T)
func TestSyncSupportsMultipleDeclarationsInDeterministicOrder(t *testing.T)
func TestSyncAggregatesTrustDenialsBeforeResolution(t *testing.T)
func TestSyncRejectsDuplicateDesiredTargetWithinArtifact(t *testing.T)
func TestSyncAggregatesOwnershipDriftAndRemovalWithoutWrites(t *testing.T)
func TestSyncRejectsReservedAndActiveSourceInputTargets(t *testing.T)
func TestSyncRejectsNonRegularUnownedTarget(t *testing.T)
func TestSyncAdoptsIdenticalUnownedFile(t *testing.T)
func TestSyncRevalidatesAdoptedFileBeforePersistence(t *testing.T)
func TestSyncReportsMissingManagedFileAsDrift(t *testing.T)
func TestSyncRejectsManagedVersionAndPathSetMismatch(t *testing.T)
func TestSyncReconstructsMissingLockOnlyOnExactManagedMatch(t *testing.T)
func TestSyncPrunesStaleUnmanagedLockState(t *testing.T)
func TestSyncEmptyManifestPrunesOrRequiresRemoval(t *testing.T)
func TestSyncRejectsUnsupportedStepOnlyWhenSelected(t *testing.T)
func TestSyncFailsAtFirstResolutionErrorInDeclarationOrder(t *testing.T)
func TestSyncUsesCapturedBytesAndRevalidatesTarget(t *testing.T)
func TestSyncWritesNoStateOnAnyPreflightConflict(t *testing.T)
```

Whole-operation assertion pattern:

```go
beforeManifest := mustRead(t, filepath.Join(root, repositorystate.ManifestFileName))
beforeLock := mustRead(t, filepath.Join(root, repositorystate.LockfileFileName))
beforeManaged := mustRead(t, filepath.Join(root, repositorystate.MaterializationRecordFileName))

_, err := service.Sync(context.Background(), SyncRequest{Root: root})
var conflict UserActionError
if !errors.As(err, &conflict) {
 t.Fatalf("Sync() error = %T %v, want UserActionError", err, err)
}
if got := mustRead(t, filepath.Join(root, repositorystate.ManifestFileName)); !bytes.Equal(got, beforeManifest) {
 t.Fatal("Manifest changed after failed preflight")
}
if got := mustRead(t, filepath.Join(root, repositorystate.LockfileFileName)); !bytes.Equal(got, beforeLock) {
 t.Fatal("Lockfile changed after failed preflight")
}
if got := mustRead(t, filepath.Join(root, repositorystate.MaterializationRecordFileName)); !bytes.Equal(got, beforeManaged) {
 t.Fatal("Materialization Record changed after failed preflight")
}
```

- [ ] **Step 2: Run Sync tests and confirm failure**

Run: `go test ./internal/install -run '^TestSync' -count=1`

Expected: FAIL because current Sync requires one declaration and existing lock/managed files, mutates per Artifact, and stops on first conflict.

- [ ] **Step 3: Define concrete preparation types in `sync.go`**

Use package-private concrete types, not interfaces:

```go
type desiredArtifact struct {
 Key             repositorystate.ArtifactKey
 Resolution      repositorystate.ArtifactResolution
 ResolvedVersion string
 Descriptor      source.ArtifactDescriptor
 InputPaths      []string
}

type plannedFile struct {
 Artifact   desiredArtifact
 Step       source.MaterializationStep
 Observed   materialize.Observation
 Digest     string
 Change     ChangeKind
}

type preparedOperation struct {
 Desired       []desiredArtifact
 Files         []plannedFile
 Lockfile      repositorystate.Lockfile
 Record        repositorystate.MaterializationRecord
 Changes       []Change
 Conflicts     []Conflict
}
```

Provide these exact helpers:

```go
func (service Service) prepare(ctx context.Context, root string, manifest repositorystate.Manifest, lockfile repositorystate.Lockfile, record repositorystate.MaterializationRecord) (preparedOperation, error)
func (service Service) resolveDeclaration(ctx context.Context, root string, declaration repositorystate.Declaration, lockfile repositorystate.Lockfile, record repositorystate.MaterializationRecord) ([]desiredArtifact, *repositorystate.Resolution, error)
func verifyLocked(declaration repositorystate.Declaration, locked repositorystate.Resolution, resolved source.ResolvedSource) ([]source.ArtifactDescriptor, error)
func preflightFiles(root string, desired []desiredArtifact, record repositorystate.MaterializationRecord) ([]plannedFile, []Conflict, error)
func prepareSyncUndesired(root string, desired []desiredArtifact, lockfile repositorystate.Lockfile, record repositorystate.MaterializationRecord) (repositorystate.Lockfile, []Change, []Conflict, error)
func applyPrepared(root string, prepared preparedOperation) (repositorystate.MaterializationRecord, []Change, []string, error)
func revalidateAdoptions(files []plannedFile) error
func (service Service) persistPrepared(ctx context.Context, root string, operation string, prepared preparedOperation, manifest *repositorystate.Manifest) (Result, error)
```

`preflightFiles` evaluates only desired Artifacts. It checks their targets against the complete record for ownership and drift but never infers that an unselected managed Artifact should be removed. `prepareSyncUndesired` is the only helper that adds `removal_required`, checks drift on removed Artifacts, and prunes stale unmanaged Lockfile entries; only `prepare` calls it. `revalidateAdoptions` calls `materialize.Observe(file.Observed.Root, file.Observed.Path)` for every `ownership_adopted` file and returns `materialize.ChangedSincePreflightError` unless `materialize.SameObservation` remains true.

`persistPrepared` calls `applyPrepared`, then `revalidateAdoptions` after consumer writes and immediately before any state write. It maps a changed adoption to a drift `UserActionError`. On success it writes each changed state file in Lockfile, Materialization Record, then optional Manifest order; `manifest == nil` means Sync leaves Manifest unchanged. It omits every state write for a no-op. On application or persistence failure it removes only consumer files created by this operation. Already replaced consumer or state files retain phase 1 best-effort semantics; this function does not claim transactionality.

- [ ] **Step 4: Implement ordered load, trust admission, resolution, and expansion**

`Sync` must:

```go
func (service Service) Sync(ctx context.Context, request SyncRequest) (Result, error) {
 if request.Root == "" {
  return Result{}, fmt.Errorf("repository root is required for sync")
 }
 manifest, err := service.store.LoadManifest(ctx, request.Root)
 if err != nil {
  return Result{}, err
 }
 lockfile, err := service.loadLockfileOrEmpty(ctx, request.Root)
 if err != nil {
  return Result{}, err
 }
 record, err := service.loadMaterializationRecordOrEmpty(ctx, request.Root)
 if err != nil {
  return Result{}, err
 }
 prepared, err := service.prepare(ctx, request.Root, manifest, lockfile, record)
 if err != nil {
  return Result{}, err
 }
 if len(prepared.Conflicts) != 0 {
  result := resultForConflicts("sync", len(prepared.Desired), prepared.Conflicts)
  return result, UserActionError{Result: result}
 }
 return service.persistPrepared(ctx, request.Root, "sync", prepared, nil)
}
```

Before any lookup or Resolve, sort declarations by `DeclarationKey`, collect every denied normalized Source, sort and return one `TrustPolicyError`. Then resolve declarations in order, stopping on first registry, validation, or resolution failure. Convert persisted identities to canonical acquisition locators with `AcquisitionLocator`. For locked artifact scope, use `Lockfile.Artifact`; for locked source scope, require exactly one snapshot for Source Identity. Resolve current `file:` content with no requested version and verify exact resolved version plus Artifact versions/set. For no lock, resolve current content and append `resolution_locked` once per declaration. Union contributions by `ArtifactKey`, sort by Source Identity and Artifact name, and reject incompatible duplicates.

When Lockfile is absent but managed state exists, accept reconstruction only if resolved Source version, Artifact version, and canonical desired path set equal managed state exactly. Otherwise return exit-class `1` error.

- [ ] **Step 5: Implement complete preflight and conflict ordering**

For every selected Artifact, reject unsupported steps. Normalize each target through `materialize.Observe`. Compare targets and active Source inputs with `materialize.PathKey`. Reject duplicate paths within one Artifact as validation errors. Record collisions between Artifacts as ownership conflicts. Reject the three repository-state filenames. Reject target equality with any active Source `InputPaths` using canonical absolute paths.

For each observed target:

- Managed regular with recorded digest unequal current digest: drift.
- Managed absent, symlink, or other: drift.
- Unmanaged absent: `file_created`.
- Unmanaged regular with desired digest: `ownership_adopted`.
- Unmanaged regular with different digest, symlink, or other: ownership.
- Managed regular matching recorded digest but differing desired bytes: `file_updated`.
- Managed record whose resolved Source version, Artifact version, or canonical path set differs from desired: operational persisted-state error.

After `preflightFiles`, `prepare` calls `prepareSyncUndesired`. That Sync-only helper adds `removal_required` for every managed Artifact absent from final Manifest-derived desired state, plus drift paths detectable from its recorded files. It never auto-removes managed state. It prunes only stale Lockfile Artifacts without managed state. Explicit install does not call this helper. Sort combined conflicts with:

```go
slices.SortFunc(conflicts, func(left, right Conflict) int {
 leftKey := string(left.Kind) + "\x00" + repositorystate.SourceIdentityKey(left.Source) + "\x00" + left.Artifact + "\x00" + strings.Join(left.Paths, "\x00")
 rightKey := string(right.Kind) + "\x00" + repositorystate.SourceIdentityKey(right.Source) + "\x00" + right.Artifact + "\x00" + strings.Join(right.Paths, "\x00")
 return strings.Compare(leftKey, rightKey)
})
```

Any conflict returns before consumer or state writes.

- [ ] **Step 6: Implement deterministic application and persistence**

Sort planned files by Source Identity, Artifact, and path. Call `materialize.Write` only for `file_created` and `file_updated`; adoption writes no consumer bytes. Build managed records from desired digests. Before any state write, call `revalidateAdoptions` so adopted files cannot change between preflight and ownership recording without producing drift. Emit effective changes only. Persist Lockfile when newly locked or pruned and Materialization Record when files changed or ownership was adopted. A no-op performs no state write. Use `resolution_locked` once per newly pinned declaration and `lock_pruned` once per pruned Artifact. Return Artifact count from final desired set.

On `materialize.ChangedSincePreflightError`, return a drift-class `UserActionError`. Track newly created absolute paths and remove only those if later apply/state persistence fails. Do not revert updated consumer files or state files already replaced before a later operational failure; phase 1 explicitly makes no full-rollback claim.

- [ ] **Step 7: Run install package tests and race detector**

Run: `gofmt -w internal/install/*.go`

Run: `go test -race ./... -count=1`

Expected: PASS. This completes the Tasks 1-5 compile migration; no package retains a removed repository-state, Source, or materialization API.

- [ ] **Step 8: Pause for commit approval**

State intent to run:

```bash
git add internal/repositorystate internal/source internal/materialize internal/install
git commit -m "feat: reconcile multiple locked declarations"
```

Wait for explicit approval. Run commands only after approval.

### Task 6: Render shared CLI outcomes and prove the real path

**Files:**

- Modify: `cmd/tbboot/install.go`
- Modify: `cmd/tbboot/root.go`
- Modify: `cmd/tbboot/root_test.go`
- Modify: `cmd/tbboot/examples_e2e_test.go`

**Interfaces:**

- Consumes: `install.Result`, `install.UserActionError`, and `install.TrustPolicyError`.
- Produces: stable human summaries, shared JSON detail shapes, and exit codes `0`, `1`, `2`, and `3` on correct streams.

- [ ] **Step 1: Write failing CLI contract tests**

Add tests named:

```go
func TestSyncHumanNoOpAndAppliedOutput(t *testing.T)
func TestSyncJSONOmitsChangesForNoOp(t *testing.T)
func TestSyncJSONIncludesTypedEffectiveChanges(t *testing.T)
func TestSyncJSONIncludesAllTypedConflictsOnStderr(t *testing.T)
func TestSyncExitCodesForValidationConflictAndTrust(t *testing.T)
func TestExplicitSourceScopeInstallsAllArtifacts(t *testing.T)
func TestExplicitArtifactScopeInstallsOnlyNamedArtifact(t *testing.T)
func TestRepeatedExplicitInstallDoesNotUpgradeSnapshot(t *testing.T)
func TestMultipleDeclarationRealPathSync(t *testing.T)
```

JSON no-op assertion:

```go
var envelope app.Result
if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
 t.Fatal(err)
}
if envelope.Code != app.ExitSuccess {
 t.Fatalf("code = %d, want 0", envelope.Code)
}
if envelope.Details["outcome"] != "no_op" {
 t.Fatalf("outcome = %v, want no_op", envelope.Details["outcome"])
}
if _, exists := envelope.Details["changes"]; exists {
 t.Fatal("no_op JSON contains changes")
}
```

- [ ] **Step 2: Run CLI tests and confirm old envelope failure**

Run: `go test ./cmd/tbboot -count=1`

Expected: FAIL because current renderer expects one Source/Artifact and old file actions.

- [ ] **Step 3: Replace install rendering with one result mapper**

In `cmd/tbboot/install.go`, remove old `sourceIdentityJSON`, `artifactDescriptorJSON`, `fileChangeJSON`, and mapper functions. Add:

```go
func resultEnvelope(message string, result installsvc.Result) app.Result {
 details := map[string]any{
  "operation":      result.Operation,
  "outcome":        result.Outcome,
  "artifact_count": result.ArtifactCount,
 }
 if len(result.Changes) != 0 {
  details["changes"] = result.Changes
 }
 if len(result.Conflicts) != 0 {
  details["conflicts"] = result.Conflicts
 }
 return app.Result{Code: app.ExitSuccess, Message: message, Details: details}
}
```

Human success output must use `string(result.Operation)` as its prefix and one summary line:

```text
install: no changes (2 artifacts)
install: applied 3 changes (2 artifacts)
sync: no changes (2 artifacts)
sync: applied 3 changes (2 artifacts)
```

Then print one line per effective change and no unchanged file lines. Human conflict output is written by root error handling and includes sorted conflict kind, Source locator, Artifact, and paths.

- [ ] **Step 4: Centralize typed error classification in root command**

Replace individual old conflict checks in `cmd/tbboot/root.go` with:

```go
var userAction installsvc.UserActionError
if errors.As(err, &userAction) {
 code = app.ExitUserActionConflict
}
var trust installsvc.TrustPolicyError
if errors.As(err, &trust) {
 code = app.ExitTrustOrPolicyDenial
}
```

For JSON errors, when error is `UserActionError`, encode its Result fields in `details` using the same result mapper and set envelope code to `2`; for Trust denial, include sorted denied Sources and code `3`; otherwise keep code `1`, message, warnings shape, and stderr placement.

- [ ] **Step 5: Add one real multi-declaration end-to-end case**

In `cmd/tbboot/examples_e2e_test.go`, create a temporary repo containing two `file:` Sources and a Manifest with one source-level declaration plus one artifact-level declaration. First bare install must create all selected files, Lockfile snapshot blocks, and Materialization Records. Second bare install must return `no_op`, leave all bytes unchanged, and report exact Artifact count. Modify an owned file and remove one declaration; third run must return exit `2`, report both drift and removal-required, and leave consumer and all three state files unchanged.

- [ ] **Step 6: Run full validation**

Run: `gofmt -w cmd/tbboot/*.go`

Run: `just check`

Expected: PASS.

- [ ] **Step 7: Pause for commit approval**

State intent to run:

```bash
git add cmd/tbboot docs/superpowers/plans/2026-07-14-phase-1-install-sync.md
git commit -m "feat: expose phase 1 install sync outcomes"
```

Wait for explicit approval. Run commands only after approval.

## Final verification checklist

- [ ] `rg -n 'SourceIdentity\{[^}]*Name:|\.Source\.Name|Resolution\{[^}]*Artifact:' --glob '*.go'` prints no old schema consumers.
- [ ] `git diff --check` prints no output.
- [ ] `git status --short` contains only intended changes.
- [ ] No commit is created without fresh explicit user approval.
