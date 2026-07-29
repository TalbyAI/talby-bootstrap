# Rollback and Recovery State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Protect non-Dry Run `Install` and `Sync` mutations with verified in-memory rollback, persist sanitized Recovery State when rollback is incomplete, and block or clear that state on later operations.

**Architecture:** Add only the filesystem primitives needed to capture and restore a prior file state, then keep the journal and recovery gate private to `internal/install`. Route materialization and repository-state writes through that journal; keep `repositorystate.Store` responsible only for strict deterministic persistence and Recovery State removal. Report later recovery conflicts through one typed service error and the existing shared CLI envelope.

**Tech Stack:** Go 1.26.4, `os.Root`, Go standard library, existing `internal/materialize`, existing `internal/repositorystate`, Cobra, YAML v3, and repository `just` tasks.

## Global Constraints

- Protect `Install` and `Sync` materialization and repository-state writes from controlled in-process failures.
- Crash recovery and durable backups remain outside product 0.1.
- Acquire the existing root operation lock before inspecting or changing Recovery State on non-Dry Run operations.
- Record each journal entry immediately before its mutation, including the target's prior bytes, permission bits, absence, and existing parent topology.
- Roll back in reverse order, attempt every restoration, and verify every restored target against its captured prior state.
- Re-observe after every restoration attempt; verified final state determines success even when the restoration action reported an error.
- Persist no prior contents, raw errors, or absolute paths in Recovery State.
- Write `tbboot-artifacts.recovery.yaml` deterministically with schema version 1 and mode `0600`.
- Verify written Recovery State by safe rooted observation, exact mode `0600`, strict reload, value comparison, and post-load revalidation.
- Treat changed parent topology as a change detected during the current inspection and revalidation window; Recovery State does not persist cross-session directory identities.
- Use stable recovery code `rollback_incomplete` and the existing numeric user-action-conflict exit class.
- Dry Run inspects Recovery State but never clears it.
- Introduce no general transaction interface, durable journal, or new dependency.
- Test behavior through public `Service.Install` and `Service.Sync`; retain repository-state schema tests as the strict serialization seam.
- Use repository tasks such as `just check-go`, `just fmt-go`, and `just check`; do not substitute direct tool commands in the plan.
- Do not create any commit until the user confirms that specific commit at that moment.

---

## File map

- Modify `internal/materialize/service.go`: expose safe prior-byte capture, explicit-mode atomic restoration, prior-state comparison, and missing-parent paths using the existing rooted observation model.
- Modify `internal/materialize/service_test.go`: lock down the new restoration primitives and topology behavior.
- Modify `internal/repositorystate/store.go`: add the one Recovery State removal operation.
- Modify `internal/repositorystate/store_test.go`: prove removal and missing-file behavior.
- Create `internal/install/transaction.go`: private journal, mutation hook, reverse rollback, verification, Recovery State construction, and recovery-write verification.
- Create `internal/install/transaction_test.go`: exercise rollback through public install/sync methods, including state-write failures and incomplete rollback.
- Create `internal/install/recovery.go`: inspect, compare, report, and clear existing Recovery State before normal repository reads or Source resolution.
- Create `internal/install/recovery_test.go`: exercise blocking, repair, clearing, and Dry Run through public install/sync methods.
- Modify `internal/install/service.go`: add the private test hook, typed recovery conflict, early recovery gate, and transactional declaration-only Manifest write.
- Modify `internal/install/sync.go`: route materialization and state persistence through one journal and remove best-effort created-file cleanup.
- Modify `cmd/tbboot/root.go`: render typed recovery conflicts in human and JSON modes.
- Modify `cmd/tbboot/root_test.go`: prove stable recovery reporting and sanitization.
- Modify `CONTEXT.md`, `ARCHITECTURE.md`, and `docs/adr/0004-materialization-ownership-drift-and-recovery.md`: replace obsolete deferrals with the implemented in-process lifecycle while retaining crash-recovery exclusions.

### Task 1: Add minimal rooted restore primitives

**Files:**

- Modify: `internal/materialize/service.go:23-230`
- Test: `internal/materialize/service_test.go:11-410`

**Interfaces:**

- Consumes: existing `Observation`, `Observe`, `Remove`, `Digest`, rooted path confinement, and atomic same-directory replacement.
- Produces: `ReadPrior(Observation) ([]byte, error)`, `Restore(Observation, []byte, os.FileMode) error`, `MatchesPrior(Observation, Observation) bool`, and `MissingParents(Observation) []string`.

- [ ] **Step 1: Write failing tests for capture, restoration, and topology**

Add focused tests with these assertions:

```go
func TestReadPriorRestoreAndMatchesPrior(t *testing.T) {
    root := t.TempDir()
    path := filepath.Join(root, "nested", "file")
    if err := os.Mkdir(filepath.Dir(path), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(path, []byte("before"), 0o640); err != nil {
        t.Fatal(err)
    }
    prior, err := Observe(root, "nested/file")
    if err != nil {
        t.Fatal(err)
    }
    bytes, err := ReadPrior(prior)
    if err != nil || string(bytes) != "before" {
        t.Fatalf("ReadPrior() = %q, %v", bytes, err)
    }
    if err := Write(prior, []byte("after")); err != nil {
        t.Fatal(err)
    }
    current, err := Observe(root, "nested/file")
    if err != nil {
        t.Fatal(err)
    }
    if err := Restore(current, bytes, prior.Mode.Perm()); err != nil {
        t.Fatal(err)
    }
    restored, err := Observe(root, "nested/file")
    if err != nil || !MatchesPrior(prior, restored) {
        t.Fatalf("restored = %#v, %v", restored, err)
    }
}

func TestMissingParentsReturnsShallowToDeepCanonicalPaths(t *testing.T) {
    observed, err := Observe(t.TempDir(), "a/b/file")
    if err != nil {
        t.Fatal(err)
    }
    if got, want := MissingParents(observed), []string{"a", "a/b"}; !reflect.DeepEqual(got, want) {
        t.Fatalf("MissingParents() = %#v, want %#v", got, want)
    }
}

func TestReadPriorHandlesAbsentAndRejectsChangedTarget(t *testing.T) {
    root := t.TempDir()
    absent, err := Observe(root, "file")
    if err != nil {
        t.Fatal(err)
    }
    if data, err := ReadPrior(absent); err != nil || data != nil {
        t.Fatalf("ReadPrior(absent) = %q, %v", data, err)
    }
    if err := os.WriteFile(filepath.Join(root, "file"), []byte("before"), 0o644); err != nil {
        t.Fatal(err)
    }
    prior, err := Observe(root, "file")
    if err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(root, "file"), []byte("after"), 0o644); err != nil {
        t.Fatal(err)
    }
    var changed ChangedSincePreflightError
    if _, err := ReadPrior(prior); !errors.As(err, &changed) {
        t.Fatalf("ReadPrior(changed) error = %T %v", err, err)
    }
}
```

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `just check-go`

