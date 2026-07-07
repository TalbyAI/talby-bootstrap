# Declare-only install implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `tbboot install --declare-only` so it resolves a single artifact, writes manifest intent to `talby-artifacts.yaml` at the repository root, never writes `talby-artifacts.lock.yaml`, and rejects declaration replacement that belongs to `upgrade`.

**Architecture:** Keep `cmd/tbboot` thin. Extend `internal/install.Service` with a small declare-only branch that reuses source resolution plus `ManifestDeclaration(req, result)`, loads the manifest through `internal/repositorystate.Store`, treats `manifest not_found` as bootstrap, and uses the existing `Manifest.UpsertDeclaration` result to distinguish `declared`, `noop`, and conflict. Validate `Request.Root` when `DeclareOnly` is enabled so manifest writes always target the repository root, not the source locator or ambient working directory. Keep conflict as an error and map it to exit code `2` at the CLI root in both human and JSON output.

**Tech Stack:** Go 1.26.4, Cobra, `gopkg.in/yaml.v3`, `gofmt`, `go test`, `just`.

---

## Constraints

- Do not add a new declaration module.
- Do not write `talby-artifacts.lock.yaml`.
- Do not implement sync, upgrade, trust enforcement, or materialization.
- Do not create commits from an agent. `AGENTS.md` requires explicit user approval, so use review checkpoints instead of commit steps.

## File structure

- Modify: `internal/install/service.go` - add `DeclareOnly` request handling, repository root input, typed change result, manifest bootstrap/load/write path, and typed conflict error.
- Modify: `internal/install/service_test.go` - add declare-only service coverage for separate repository/source roots, bootstrap, root validation, no-op, conflict, and no-lockfile behavior.
- Modify: `cmd/tbboot/install.go` - add `--declare-only`, require `<source>` in that mode, resolve and pass the repository root to the service, and render declare-only human/JSON success output.
- Modify: `cmd/tbboot/root.go` - map declare-only conflict errors to exit code `2` in both human and JSON output paths.
- Modify: `cmd/tbboot/root_test.go` - cover declare-only success, JSON output, missing-source validation, and conflict exit-path behavior for both human and JSON output.

No new package is needed. `internal/install/repository_state.go` already has the declaration mapper, and `internal/repositorystate.Manifest.UpsertDeclaration` already gives the three-way branch needed for this slice.

## Tasks

### Task 1: Add the declare-only branch to the install service

**Files:**

- Modify: `internal/install/service.go`
- Modify: `internal/install/service_test.go`

- [ ] **Step 1: Write the failing declare-only service tests**

