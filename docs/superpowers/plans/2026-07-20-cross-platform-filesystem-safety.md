# Cross-platform filesystem safety implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every mutating install/Sync operation safe and deterministic across supported platforms while keeping Dry Run fully non-mutating.

**Architecture:** Keep root discovery and --dry-run in cmd/tbboot, operation locking and preflight orchestration in internal/install, and confined observation, identity comparison, race revalidation, and atomic replacement in internal/materialize. Tighten existing internal/source/file reads to reject unsafe inputs; keep internal/repositorystate atomic state writers unchanged.

**Tech Stack:** Go 1.26.4, standard-library os.Root, os, filepath, os/exec, and os.SameFile; existing Cobra CLI and Go testing; no new dependencies.

## Global Constraints

- Every mutating install operation uses one canonical Operation Root.
- An exclusive .tbboot-operation.lock directory in the Operation Root is the cross-platform lock.
- Creating the lock with os.Mkdir is atomic; an existing path fails the operation.
- A stale lock is not taken over automatically.
- Releasing the lock returns an error; a failed release is reported instead of being ignored.
- Acquire the root lock before reading repository state or resolving a source.
- Dry Run runs the same read, resolve, and preflight flow without acquiring the lock and without writing files, manifest, lockfile, or materialization state.
- Changes are reported with planned; no changes remain no_op. Declaration-only Dry Run additions are planned; unchanged declarations remain no_op.
- repositoryRoot runs git rev-parse --show-toplevel from the current working directory and canonicalizes the result.
- runGit forces a single `LC_ALL=C` child-environment setting before stderr classification, so the non-repository diagnostic is locale-stable.
- Fall back to the canonical current directory only when Git reports the locale-stable non-repository diagnostic.
- Missing Git, permission errors, malformed output, and other Git failures are returned instead of silently changing the scope of the operation.
- Relative source and target paths are normalized against the canonical root.
- Every consumed source path and every target parent component must be a real directory or regular file as appropriate; symlinks, Windows reparse points detected through `FILE_ATTRIBUTE_REPARSE_POINT`, and special files are rejected.
- Escapes outside the Operation Root or source root are rejected before mutation; approved external file: sources keep their existing trust-policy behavior.
- Target collisions are checked using canonical path keys and filesystem identity, including platform case behavior and hard-link aliases where the operating system exposes identity.
- Existing target modes are preserved; new files use 0644.
- Replacement uses a unique temporary file in the target's directory followed by confined same-directory rename.
- Root, parent, target, and opened-directory identity are rechecked immediately before replacement.
- A race becomes the existing typed drift/conflict path.
- Add DryRun to install and sync requests.
- Add OutcomePlanned and DryRun to operation results.
- Include dry_run in JSON operation details for every result.
- Add --dry-run to install, including explicit installs and declaration-only installs.
- Preserve existing human output shape while labeling planned changes as planned rather than applied.
- Do not add Git source acquisition, crash-recoverable advisory locks, rollback/recovery lifecycle, new materialization step types, or a broad filesystem interface.
- Do not create commits unless the user explicitly approves the commit immediately before it is run.

The seven-task split is execution order and file ownership only; it does not add public API boundaries. The plan deliberately keeps pairwise filesystem-identity checks at O(n²) until artifact-set size makes an indexed identity map measurable. `materialize.Write` remains a confined internal replacement primitive: install preflight rejects unsafe final entries before calling it, while the low-level primitive preserves existing final-symlink replacement behavior.

## File Map

- Modify cmd/tbboot/install.go: canonical Operation Root discovery, --dry-run propagation, JSON dry_run, and human planned-output label.
- Modify cmd/tbboot/root_test.go: root-discovery, Dry Run, human output, and JSON output tests.
- Create internal/install/operation_lock.go: exclusive root lock and release closure.
- Create internal/install/operation_lock_test.go: lock acquisition, rejection, and cleanup tests.
- Modify internal/install/service.go: request/result contracts, lock lifecycle, Dry Run declaration handling, and conflict propagation.
- Modify internal/install/sync.go: lock lifecycle, Dry Run persistence branch, and filesystem-identity preflight checks.
- Modify internal/install/service_test.go and internal/install/sync_test.go: service-level lock, Dry Run, topology, collision, and no-write tests.
- Modify internal/install/service_windows_test.go: Windows case-normalization and reparse-point coverage.
- Modify internal/materialize/service.go: confined target observation, entry identity capture, parent/target/root revalidation, and atomic replacement checks.
- Modify internal/materialize/service_test.go: target topology, hard-link identity, mode, atomic replacement, and race tests.
- Create internal/materialize/service_windows_test.go: Windows reparse-point and case behavior tests.
- Modify internal/source/file/source.go: confined source-root reads and rejection of unsafe consumed inputs.
- Modify internal/source/file/source_test.go: source topology, special-file, and escape tests.
- Verify internal/repositorystate/store.go: retain existing same-directory atomic state writers; add no persistence abstraction.

---

### Task 1: Canonical Operation Root discovery

**Files:**

- Modify: cmd/tbboot/install.go
- Test: cmd/tbboot/root_test.go