Expected: compilation fails because `ReadPrior`, `Restore`, `MatchesPrior`, and `MissingParents` do not exist.

- [ ] **Step 3: Implement the four small primitives by reusing rooted internals**

Use these exact signatures and behavior:

```go
func ReadPrior(observed Observation) ([]byte, error) {
    if observed.Kind == EntryAbsent {
        return nil, nil
    }
    if observed.Kind != EntryRegular {
        return nil, fmt.Errorf("target %q must be a regular file or absent", observed.Path)
    }
    root, err := os.OpenRoot(observed.Root)
    if err != nil {
        return nil, err
    }
    defer func() { _ = root.Close() }()
    if err := revalidateRoot(root, observed); err != nil {
        return nil, err
    }
    data, err := root.ReadFile(filepath.FromSlash(observed.Path))
    if err != nil {
        return nil, err
    }
    if Digest(data) != observed.Digest {
        return nil, ChangedSincePreflightError{Path: observed.Path}
    }
    if err := revalidateRoot(root, observed); err != nil {
        return nil, err
    }
    return data, nil
}

func Restore(observed Observation, content []byte, mode os.FileMode) error {
    root, err := os.OpenRoot(observed.Root)
    if err != nil {
        return err
    }
    defer func() { _ = root.Close() }()
    return writeRooted(root, observed, content, mode.Perm())
}

func MatchesPrior(prior, current Observation) bool {
    return prior.Root == current.Root && prior.Path == current.Path &&
        sameFileIdentity(prior.rootInfo, current.rootInfo) &&
        sameParentObservations(prior.parents, current.parents) &&
        prior.Kind == current.Kind && prior.Digest == current.Digest &&
        prior.Mode.Perm() == current.Mode.Perm()
}

func MissingParents(observed Observation) []string {
    existing := make(map[string]struct{}, len(observed.parents))
    for _, parent := range observed.parents {
        existing[PathKey(parent.Path)] = struct{}{}
    }
    var missing []string
    path := ""
    for _, component := range strings.Split(filepath.Dir(filepath.FromSlash(observed.Path)), string(filepath.Separator)) {
        if component == "." {
            continue
        }
        path = filepath.Join(path, component)
        if _, ok := existing[PathKey(path)]; !ok {
            missing = append(missing, filepath.ToSlash(path))
        }
    }
    return missing
}
```

Change private `writeRooted` to accept an explicit mode. Keep `Write` behavior by passing prior mode for regular targets and `0644` otherwise; `Restore` passes the captured prior permission bits. Do not add a second atomic-write implementation.

- [ ] **Step 4: Format and run focused package checks**

Run: `just fmt-go && just check-go`

Expected: all Go tests pass.

- [ ] **Step 5: Request commit approval**

State intent to create the following commit and wait for explicit approval. After approval only:

```bash
git add internal/materialize/service.go internal/materialize/service_test.go
git commit -m "feat: add materialization restore primitives"
```

### Task 2: Add Recovery State removal to repository persistence

**Files:**

- Modify: `internal/repositorystate/store.go:23-31,364-456`
- Test: `internal/repositorystate/store_test.go:28-121`

**Interfaces:**

- Consumes: `RecoveryStateFileName`, existing typed missing-file errors, and `fileStore`.
- Produces: `Store.RemoveRecoveryState(context.Context, string) error`.

- [ ] **Step 1: Write the failing store removal test**

```go
func TestStoreRemovesRecoveryState(t *testing.T) {
    root := t.TempDir()
    store := NewStore()
    state := RecoveryState{Code: RecoveryCodeRollbackIncomplete, Summary: "rollback incomplete", Observations: []RecoveryObservation{{Path: "file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}}
    if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
        t.Fatal(err)
    }
    if err := store.RemoveRecoveryState(context.Background(), root); err != nil {
        t.Fatal(err)
    }
    if _, err := store.LoadRecoveryState(context.Background(), root); !stateFileNotFoundForTest(err, StateFileRecovery) {
        t.Fatalf("LoadRecoveryState() error = %v, want typed not-found", err)
    }
    if err := store.RemoveRecoveryState(context.Background(), root); err == nil {
        t.Fatal("second RemoveRecoveryState() error = nil")
    }
}
```

Add this local test helper rather than exporting production behavior:

```go
func stateFileNotFoundForTest(err error, file StateFile) bool {
    var state StateFileError
    return errors.As(err, &state) && state.File == file && state.Kind == StateFileErrorNotFound
}
```

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `just check-go`

Expected: compilation fails because `Store.RemoveRecoveryState` does not exist.

- [ ] **Step 3: Add only the required removal method**

Extend `Store` and implement `fileStore`:

```go
type Store interface {
    LoadManifest(context.Context, string) (Manifest, error)
    WriteManifest(context.Context, string, Manifest) error
    LoadLockfile(context.Context, string) (Lockfile, error)
    WriteLockfile(context.Context, string, Lockfile) error
    LoadMaterializationRecord(context.Context, string) (MaterializationRecord, error)
    WriteMaterializationRecord(context.Context, string, MaterializationRecord) error
    LoadRecoveryState(context.Context, string) (RecoveryState, error)
    WriteRecoveryState(context.Context, string, RecoveryState) error
    RemoveRecoveryState(context.Context, string) error
}

func (fileStore) RemoveRecoveryState(_ context.Context, root string) error {
    return os.Remove(filepath.Join(root, RecoveryStateFileName))
}
```

Do not add generic deletion, transaction, rename, or backup APIs. The install layer performs the required absence verification by loading Recovery State after removal.

- [ ] **Step 4: Format and run Go checks**

Run: `just fmt-go && just check-go`

Expected: all Go tests pass, including embedded test stores that inherit the expanded interface.

- [ ] **Step 5: Request commit approval**

Wait for explicit approval, then:

```bash
git add internal/repositorystate/store.go internal/repositorystate/store_test.go
git commit -m "feat: remove recovery state through store"
```

### Task 3: Journal all operation mutations and perform verified rollback

**Files:**

- Create: `internal/install/transaction.go`
- Create: `internal/install/transaction_test.go`
- Modify: `internal/install/service.go:106-112,117-207`
- Modify: `internal/install/sync.go:457-606`