```go
func TestDeclareOnlyInstallWritesManifestAtRepositoryRoot(t *testing.T) {
 repoRoot := t.TempDir()
 sourceRoot := t.TempDir()
 writeInstallFixture(t, sourceRoot)

 svc := NewService(
  source.NewStaticRegistry(map[string]source.Source{
   "file": file.New(),
  }),
  repositorystate.NewStore(),
 )

 got, err := svc.Install(context.Background(), Request{
  Root:        repoRoot,
  Source:      source.Ref{Type: "file", Locator: sourceRoot},
  Artifact:    "base-readme",
  DeclareOnly: true,
 })
 if err != nil {
  t.Fatalf("Install() error = %v", err)
 }
 if got.Change != ChangeDeclared {
  t.Fatalf("Change = %q, want %q", got.Change, ChangeDeclared)
 }

 manifest, err := repositorystate.NewStore().LoadManifest(context.Background(), repoRoot)
 if err != nil {
  t.Fatalf("LoadManifest() error = %v", err)
 }
 if len(manifest.Declarations) != 1 {
  t.Fatalf("len(Declarations) = %d, want 1", len(manifest.Declarations))
 }
 if manifest.Declarations[0].Input == nil || manifest.Declarations[0].Input.Locator != sourceRoot {
  t.Fatalf("declaration input = %#v, want locator %q", manifest.Declarations[0].Input, sourceRoot)
 }
 if _, err := os.Stat(filepath.Join(sourceRoot, repositorystate.ManifestFileName)); !errors.Is(err, os.ErrNotExist) {
  t.Fatalf("source root manifest state error = %v, want not exist", err)
 }
}

func TestDeclareOnlyInstallBootstrapsManifestAndWritesDeclaration(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)

 svc := NewService(
  source.NewStaticRegistry(map[string]source.Source{
   "file": file.New(),
  }),
  repositorystate.NewStore(),
 )

 got, err := svc.Install(context.Background(), Request{
  Root:        repoRoot,
  Source:      source.Ref{Type: "file", Locator: repoRoot},
  Artifact:    "base-readme",
  DeclareOnly: true,
 })
 if err != nil {
  t.Fatalf("Install() error = %v", err)
 }
 if got.Change != ChangeDeclared {
  t.Fatalf("Change = %q, want %q", got.Change, ChangeDeclared)
 }

 manifest, err := repositorystate.NewStore().LoadManifest(context.Background(), repoRoot)
 if err != nil {
  t.Fatalf("LoadManifest() error = %v", err)
 }
 if len(manifest.Declarations) != 1 {
  t.Fatalf("len(Declarations) = %d, want 1", len(manifest.Declarations))
 }
 if manifest.Declarations[0].Input == nil || manifest.Declarations[0].Input.Locator != repoRoot {
  t.Fatalf("declaration input = %#v, want locator %q", manifest.Declarations[0].Input, repoRoot)
 }
}

func TestDeclareOnlyInstallRejectsEmptyRoot(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)

 svc := NewService(
  source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
  repositorystate.NewStore(),
 )
 _, err := svc.Install(context.Background(), Request{
  Source:      source.Ref{Type: "file", Locator: repoRoot},
  Artifact:    "base-readme",
  DeclareOnly: true,
 })
 if err == nil {
  t.Fatal("Install() error = nil, want missing root error")
 }
 if got, want := err.Error(), "repository root is required for declare-only install"; got != want {
  t.Fatalf("Install() error = %q, want %q", got, want)
 }
}

func TestDeclareOnlyInstallReturnsNoOpForEquivalentDeclaration(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)

 store := repositorystate.NewStore()
 if err := store.WriteManifest(context.Background(), repoRoot, repositorystate.Manifest{
  Declarations: []repositorystate.Declaration{
   {
    Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: repositorystate.DeclarationTarget{
     Scope:    repositorystate.DeclarationScopeArtifact,
     Artifact: "base-readme",
    },
    Input: &repositorystate.SourceInput{Locator: repoRoot},
   },
  },
 }); err != nil {
  t.Fatalf("WriteManifest() error = %v", err)
 }

 before, err := os.ReadFile(filepath.Join(repoRoot, repositorystate.ManifestFileName))
 if err != nil {
  t.Fatalf("ReadFile(before) error = %v", err)
 }

 svc := NewService(
  source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
  store,
 )
 got, err := svc.Install(context.Background(), Request{
  Root:        repoRoot,
  Source:      source.Ref{Type: "file", Locator: repoRoot},
  Artifact:    "base-readme",
  DeclareOnly: true,
 })
 if err != nil {
  t.Fatalf("Install() error = %v", err)
 }
 if got.Change != ChangeNoOp {
  t.Fatalf("Change = %q, want %q", got.Change, ChangeNoOp)
 }

 after, err := os.ReadFile(filepath.Join(repoRoot, repositorystate.ManifestFileName))
 if err != nil {
  t.Fatalf("ReadFile(after) error = %v", err)
 }
 if string(after) != string(before) {
  t.Fatalf("manifest changed on noop:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
 }
}

func TestDeclareOnlyInstallRejectsChangedInputAsConflict(t *testing.T) {
 repoRoot := t.TempDir()
 sourceRoot := t.TempDir()
 writeInstallFixture(t, sourceRoot)

 store := repositorystate.NewStore()
 if err := store.WriteManifest(context.Background(), repoRoot, repositorystate.Manifest{
  Declarations: []repositorystate.Declaration{
   {
    Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: repositorystate.DeclarationTarget{
     Scope:    repositorystate.DeclarationScopeArtifact,
     Artifact: "base-readme",
    },
    Input: &repositorystate.SourceInput{Locator: "/tmp/old-location"},
   },
  },
 }); err != nil {
  t.Fatalf("WriteManifest() error = %v", err)
 }

 svc := NewService(
  source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
  store,
 )
 _, err := svc.Install(context.Background(), Request{
  Root:        repoRoot,
  Source:      source.Ref{Type: "file", Locator: sourceRoot},
  Artifact:    "base-readme",
  DeclareOnly: true,
 })
 if err == nil {
  t.Fatal("Install() error = nil, want conflict")
 }

 var conflictErr ConflictError
 if !errors.As(err, &conflictErr) {
  t.Fatalf("error = %T, want ConflictError", err)
 }
 if got, want := err.Error(), `artifact "base-readme" from source "local-example-source" is already declared with different input; use upgrade`; got != want {
  t.Fatalf("error = %q, want %q", got, want)
 }
}

func TestDeclareOnlyInstallRejectsChangedRequestedVersionAsConflict(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)

 store := repositorystate.NewStore()
 if err := store.WriteManifest(context.Background(), repoRoot, repositorystate.Manifest{
  Declarations: []repositorystate.Declaration{
   {
    Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: repositorystate.DeclarationTarget{
     Scope:    repositorystate.DeclarationScopeArtifact,
     Artifact: "base-readme",
    },
    Input: &repositorystate.SourceInput{
     Locator: repoRoot,
     Version: "v1.2.3",
    },
   },
  },
 }); err != nil {
  t.Fatalf("WriteManifest() error = %v", err)
 }

 svc := NewService(
  source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
  store,
 )
 _, err := svc.Install(context.Background(), Request{
  Root:        repoRoot,
  Source:      source.Ref{Type: "file", Locator: repoRoot, Version: "v9.9.9"},
  Artifact:    "base-readme",
  DeclareOnly: true,
 })
 if err == nil {
  t.Fatal("Install() error = nil, want conflict")
 }

 var conflictErr ConflictError
 if !errors.As(err, &conflictErr) {
  t.Fatalf("error = %T, want ConflictError", err)
 }
}

func TestDeclareOnlyInstallDoesNotCreateOrMutateLockfile(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)
 writeTestFile(t, filepath.Join(repoRoot, repositorystate.LockfileFileName), ""+
  "schema_version: 1\n"+
  "resolutions:\n"+
  "  - source:\n"+
  "      type: file\n"+
  "      name: local-example-source\n"+
  "    resolved_version: local-snapshot-001\n"+
  "    artifact:\n"+
  "      name: base-readme\n"+
  "      version: 1.0.0\n")

 before, err := os.ReadFile(filepath.Join(repoRoot, repositorystate.LockfileFileName))
 if err != nil {
  t.Fatalf("ReadFile(before lockfile) error = %v", err)
 }

 svc := NewService(
  source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
  repositorystate.NewStore(),
 )
 _, err = svc.Install(context.Background(), Request{
  Root:        repoRoot,
  Source:      source.Ref{Type: "file", Locator: repoRoot},
  Artifact:    "base-readme",
  DeclareOnly: true,
 })
 if err != nil {
  t.Fatalf("Install() error = %v", err)
 }

 after, err := os.ReadFile(filepath.Join(repoRoot, repositorystate.LockfileFileName))
 if err != nil {
  t.Fatalf("ReadFile(after lockfile) error = %v", err)
 }
 if string(after) != string(before) {
  t.Fatalf("lockfile changed on declare-only:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
 }
}
```