**Interfaces:**

- Consumes: current working directory from os.Getwd and Git command git rev-parse --show-toplevel.
- Produces: repositoryRoot(context.Context) (string, error) returning a canonical absolute directory, plus an unexported testable repositoryRootAt seam.

- [ ] **Step 1: Add failing root-discovery tests**

Add tests for the pure discovery seam. Inject stdout, stderr, and error so tests do not modify PATH or invoke shell scripts. Add a focused test for the child-environment helper proving that `runGit` supplies exactly one `LC_ALL=C` entry.

~~~go
func TestRepositoryRootAtCanonicalizesGitOutput(t *testing.T) {
    root := t.TempDir()
    alias := filepath.Join(t.TempDir(), "alias")
    if err := os.Symlink(root, alias); err != nil {
        t.Skipf("symlink: %v", err)
    }

    got, err := repositoryRootAt(context.Background(), alias, func(context.Context, string) ([]byte, []byte, error) {
        return []byte(root + "\n"), nil, nil
    })
    if err != nil {
        t.Fatal(err)
    }
    want, err := filepath.EvalSymlinks(root)
    if err != nil {
        t.Fatal(err)
    }
    if got != want {
        t.Fatalf("repositoryRootAt() = %q, want %q", got, want)
    }
}

func TestRepositoryRootAtFallsBackOnlyForExplicitNonRepository(t *testing.T) {
    cwd := t.TempDir()
    got, err := repositoryRootAt(context.Background(), cwd, func(context.Context, string) ([]byte, []byte, error) {
        return nil, []byte("fatal: not a git repository (or any of the parent directories): .git\n"), errors.New("exit status 128")
    })
    if err != nil {
        t.Fatal(err)
    }
    want, err := filepath.EvalSymlinks(cwd)
    if err != nil {
        t.Fatal(err)
    }
    if got != want {
        t.Fatalf("fallback root = %q, want %q", got, want)
    }
}

func TestRepositoryRootAtPropagatesNonRepositoryFailures(t *testing.T) {
    cases := []struct {
        name   string
        stdout []byte
        stderr []byte
        err    error
    }{
        {name: "missing git", err: exec.ErrNotFound},
        {name: "permission failure", stderr: []byte("fatal: permission denied\n"), err: errors.New("exit status 128")},
        {name: "malformed output", stdout: []byte("one\ntwo\n")},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := repositoryRootAt(context.Background(), t.TempDir(), func(context.Context, string) ([]byte, []byte, error) {
                return tc.stdout, tc.stderr, tc.err
            })
            if err == nil {
                t.Fatal("repositoryRootAt() error = nil")
            }
        })
    }
}
~~~

- [ ] **Step 2: Run focused tests and verify failure**

Run:

~~~sh
go test ./cmd/tbboot -run 'TestRepositoryRootAt' -count=1
~~~

Expected: FAIL because repositoryRootAt does not exist and the current implementation treats every Git error as a non-repository fallback.

- [ ] **Step 3: Implement canonical root discovery**

Replace the unconditional fallback with a runner seam and canonicalization helper:

~~~go
type gitRunner func(context.Context, string) ([]byte, []byte, error)

func repositoryRoot(ctx context.Context) (string, error) {
    cwd, err := os.Getwd()
    if err != nil {
        return "", err
    }
    return repositoryRootAt(ctx, cwd, runGit)
}

func runGit(ctx context.Context, cwd string) ([]byte, []byte, error) {
    cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
    cmd.Dir = cwd
    cmd.Env = stableGitEnvironment(os.Environ())
    stdout, err := cmd.Output()
    if err == nil {
        return stdout, nil, nil
    }
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        return nil, exitErr.Stderr, err
    }
    return nil, nil, err
}

func stableGitEnvironment(environment []string) []string {
    filtered := make([]string, 0, len(environment)+1)
    for _, value := range environment {
        if !strings.HasPrefix(value, "LC_ALL=") {
            filtered = append(filtered, value)
        }
    }
    return append(filtered, "LC_ALL=C")
}

func repositoryRootAt(ctx context.Context, cwd string, run gitRunner) (string, error) {
    stdout, stderr, err := run(ctx, cwd)
    if err != nil {
        if !strings.Contains(strings.ToLower(string(stderr)), "not a git repository") {
            return "", fmt.Errorf("discover repository root: %w", err)
        }
        return canonicalPath(cwd)
    }
    root := strings.TrimSpace(string(stdout))
    if root == "" || strings.ContainsAny(root, "\r\n") {
        return "", fmt.Errorf("git returned malformed repository root")
    }
    return canonicalPath(root)
}

func canonicalPath(value string) (string, error) {
    absolute, err := filepath.Abs(value)
    if err != nil {
        return "", err
    }
    return filepath.EvalSymlinks(filepath.Clean(absolute))
}
~~~

Use cmd.Output so ExitError.Stderr remains available for explicit non-repository classification. Do not treat empty successful stdout, missing Git, permission errors, or malformed output as fallback. The locale-setting helper must preserve all unrelated environment entries and replace any inherited `LC_ALL` rather than appending a duplicate.