**Interfaces:**

- Consumes: Task 1 `materialize.ReadPrior`, `Restore`, `MatchesPrior`, and `MissingParents`; existing repository-state writes; existing deterministic `plannedFile` order.
- Produces: private `mutationHook`, `runMutation`, `transaction.apply`, `transaction.rollback`, and `transaction.fail(error) (error, bool)`; `Service.mutationHook` as an unexported deterministic test seam. The boolean reports whether Recovery State was created so callers retain the operational error class. No exported transaction type.

- [ ] **Step 1: Write failing public-service rollback tests**

Create tests that call `Service.Install` or `Service.Sync`, never transaction methods directly. Use an unexported hook that can call the real mutation and then return a controlled error:

```go
type mutationHook func(kind mutationKind, path string, apply func() error) error

func failAfter(kind mutationKind, path string, failure error) mutationHook {
    return func(gotKind mutationKind, gotPath string, apply func() error) error {
        if err := apply(); err != nil {
            return err
        }
        if gotKind == kind && gotPath == path {
            return failure
        }
        return nil
    }
}
```

Use these complete test bodies; existing `testService`, `testResolved`, and `testArtifact` helpers supply the Source seam:

```go
func TestInstallRollbackRestoresFilesModesAbsenceAndDirectories(t *testing.T) {
    root := t.TempDir()
    if err := os.WriteFile(filepath.Join(root, "old"), []byte("before"), 0o600); err != nil {
        t.Fatal(err)
    }
    artifact := source.ArtifactDescriptor{Name: "a", Version: "1.0.0", Steps: []source.MaterializationStep{
        {Type: "file", TargetPath: "old", SourceBytes: []byte("after")},
        {Type: "file", TargetPath: "nested/new", SourceBytes: []byte("new")},
    }}
    service, _ := testService(testResolved(artifact))
    failure := errors.New("controlled write failure")
    service.mutationHook = failAfter(mutationWrite, repositorystate.MaterializationRecordFileName, failure)

    _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a"})
    if !errors.Is(err, failure) {
        t.Fatalf("Install() error = %v", err)
    }
    data, readErr := os.ReadFile(filepath.Join(root, "old"))
    info, statErr := os.Stat(filepath.Join(root, "old"))
    if readErr != nil || statErr != nil || string(data) != "before" || info.Mode().Perm() != 0o600 {
        t.Fatalf("old = %q mode %v, read %v stat %v", data, info.Mode(), readErr, statErr)
    }
    for _, path := range []string{"nested/new", "nested", repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName, repositorystate.RecoveryStateFileName} {
        if _, statErr := os.Stat(filepath.Join(root, path)); !os.IsNotExist(statErr) {
            t.Fatalf("%s exists after rollback: %v", path, statErr)
        }
    }
}

func TestInstallStateWriteFailureRollsBackTargetsAndEarlierState(t *testing.T) {
    root := t.TempDir()
    service, impl := testService(testResolved(testArtifact("a", "a"), testArtifact("b", "b")))
    if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a"}); err != nil {
        t.Fatal(err)
    }
    names := []string{repositorystate.ManifestFileName, repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName}
    before := map[string][]byte{}
    modes := map[string]os.FileMode{}
    for _, name := range names {
        before[name], _ = os.ReadFile(filepath.Join(root, name))
        info, err := os.Stat(filepath.Join(root, name))
        if err != nil {
            t.Fatal(err)
        }
        modes[name] = info.Mode().Perm()
    }
    impl.resolved = testResolved(testArtifact("a", "a"), testArtifact("b", "b"))
    failure := errors.New("manifest write failed")
    service.mutationHook = failAfter(mutationWrite, repositorystate.ManifestFileName, failure)
    _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "b"})
    if !errors.Is(err, failure) {
        t.Fatalf("Install() error = %v", err)
    }
    for _, name := range names {
        data, readErr := os.ReadFile(filepath.Join(root, name))
        info, statErr := os.Stat(filepath.Join(root, name))
        if readErr != nil || statErr != nil || !bytes.Equal(data, before[name]) || info.Mode().Perm() != modes[name] {
            t.Fatalf("%s was not restored", name)
        }
    }
    if _, err := os.Stat(filepath.Join(root, "b")); !os.IsNotExist(err) {
        t.Fatalf("b exists after rollback: %v", err)
    }
}

func TestSyncRollbackRunsReverseAttemptsEveryRestoreAndWritesSanitizedRecovery(t *testing.T) {
    root := t.TempDir()
    syncManifest(t, root, artifactDeclaration("a"))
    service, impl := testService(testResolved(testArtifact("a", "a")))
    if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
        t.Fatal(err)
    }
    prior := []byte("prior secret contents")
    if err := os.WriteFile(filepath.Join(root, "a"), prior, 0o640); err != nil {
        t.Fatal(err)
    }
    record, err := repositorystate.NewStore().LoadMaterializationRecord(context.Background(), root)
    if err != nil {
        t.Fatal(err)
    }
    record.Artifacts[0].Files[0].Digest = materialize.Digest(prior)
    if err := repositorystate.NewStore().WriteMaterializationRecord(context.Background(), root, record); err != nil {
        t.Fatal(err)
    }
    changed := testArtifact("a", "a")
    changed.Steps[0].SourceBytes = []byte("after")
    impl.resolved = testResolved(changed)
    operationFailure := errors.New("raw operation secret")
    restoreFailure := errors.New("raw restoration secret")
    var mutations, restorations []string
    service.mutationHook = func(kind mutationKind, path string, apply func() error) error {
        switch kind {
        case mutationWrite, mutationRemove:
            if err := apply(); err != nil {
                return err
            }
            mutations = append(mutations, path)
            if path == repositorystate.MaterializationRecordFileName {
                return operationFailure
            }
            return nil
        case mutationRestore:
            restorations = append(restorations, path)
            if path == "a" {
                return restoreFailure
            }
            return apply()
        default:
            return apply()
        }
    }
    _, err = service.Sync(context.Background(), SyncRequest{Root: root})
    if !errors.Is(err, operationFailure) {
        t.Fatalf("Sync() error = %v", err)
    }
    wantOrder := slices.Clone(mutations)
    slices.Reverse(wantOrder)
    if !reflect.DeepEqual(restorations, wantOrder) {
        t.Fatalf("restore order = %#v, want %#v", restorations, wantOrder)
    }
    state, err := repositorystate.NewStore().LoadRecoveryState(context.Background(), root)
    if err != nil || state.Code != repositorystate.RecoveryCodeRollbackIncomplete || state.Summary != recoverySummary || len(state.Observations) != 1 {
        t.Fatalf("Recovery State = %#v, %v", state, err)
    }
    observation := state.Observations[0]
    if observation.Path != "a" || observation.Result != repositorystate.RecoveryResultRestoreFailed || observation.ExpectedState != repositorystate.RecoveryExpectedFile || observation.Digest != materialize.Digest(prior) || observation.Mode != 0o640 || observation.Owner == nil || observation.Owner.Artifact != "a" {
        t.Fatalf("observation = %#v", observation)
    }
    data, err := os.ReadFile(filepath.Join(root, repositorystate.RecoveryStateFileName))
    info, statErr := os.Stat(filepath.Join(root, repositorystate.RecoveryStateFileName))
    if err != nil || statErr != nil || info.Mode().Perm() != 0o600 || bytes.Contains(data, prior) || bytes.Contains(data, []byte("raw operation secret")) || bytes.Contains(data, []byte("raw restoration secret")) || bytes.Contains(data, []byte(root)) {
        t.Fatalf("unsafe Recovery State: %s, read %v stat %v", data, err, statErr)
    }
}

func TestSyncJoinsRecoveryModeVerificationFailureWithOriginalFailure(t *testing.T) {
    root := t.TempDir()
    syncManifest(t, root, artifactDeclaration("a"))
    service, impl := testService(testResolved(testArtifact("a", "a")))
    if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
        t.Fatal(err)
    }
    changed := testArtifact("a", "a")
    changed.Steps[0].SourceBytes = []byte("after")
    impl.resolved = testResolved(changed)
    operationFailure := errors.New("operation failed")
    service.mutationHook = func(kind mutationKind, path string, apply func() error) error {
        switch {
        case kind == mutationWrite && path == repositorystate.MaterializationRecordFileName:
            if err := apply(); err != nil {
                return err
            }
            return operationFailure
        case kind == mutationRestore && path == "a":
            return errors.New("restore failed")
        case kind == mutationRecovery:
            if err := apply(); err != nil {
                return err
            }
            return os.Chmod(filepath.Join(root, repositorystate.RecoveryStateFileName), 0o644)
        default:
            return apply()
        }
    }
    _, err := service.Sync(context.Background(), SyncRequest{Root: root})
    if !errors.Is(err, operationFailure) || !strings.Contains(err.Error(), "write recovery state") {
        t.Fatalf("Sync() error = %v", err)
    }
    info, statErr := os.Stat(filepath.Join(root, repositorystate.RecoveryStateFileName))
    if statErr != nil {
        t.Fatal(statErr)
    }
    if info.Mode().Perm() != 0o644 {
        t.Fatalf("Recovery State mode = %v", info.Mode())
    }
}

func TestSyncAcceptsVerifiedRestorationDespiteReportedActionError(t *testing.T) {
    root := t.TempDir()
    syncManifest(t, root, artifactDeclaration("a"))
    service, impl := testService(testResolved(testArtifact("a", "a")))
    if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
        t.Fatal(err)
    }
    prior, err := os.ReadFile(filepath.Join(root, "a"))
    if err != nil {
        t.Fatal(err)
    }
    changed := testArtifact("a", "a")
    changed.Steps[0].SourceBytes = []byte("after")
    impl.resolved = testResolved(changed)
    operationFailure := errors.New("operation failed")
    restorationReported := errors.New("restoration reported an error after success")
    service.mutationHook = func(kind mutationKind, path string, apply func() error) error {
        switch {
        case kind == mutationWrite && path == repositorystate.MaterializationRecordFileName:
            if err := apply(); err != nil {
                return err
            }
            return operationFailure
        case kind == mutationRestore && path == "a":
            if err := apply(); err != nil {
                return err
            }
            return restorationReported
        default:
            return apply()
        }
    }
    _, err = service.Sync(context.Background(), SyncRequest{Root: root})
    if !errors.Is(err, operationFailure) || errors.Is(err, restorationReported) {
        t.Fatalf("Sync() error = %v", err)
    }
    restored, readErr := os.ReadFile(filepath.Join(root, "a"))
    if readErr != nil || !bytes.Equal(restored, prior) {
        t.Fatalf("restored = %q, %v", restored, readErr)
    }
    if _, loadErr := repositorystate.NewStore().LoadRecoveryState(context.Background(), root); !stateNotFound(loadErr, repositorystate.StateFileRecovery) {
        t.Fatalf("Recovery State exists after verified rollback: %v", loadErr)
    }
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `just check-go`

Expected: compilation fails because the transaction and `Service.mutationHook` do not exist.

- [ ] **Step 3: Implement the private journal data and capture path**

Use a narrow set of mutation kinds and exact private shapes:

```go
type mutationKind string

