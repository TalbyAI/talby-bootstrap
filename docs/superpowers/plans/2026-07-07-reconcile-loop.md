# Reconcile loop implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the first complete reconcile loop so `tbboot install <source>` enforces local trust, persists manifest plus lockfile plus managed state, materializes simple `file` steps, detects whole-file drift, and makes bare `tbboot install` replay persisted state safely.

**Architecture:** Keep `cmd/tbboot` thin and keep the product loop in `internal/install`. Add one narrow trust check before any write, one minimal whole-file materialization engine, and one new repository-state file for the **Materialization Record**. Reuse `internal/repositorystate` for persisted state, extend `internal/source/file` to expose `file` steps, and make bare `install` call a real `Sync` path instead of the current placeholder branch.

**Tech Stack:** Go 1.26.4, Cobra, `gopkg.in/yaml.v3`, `os`/`filepath`/`crypto/sha256`, `gofmt`, `go test`, `just`.

---

## Constraints

- Do not add new CLI commands.
- Do not implement `git` source support, fragment insertion, `script`, `prompt`, or `upgrade`.
- Do not add interactive approval or drift resolution.
- Do not create commits from an agent. `AGENTS.md` requires explicit user confirmation at commit time.

## File structure

- Modify: `internal/source/model.go` - extend resolved artifact data with materialization steps and whole-file source paths.
- Modify: `internal/source/file/source.go` - parse `steps` from `talby-artifact.yaml`, validate minimal `file` step fields, and include source-relative file paths in the resolved artifact.
- Modify: `internal/source/file/source_test.go` - cover parsed `file` steps, missing step fields, and snapshot changes when file payload changes.
- Modify: `internal/install/service.go` - add trust admission before writes, non-declare install writes, and a real `Sync` entrypoint for bare install.
- Modify: `internal/install/repository_state.go` - keep manifest and lockfile mappers and add record mappers shared by install and sync.
- Modify: `internal/install/service_test.go` - cover trust denial, lockfile persistence, materialization, managed-state persistence, drift, and sync.
- Modify: `internal/repositorystate/model.go` - add the **Materialization Record** domain types and a new `StateFile` constant.
- Modify: `internal/repositorystate/store.go` - add load/write support for the new state file.
- Add: `internal/repositorystate/materialization_record.go` - validation and upsert/remove helpers for managed state.
- Add: `internal/repositorystate/materialization_record_test.go` - round-trip and validation coverage for the new state file.
- Add: `internal/materialize/service.go` - whole-file apply logic, ownership checks, digesting, and drift detection.
- Add: `internal/materialize/service_test.go` - focused tests for first write, no-op, ownership conflict, persistence-failure rollback, and drift.
- Modify: `cmd/tbboot/install.go` - replace the sync placeholder with real service calls and render success or no-op or drift results.
- Modify: `cmd/tbboot/root.go` - map trust denials to `app.ExitTrustOrPolicyDenial` and reuse conflict mapping for drift and ownership conflicts.
- Modify: `cmd/tbboot/root_test.go` - CLI coverage for trust denial, install materialization, drift conflict, and bare install sync.
- Modify: `testdata/examples/...` fixtures as needed - add expected managed-state file and sync/drift scenarios instead of inventing ad hoc test-only formats.

No new top-level domain package is needed for trust or sync. The smallest seam is `internal/install` coordinating `internal/materialize` and `internal/repositorystate`.

## State model decisions locked by this plan

- Use `talby-artifacts.managed.yaml` as the **Materialization Record** file.
- Keep whole-file ownership only in this tranche.
- Model one managed artifact key as `source type + source name + resolved source version + artifact name`.
- Store SHA-256 digests of managed file bytes rather than full prior file content.
- Treat missing managed state during PR 4 bootstrap as empty state, but once the file exists, use it as the ownership, overwrite, and drift baseline.
- Treat managed-artifact removal during sync as a user-action conflict in PR 5 and PR 6; do not delete automatically yet.

## Tasks

### Task 1: PR 3 minimal trust-policy enforcement for local `file:` installs

**Files:**

- Modify: `internal/install/service.go`
- Modify: `internal/install/service_test.go`
- Modify: `cmd/tbboot/root.go`
- Modify: `cmd/tbboot/root_test.go`