- [ ] **Step 2: Run the declare-only service tests to verify they fail**

Run:

```bash
go test ./internal/install -run 'TestDeclareOnlyInstall' -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/install [build failed]
```

- [ ] **Step 3: Extend the install request, result, and service wiring**

```go
type Request struct {
 Root        string
 Source      source.Ref
 Artifact    string
 DeclareOnly bool
}

type ChangeKind string

const (
 ChangeDeclared ChangeKind = "declared"
 ChangeNoOp     ChangeKind = "noop"
)

type Result struct {
 Source   source.Identity
 Artifact source.ArtifactDescriptor
 Change   ChangeKind
}

type Service struct {
 registry source.Registry
 store    repositorystate.Store
}

func NewService(registry source.Registry, store repositorystate.Store) Service {
 return Service{
  registry: registry,
  store:    store,
 }
}
```

- [ ] **Step 4: Implement the minimal declare-only branch**

```go
type ConflictError struct {
 SourceName string
 Artifact   string
}

func (e ConflictError) Error() string {
 return fmt.Sprintf(
  `artifact %q from source %q is already declared with different input; use upgrade`,
  e.Artifact,
  e.SourceName,
 )
}

func (s Service) Install(ctx context.Context, req Request) (Result, error) {
 if req.Source.Type == "" {
  return Result{}, fmt.Errorf("source type is required")
 }
 if req.Source.Locator == "" {
  return Result{}, fmt.Errorf("source locator is required")
 }
 if req.DeclareOnly && req.Root == "" {
  return Result{}, fmt.Errorf("repository root is required for declare-only install")
 }

 sourceImpl, err := s.registry.Lookup(req.Source.Type)
 if err != nil {
  return Result{}, err
 }

 resolved, err := sourceImpl.Resolve(ctx, source.ResolveRequest{Ref: req.Source})
 if err != nil {
  return Result{}, err
 }

 artifact, err := selectArtifact(resolved.Artifacts, req.Artifact)
 if err != nil {
  return Result{}, err
 }

 result := Result{
  Source:   resolved.Identity,
  Artifact: artifact,
 }
 if !req.DeclareOnly {
  return result, nil
 }

 manifest, err := s.loadManifestOrEmpty(ctx, req.Root)
 if err != nil {
  return Result{}, err
 }

 decl := ManifestDeclaration(req, result)
 next, change := manifest.UpsertDeclaration(decl)
 switch change {
 case repositorystate.ChangeKindInserted:
  if err := s.store.WriteManifest(ctx, req.Root, next); err != nil {
   return Result{}, err
  }
  result.Change = ChangeDeclared
  return result, nil
 case repositorystate.ChangeKindUnchanged:
  result.Change = ChangeNoOp
  return result, nil
 default:
  return Result{}, ConflictError{
   SourceName: decl.Source.Name,
   Artifact:   decl.Target.Artifact,
  }
 }
}

func (s Service) loadManifestOrEmpty(ctx context.Context, root string) (repositorystate.Manifest, error) {
 manifest, err := s.store.LoadManifest(ctx, root)
 if err == nil {
  return manifest, nil
 }

 var stateErr repositorystate.StateFileError
 if errors.As(err, &stateErr) &&
  stateErr.File == repositorystate.StateFileManifest &&
  stateErr.Kind == repositorystate.StateFileErrorNotFound {
  return repositorystate.Manifest{}, nil
 }

 return repositorystate.Manifest{}, err
}
```