const (
    mutationWrite       mutationKind = "write"
    mutationRemove      mutationKind = "remove"
    mutationRestore     mutationKind = "restore"
    mutationRecovery    mutationKind = "recovery_write"
)

const recoverySummary = "rollback could not restore every path"

type journalEntry struct {
    prior          materialize.Observation
    bytes          []byte
    missingParents []string
    owner          *repositorystate.RecoveryOwner
}

type transaction struct {
    root  string
    store repositorystate.Store
    hook  mutationHook
    items []journalEntry
}
```

`transaction.apply(kind, path, owner, apply)` must:

1. Call `materialize.Observe(tx.root, path)` immediately before mutation.
2. Call `materialize.ReadPrior(prior)` and copy returned bytes.
3. Append the entry before invoking mutation.
4. Invoke the hook when present, otherwise invoke `apply` directly.

Implement `runMutation(hook, kind, path, apply)` once and make `transaction.run` delegate to it; Task 4 reuses the same internal test seam while clearing Recovery State. Copy `RecoveryOwner` values when recording them. State-file entries pass `nil`; materialized files pass source, resolved version, and artifact from their `plannedFile`.

- [ ] **Step 4: Implement reverse restoration, directory cleanup, verification, and Recovery State persistence**

For each journal entry from last to first:

```go
current, observeErr := materialize.Observe(tx.root, entry.prior.Path)
if observeErr == nil {
    if entry.prior.Kind == materialize.EntryAbsent {
        if current.Kind != materialize.EntryAbsent {
            restoreErr = tx.run(mutationRestore, entry.prior.Path, func() error { return materialize.Remove(current) })
        }
    } else {
        restoreErr = tx.run(mutationRestore, entry.prior.Path, func() error {
            return materialize.Restore(current, entry.bytes, entry.prior.Mode.Perm())
        })
    }
}
```

Always re-observe after the restoration attempt, including when observation or restoration returned an error. Require `materialize.MatchesPrior(entry.prior, restored)` to decide whether the path was restored. When final state matches, treat that entry as successfully restored and retain any action error only in internal rollback error handling. When final state does not match, record `restore_failed` if observation/restoration failed; otherwise record `verification_failed`.

After targets, deduplicate every `missingParents` path, sort by descending depth then descending path, observe each directory, remove it only when present, and verify absence. Existing parents are absent from this list and therefore never removed.

Build one sorted Recovery Observation per path whose final state is not verified. Expected files use captured `Digest`, `Mode.Perm()`, and copied owner; expected-absent paths omit digest, mode, and owner unless the target itself has managed ownership. Persist through `tx.run(mutationRecovery, repositorystate.RecoveryStateFileName, write)` so tests can force a controlled partial recovery write. Verify with `materialize.Observe`: the target must be a regular file with mode `0600` and safe topology. Then load it strictly, require `reflect.DeepEqual` with the constructed value, and call `materialize.Revalidate` on the observed Recovery State file to detect change during verification. Recovery write, rooted observation, mode, reload, value-comparison, or revalidation failure returns `errors.Join(original, fmt.Errorf("write recovery state: %w", recoveryErr)), false`; verified persistence returns `original, true`. A complete rollback returns `original, false`.

- [ ] **Step 5: Route Install and Sync mutations through the journal**

Add only this private field:

```go
type Service struct {
    registry     source.Registry
    store        repositorystate.Store
    mutationHook mutationHook
}
```

Keep `NewService` unchanged for callers. In declaration-only Install, create a transaction after final root validation and route `WriteManifest` through `transaction.apply`; call `transaction.fail` on mutation failure.

Change `applyPrepared` to accept `*transaction`, replace direct `materialize.Write`/`Remove` calls with journaled calls, and delete the `created []string` return plus `cleanup`. In `persistPrepared`, create one transaction for non-Dry Run, route Lockfile, Materialization Record, and Manifest writes through it, and call `transaction.fail` for every error after the first journal entry, including adoption and root revalidation failures. When its boolean is true, return the error immediately as operational; otherwise preserve existing typed drift classification after successful rollback. This prevents an initial mutation failure that created Recovery State from being reclassified as a user-action conflict.

- [ ] **Step 6: Run transaction tests and full Go checks**

Run: `just fmt-go && just check-go`

Expected: all Go tests pass. Recovery State exists only for the deliberately incomplete rollback case.

- [ ] **Step 7: Request commit approval**

Wait for explicit approval, then:

```bash
git add internal/install/transaction.go internal/install/transaction_test.go internal/install/service.go internal/install/sync.go
git commit -m "feat: roll back failed install mutations"
```

### Task 4: Gate later operations on Recovery State

**Files:**

- Create: `internal/install/recovery.go`
- Create: `internal/install/recovery_test.go`
- Modify: `internal/install/service.go:117-168`
- Modify: `internal/install/sync.go:40-82`

**Interfaces:**

- Consumes: `Store.LoadRecoveryState`, Task 2 `Store.RemoveRecoveryState`, `materialize.Observe`, `RecoveryObservation`, and existing `stateNotFound`.
- Produces: `RecoveryConflictError{Observations []repositorystate.RecoveryObservation}`, private `recoveryClearError`, `mutationRecoveryClear`, `Service.inspectRecovery(context.Context, operationRoot, bool) error`, and inspect-then-revalidate observation matching before normal state reads.

- [ ] **Step 1: Write failing public-service recovery gate tests**

Add this store seam and complete tests. Existing install test helpers remain reused:

```go
type removeFailingStore struct {
    repositorystate.Store
    err error
}