- [ ] **Step 4: Run focused tests and verify pass**

Run:

~~~sh
go test ./cmd/tbboot -run 'TestRepositoryRootAt' -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Commit checkpoint**

Do not run git commit without explicit user approval at that moment. If approved:

~~~sh
git add cmd/tbboot/install.go cmd/tbboot/root_test.go
git commit -m "fix: canonicalize operation root discovery"
~~~

### Task 2: Add the exclusive Operation Root lock and result contracts

**Files:**

- Create: internal/install/operation_lock.go
- Create: internal/install/operation_lock_test.go
- Modify: internal/install/service.go
- Modify: internal/install/sync.go
- Test: internal/install/service_test.go
- Test: internal/install/sync_test.go

**Interfaces:**

- Consumes: canonical root strings from Task 1's CLI boundary and existing Service.Install/Service.Sync entrypoints.
- Produces: acquireOperationLock(string) (func() error, error), Request.DryRun, SyncRequest.DryRun, OutcomePlanned, Result.DryRun, and conflict results carrying Dry Run state.

- [ ] **Step 1: Add failing lock and contract tests**

Create operation_lock_test.go with direct filesystem assertions:

~~~go
func TestAcquireOperationLockRejectsExistingPathAndReleases(t *testing.T) {
    root := t.TempDir()
    release, err := acquireOperationLock(root)
    if err != nil {
        t.Fatal(err)
    }
    if _, err := os.Stat(filepath.Join(root, operationLockName)); err != nil {
        t.Fatal(err)
    }
    if _, err := acquireOperationLock(root); err == nil {
        t.Fatal("second acquire succeeded")
    }
    if err := release(); err != nil {
        t.Fatal(err)
    }
    if _, err := os.Stat(filepath.Join(root, operationLockName)); !os.IsNotExist(err) {
        t.Fatalf("lock after release = %v, want not exist", err)
    }
}

func TestAcquireOperationLockDoesNotTakeOverFile(t *testing.T) {
    root := t.TempDir()
    path := filepath.Join(root, operationLockName)
    if err := os.WriteFile(path, []byte("owned"), 0o600); err != nil {
        t.Fatal(err)
    }
    if _, err := acquireOperationLock(root); err == nil {
        t.Fatal("acquire succeeded over existing file")
    }
    data, err := os.ReadFile(path)
    if err != nil || string(data) != "owned" {
        t.Fatalf("existing lock content = %q, %v", data, err)
    }
}
~~~

Add service tests that place .tbboot-operation.lock before Install/Sync and assert the source resolver and state store are not reached. Add a release test that makes the lock directory non-empty and asserts the release closure returns the removal error. Add request/result assertions for DryRun: true, OutcomePlanned, and resultForConflicts(..., true).DryRun.

- [ ] **Step 2: Run focused tests and verify failure**

Run:

~~~sh
go test ./internal/install -run 'TestAcquireOperationLock|Test.*DryRun|Test.*Lock' -count=1
~~~

Expected: FAIL because the lock helper and new request/result fields do not exist.

- [ ] **Step 3: Implement the minimal lock and public contracts**

Create internal/install/operation_lock.go:

~~~go
package install

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
)

const operationLockName = ".tbboot-operation.lock"

func acquireOperationLock(root string) (func() error, error) {
    path := filepath.Join(root, operationLockName)
    if err := os.Mkdir(path, 0o700); err != nil {
        if errors.Is(err, os.ErrExist) {
            return nil, fmt.Errorf("operation root is already locked")
        }
        return nil, err
    }
    return func() error { return os.Remove(path) }, nil
}
~~~

Update internal/install/service.go:

~~~go
type Request struct {
    Root        string
    Source      source.Ref
    Artifact    string
    DeclareOnly bool
    DryRun      bool
}

type SyncRequest struct {
    Root   string
    DryRun bool
}

const (
    OutcomeNoOp     Outcome = "no_op"
    OutcomeApplied  Outcome = "applied"
    OutcomePlanned  Outcome = "planned"
    OutcomeConflict Outcome = "conflict"
)

type Result struct {
    Operation     string     @@json:"operation"@@
    Outcome       Outcome    @@json:"outcome"@@
    DryRun        bool       @@json:"dry_run"@@
    ArtifactCount int        @@json:"artifact_count"@@
    Changes       []Change   @@json:"changes,omitempty"@@
    Conflicts     []Conflict @@json:"conflicts,omitempty"@@
}
~~~

Change resultForConflicts to accept dryRun bool and return that value in Result. The operation methods use the lock only when DryRun is false; the full lifecycle branch is added in Task 5 after the Dry Run persistence path exists. Any method that acquires the lock must use named returns or an equivalent finalization path so a release error replaces a nil operation error with `release operation lock: ...`.

- [ ] **Step 4: Run focused tests and verify pass**

Run:

~~~sh
go test ./internal/install -run 'TestAcquireOperationLock|Test.*DryRun|Test.*Lock' -count=1
~~~

Expected: PASS for lock creation/rejection/release and contract-level assertions. Existing package tests must also compile with updated resultForConflicts calls.