- [ ] **Step 1: Write the failing trust tests in the service**

```go
func TestInstallAllowsFileSourceInsideOperationRoot(t *testing.T) {
    root := t.TempDir()
    sourceRoot := filepath.Join(root, "examples")
    writeInstallFixture(t, sourceRoot)

    svc := NewService(
        source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
        repositorystate.NewStore(),
    )

    _, err := svc.Install(context.Background(), Request{
        Root:     root,
        Source:   source.Ref{Type: "file", Locator: sourceRoot},
        Artifact: "base-readme",
    })
    if err != nil {
        t.Fatalf("Install() error = %v", err)
    }
}

func TestInstallDeniesFileSourceOutsideOperationRootBeforeWrites(t *testing.T) {
    root := t.TempDir()
    sourceRoot := t.TempDir()
    writeInstallFixture(t, sourceRoot)

    svc := NewService(
        source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
        repositorystate.NewStore(),
    )

    _, err := svc.Install(context.Background(), Request{
        Root:     root,
        Source:   source.Ref{Type: "file", Locator: sourceRoot},
        Artifact: "base-readme",
    })
    if err == nil {
        t.Fatal("Install() error = nil, want trust denial")
    }

    var denyErr TrustPolicyError
    if !errors.As(err, &denyErr) {
        t.Fatalf("error = %T, want TrustPolicyError", err)
    }
    if _, statErr := os.Stat(filepath.Join(root, repositorystate.ManifestFileName)); !errors.Is(statErr, os.ErrNotExist) {
        t.Fatalf("manifest stat error = %v, want not exist", statErr)
    }
    if _, statErr := os.Stat(filepath.Join(root, repositorystate.LockfileFileName)); !errors.Is(statErr, os.ErrNotExist) {
        t.Fatalf("lockfile stat error = %v, want not exist", statErr)
    }
}
```

- [ ] **Step 2: Run the targeted service tests and confirm they fail**

Run: `go test ./internal/install -run 'TestInstall(AllowsFileSourceInsideOperationRoot|DeniesFileSourceOutsideOperationRootBeforeWrites)' -count=1`

Expected: FAIL because `Install` has no trust check and `TrustPolicyError` does not exist yet.

- [ ] **Step 3: Add the minimal trust gate in `internal/install/service.go`**

```go
type TrustPolicyError struct {
    SourceType    string
    Locator       string
    OperationRoot string
}

func (e TrustPolicyError) Error() string {
    return fmt.Sprintf(
        `source %q is outside the operation root %q and is not allowed by default`,
        e.Locator,
        e.OperationRoot,
    )
}

func enforceDirectSourceTrust(root string, ref source.Ref) error {
    if ref.Type != repositorystate.SourceTypeFile {
        return nil
    }
    if root == "" {
        return fmt.Errorf("repository root is required")
    }

    absRoot, err := filepath.Abs(root)
    if err != nil {
        return err
    }
    absLocator, err := filepath.Abs(ref.Locator)
    if err != nil {
        return err
    }
    rel, err := filepath.Rel(absRoot, absLocator)
    if err != nil {
        return err
    }
    if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
        return TrustPolicyError{
            SourceType:    ref.Type,
            Locator:       absLocator,
            OperationRoot: absRoot,
        }
    }
    return nil
}
```

Add the call early in `Install`, after request validation and before any manifest, lockfile, or filesystem writes:

```go
if err := enforceDirectSourceTrust(req.Root, req.Source); err != nil {
    return Result{}, err
}
```

- [ ] **Step 4: Map trust denial to the existing policy exit code**

```go
var trustErr installsvc.TrustPolicyError
if errors.As(err, &trustErr) {
    code = app.ExitTrustOrPolicyDenial
}
```

Add a CLI test in `cmd/tbboot/root_test.go` that runs install with a `file:` source outside the repo root and expects exit code `3` plus the denial text in human and JSON output.

- [ ] **Step 5: Run the focused tests and then the repo check**

Run: `go test ./internal/install ./cmd/tbboot -count=1`

Expected: PASS with inside-root allowed and outside-root denied paths covered.

Run: `just check-go`

Expected: PASS.

### Task 2: PR 4 minimal materialization engine for `file` steps only