func (store removeFailingStore) RemoveRecoveryState(context.Context, string) error {
    return store.err
}

func recoveryState(observation repositorystate.RecoveryObservation) repositorystate.RecoveryState {
    return repositorystate.RecoveryState{Code: repositorystate.RecoveryCodeRollbackIncomplete, Summary: recoverySummary, Observations: []repositorystate.RecoveryObservation{observation}}
}

func TestInstallMismatchingRecoveryBlocksBeforeSourceResolution(t *testing.T) {
    root := t.TempDir()
    if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("changed"), 0o644); err != nil {
        t.Fatal(err)
    }
    store := repositorystate.NewStore()
    state := recoveryState(repositorystate.RecoveryObservation{Path: "blocked", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
    if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
        t.Fatal(err)
    }
    service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
    _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a"})
    var conflict RecoveryConflictError
    if !errors.As(err, &conflict) || sourceImpl.calls != 0 || !reflect.DeepEqual(conflict.Observations, state.Observations) {
        t.Fatalf("Install() error = %T %v, calls = %d", err, err, sourceImpl.calls)
    }
    if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
        t.Fatalf("Recovery State was removed: %v", err)
    }
    if _, err := os.Stat(filepath.Join(root, repositorystate.ManifestFileName)); !os.IsNotExist(err) {
        t.Fatalf("Manifest exists: %v", err)
    }
}

func TestSyncMatchingRecoveryClearsBeforeNormalOperation(t *testing.T) {
    root := t.TempDir()
    syncManifest(t, root, artifactDeclaration("a"))
    marker := []byte("fixed")
    if err := os.WriteFile(filepath.Join(root, "marker"), marker, 0o640); err != nil {
        t.Fatal(err)
    }
    store := repositorystate.NewStore()
    state := recoveryState(repositorystate.RecoveryObservation{Path: "marker", Result: repositorystate.RecoveryResultVerificationFailed, ExpectedState: repositorystate.RecoveryExpectedFile, Digest: materialize.Digest(marker), Mode: 0o640})
    if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
        t.Fatal(err)
    }
    sourceImpl := &testSource{resolve: func(source.ResolveRequest) (source.ResolvedSource, error) {
        _, err := store.LoadRecoveryState(context.Background(), root)
        if !stateNotFound(err, repositorystate.StateFileRecovery) {
            return source.ResolvedSource{}, fmt.Errorf("Recovery State not cleared before resolve: %v", err)
        }
        return testResolved(testArtifact("a", "a")), nil
    }}
    service := NewService(source.NewStaticRegistry(map[string]source.Source{"file": sourceImpl}), store)
    if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
        t.Fatal(err)
    }
    if sourceImpl.calls != 1 {
        t.Fatalf("Resolve calls = %d", sourceImpl.calls)
    }
}