- [ ] **Step 5: Commit checkpoint**

Do not run git commit without explicit user approval at that moment. If approved:

~~~sh
git add internal/install/operation_lock.go internal/install/operation_lock_test.go internal/install/service.go internal/install/sync.go internal/install/service_test.go internal/install/sync_test.go
git commit -m "fix: serialize operations per root"
~~~

### Task 3: Harden confined target observation and atomic replacement

**Files:**

- Modify: internal/materialize/service.go
- Create: internal/materialize/reparse_windows.go
- Create: internal/materialize/reparse_other.go
- Modify: internal/materialize/service_test.go
- Create: internal/materialize/service_windows_test.go

**Interfaces:**

- Consumes: root-relative target paths and existing os.Root replacement flow.
- Produces: Observation values carrying target identity, SameEntryIdentity(Observation, Observation) bool, SamePathIdentity(Observation, string) (bool, error), and typed ChangedSincePreflightError behavior for root/parent/target races.

- [ ] **Step 1: Add failing topology and identity tests**

Extend service_test.go:

~~~go
func TestObserveCapturesHardLinkIdentity(t *testing.T) {
    root := t.TempDir()
    first := filepath.Join(root, "first")
    second := filepath.Join(root, "second")
    if err := os.WriteFile(first, []byte("same"), 0o644); err != nil {
        t.Fatal(err)
    }
    if err := os.Link(first, second); err != nil {
        t.Skipf("hard link: %v", err)
    }
    a, err := Observe(root, "first")
    if err != nil {
        t.Fatal(err)
    }
    b, err := Observe(root, "second")
    if err != nil {
        t.Fatal(err)
    }
    if !SameEntryIdentity(a, b) {
        t.Fatal("hard-link observations have different identities")
    }
}

func TestObserveRejectsSpecialTargetParent(t *testing.T) {
    root := t.TempDir()
    parent := filepath.Join(root, "parent")
    if err := os.WriteFile(parent, []byte("not a directory"), 0o644); err != nil {
        t.Fatal(err)
    }
    if _, err := Observe(root, "parent/file"); err == nil || err.Error() != "target parent must be a real directory" {
        t.Fatalf("Observe() error = %v, want real-directory rejection", err)
    }
}
~~~

Keep existing tests for parent symlinks, final non-regular targets, mode preservation, same-directory temporary files, root replacement, and parent races. Add a Windows-tagged test that creates a symlink/reparse-point parent when the host permits it and asserts the same rejection, including the `FILE_ATTRIBUTE_REPARSE_POINT` path. Skip only when the platform refuses test-link creation.

- [ ] **Step 2: Run focused tests and verify failure**

Run:

~~~sh
go test ./internal/materialize -run 'TestObserveCapturesHardLinkIdentity|TestObserveRejectsSpecialTargetParent' -count=1
~~~

Expected: FAIL because observations do not retain target identity and no public identity comparison exists.

- [ ] **Step 3: Implement confined observation and identity comparison**

Extend Observation with private identity data and use a root handle for every target inspection:

~~~go
type Observation struct {
    Root         string
    Path         string
    AbsolutePath string
    Kind         EntryKind
    Mode         os.FileMode
    Digest       string
    rootInfo     os.FileInfo
    targetInfo   os.FileInfo
}

func SameEntryIdentity(a, b Observation) bool {
    return a.targetInfo != nil && b.targetInfo != nil && os.SameFile(a.targetInfo, b.targetInfo)
}

func SamePathIdentity(observed Observation, path string) (bool, error) {
    if observed.targetInfo == nil {
        return false, nil
    }
    info, err := os.Lstat(path)
    if err != nil {
        return false, err
    }
    return os.SameFile(observed.targetInfo, info), nil
}
~~~

In Observe, keep portable relative-path rejection, canonicalize the root once, open it with os.OpenRoot, inspect existing parent components with root.Lstat, and reject any parent that is not a real directory or has os.ModeSymlink/os.ModeIrregular or the Windows `FILE_ATTRIBUTE_REPARSE_POINT` bit. Capture targetInfo from root.Lstat; classify a final symlink as EntrySymlink, classify any other reparse point or special entry as EntryOther, and read only regular bytes through root.ReadFile. Missing parent components remain creatable by the existing writer.

Implement `isWindowsReparsePoint(os.FileInfo) bool` in `reparse_windows.go` and `reparse_other.go`. On Windows, inspect `info.Sys().(*syscall.Win32FileAttributeData).FileAttributes` for `syscall.FILE_ATTRIBUTE_REPARSE_POINT`; do not rely on `os.ModeIrregular`, because Go intentionally treats `IO_REPARSE_TAG_DEDUP` as regular. On other platforms, the predicate returns false.