**Files:**

- Modify: `internal/source/model.go`
- Modify: `internal/source/file/source.go`
- Modify: `internal/source/file/source_test.go`
- Add: `internal/materialize/service.go`
- Add: `internal/materialize/service_test.go`
- Modify: `internal/repositorystate/model.go`
- Modify: `internal/repositorystate/store.go`
- Add: `internal/repositorystate/materialization_record.go`
- Add: `internal/repositorystate/materialization_record_test.go`
- Modify: `internal/install/repository_state.go`
- Modify: `internal/install/service.go`
- Modify: `internal/install/service_test.go`
- Modify: `cmd/tbboot/install.go`
- Modify: `cmd/tbboot/root.go`
- Modify: `cmd/tbboot/root_test.go`

- [ ] **Step 1: Write failing parser tests for `file` steps**

```go
func TestResolveLoadsFileSteps(t *testing.T) {
    root := t.TempDir()
    writeFile(t, filepath.Join(root, "talby-source.yaml"), ""+
        "schema_version: 1\n"+
        "source:\n  name: local-example-source\n"+
        "artifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
    writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), ""+
        "schema_version: 1\n"+
        "artifact:\n  name: base-readme\n  version: 1.0.0\n"+
        "steps:\n  - type: file\n    path: README.md\n    source: README.md\n")
    writeFile(t, filepath.Join(root, "artifacts", "base-readme", "README.md"), "hello\n")

    resolved, err := New().Resolve(context.Background(), source.ResolveRequest{
        Ref: source.Ref{Type: "file", Locator: root},
    })
    if err != nil {
        t.Fatalf("Resolve() error = %v", err)
    }
    if len(resolved.Artifacts[0].Steps) != 1 {
        t.Fatalf("len(Steps) = %d, want 1", len(resolved.Artifacts[0].Steps))
    }
    if got := resolved.Artifacts[0].Steps[0]; got.TargetPath != "README.md" || !strings.HasSuffix(got.SourcePath, "/artifacts/base-readme/README.md") {
        t.Fatalf("step = %#v, want resolved whole-file source path", got)
    }
}
```

- [ ] **Step 2: Extend the source model with the smallest useful step shape**

```go
type MaterializationStep struct {
    Type       string
    TargetPath string
    SourcePath string
}

type ArtifactDescriptor struct {
    Name    string
    Version string
    Path    string
    Steps   []MaterializationStep
}
```

In `internal/source/file/source.go`, parse only `file` steps into that shape, validate `path` and `source`, and fold the file bytes into the existing snapshot hash so lockfile replay notices changed payloads:

```go
stepBytes, err := os.ReadFile(filepath.Join(artifactDir, step.Source))
if err != nil {
    return source.ResolvedSource{}, fmt.Errorf("read %s: %w", filepath.Join(artifactDir, step.Source), err)
}
snapshot.Write([]byte(step.Path))
snapshot.Write([]byte{0})
snapshot.Write(stepBytes)
```

- [ ] **Step 3: Add the minimal managed-state model and store round-trip**

In `internal/repositorystate/model.go`:

```go
type MaterializationRecord struct {
    Artifacts []ManagedArtifactRecord
}

type ManagedArtifactRecord struct {
    Key   ManagedArtifactKey
    Files []ManagedFileRecord
}

type ManagedArtifactKey struct {
    Source          SourceIdentity
    ResolvedVersion string
    Artifact        string
}

type ManagedFileRecord struct {
    Path   string
    Digest string
}
```

Also add:

```go
const StateFileMaterializationRecord StateFile = "materialization_record"
const MaterializationRecordFileName = "talby-artifacts.managed.yaml"
```

Then extend `Store` with:

```go
LoadMaterializationRecord(context.Context, string) (MaterializationRecord, error)
WriteMaterializationRecord(context.Context, string, MaterializationRecord) error
```

Add focused round-trip coverage in `internal/repositorystate/materialization_record_test.go` and `internal/repositorystate/store.go` so PR 4 already persists the third state layer required by the design.

- [ ] **Step 4: Add the minimal materialization service and its tests**

Create `internal/materialize/service.go` with one boring API:

```go
type Request struct {
    Root     string
    Key      repositorystate.ManagedArtifactKey
    Record   repositorystate.MaterializationRecord
    Artifact source.ArtifactDescriptor
}

type FileChange struct {
    Path   string
    Action string
    Digest string
}

type Result struct {
    Changes []FileChange
}

func Apply(ctx context.Context, req Request) (Result, error)
```

Core behavior:

```go
owned := indexOwnedFiles(req.Record)
for _, step := range req.Artifact.Steps {
    if step.Type != "file" {
        continue
    }
    if priorOwner, claimed := owned[step.TargetPath]; claimed && priorOwner != req.Key {
        return Result{}, OwnershipConflictError{Path: step.TargetPath}
    }
    target := filepath.Join(req.Root, step.TargetPath)
    sourceBytes, err := os.ReadFile(step.SourcePath)
    if err != nil {
        return Result{}, err
    }
    action := "created"
    if currentBytes, err := os.ReadFile(target); err == nil && bytes.Equal(currentBytes, sourceBytes) {
        action = "unchanged"
    }
    if action != "unchanged" {
        if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
            return Result{}, err
        }
        if err := os.WriteFile(target, sourceBytes, 0o644); err != nil {
            return Result{}, err
        }
    }
    result.Changes = append(result.Changes, FileChange{
        Path:   step.TargetPath,
        Action: action,
        Digest: sha256Hex(sourceBytes),
    })
}
```

Add typed ownership conflict:

```go
type OwnershipConflictError struct {
    Path string
}
```

Test first install, no-op, and ownership conflict:

```go
func TestApplyWritesManagedFiles(t *testing.T) {}
func TestApplyLeavesIdenticalFileUnchanged(t *testing.T) {}
func TestApplyStopsWhenAnotherManagedArtifactOwnsTargetFile(t *testing.T) {}
```

- [ ] **Step 5: Make non-declare install persist lockfile, managed-state, and materialize files**

Add mapper in `internal/install/repository_state.go`:

```go
func ManagedArtifactKeyFor(result Result) repositorystate.ManagedArtifactKey {
    return repositorystate.ManagedArtifactKey{
        Source: repositorystate.SourceIdentity{
            Type: result.Source.Type,
            Name: result.Source.Name,
        },
        ResolvedVersion: result.Source.Version,
        Artifact:        result.Artifact.Name,
    }
}
```

In `internal/install/service.go`, replace the current early return:

```go
if req.DeclareOnly {
    return s.declareOnly(ctx, req, result)
}

lockfile, err := s.loadLockfileOrEmpty(ctx, req.Root)
if err != nil {
    return Result{}, err
}
nextLockfile, _ := lockfile.UpsertResolution(LockfileResolution(result))
if err := s.store.WriteLockfile(ctx, req.Root, nextLockfile); err != nil {
    return Result{}, err
}

record, err := s.loadMaterializationRecordOrEmpty(ctx, req.Root)
if err != nil {
    return Result{}, err
}

matResult, err := materialize.Apply(ctx, materialize.Request{
    Root:     req.Root,
    Key:      ManagedArtifactKeyFor(result),
    Record:   record,
    Artifact: result.Artifact,
})
if err != nil {
    return Result{}, err
}

nextRecord := repositorystate.UpsertManagedArtifact(record, ManagedArtifactRecordFor(result, matResult))
if err := s.store.WriteMaterializationRecord(ctx, req.Root, nextRecord); err != nil {
    return Result{}, err
}

result.Change = changeFromMaterialization(matResult)
result.Files = matResult.Changes
return result, nil
```

Add result rendering in `cmd/tbboot/install.go`:

```go
_, err = fmt.Fprintf(stdout, "installed artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
```

JSON details should include `change` and `files`.

- [ ] **Step 6: Prevent orphan managed files when later persistence fails**

PR 4 still needs one guard for design rule "no managed files without matching evidence". Keep it small:

- make `materialize.Apply` return enough metadata to remove files it created in this operation;
- if `WriteMaterializationRecord` fails after file writes, remove newly created managed files before returning error;
- do not try to build full transaction machinery or rollback pre-existing files; strong rollback is out of scope.

Add one focused test:

```go
func TestInstallRemovesNewManagedFilesWhenManagedStateWriteFails(t *testing.T) {}
```