func TestSyncDryRunInspectsButNeverClearsRecovery(t *testing.T) {
    root := t.TempDir()
    syncManifest(t, root, artifactDeclaration("a"))
    store := repositorystate.NewStore()
    state := recoveryState(repositorystate.RecoveryObservation{Path: "missing", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
    if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
        t.Fatal(err)
    }
    service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
    if _, err := service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true}); err != nil {
        t.Fatal(err)
    }
    if sourceImpl.calls != 1 {
        t.Fatalf("Resolve calls = %d", sourceImpl.calls)
    }
    if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
        t.Fatalf("Dry Run cleared Recovery State: %v", err)
    }
    if err := os.WriteFile(filepath.Join(root, "missing"), []byte("changed"), 0o644); err != nil {
        t.Fatal(err)
    }
    sourceImpl.calls = 0
    _, err := service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true})
    var conflict RecoveryConflictError
    if !errors.As(err, &conflict) || sourceImpl.calls != 0 {
        t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
    }
    if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
        t.Fatalf("blocked Dry Run cleared Recovery State: %v", err)
    }
}

func TestRecoveryClearFailureLeavesOperationBlocked(t *testing.T) {
    root := t.TempDir()
    syncManifest(t, root, artifactDeclaration("a"))
    base := repositorystate.NewStore()
    state := recoveryState(repositorystate.RecoveryObservation{Path: "missing", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
    if err := base.WriteRecoveryState(context.Background(), root, state); err != nil {
        t.Fatal(err)
    }
    failure := errors.New("remove failed")
    store := removeFailingStore{Store: base, err: failure}
    sourceImpl := &testSource{resolved: testResolved(testArtifact("a", "a"))}
    service := NewService(source.NewStaticRegistry(map[string]source.Source{"file": sourceImpl}), store)
    _, err := service.Sync(context.Background(), SyncRequest{Root: root})
    var conflict RecoveryConflictError
    if !errors.Is(err, failure) || errors.As(err, &conflict) || sourceImpl.calls != 0 || err.Error() != "rollback_incomplete: recovery state could not be cleared" {
        t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
    }
    if strings.Contains(err.Error(), failure.Error()) || strings.Contains(err.Error(), root) {
        t.Fatalf("clear error leaked details: %q", err.Error())
    }
    if _, err := base.LoadRecoveryState(context.Background(), root); err != nil {
        t.Fatalf("Recovery State missing: %v", err)
    }
}

func TestRecoveryParentChangeDuringClearReturnsSanitizedConflict(t *testing.T) {
    root := t.TempDir()
    syncManifest(t, root, artifactDeclaration("a"))
    if err := os.Mkdir(filepath.Join(root, "parent"), 0o755); err != nil {
        t.Fatal(err)
    }
    store := repositorystate.NewStore()
    state := recoveryState(repositorystate.RecoveryObservation{Path: "parent/missing", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
    if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
        t.Fatal(err)
    }
    service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
    service.mutationHook = func(kind mutationKind, _ string, apply func() error) error {
        if kind == mutationRecoveryClear {
            if err := os.Rename(filepath.Join(root, "parent"), filepath.Join(root, "old-parent")); err != nil {
                return err
            }
            if err := os.Mkdir(filepath.Join(root, "parent"), 0o755); err != nil {
                return err
            }
        }
        return apply()
    }
    _, err := service.Sync(context.Background(), SyncRequest{Root: root})
    var conflict RecoveryConflictError
    if !errors.As(err, &conflict) || sourceImpl.calls != 0 || !reflect.DeepEqual(conflict.Observations, state.Observations) {
        t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
    }
    if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
        t.Fatalf("Recovery State was cleared after topology change: %v", err)
    }
}
```

Add a focused mismatch test with one fresh root per case:

```go
func TestRecoveryExpectedFileRejectsTypeDigestModeAndUnsafeTopology(t *testing.T) {
    cases := []struct {
        name string
        path string
        setUp func(*testing.T, string)
    }{
        {name: "non-regular", path: "target", setUp: func(t *testing.T, root string) { t.Helper(); if err := os.Mkdir(filepath.Join(root, "target"), 0o755); err != nil { t.Fatal(err) } }},
        {name: "digest", path: "target", setUp: func(t *testing.T, root string) { t.Helper(); if err := os.WriteFile(filepath.Join(root, "target"), []byte("other"), 0o644); err != nil { t.Fatal(err) } }},
        {name: "mode", path: "target", setUp: func(t *testing.T, root string) { t.Helper(); if err := os.WriteFile(filepath.Join(root, "target"), []byte("expected"), 0o600); err != nil { t.Fatal(err) } }},
        {name: "unsafe topology", path: "parent/target", setUp: func(t *testing.T, root string) { t.Helper(); outside := t.TempDir(); if err := os.Symlink(outside, filepath.Join(root, "parent")); err != nil { t.Skipf("symlink: %v", err) } }},
    }
    for _, test := range cases {
        t.Run(test.name, func(t *testing.T) {
            root := t.TempDir()
            syncManifest(t, root, artifactDeclaration("a"))
            test.setUp(t, root)
            store := repositorystate.NewStore()
            state := recoveryState(repositorystate.RecoveryObservation{Path: test.path, Result: repositorystate.RecoveryResultVerificationFailed, ExpectedState: repositorystate.RecoveryExpectedFile, Digest: materialize.Digest([]byte("expected")), Mode: 0o644})
            if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
                t.Fatal(err)
            }
            service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
            _, err := service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true})
            var conflict RecoveryConflictError
            if !errors.As(err, &conflict) || sourceImpl.calls != 0 || conflict.Observations[0].Path != test.path {
                t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
            }
        })
    }
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `just check-go`

Expected: compilation fails because `RecoveryConflictError` and `inspectRecovery` do not exist.

- [ ] **Step 3: Implement typed conflict and exact matching**

Use this public error shape:

```go
type RecoveryConflictError struct {
    Observations []repositorystate.RecoveryObservation
}

func (RecoveryConflictError) Error() string {
    return repositorystate.RecoveryCodeRollbackIncomplete + ": repository recovery requires user action"
}

type recoveryClearError struct {
    cause error
}

func (recoveryClearError) Error() string {
    return repositorystate.RecoveryCodeRollbackIncomplete + ": recovery state could not be cleared"
}

func (err recoveryClearError) Unwrap() error {
    return err.cause
}
```

Add `mutationRecoveryClear mutationKind = "recovery_clear"`. `inspectRecovery` loads Recovery State. Typed not-found continues. Other load failures remain operational errors. For each sorted observation:

- call `materialize.Observe(root.path, observation.Path)`;
- treat any observe error, including unsafe topology, as mismatch;
- require `EntryAbsent` for expected absent;
- require `EntryRegular`, exact digest, and `Mode.Perm() == os.FileMode(observation.Mode).Perm()` for expected file;
- retain each matching `materialize.Observation` for non-Dry Run revalidation.