- [ ] **Step 5: Run the install package tests**

Run:

```bash
go test ./internal/install
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/install
```

- [ ] **Step 6: Review the declare-only branch for accidental extra behavior**

Check that the branch:

- only writes the manifest on `ChangeKindInserted`;
- returns `ChangeNoOp` without rewriting the manifest;
- never creates or mutates the lockfile;
- does not change non-`DeclareOnly` behavior.

### Task 2: Map conflict errors to exit code 2

**Files:**

- Modify: `cmd/tbboot/root.go`
- Modify: `cmd/tbboot/root_test.go`

- [ ] **Step 1: Write the failing exit-code regression test**

```go
func TestDeclareOnlyConflictReturnsUserActionConflictExitCode(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)
 writeTestFile(t, filepath.Join(repoRoot, "talby-artifacts.yaml"), ""+
  "schema_version: 1\n"+
  "declarations:\n"+
  "  - source:\n"+
  "      type: file\n"+
  "      name: local-example-source\n"+
  "    target:\n"+
  "      scope: artifact\n"+
  "      artifact: base-readme\n"+
  "    input:\n"+
  "      locator: /tmp/other\n")

 cwd, err := os.Getwd()
 if err != nil {
  t.Fatalf("Getwd() error = %v", err)
 }
 if err := os.Chdir(repoRoot); err != nil {
  t.Fatalf("Chdir() error = %v", err)
 }
 defer func() {
  if err := os.Chdir(cwd); err != nil {
   t.Fatalf("restore Chdir() error = %v", err)
  }
 }()

 var stderr bytes.Buffer
 code := execute(
  context.Background(),
  []string{"install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
  &bytes.Buffer{},
  &stderr,
 )
 if code != int(app.ExitUserActionConflict) {
  t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
 }
 if got := strings.TrimSpace(stderr.String()); got != `artifact "base-readme" from source "local-example-source" is already declared with different input; use upgrade` {
  t.Fatalf("stderr = %q, want conflict message", got)
 }
}

func TestDeclareOnlyConflictReturnsUserActionConflictExitCodeAsJSON(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)
 writeTestFile(t, filepath.Join(repoRoot, "talby-artifacts.yaml"), ""+
  "schema_version: 1\n"+
  "declarations:\n"+
  "  - source:\n"+
  "      type: file\n"+
  "      name: local-example-source\n"+
  "    target:\n"+
  "      scope: artifact\n"+
  "      artifact: base-readme\n"+
  "    input:\n"+
  "      locator: /tmp/other\n")

 cwd, err := os.Getwd()
 if err != nil {
  t.Fatalf("Getwd() error = %v", err)
 }
 if err := os.Chdir(repoRoot); err != nil {
  t.Fatalf("Chdir() error = %v", err)
 }
 defer func() {
  if err := os.Chdir(cwd); err != nil {
   t.Fatalf("restore Chdir() error = %v", err)
  }
 }()

 var stdout bytes.Buffer
 var stderr bytes.Buffer
 code := execute(
  context.Background(),
  []string{"--output", "json", "install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
  &stdout,
  &stderr,
 )
 if code != int(app.ExitUserActionConflict) {
  t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
 }
 if got := strings.TrimSpace(stdout.String()); got != "" {
  t.Fatalf("stdout = %q, want empty", got)
 }

 var result app.Result
 if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
  t.Fatalf("Unmarshal(stderr) error = %v", err)
 }
 if result.Code != app.ExitUserActionConflict {
  t.Fatalf("result.code = %d, want %d", result.Code, app.ExitUserActionConflict)
 }
 if result.Message != `artifact "base-readme" from source "local-example-source" is already declared with different input; use upgrade` {
  t.Fatalf("result.message = %q, want conflict message", result.Message)
 }
}
```