In revalidateRoot, compare the opened root's root.Stat(".") with the observed root identity, repeat parent topology checks using the platform predicate, capture current target identity, and compare target kind, mode, digest, path, and root identity. In writeRooted, keep root.MkdirAll, open the target parent through the same os.Root, create a unique 0600 temporary file in that directory, write and chmod the complete payload, revalidate root/target, compare root.Lstat(parent) with dir.Stat(".") using os.SameFile, then call dir.Rename(tmp, base). Return ChangedSincePreflightError for topology or identity drift. Preserve final symlink replacement at this low-level atomic seam; install preflight rejects unsafe final entries before it calls Write. `Write` is not the install policy gate and has no production caller outside `internal/install`.

Add this comment at the caller in Task 5 if preflight remains pairwise:

~~~go
// ponytail: identity comparisons scan existing planned paths linearly;
// replace with an indexed identity map only if large artifact sets make this measurable.
~~~

- [ ] **Step 4: Run focused tests and verify pass**

Run:

~~~sh
go test ./internal/materialize -count=1
~~~

Expected: PASS, including existing atomic replacement and race tests plus new identity/topology tests.

- [ ] **Step 5: Commit checkpoint**

Do not run git commit without explicit user approval at that moment. If approved:

~~~sh
git add internal/materialize/service.go internal/materialize/service_test.go internal/materialize/service_windows_test.go
git commit -m "fix: revalidate filesystem identities before replacement"
~~~

### Task 4: Reject unsafe consumed Source inputs

**Files:**

- Modify: internal/source/file/source.go
- Create: internal/source/file/reparse_windows.go
- Create: internal/source/file/reparse_other.go
- Modify: internal/source/file/source_test.go

**Interfaces:**

- Consumes: canonical source locators, tbboot-source.yaml, tbboot-artifact.yaml, and whole-file file steps.
- Produces: source.Source.Resolve that reads descriptors and step inputs through an opened os.Root only after each consumed component is verified as a real directory or regular file; ResolvedSource.InputPaths remains the existing absolute-path list used by install preflight.

- [ ] **Step 1: Add failing source-topology tests**

Add tests that create links and special inputs inside a valid fixture:

~~~go
func TestResolveRejectsSymlinkedInputEvenWhenItStaysInsideRoot(t *testing.T) {
    root := fixture(t)
    input := filepath.Join(root, "a", "in")
    if err := os.Remove(input); err != nil {
        t.Fatal(err)
    }
    if err := os.Symlink("other", input); err != nil {
        t.Skipf("symlink: %v", err)
    }
    write(t, filepath.Join(root, "a", "other"), "captured\n")
    if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Type: "file", Locator: root}}); err == nil {
        t.Fatal("Resolve() accepted symlinked source input")
    }
}

func TestResolveRejectsSymlinkedArtifactDirectoryInsideRoot(t *testing.T) {
    root, outside := t.TempDir(), t.TempDir()
    write(t, filepath.Join(outside, "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps:\n  - type: file\n    path: out\n    source: in\n")
    write(t, filepath.Join(outside, "in"), "captured\n")
    write(t, filepath.Join(root, "tbboot-source.yaml"), "schema_version: 1\nartifacts:\n  - name: a\n    path: a\n")
    if err := os.Symlink(outside, filepath.Join(root, "a")); err != nil {
        t.Skipf("symlink: %v", err)
    }
    if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Type: "file", Locator: root}}); err == nil {
        t.Fatal("Resolve() accepted symlinked artifact directory")
    }
}
~~~

Add a Unix-only FIFO test with os.Mkfifo; use a build-tagged test file when the platform cannot compile that call. Add Windows coverage proving a reparse-point input is rejected through the `FILE_ATTRIBUTE_REPARSE_POINT` predicate, not only through ModeIrregular. Keep existing escape, descriptor, snapshot, and descriptor-validation tests.

- [ ] **Step 2: Run focused tests and verify failure**

Run:

~~~sh
go test ./internal/source/file -run 'TestResolveRejectsSymlinked|TestResolveRejectsSpecial' -count=1
~~~

Expected: FAIL because current EvalSymlinks-based reads accept links that resolve inside the source root and can follow a changed path between validation and read.

- [ ] **Step 3: Implement rooted real-path reads**

Replace canonicalContained-then-os.ReadFile calls with small rooted helpers. Keep canonicalExistingDir only for obtaining the canonical source root. Implement the package-local `isWindowsReparsePoint(os.FileInfo) bool` in `reparse_windows.go` and `reparse_other.go`, matching materialize so source reads reject the same Windows entries:

~~~go
func realDir(root *os.Root, relative string) error {
    info, err := root.Lstat(filepath.FromSlash(relative))
    if err != nil {
        return err
    }
    if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || isWindowsReparsePoint(info) || !info.IsDir() {
        return fmt.Errorf("source path must be a real directory")
    }
    return nil
}

func readRealFile(root *os.Root, relative string) ([]byte, error) {
    path := filepath.FromSlash(relative)
    info, err := root.Lstat(path)
    if err != nil {
        return nil, err
    }
    if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || isWindowsReparsePoint(info) || !info.Mode().IsRegular() {
        return nil, fmt.Errorf("source input must be a regular file")
    }
    return root.ReadFile(path)
}
~~~