Any mismatch returns `RecoveryConflictError` carrying a copy of the sanitized persisted observations. Matching Dry Run returns nil without removal. Matching non-Dry Run calls `runMutation(service.mutationHook, mutationRecoveryClear, repositorystate.RecoveryStateFileName, clear)`; `clear` first calls `materialize.Revalidate` for every retained observation, then calls `RemoveRecoveryState`, then `LoadRecoveryState`. A revalidation change returns the same typed sanitized conflict and leaves Recovery State present. Only typed not-found verifies successful removal. Removal, removal-hook, present-file, or unreadable-file failure returns `recoveryClearError{cause: err}`: operational exit class, fixed safe message, and preserved `errors.Is` inspection without exposing the cause or absolute root through `Error()`.

- [ ] **Step 4: Invoke the gate at the required point in both operations**

In `Install` and `Sync`, call:

```go
if err := operation.validate(); err != nil {
    return Result{}, err
}
if err := service.inspectRecovery(ctx, operation, request.DryRun); err != nil {
    return Result{}, err
}
```

Place it immediately after root identity validation and before `NormalizeSourceIdentity`, Manifest/Lockfile/Materialization Record loads, registry lookup, Source resolution, or preflight. Request-shape validation remains before opening the root because it performs no repository inspection or mutation.

- [ ] **Step 5: Format and run Go checks**

Run: `just fmt-go && just check-go`

Expected: all Go tests pass, including both matching and mismatching Dry Run paths.

- [ ] **Step 6: Request commit approval**

Wait for explicit approval, then:

```bash
git add internal/install/recovery.go internal/install/recovery_test.go internal/install/service.go internal/install/sync.go
git commit -m "feat: gate operations on recovery state"
```

### Task 5: Report Recovery State conflicts in human and JSON output

**Files:**

- Modify: `cmd/tbboot/root.go:32-69`
- Test: `cmd/tbboot/root_test.go:350-830`

**Interfaces:**

- Consumes: Task 4 `install.RecoveryConflictError`, `app.ExitUserActionConflict`, shared `app.Result`, and repository-state Recovery Observation values.
- Produces: private command DTOs mirroring Recovery State's nested `expected` object, human lines containing `rollback_incomplete` plus canonical paths, and JSON `details.recovery_code`/`details.observations` with numeric code 2.

- [ ] **Step 1: Write failing CLI output tests**

Create a repository with a mismatching Recovery State and use the existing CLI test helpers:

```go
func TestRecoveryConflictHumanAndJSONOutputIsStableAndSanitized(t *testing.T) {
    root := t.TempDir()
    initGitRepo(t, root)
    if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("prior contents secret"), 0o644); err != nil {
        t.Fatal(err)
    }
    state := repositorystate.RecoveryState{
        Code:    repositorystate.RecoveryCodeRollbackIncomplete,
        Summary: "raw error secret",
        Observations: []repositorystate.RecoveryObservation{
            {Path: "blocked", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent},
            {Path: "missing", Result: repositorystate.RecoveryResultVerificationFailed, ExpectedState: repositorystate.RecoveryExpectedFile, Digest: "sha256:" + strings.Repeat("a", 64), Mode: 0o640, Owner: &repositorystate.RecoveryOwner{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, ResolvedVersion: "sha256:" + strings.Repeat("b", 64), Artifact: "tool"}},
        },
    }
    if err := repositorystate.NewStore().WriteRecoveryState(context.Background(), root, state); err != nil {
        t.Fatal(err)
    }

    withDir(t, root, func() {
        var human bytes.Buffer
        if code := execute(context.Background(), []string{"install"}, &bytes.Buffer{}, &human); code != int(app.ExitUserActionConflict) {
            t.Fatalf("human exit code = %d", code)
        }
        for _, want := range []string{"rollback_incomplete blocked", "rollback_incomplete missing"} {
            if !strings.Contains(human.String(), want) {
                t.Fatalf("human stderr = %q, want %q", human.String(), want)
            }
        }
        for _, unsafe := range []string{root, "prior contents secret", "raw error secret"} {
            if strings.Contains(human.String(), unsafe) {
                t.Fatalf("human stderr leaks %q: %q", unsafe, human.String())
            }
        }

        var stdout, stderr bytes.Buffer
        if code := execute(context.Background(), []string{"--output", "json", "install"}, &stdout, &stderr); code != int(app.ExitUserActionConflict) {
            t.Fatalf("JSON exit code = %d", code)
        }
        if stdout.Len() != 0 {
            t.Fatalf("stdout = %q", stdout.String())
        }
        var envelope app.Result
        if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
            t.Fatal(err)
        }
        observations, ok := envelope.Details["observations"].([]any)
        if envelope.Code != app.ExitUserActionConflict || envelope.Details["recovery_code"] != "rollback_incomplete" || !ok || len(observations) != 2 {
            t.Fatalf("envelope = %#v", envelope)
        }
        first, ok := observations[0].(map[string]any)
        firstExpected, expectedOK := first["expected"].(map[string]any)
        if !ok || !expectedOK || first["path"] != "blocked" || firstExpected["state"] != "absent" {
            t.Fatalf("first observation = %#v", observations[0])
        }
        second, ok := observations[1].(map[string]any)
        secondExpected, expectedOK := second["expected"].(map[string]any)
        owner, ownerOK := second["owner"].(map[string]any)
        if !ok || !expectedOK || !ownerOK || second["path"] != "missing" || secondExpected["state"] != "file" || secondExpected["digest"] != state.Observations[1].Digest || owner["artifact"] != "tool" {
            t.Fatalf("second observation = %#v", observations[1])
        }
        for _, unsafe := range []string{root, "prior contents secret", "raw error secret"} {
            if strings.Contains(stderr.String(), unsafe) {
                t.Fatalf("JSON stderr leaks %q: %q", unsafe, stderr.String())
            }
        }
    })
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `just check-go`

Expected: exit code remains 1 or the recovery-specific JSON fields are absent.

- [ ] **Step 3: Add recovery-specific error mapping without changing normal conflict output**

Add private command DTOs instead of JSON tags on repository-state domain types:

```go
type recoveryObservationDetail struct {
    Path     string                  `json:"path"`
    Result   string                  `json:"result"`
    Expected recoveryExpectedDetail  `json:"expected"`
    Owner    *recoveryOwnerDetail     `json:"owner,omitempty"`
}