- [ ] **Step 2: Run the root conflict test to verify it fails**

Run:

```bash
go test ./cmd/tbboot -run TestDeclareOnlyConflictReturnsUserActionConflictExitCode -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/cmd/tbboot [build failed]
```

- [ ] **Step 3: Run the JSON conflict test to verify it fails**

Run:

```bash
go test ./cmd/tbboot -run TestDeclareOnlyConflictReturnsUserActionConflictExitCodeAsJSON -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/cmd/tbboot [build failed]
```

- [ ] **Step 4: Teach `execute` to preserve typed conflict exits**

```go
func execute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
 opts := options{output: outputHuman}
 root := newRootCommand(ctx, &opts, stdout)
 root.SetArgs(args)
 root.SetOut(stdout)
 root.SetErr(stderr)
 if err := root.Execute(); err != nil {
  code := app.ExitOperationalOrValidationError

  var conflictErr installsvc.ConflictError
  if errors.As(err, &conflictErr) {
   code = app.ExitUserActionConflict
  }

  if opts.output == outputJSON {
   _ = json.NewEncoder(stderr).Encode(app.Result{
    Code:    code,
    Message: err.Error(),
   })
   return int(code)
  }
  _, _ = fmt.Fprintln(stderr, err)
  return int(code)
 }
 return int(app.ExitSuccess)
}
```