Before realDir/readRealFile, walk each existing parent component with root.Lstat; reject symlinks, Windows reparse points detected through `FILE_ATTRIBUTE_REPARSE_POINT`, and non-directories. Open the canonical source root with os.OpenRoot. Read the source descriptor relative to that root, open each artifact directory with root.OpenRoot, read each artifact descriptor and file-step source through readRealFile, and close roots with defer. Build InputPaths from the canonical root plus validated relative paths exactly as before so source-input overlap checks remain available. Do not add a general filesystem interface.

- [ ] **Step 4: Run focused tests and verify pass**

Run:

~~~sh
go test ./internal/source/file -count=1
~~~

Expected: PASS, with symlink/special/escape inputs rejected before bytes are consumed and existing snapshot behavior unchanged.

- [ ] **Step 5: Commit checkpoint**

Do not run git commit without explicit user approval at that moment. If approved:

~~~sh
git add internal/source/file/source.go internal/source/file/source_test.go
git commit -m "fix: reject unsafe source inputs"
~~~

### Task 5: Thread Dry Run through install and Sync preflight

**Files:**

- Modify: internal/install/service.go
- Modify: internal/install/sync.go
- Test: internal/install/service_test.go
- Test: internal/install/sync_test.go
- Modify: internal/install/service_windows_test.go

**Interfaces:**

- Consumes: acquireOperationLock, materialize.SameEntryIdentity, and materialize.SamePathIdentity from Tasks 2–4.
- Produces: Service.Install and Service.Sync that lock only mutating operations, perform all reads/resolution/preflight before writes, return OutcomePlanned for Dry Run changes, and return OutcomeNoOp only when no changes exist.

- [ ] **Step 1: Add failing service Dry Run and collision tests**

Add tests proving every mutation class stays untouched:

~~~go
func TestInstallDryRunWritesNothingAndDoesNotCreateLock(t *testing.T) {
    root := t.TempDir()
    if err := os.Mkdir(filepath.Join(root, "source"), 0o755); err != nil {
        t.Fatal(err)
    }
    service, _ := testService(testResolved(testArtifact("a", "a")))

    result, err := service.Install(context.Background(), Request{
        Root:   root,
        Source: source.Ref{Type: "file", Locator: "./source"},
        DryRun: true,
    })
    if err != nil {
        t.Fatal(err)
    }
    if result.Outcome != OutcomePlanned || !result.DryRun {
        t.Fatalf("result = %#v, want planned dry run", result)
    }
    for _, name := range []string{
        repositorystate.ManifestFileName,
        repositorystate.LockfileFileName,
        repositorystate.MaterializationRecordFileName,
        operationLockName,
    } {
        if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
            t.Fatalf("%s exists after dry run: %v", name, err)
        }
    }
    if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
        t.Fatalf("target exists after dry run: %v", err)
    }
}