Use a fake `Store` that returns an error from `WriteMaterializationRecord` after lockfile write succeeds. Assert `README.md` does not remain on disk and `talby-artifacts.managed.yaml` was not created.

- [ ] **Step 7: Map PR 4 ownership conflicts to user-action conflict exit code**

In `cmd/tbboot/root.go`:

```go
var ownershipErr materialize.OwnershipConflictError
if errors.As(err, &ownershipErr) {
    code = app.ExitUserActionConflict
}
```

Add CLI coverage for human and JSON output.

- [ ] **Step 8: Add an integration-style install test with real file output**

```go
func TestInstallWritesLockfileManagedStateAndManagedFiles(t *testing.T) {
    root := t.TempDir()
    sourceRoot := filepath.Join(root, "source")
    writeInstallFixture(t, sourceRoot)

    svc := NewService(
        source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
        repositorystate.NewStore(),
    )

    _, err := svc.Install(context.Background(), Request{
        Root:     root,
        Source:   source.Ref{Type: "file", Locator: sourceRoot},
        Artifact: "base-readme",
    })
    if err != nil {
        t.Fatalf("Install() error = %v", err)
    }

    if _, err := os.Stat(filepath.Join(root, repositorystate.LockfileFileName)); err != nil {
        t.Fatalf("lockfile stat error = %v", err)
    }
    if _, err := os.Stat(filepath.Join(root, repositorystate.MaterializationRecordFileName)); err != nil {
        t.Fatalf("managed-state stat error = %v", err)
    }
    gotReadme, err := os.ReadFile(filepath.Join(root, "README.md"))
    if err != nil {
        t.Fatalf("ReadFile(README.md) error = %v", err)
    }
    if string(gotReadme) == "" {
        t.Fatal("README.md = empty, want materialized file content")
    }
}
```

- [ ] **Step 9: Run the targeted tests and then the repo check**

Run: `go test ./internal/source/file ./internal/repositorystate ./internal/materialize ./internal/install ./cmd/tbboot -count=1`

Expected: PASS with parser, managed-state round-trip, ownership conflict, rollback-on-persistence-failure, and non-declare install coverage.

Run: `just check-go`

Expected: PASS.

### Task 3: PR 5 materialization record hardening and whole-file drift detection

**Files:**

- Add: `internal/materialize/service_test.go`
- Modify: `internal/materialize/service.go`
- Modify: `internal/install/repository_state.go`
- Modify: `internal/install/service.go`
- Modify: `internal/install/service_test.go`
- Modify: `cmd/tbboot/root.go`
- Modify: `cmd/tbboot/root_test.go`

- [ ] **Step 1: Add the failing repository-state tests for managed-state helpers**

```go
func TestUpsertManagedArtifactInsertReplaceAndUnchanged(t *testing.T) {}
func TestValidateMaterializationRecordRejectsDuplicateOwnersAndInvalidDigests(t *testing.T) {}
```

- [ ] **Step 2: Add helper validation and upsert logic for the already-introduced state model**

In `internal/repositorystate/materialization_record.go`, add:

```go
func ValidateMaterializationRecord(record MaterializationRecord) error
func UpsertManagedArtifact(record MaterializationRecord, next ManagedArtifactRecord) MaterializationRecord
func RemoveManagedArtifact(record MaterializationRecord, key ManagedArtifactKey) MaterializationRecord
```

Rules:

- artifact keys remain unique by `source type + source name + resolved source version + artifact name`;
- file paths remain exclusive across artifact owners;
- digests must be non-empty hex SHA-256 strings;
- helpers keep sort order stable so no-op reapply preserves on-disk bytes.

- [ ] **Step 3: Make materialization detect drift before overwrite using prior managed-state digest**

Add typed conflicts in `internal/materialize/service.go`:

```go
type DriftError struct {
    Path string
}
```

Use the prior record as the baseline:

```go
func Apply(ctx context.Context, req Request) (Result, error) {
    for _, step := range req.Artifact.Steps {
        currentBytes, currentErr := os.ReadFile(filepath.Join(req.Root, step.TargetPath))
        if currentErr == nil {
            priorDigest, tracked := digestFor(req.Record, req.Key, step.TargetPath)
            if tracked && sha256Hex(currentBytes) != priorDigest {
                return Result{}, DriftError{Path: step.TargetPath}
            }
        }
        // write file only after the checks pass
    }
}
```