- [ ] **Step 5: Run the command package tests**

Run:

```bash
go test ./cmd/tbboot
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/cmd/tbboot
```

### Task 3: Wire the CLI flag, output, and current working directory

**Files:**

- Modify: `cmd/tbboot/install.go`
- Modify: `cmd/tbboot/root_test.go`

- [ ] **Step 1: Write the failing CLI behavior tests**

```go
func TestDeclareOnlyInstallCommandWritesHumanSuccessMessage(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)

 cwd, err := os.Getwd()
 if err != nil {
  t.Fatalf("Getwd() error = %v", err)
 }
 if err := os.Chdir(repoRoot); err != nil {
  t.Fatalf("Chdir() error = %v", err)
 }
 defer func() {
  if err := os.Chdir(cwd); err != nil {
   t.Fatalf("restore Chdir() error = %v", err)
  }
 }()

 var stdout bytes.Buffer
 code := execute(
  context.Background(),
  []string{"install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
  &stdout,
  &bytes.Buffer{},
 )
 if code != int(app.ExitSuccess) {
  t.Fatalf("exit code = %d, want 0", code)
 }
 if got := strings.TrimSpace(stdout.String()); got != "declared artifact base-readme from local-example-source" {
  t.Fatalf("stdout = %q, want declare-only message", got)
 }
}

func TestDeclareOnlyInstallCommandJSONIncludesChange(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)

 cwd, err := os.Getwd()
 if err != nil {
  t.Fatalf("Getwd() error = %v", err)
 }
 if err := os.Chdir(repoRoot); err != nil {
  t.Fatalf("Chdir() error = %v", err)
 }
 defer func() {
  if err := os.Chdir(cwd); err != nil {
   t.Fatalf("restore Chdir() error = %v", err)
  }
 }()

 var stdout bytes.Buffer
 code := execute(
  context.Background(),
  []string{"--output", "json", "install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
  &stdout,
  &bytes.Buffer{},
 )
 if code != int(app.ExitSuccess) {
  t.Fatalf("exit code = %d, want 0", code)
 }

 var got struct {
  Message string         `json:"message"`
  Details map[string]any `json:"details"`
 }
 if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
  t.Fatalf("Unmarshal(stdout) error = %v", err)
 }
 if got.Message != "declare-only succeeded" {
  t.Fatalf("message = %q, want declare-only succeeded", got.Message)
 }
 if got.Details["change"] != "declared" {
  t.Fatalf("details.change = %#v, want declared", got.Details["change"])
 }
}

func TestDeclareOnlyInstallCommandWritesNoOpHumanMessage(t *testing.T) {
 repoRoot := t.TempDir()
 writeInstallFixture(t, repoRoot)
 writeTestFile(t, filepath.Join(repoRoot, "talby-artifacts.yaml"), ""+
  "schema_version: 1\n"+
  "declarations:\n"+
  "  - source:\n"+
  "      type: file\n"+
  "      name: local-example-source\n"+
  "    target:\n"+
  "      scope: artifact\n"+
  "      artifact: base-readme\n"+
  "    input:\n"+
  "      locator: "+repoRoot+"\n")

 cwd, err := os.Getwd()
 if err != nil {
  t.Fatalf("Getwd() error = %v", err)
 }
 if err := os.Chdir(repoRoot); err != nil {
  t.Fatalf("Chdir() error = %v", err)
 }
 defer func() {
  if err := os.Chdir(cwd); err != nil {
   t.Fatalf("restore Chdir() error = %v", err)
  }
 }()

 var stdout bytes.Buffer
 code := execute(
  context.Background(),
  []string{"install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
  &stdout,
  &bytes.Buffer{},
 )
 if code != int(app.ExitSuccess) {
  t.Fatalf("exit code = %d, want 0", code)
 }
 if got := strings.TrimSpace(stdout.String()); got != "artifact base-readme from local-example-source is already declared" {
  t.Fatalf("stdout = %q, want noop message", got)
 }
}

func TestDeclareOnlyInstallCommandWritesManifestInCurrentWorkingDirectory(t *testing.T) {
 repoRoot := t.TempDir()
 sourceRoot := t.TempDir()
 writeInstallFixture(t, sourceRoot)

 cwd, err := os.Getwd()
 if err != nil {
  t.Fatalf("Getwd() error = %v", err)
 }
 if err := os.Chdir(repoRoot); err != nil {
  t.Fatalf("Chdir() error = %v", err)
 }
 defer func() {
  if err := os.Chdir(cwd); err != nil {
   t.Fatalf("restore Chdir() error = %v", err)
  }
 }()

 code := execute(
  context.Background(),
  []string{"install", "file:" + sourceRoot, "--artifact", "base-readme", "--declare-only"},
  &bytes.Buffer{},
  &bytes.Buffer{},
 )
 if code != int(app.ExitSuccess) {
  t.Fatalf("exit code = %d, want 0", code)
 }

 if _, err := os.Stat(filepath.Join(repoRoot, repositorystate.ManifestFileName)); err != nil {
  t.Fatalf("repo root manifest stat error = %v, want nil", err)
 }
 if _, err := os.Stat(filepath.Join(sourceRoot, repositorystate.ManifestFileName)); !errors.Is(err, os.ErrNotExist) {
  t.Fatalf("source root manifest state error = %v, want not exist", err)
 }
}

func TestDeclareOnlyInstallCommandRejectsMissingSource(t *testing.T) {
 var stderr bytes.Buffer
 code := execute(
  context.Background(),
  []string{"install", "--declare-only"},
  &bytes.Buffer{},
  &stderr,
 )
 if code != int(app.ExitOperationalOrValidationError) {
  t.Fatalf("exit code = %d, want 1", code)
 }
 if got := strings.TrimSpace(stderr.String()); got != "declare-only install requires an explicit <source>" {
  t.Fatalf("stderr = %q, want missing source message", got)
 }
}
```