func TestPreflightRejectsHardLinkAliasTargets(t *testing.T) {
    root := t.TempDir()
    first := filepath.Join(root, "first")
    second := filepath.Join(root, "second")
    if err := os.WriteFile(first, []byte("same"), 0o644); err != nil {
        t.Fatal(err)
    }
    if err := os.Link(first, second); err != nil {
        t.Skipf("hard link: %v", err)
    }
    desired := []desiredArtifact{
        {Key: repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source-a"}, Name: "a"}, Descriptor: testArtifact("a", "first")},
        {Key: repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source-b"}, Name: "b"}, Descriptor: testArtifact("b", "second")},
    }
    desired[0].Descriptor.Steps[0].SourceBytes = []byte("same")
    desired[1].Descriptor.Steps[0].SourceBytes = []byte("same")
    _, conflicts, err := preflightFiles(root, desired, repositorystate.MaterializationRecord{})
    if err != nil {
        t.Fatal(err)
    }
    if len(conflicts) == 0 || conflicts[0].Kind != ConflictOwnership {
        t.Fatalf("conflicts = %#v, want hard-link ownership conflict", conflicts)
    }
}
~~~

Add Sync Dry Run tests starting from a real manifest and assert target, lockfile, managed record, and lock-directory bytes/state remain unchanged. Add declaration-only Dry Run coverage proving no manifest is created. Add conflict Dry Run coverage proving UserActionError.Result.DryRun is true and no state changes occur.

- [ ] **Step 2: Run focused tests and verify failure**

Run:

~~~sh
go test ./internal/install -run 'TestInstallDryRun|TestPreflightRejectsHardLink|TestSync.*DryRun' -count=1
~~~

Expected: FAIL because current operations always write state/files and preflight compares only path keys.

- [ ] **Step 3: Implement lock lifecycle, Dry Run planning, and identity preflight**

In both Install and Sync, acquire the lock immediately after required-root validation and before any state load or source resolution, guarded by DryRun. Use a release closure returning `error`; finalize the named operation result after all normal work so a failed removal is returned as `release operation lock: ...`.

~~~go
var release func() error
if !request.DryRun {
    release, err = acquireOperationLock(request.Root)
    if err != nil {
        return Result{}, err
    }
    defer func() {
        if releaseErr := release(); releaseErr != nil {
            err = errors.Join(err, fmt.Errorf("release operation lock: %w", releaseErr))
        }
    }()
}
~~~

Use request.DryRun in Sync; keep request-field validation before lock acquisition because it does not read repository state or resolve a source. Ensure every early return after acquisition is covered by the deferred release.

Change explicit declaration-only handling to return OutcomePlanned and DryRun true with ChangeDeclarationAdded when the in-memory manifest would change, and skip WriteManifest. Preserve the existing no-op shortcut for an unchanged declaration, returning OutcomeNoOp with DryRun true.

Change persistPrepared and applyPrepared to accept dryRun bool. applyPrepared must still derive the would-be managed record and change list, but call materialize.Write only when dryRun is false. persistPrepared must use this branch before any state writer:

~~~go
if dryRun {
    if len(changes) == 0 {
        return Result{Operation: operation, Outcome: OutcomeNoOp, DryRun: true, ArtifactCount: len(prepared.Desired)}, nil
    }
    return Result{Operation: operation, Outcome: OutcomePlanned, DryRun: true, ArtifactCount: len(prepared.Desired), Changes: changes}, nil
}
~~~

Run adoption revalidation only on the mutating path immediately before materialization/state persistence. Keep existing atomic writers and cleanup behavior for non-Dry Run operations.

Extend preflightFiles after path-key checks. The nested identity loop is deliberate O(n²); keep the ponytail ceiling comment and replace it with an indexed identity map only after large artifact sets make the cost measurable:

~~~go
for _, prior := range files {
    if prior.Observed.Path != observed.Path && materialize.SameEntryIdentity(prior.Observed, observed) {
        conflicts = append(conflicts, Conflict{Kind: ConflictOwnership, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
        continue
    }
}
for _, input := range artifact.InputPaths {
    same, err := materialize.SamePathIdentity(observed, input)
    if err != nil {
        return nil, nil, err
    }
    if same {
        return nil, nil, fmt.Errorf("target %q overlaps source input", step.TargetPath)
    }
}
~~~

Also compare each observed target against managed record paths through SamePathIdentity when the path key differs, so a hard link to an existing managed file is an ownership conflict. Preserve PathKey for exact path and Windows case aliases; do not replace it with unconditional case folding on case-sensitive platforms. Keep existing symlink/special final-target classification as conflicts and parent topology rejection from materialize.Observe. This preflight classification is the install policy boundary; the confined low-level writer may retain its existing final-symlink replacement test.

- [ ] **Step 4: Run focused tests and verify pass**

Run:

~~~sh
go test ./internal/install -count=1
~~~

Expected: PASS, including existing install/Sync behavior, lock cleanup, Dry Run no-write assertions, hard-link conflicts, and typed race conflicts.

- [ ] **Step 5: Commit checkpoint**

Do not run git commit without explicit user approval at that moment. If approved:

~~~sh
git add internal/install/service.go internal/install/sync.go internal/install/service_test.go internal/install/sync_test.go internal/install/service_windows_test.go
git commit -m "feat: add non-mutating install dry runs"
~~~

### Task 6: Expose Dry Run and planned output at the CLI boundary

**Files:**

- Modify: cmd/tbboot/install.go
- Modify: cmd/tbboot/root_test.go

**Interfaces:**

- Consumes: Request.DryRun, SyncRequest.DryRun, OutcomePlanned, and Result.DryRun from Tasks 2 and 5.
- Produces: tbboot install --dry-run for explicit, declaration-only, and targetless reconciliation modes; JSON operation details always contain dry_run.

- [ ] **Step 1: Add failing CLI output and no-write tests**

Add a test around an initialized Git workspace and fixture Source:

~~~go
func TestInstallDryRunReportsPlannedWithoutWriting(t *testing.T) {
    root := t.TempDir()
    sourceRoot := filepath.Join(root, "source")
    writeInstallFixture(t, sourceRoot)
    initGitRepo(t, root)
    before := stateFiles(t, root)

    withDir(t, root, func() {
        var stdout bytes.Buffer
        if code := execute(context.Background(), []string{"install", "--dry-run", "file:" + sourceRoot}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
            t.Fatalf("exit code = %d, want 0", code)
        }
        if !strings.Contains(stdout.String(), "install: planned") {
            t.Fatalf("stdout = %q, want planned label", stdout.String())
        }
        if got := stateFiles(t, root); !reflect.DeepEqual(got, before) {
            t.Fatal("dry run changed repository state")
        }
        if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
            t.Fatalf("target after dry run = %v, want absent", err)
        }
    })
}
~~~

Add JSON assertions for explicit and bare install --dry-run:

~~~go
var envelope app.Result
if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
    t.Fatal(err)
}
if envelope.Details["dry_run"] != true || envelope.Details["outcome"] != "planned" {
    t.Fatalf("details = %#v, want dry_run=true and outcome=planned", envelope.Details)
}
~~~

Add declaration-only Dry Run coverage proving no manifest is created, and a no-op Dry Run assertion proving the JSON outcome stays no_op while dry_run is true and changes remains omitted.

- [ ] **Step 2: Run focused tests and verify failure**

Run:

~~~sh
go test ./cmd/tbboot -run 'TestInstallDryRun' -count=1
~~~

Expected: FAIL because the flag is not defined, requests do not receive Dry Run, and human/JSON rendering has no planned branch or dry_run field.

- [ ] **Step 3: Implement the CLI flag and output mapping**

Add the flag and pass it in both command paths:

~~~go
var dryRun bool

// bare install
result, err := service.Sync(ctx, installsvc.SyncRequest{Root: root, DryRun: dryRun})

// explicit install
result, err := service.Install(ctx, installsvc.Request{
    Root:        root,
    Source:      ref,
    Artifact:    artifact,
    DeclareOnly: declareOnly,
    DryRun:      dryRun,
})

cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report changes without writing files or state")
~~~

Always include the field in operation JSON details:

~~~go
details := map[string]any{
    "operation":      result.Operation,
    "outcome":        result.Outcome,
    "dry_run":        result.DryRun,
    "artifact_count": result.ArtifactCount,
}
~~~

Keep change/conflict omission rules unchanged. Change only the applied/planned label in writeResult:

~~~go
label := "applied"
if result.Outcome == installsvc.OutcomePlanned {
    label = "planned"
}
_, err := fmt.Fprintf(stdout, "%s: %s %d changes (%d artifacts)\n", result.Operation, label, len(result.Changes), result.ArtifactCount)
~~~

Retain no changes output for OutcomeNoOp, including Dry Run no-ops. Do not add a separate sync command or alter error exit-code mapping.

- [ ] **Step 4: Run focused tests and verify pass**

Run:

~~~sh
go test ./cmd/tbboot -count=1
~~~

Expected: PASS, including existing human/JSON output, explicit install, declaration-only, targetless reconciliation, and new Dry Run assertions.

- [ ] **Step 5: Commit checkpoint**

Do not run git commit without explicit user approval at that moment. If approved:

~~~sh
git add cmd/tbboot/install.go cmd/tbboot/root_test.go
git commit -m "feat: expose install dry run output"
~~~

### Task 7: Cross-platform verification and final safety gate

**Files:**

- Verify: all files listed in Tasks 1–6
- Verify: internal/repositorystate/store.go remains on existing atomic same-directory writers

**Interfaces:**

- Consumes: completed root, lock, source, materialization, install, and CLI behavior from Tasks 1–6.
- Produces: passing repository checks, clean pending-diff validation, and a Windows test-binary compilation without executing a foreign binary.

- [ ] **Step 1: Run focused package checks after formatting**

Run:

~~~sh
just fmt-go
go test ./internal/materialize ./internal/source/file ./internal/install ./cmd/tbboot
~~~

Expected: formatting changes only intended Go files; focused tests pass.

- [ ] **Step 2: Compile the CLI test binary for Windows**

Run without executing the Windows binary:

~~~sh
cross_dir="$(mktemp -d)"
trap 'rm -rf "$cross_dir"' EXIT
GOOS=windows go test -c -o "$cross_dir/tbboot.test.exe" ./cmd/tbboot
~~~

Expected: command exits 0 and creates a temporary Windows test binary; the trap removes it.

- [ ] **Step 3: Run repository checks**

Run:

~~~sh
just check
git diff --check HEAD
git diff --check <base>...HEAD
~~~

Expected: just check passes Markdown and Go checks; both diff checks print no output. Replace `<base>` with the actual pull-request base branch before running the range check.

- [ ] **Step 4: Self-review every requirement against the implementation**

Check each item directly:

- repositoryRoot canonicalizes Git output and only falls back for explicit non-repository stderr.
- runGit supplies one `LC_ALL=C` entry, preserving unrelated environment entries, before classifying Git stderr.
- Mutating explicit and bare installs acquire and release .tbboot-operation.lock around all state reads and source resolution.
- Lock-release errors are returned, including when the operation already has another error.
- Dry Run creates no lock and no target, manifest, lockfile, or materialization-record writes.
- Source descriptors, artifact descriptors, source inputs, target parents, reserved state paths, symlink/reparse/special entries, traversal escapes, hard-link aliases, and Windows case aliases are rejected before mutation; Windows reparse detection checks `FILE_ATTRIBUTE_REPARSE_POINT`, not only `ModeIrregular`.
- Existing modes survive replacement; new files are 0644.
- Atomic replacement uses a unique same-directory temporary and confined rename.
- Root, parent, target, and opened-directory identities are checked immediately before rename, with drift returned as UserActionError conflict data.
- Final-symlink replacement remains confined to the low-level writer; install preflight remains the policy boundary.
- Human planned output says planned; JSON operation details always include dry_run.
- Existing repository-state atomic writers remain the only state persistence mechanism.
- Scope boundaries remain unchanged: no Git acquisition, stale-lock takeover, rollback lifecycle, new step types, or broad filesystem interface.

Run a literal placeholder-pattern scan over this plan; expected: no placeholder markers.

- [ ] **Step 5: Final handoff without an unapproved commit**

Report exact verification output and current git status --short. Do not create a commit in this task. If the user later approves one explicitly, use a short conventional subject scoped to the actual implementation.