- [ ] **Step 4: Persist the managed-state file on successful install**

PR 4 already writes the managed-state file. In PR 5, harden the mapper and no-op behavior rather than introducing the file for the first time.

Add a mapper in `internal/install/repository_state.go`:

```go
func ManagedArtifactKeyFor(result Result) repositorystate.ManagedArtifactKey {
    return repositorystate.ManagedArtifactKey{
        Source: repositorystate.SourceIdentity{
            Type: result.Source.Type,
            Name: result.Source.Name,
        },
        ResolvedVersion: result.Source.Version,
        Artifact:        result.Artifact.Name,
    }
}
```

Then in `internal/install/service.go`:

```go
record, err := s.loadMaterializationRecordOrEmpty(ctx, req.Root)
if err != nil {
    return Result{}, err
}

matResult, err := materialize.Apply(ctx, materialize.Request{
    Root:     req.Root,
    Key:      ManagedArtifactKeyFor(result),
    Record:   record,
    Artifact: result.Artifact,
})
if err != nil {
    return Result{}, err
}

nextRecord := repositorystate.UpsertManagedArtifact(record, ManagedArtifactRecordFor(result, matResult))
if err := s.store.WriteMaterializationRecord(ctx, req.Root, nextRecord); err != nil {
    return Result{}, err
}
```

Also add a no-op assertion in tests: reapplying unchanged input leaves `talby-artifacts.managed.yaml` byte-for-byte identical.

- [ ] **Step 5: Add the drift and no-op reapply tests**

```go
func TestInstallStopsWhenManagedFileHasDrifted(t *testing.T) {}
func TestInstallReusesManagedStateForUnchangedReapply(t *testing.T) {}
```

Concrete assertions:

- first install writes `README.md` plus `talby-artifacts.managed.yaml`
- manual edit to `README.md` makes the second install return `DriftError`
- no-op reapply does not change the managed-state file bytes

- [ ] **Step 6: Map drift to user-action conflict exit code**

In `cmd/tbboot/root.go`:

```go
var driftErr materialize.DriftError
if errors.As(err, &driftErr) {
    code = app.ExitUserActionConflict
}
```

Add CLI tests for both human and JSON output.

- [ ] **Step 7: Run the targeted tests and then the repo check**

Run: `go test ./internal/repositorystate ./internal/materialize ./internal/install ./cmd/tbboot -count=1`

Expected: PASS with managed-state round-trip and whole-file drift coverage.

Run: `just check-go`

Expected: PASS.

### Task 4: PR 6 real sync behavior for bare `install`

**Files:**

- Modify: `internal/install/service.go`
- Modify: `internal/install/service_test.go`
- Modify: `cmd/tbboot/install.go`
- Modify: `cmd/tbboot/root_test.go`
- Modify: `testdata/examples/...` fixtures for sync and drift scenarios

- [ ] **Step 1: Write the failing sync tests in the install service**

```go
func TestSyncReappliesManagedFilesFromPersistedState(t *testing.T) {
    root := t.TempDir()
    sourceRoot := filepath.Join(root, "source")
    writeInstallFixture(t, sourceRoot)

    svc := newRealInstallService()
    _, err := svc.Install(context.Background(), Request{
        Root:     root,
        Source:   source.Ref{Type: "file", Locator: sourceRoot},
        Artifact: "base-readme",
    })
    if err != nil {
        t.Fatalf("Install() error = %v", err)
    }

    writeFile(t, filepath.Join(root, "README.md"), "user edit\n")

    _, err = svc.Sync(context.Background(), SyncRequest{Root: root})
    if err == nil {
        t.Fatal("Sync() error = nil, want drift conflict")
    }
}

func TestSyncReturnsNoOpWhenRepositoryAlreadyMatchesPersistedState(t *testing.T) {}
```

- [ ] **Step 2: Add a real `Sync` entrypoint to `internal/install/service.go`**