- [ ] **Step 2: Run the declare-only CLI tests to verify they fail**

Run:

```bash
go test ./cmd/tbboot -run 'TestDeclareOnlyInstallCommand' -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/cmd/tbboot [build failed]
```

- [ ] **Step 3: Add the CLI flag and request wiring**

```go
func installCommand(ctx context.Context, opts *options, stdout io.Writer) *cobra.Command {
 var artifact string
 var declareOnly bool
 service := installsvc.NewService(
  source.NewStaticRegistry(map[string]source.Source{
   "file": sourcefile.New(),
  }),
  repositorystate.NewStore(),
 )

 cmd := &cobra.Command{
  Use:     "install [<source>]",
  Aliases: []string{"i"},
  Short:   "Install or sync artifacts",
  Args:    cobra.MaximumNArgs(1),
  RunE: func(cmd *cobra.Command, args []string) error {
   if len(args) == 0 {
    if declareOnly {
     return fmt.Errorf("declare-only install requires an explicit <source>")
    }
    result := app.Success("sync not implemented")
    if opts.output == outputJSON {
     return json.NewEncoder(stdout).Encode(result)
    }
    _, err := fmt.Fprintln(stdout, result.Message)
    return err
   }

   ref, err := parseSourceRef(args[0])
   if err != nil {
    return err
   }
   root, err := os.Getwd()
   if err != nil {
    return err
   }

   result, err := service.Install(ctx, installsvc.Request{
    Root:        root,
    Source:      ref,
    Artifact:    artifact,
    DeclareOnly: declareOnly,
   })
   if err != nil {
    return err
   }

   if opts.output == outputJSON {
    message := "install succeeded"
    if declareOnly {
     message = "declare-only succeeded"
    }
    envelope := app.Success(message)
    envelope.Details = map[string]any{
     "source":   mapSourceIdentity(result.Source),
     "artifact": mapArtifactDescriptor(result.Artifact),
    }
    if declareOnly {
     envelope.Details["change"] = result.Change
    }
    return json.NewEncoder(stdout).Encode(envelope)
   }

   if declareOnly {
    if result.Change == installsvc.ChangeNoOp {
     _, err = fmt.Fprintf(stdout, "artifact %s from %s is already declared\n", result.Artifact.Name, result.Source.Name)
     return err
    }
    _, err = fmt.Fprintf(stdout, "declared artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
    return err
   }

   _, err = fmt.Fprintf(stdout, "selected artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
   return err
  },
 }

 cmd.Flags().StringVar(&artifact, "artifact", "", "artifact to install")
 cmd.Flags().BoolVar(&declareOnly, "declare-only", false, "declare artifact intent without materializing files")
 return cmd
}
```