type recoveryExpectedDetail struct {
    State  string `json:"state"`
    Digest string `json:"digest,omitempty"`
    Mode   uint32 `json:"mode,omitempty"`
}

type recoveryOwnerDetail struct {
    Source        repositorystate.SourceIdentity `json:"source"`
    SourceVersion string                         `json:"source_version"`
    Artifact      string                         `json:"artifact"`
}

func recoveryDetails(observations []repositorystate.RecoveryObservation) []recoveryObservationDetail {
    details := make([]recoveryObservationDetail, 0, len(observations))
    for _, observation := range observations {
        detail := recoveryObservationDetail{
            Path:   observation.Path,
            Result: observation.Result,
            Expected: recoveryExpectedDetail{
                State:  observation.ExpectedState,
                Digest: observation.Digest,
                Mode:   observation.Mode,
            },
        }
        if observation.Owner != nil {
            detail.Owner = &recoveryOwnerDetail{
                Source:        observation.Owner.Source,
                SourceVersion: observation.Owner.ResolvedVersion,
                Artifact:      observation.Owner.Artifact,
            }
        }
        details = append(details, detail)
    }
    return details
}
```

In `execute`, classify `RecoveryConflictError` as `app.ExitUserActionConflict`. For JSON, construct:

```go
result := app.Result{
    Code:    app.ExitUserActionConflict,
    Message: recovery.Error(),
    Details: map[string]any{
        "recovery_code": repositorystate.RecoveryCodeRollbackIncomplete,
        "observations":  recoveryDetails(recovery.Observations),
    },
}
```

For human output, print one deterministic line per sorted observation:

```go
fmt.Fprintf(stderr, "%s %s\n", repositorystate.RecoveryCodeRollbackIncomplete, observation.Path)
```

Check recovery conflict before generic `UserActionError` rendering. Do not add prior bytes, absolute root, or wrapped raw errors to either format.

- [ ] **Step 4: Format and run Go checks**

Run: `just fmt-go && just check-go`

Expected: all CLI and service tests pass with exit classes unchanged for existing conflicts, trust denial, and operational failures.

- [ ] **Step 5: Request commit approval**

Wait for explicit approval, then:

```bash
git add cmd/tbboot/root.go cmd/tbboot/root_test.go
git commit -m "feat: report rollback recovery conflicts"
```

### Task 6: Update canonical contracts and run final validation

**Files:**

- Modify: `CONTEXT.md:5-12,350-365`
- Modify: `ARCHITECTURE.md:5-30,45-55`
- Modify: `docs/adr/0004-materialization-ownership-drift-and-recovery.md:13-29`

**Interfaces:**

- Consumes: implemented behavior from Tasks 1 through 5 and the repository documentation lifecycle.
- Produces: canonical product, architecture, and ADR language that distinguishes implemented in-process rollback from deferred crash recovery.

- [ ] **Step 1: Replace specific obsolete product-contract claims**

In `CONTEXT.md`, replace claims that the full/runtime rollback lifecycle and recovery blocking are deferred. State these exact durable rules in the product 0.1 contract and active decisions:

```markdown
Controlled in-process mutation failures trigger reverse-order, verified best-effort rollback from in-memory prior-state captures. When any restoration cannot be verified, `tbboot` writes sanitized **Recovery State** and later Install or Sync operations block until its observations match repaired filesystem state. A matching non-Dry Run clears Recovery State before normal state loading and Source resolution; Dry Run inspects but never clears it.
```

Keep durable backups, process-crash recovery, and crash-recoverable lock takeover explicitly deferred. Search the full file for the specific obsolete phrases `full rollback lifecycle`, `runtime creation and rollback lifecycle`, and `Recovery blocking and manual-repair verification`; update every active-contract occurrence without rewriting historical deferred-design discussion that is clearly labeled non-canonical.

- [ ] **Step 2: Update architecture and ADR ownership**

In `ARCHITECTURE.md`, add the in-memory mutation journal to “Persistence and filesystem safety.” State that every restoration attempt is re-observed, verified final state determines rollback success, and Recovery State write verification requires a regular file, mode `0600`, strict reload, value equality, and topology revalidation. Replace “full rollback lifecycle” deferrals with only:

```markdown
- Crash-recoverable operation locks, durable backups, and process-crash recovery.
```

In ADR-0004, replace runtime deferral language and obsolete consequences with:

```markdown
Each non-Dry Run operation journals prior target state in memory immediately before every materialization or repository-state mutation. Controlled mutation failure rolls entries back in reverse order, attempts and re-observes every restoration, verifies prior bytes, permission bits, absence, and created-directory cleanup, and records sanitized Recovery State when final state remains unverified. Verified final state counts as restored even when the restoration action reported an error. Recovery State itself is accepted as recorded only after rooted observation confirms a regular file with mode `0600`, strict reload reproduces the intended values, and revalidation detects no change during verification.

Later Install and Sync operations inspect Recovery State before normal state loading or Source resolution. Mismatches and changes during the current inspect/revalidate window are user-action conflicts; Recovery State does not claim to detect safe parent-directory replacement between processes because it stores no directory identities. Matching non-Dry Run operations clear and verify its absence, while Dry Run never clears it. Clear failures remain sanitized operational errors. Human output reports only the fixed recovery code and canonical paths; JSON uses a private command DTO with nested `expected` data; neither output emits the persisted summary. Durable backups, process-crash recovery, and crash-recoverable lock takeover remain deferred.
```

- [ ] **Step 3: Search canonical documents for remaining obsolete claims**

Run:

```bash
rg -n -i 'full rollback lifecycle|runtime rollback|recovery blocking|manual-repair verification|durable backup|process-crash|crash-recoverable' CONTEXT.md ARCHITECTURE.md UBIQUITOUS_LANGUAGE.md docs/adr
```

Expected: no active canonical claim defers in-process rollback, blocking, verification, or clearing; every remaining deferral is limited to durable backups, process-crash recovery, or crash-recoverable lock takeover.

- [ ] **Step 4: Run final aggregate validation once**

Determine the actual base branch with `git merge-base`/branch metadata, then run:

```bash
just check
git diff --check HEAD
git diff --check <actual-base>...HEAD
```

Expected: `just check` exits 0; both diff checks exit 0 and print no output. If the committed range check reports a pre-existing failure, record it exactly and do not report validation as fully passed.

- [ ] **Step 5: Request final documentation commit approval**

Wait for explicit approval, then:

```bash
git add CONTEXT.md ARCHITECTURE.md docs/adr/0004-materialization-ownership-drift-and-recovery.md
git commit -m "docs: document rollback recovery lifecycle"
```