```go
type SyncRequest struct {
    Root string
}

func (s Service) Sync(ctx context.Context, req SyncRequest) (Result, error) {
    if req.Root == "" {
        return Result{}, fmt.Errorf("repository root is required for sync")
    }

    manifest, err := s.store.LoadManifest(ctx, req.Root)
    if err != nil {
        return Result{}, err
    }
    lockfile, err := s.store.LoadLockfile(ctx, req.Root)
    if err != nil {
        return Result{}, err
    }
    record, err := s.loadMaterializationRecordOrEmpty(ctx, req.Root)
    if err != nil {
        return Result{}, err
    }

    decl := requireSingleDeclaration(manifest)
    res := requireMatchingResolution(lockfile, decl)
    ref := source.Ref{Type: decl.Source.Type, Locator: decl.Input.Locator, Version: res.ResolvedVersion}
    if err := enforcePersistedTrust(req.Root, manifest.TrustPolicy, ref, decl.Source); err != nil {
        return Result{}, err
    }

    resolved, err := s.resolveForSync(ctx, ref, decl.Target.Artifact)
    if err != nil {
        return Result{}, err
    }
    if resolved.Source.Version != res.ResolvedVersion {
        return Result{}, fmt.Errorf(
            "locked source version %q no longer matches current source snapshot %q",
            res.ResolvedVersion,
            resolved.Source.Version,
        )
    }

    return s.installResolved(ctx, req.Root, resolved, record)
}
```

Keep the first sync slice intentionally narrow: one declaration, one resolution, one artifact. Return direct validation errors if persisted state is missing or ambiguous.

This check is required for `file:` replay semantics: sync may re-read source content, but it must stop when current source no longer matches locked snapshot baseline.

- [ ] **Step 3: Replace the CLI sync placeholder**

In `cmd/tbboot/install.go`, replace:

```go
result := app.Success("sync not implemented")
```

with:

```go
root, err := repositoryRoot(ctx)
if err != nil {
    return err
}
result, err := service.Sync(ctx, installsvc.SyncRequest{Root: root})
if err != nil {
    return err
}
```

Human output:

```go
if result.Change == installsvc.ChangeNoOp {
    _, err = fmt.Fprintln(stdout, "sync: no changes")
    return err
}
_, err = fmt.Fprintf(stdout, "synced artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
return err
```

- [ ] **Step 4: Add a real CLI sync scenario**

Create or extend fixture coverage so one example proves:

- `tbboot install file:<source>` writes manifest, lockfile, managed-state file, and `README.md`
- bare `tbboot install` after no user edits returns success with `sync: no changes`
- bare `tbboot install` after editing `README.md` returns exit code `2` and drift text
- bare `tbboot install` after changing source payload so snapshot hash changes returns a safe stop before rewrite

Use existing scenario-style tests in `cmd/tbboot/root_test.go` rather than a new harness.

- [ ] **Step 5: Run the targeted tests and then the repo check**

Run: `go test ./internal/install ./cmd/tbboot -count=1`

Expected: PASS with real bare-install sync behavior.

Run: `just check`

Expected: PASS.

## Validation checklist

- Trust denial happens before `talby-artifacts.yaml`, `talby-artifacts.lock.yaml`, `talby-artifacts.managed.yaml`, or managed files are written.
- Successful non-declare install writes all three state layers plus whole-file output.
- Successful non-declare install does not leave managed files behind if later managed-state persistence fails.
- Repeating the same install with unchanged inputs is a no-op for managed files.
- Editing a managed whole file causes a safe stop instead of silent overwrite.
- Bare `tbboot install` replays persisted state instead of returning the old placeholder message.
- Bare `tbboot install` stops if current `file:` source snapshot no longer matches locked resolved version.

## Self-review

- Spec coverage: PR 3 covers trust admission; PR 4 covers lockfile plus managed-state persistence plus first file materialization; PR 5 covers managed-state hardening plus drift; PR 6 covers real sync from manifest plus lockfile plus managed state plus locked-snapshot replay checks.
- Placeholder scan: no `TODO`, `TBD`, or "appropriate handling" filler remains.
- Type consistency: `TrustPolicyError`, `MaterializationRecord`, `ManagedArtifactKey`, `DriftError`, `OwnershipConflictError`, and `SyncRequest` are named consistently across tasks.

Plan complete and saved to `docs/superpowers/plans/2026-07-07-reconcile-loop.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