- [ ] **Step 4: Run focused command tests, then the repo checks for touched code**

Run:

```bash
go test ./cmd/tbboot -run 'Test(DeclareOnlyInstallCommand|DeclareOnlyConflictReturnsUserActionConflictExitCode|JSONOutput)' -v
go test ./internal/install -run 'Test(DeclareOnlyInstall|Install)' -v
just check-go
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/cmd/tbboot
ok   github.com/talby/talby-bootstrap/internal/install
```

- [ ] **Step 5: Do the final behavior pass**

Verify these end-to-end outcomes manually in a temp repo root:

```bash
tbboot install file:/tmp/example --artifact base-readme --declare-only
tbboot install file:/tmp/example --artifact base-readme --declare-only
tbboot --output json install file:/tmp/example --artifact base-readme --declare-only
```

Expected:

- first command writes `talby-artifacts.yaml` and prints `declared artifact base-readme from local-example-source`
- second command prints `artifact base-readme from local-example-source is already declared`
- JSON command returns `message: "declare-only succeeded"` and `details.change` of `declared` or `noop`
- if `talby-artifacts.lock.yaml` does not exist before declare-only, it is not created
- if `talby-artifacts.lock.yaml` already exists before declare-only, it remains byte-for-byte unchanged

## Self-review

- Spec coverage:
  - explicit source required: Task 3
  - resolve source and select one artifact: Task 1 reuses current service path
  - repository root is distinct from source locator: Tasks 1 and 3
  - create manifest when missing: Task 1 bootstrap test and implementation
  - reject missing repository root for direct service use: Task 1
  - add artifact-scoped declaration: Task 1 via `ManifestDeclaration`
  - leave lockfile untouched: Task 1 lockfile test
  - reject changed declaration input: Tasks 1 and 2
  - human and JSON output: Task 3
  - conflict exit handling in human and JSON output: Task 2
- Placeholder scan: no `TODO`, `TBD`, or implied “handle edge cases” steps remain.
- Type consistency:
  - service request fields are `Root`, `Source`, `Artifact`, `DeclareOnly`
  - install result change values are `declared` and `noop`
  - manifest conflict continues to use `repositorystate.ChangeKindReplaced` internally and maps to `ConflictError`

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-07-declare-only-install.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
