# Install core implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first `install` implementation slice around a command-independent core, a pluggable source registry, and a real `file` source without making Cobra or real process execution the main testing seam.

**Architecture:** Keep `cmd/tbboot` thin and move install behavior into `internal/install` plus a source subsystem under `internal/source`. The install service validates normalized requests, resolves a typed source through a registry, selects exactly one artifact, and returns structured outcome data that the CLI maps to human or JSON output.

**Tech Stack:** Go 1.26.4, Cobra, `gopkg.in/yaml.v3`, `just`, `gofmt`, `go test`.

---

## Constraints

- Do not add process-level example execution in this slice.
- Do not add a real `git` source implementation in this slice.
- Do not write commits from an agent. `AGENTS.md` explicitly forbids it, so use review checkpoints instead of commit steps.

## File structure

- Create: `internal/source/model.go` - typed source references, capabilities, resolved-source data, and descriptor models shared by the core and concrete sources.
- Create: `internal/source/registry.go` - source registry interface and a small static registry implementation used by CLI wiring and tests.
- Create: `internal/source/registry_test.go` - focused tests for registry lookup behavior.
- Create: `internal/source/file/source.go` - real `file` source implementation that resolves a source directory into typed artifact data.
- Create: `internal/source/file/source_test.go` - temp-directory integration tests for descriptor parsing and artifact discovery.
- Create: `internal/install/service.go` - command-independent install service, request/result types, and artifact selection logic.
- Create: `internal/install/service_test.go` - primary TDD seam with fake registry and fake source implementations.
- Create: `cmd/tbboot/install.go` - Cobra `install` command, request normalization, and output rendering through the install service.
- Modify: `cmd/tbboot/root.go` - replace the `install` placeholder with real wiring while leaving other commands as placeholders.
- Modify: `cmd/tbboot/root_test.go` - keep smoke-test coverage on aliases, output modes, and install CLI validation.

The first slice should not create a manifest writer, lockfile writer, or materialization engine. `InstallResult` should expose the data those later steps will need without performing those operations yet.

## Tasks

### Task 1: Define source contracts and lock the registry seam

**Files:**

- Create: `internal/source/model.go`
- Create: `internal/source/registry.go`
- Create: `internal/source/registry_test.go`

- [x] **Step 1: Write the failing registry tests**

```go
package source

import (
 "context"
 "testing"
)

type stubSource struct{}

func (stubSource) Capabilities() Capabilities { return Capabilities{} }

func (stubSource) Resolve(context.Context, ResolveRequest) (ResolvedSource, error) {
 return ResolvedSource{}, nil
}

func TestStaticRegistryLookupReturnsRegisteredSource(t *testing.T) {
 registry := NewStaticRegistry(map[string]Source{
  "file": stubSource{},
 })

 got, err := registry.Lookup("file")
 if err != nil {
  t.Fatalf("Lookup() error = %v", err)
 }
 if got == nil {
  t.Fatal("Lookup() returned nil source")
 }
}

func TestStaticRegistryLookupRejectsUnknownType(t *testing.T) {
 registry := NewStaticRegistry(nil)

 _, err := registry.Lookup("git")
 if err == nil {
  t.Fatal("Lookup() error = nil, want unknown source type error")
 }
}
```

- [x] **Step 2: Run the registry tests to verify they fail**

Run:

```bash
go test ./internal/source -run TestStaticRegistry -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/source [build failed]
```

- [x] **Step 3: Add the shared source model**

```go
package source

import "context"

type Ref struct {
 Type    string
 Locator string
 Version string
}

type Capabilities struct {
 SupportsVersions bool
 ProvidesIdentity bool
 ProvidesTimestamp bool
 EnumeratesVersions bool
}

type ResolveRequest struct {
 Ref Ref
}

type ArtifactDescriptor struct {
 Name    string
 Version string
 Path    string
}

type Identity struct {
 Type    string
 Name    string
 Version string
}

type ResolvedSource struct {
 Identity    Identity
 Artifacts   []ArtifactDescriptor
 SourcePath  string
}

type Source interface {
 Capabilities() Capabilities
 Resolve(context.Context, ResolveRequest) (ResolvedSource, error)
}
```

- [x] **Step 4: Implement the registry**

```go
package source

import "fmt"

type Registry interface {
 Lookup(sourceType string) (Source, error)
}

type StaticRegistry struct {
 sources map[string]Source
}

func NewStaticRegistry(sources map[string]Source) StaticRegistry {
 if sources == nil {
  sources = map[string]Source{}
 }
 return StaticRegistry{sources: sources}
}

func (r StaticRegistry) Lookup(sourceType string) (Source, error) {
 sourceImpl, ok := r.sources[sourceType]
 if !ok {
  return nil, fmt.Errorf("unsupported source type %q", sourceType)
 }
 return sourceImpl, nil
}
```

- [x] **Step 5: Run the package tests**

Run:

```bash
go test ./internal/source
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/source
```

- [x] **Step 6: Review the exported names before moving on**

Check that `Ref`, `ResolvedSource`, `Registry`, and `Source` are the only cross-package types required by the next tasks. If more types feel necessary here, stop and confirm they are actually shared instead of prebuilding a large abstraction.

### Task 2: Drive the install service from tests instead of Cobra

**Files:**

- Create: `internal/install/service.go`
- Create: `internal/install/service_test.go`

- [x] **Step 1: Write the core failing tests**

```go
package install

import (
 "context"
 "testing"

 "github.com/talby/talby-bootstrap/internal/source"
)

type fakeRegistry struct {
 source source.Source
 err    error
}

func (r fakeRegistry) Lookup(string) (source.Source, error) {
 return r.source, r.err
}

type fakeSource struct {
 resolved source.ResolvedSource
 err      error
}

func (f fakeSource) Capabilities() source.Capabilities { return source.Capabilities{} }

func (f fakeSource) Resolve(context.Context, source.ResolveRequest) (source.ResolvedSource, error) {
 return f.resolved, f.err
}

func TestInstallReturnsSelectedArtifact(t *testing.T) {
 svc := NewService(fakeRegistry{
  source: fakeSource{
   resolved: source.ResolvedSource{
    Identity: source.Identity{Type: "file", Name: "local-example-source", Version: "local-snapshot-001"},
    Artifacts: []source.ArtifactDescriptor{
     {Name: "base-readme", Version: "1.0.0", Path: "artifacts/base-readme"},
    },
   },
  },
 })

 got, err := svc.Install(context.Background(), Request{
  Source:   source.Ref{Type: "file", Locator: "/tmp/example"},
  Artifact: "base-readme",
 })
 if err != nil {
  t.Fatalf("Install() error = %v", err)
 }
 if got.Source.Name != "local-example-source" {
  t.Fatalf("Source.Name = %q, want local-example-source", got.Source.Name)
 }
 if got.Artifact.Name != "base-readme" {
  t.Fatalf("Artifact.Name = %q, want base-readme", got.Artifact.Name)
 }
}

func TestInstallRejectsAmbiguousArtifactTarget(t *testing.T) {
 svc := NewService(fakeRegistry{
  source: fakeSource{
   resolved: source.ResolvedSource{
    Artifacts: []source.ArtifactDescriptor{
     {Name: "base-readme"},
     {Name: "ci-github"},
    },
   },
  },
 })

 _, err := svc.Install(context.Background(), Request{
  Source:   source.Ref{Type: "file", Locator: "/tmp/example"},
  Artifact: "",
 })
 if err == nil {
  t.Fatal("Install() error = nil, want ambiguity error")
 }
}

func TestInstallRejectsUnknownSourceType(t *testing.T) {
 svc := NewService(fakeRegistry{err: fmt.Errorf("unsupported source type %q", "git")})

 _, err := svc.Install(context.Background(), Request{
  Source:   source.Ref{Type: "git", Locator: "github.com/example/library"},
  Artifact: "base-readme",
 })
 if err == nil {
  t.Fatal("Install() error = nil, want source lookup error")
 }
 if !strings.Contains(err.Error(), `unsupported source type "git"`) {
  t.Fatalf("error = %q, want unsupported source type", err)
 }
}
```

- [x] **Step 2: Run the install tests to verify they fail**

Run:

```bash
go test ./internal/install -run TestInstall -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/install [build failed]
```

- [x] **Step 3: Implement the service boundary**

```go
package install

import (
 "context"
 "fmt"

 "github.com/talby/talby-bootstrap/internal/source"
)

type Request struct {
 Source   source.Ref
 Artifact string
}

type Result struct {
 Source   source.Identity
 Artifact source.ArtifactDescriptor
}

type Service struct {
 registry source.Registry
}

func NewService(registry source.Registry) Service {
 return Service{registry: registry}
}

func (s Service) Install(ctx context.Context, req Request) (Result, error) {
 if req.Source.Type == "" {
  return Result{}, fmt.Errorf("source type is required")
 }
 if req.Source.Locator == "" {
  return Result{}, fmt.Errorf("source locator is required")
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

 return Result{
  Source:   resolved.Identity,
  Artifact: artifact,
 }, nil
}

func selectArtifact(artifacts []source.ArtifactDescriptor, wanted string) (source.ArtifactDescriptor, error) {
 if wanted != "" {
  for _, artifact := range artifacts {
   if artifact.Name == wanted {
    return artifact, nil
   }
  }
  return source.ArtifactDescriptor{}, fmt.Errorf("artifact %q was not found", wanted)
 }
 if len(artifacts) != 1 {
  return source.ArtifactDescriptor{}, fmt.Errorf("install target must resolve to exactly one artifact")
 }
 return artifacts[0], nil
}
```

- [x] **Step 4: Add the missing imports and keep the assertions explicit**

The test file from Step 1 now needs:

```go
import (
 "context"
 "fmt"
 "strings"
 "testing"

 "github.com/talby/talby-bootstrap/internal/source"
)
```

Keep the string assertion instead of introducing custom test-only error types.

- [x] **Step 5: Run the focused install tests**

Run:

```bash
go test ./internal/install -run TestInstall -v
```

Expected:

```text
=== RUN   TestInstallReturnsSelectedArtifact
=== RUN   TestInstallRejectsAmbiguousArtifactTarget
=== RUN   TestInstallRejectsUnknownSourceType
--- PASS: TestInstallReturnsSelectedArtifact
--- PASS: TestInstallRejectsAmbiguousArtifactTarget
--- PASS: TestInstallRejectsUnknownSourceType
PASS
```

- [x] **Step 6: Run the full package tests**

Run:

```bash
go test ./internal/install ./internal/source
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/install
ok   github.com/talby/talby-bootstrap/internal/source
```

### Task 3: Add a real `file` source resolver with descriptor parsing

**Files:**

- Create: `internal/source/file/source.go`
- Create: `internal/source/file/source_test.go`

- [x] **Step 1: Write the failing resolver tests with temp directories**

```go
package file

import (
 "context"
 "path/filepath"
 "testing"

 "github.com/talby/talby-bootstrap/internal/source"
)

func TestResolveLoadsSourceIdentityAndArtifacts(t *testing.T) {
 root := t.TempDir()
 writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
 writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\nsteps:\n  - type: file\n    path: README.md\n    source: README.md\n")

 resolved, err := New().Resolve(context.Background(), source.ResolveRequest{
  Ref: source.Ref{Type: "file", Locator: root},
 })
 if err != nil {
  t.Fatalf("Resolve() error = %v", err)
 }
 if resolved.Identity.Name != "local-example-source" {
  t.Fatalf("Identity.Name = %q, want local-example-source", resolved.Identity.Name)
 }
 if len(resolved.Artifacts) != 1 || resolved.Artifacts[0].Version != "1.0.0" {
  t.Fatalf("Artifacts = %#v, want one versioned artifact", resolved.Artifacts)
 }
}

func TestResolveRejectsMissingArtifactDescriptor(t *testing.T) {
 root := t.TempDir()
 writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")

 _, err := New().Resolve(context.Background(), source.ResolveRequest{
  Ref: source.Ref{Type: "file", Locator: root},
 })
 if err == nil {
  t.Fatal("Resolve() error = nil, want missing talby-artifact.yaml error")
 }
}
```

- [x] **Step 2: Run the file-source tests to verify they fail**

Run:

```bash
go test ./internal/source/file -run TestResolve -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/source/file [build failed]
```

- [x] **Step 3: Implement the file source**

```go
package file

import (
 "context"
 "fmt"
 "os"
 "path/filepath"

 "github.com/talby/talby-bootstrap/internal/source"
 "gopkg.in/yaml.v3"
)

type Source struct{}

func New() Source { return Source{} }

func (Source) Capabilities() source.Capabilities {
 return source.Capabilities{
  SupportsVersions:  false,
  ProvidesIdentity:  true,
  ProvidesTimestamp: false,
  EnumeratesVersions: false,
 }
}

func (Source) Resolve(_ context.Context, req source.ResolveRequest) (source.ResolvedSource, error) {
 descriptorPath := filepath.Join(req.Ref.Locator, "talby-source.yaml")
 data, err := os.ReadFile(descriptorPath)
 if err != nil {
  return source.ResolvedSource{}, fmt.Errorf("read %s: %w", descriptorPath, err)
 }

 var descriptor struct {
  SchemaVersion int `yaml:"schema_version"`
  Source struct {
   Name string `yaml:"name"`
  } `yaml:"source"`
  Artifacts []struct {
   Name string `yaml:"name"`
   Path string `yaml:"path"`
  } `yaml:"artifacts"`
 }
 if err := yaml.Unmarshal(data, &descriptor); err != nil {
  return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", descriptorPath, err)
 }

 resolved := source.ResolvedSource{
  Identity: source.Identity{
   Type:    "file",
   Name:    descriptor.Source.Name,
   Version: "local-snapshot-001",
  },
  SourcePath: req.Ref.Locator,
 }

 for _, artifactRef := range descriptor.Artifacts {
  artifactPath := filepath.Join(req.Ref.Locator, artifactRef.Path, "talby-artifact.yaml")
  artifactData, err := os.ReadFile(artifactPath)
  if err != nil {
   return source.ResolvedSource{}, fmt.Errorf("read %s: %w", artifactPath, err)
  }

  var artifactDescriptor struct {
   Artifact struct {
    Name    string `yaml:"name"`
    Version string `yaml:"version"`
   } `yaml:"artifact"`
  }
  if err := yaml.Unmarshal(artifactData, &artifactDescriptor); err != nil {
   return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, err)
  }

  resolved.Artifacts = append(resolved.Artifacts, source.ArtifactDescriptor{
   Name:    artifactDescriptor.Artifact.Name,
   Version: artifactDescriptor.Artifact.Version,
   Path:    artifactRef.Path,
  })
 }

 return resolved, nil
}
```

- [x] **Step 4: Add the tiny test helper locally in `source_test.go`**

```go
func writeFile(t *testing.T, path string, content string) {
 t.Helper()
 if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
  t.Fatalf("MkdirAll(%q) error = %v", path, err)
 }
 if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
  t.Fatalf("WriteFile(%q) error = %v", path, err)
 }
}
```

- [x] **Step 5: Run the file-source package tests**

Run:

```bash
go test ./internal/source/file
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/source/file
```

- [x] **Step 6: Run the higher-level service tests against the real file source**

Add one more test in `internal/install/service_test.go` that constructs:

```go
registry := source.NewStaticRegistry(map[string]source.Source{
 "file": file.New(),
})
```

Use a temp directory containing the same descriptor content as `file-direct-install-single-artifact`, then run:

```bash
go test ./internal/install -run TestInstallWithRealFileSource -v
```

Expected:

```text
--- PASS: TestInstallWithRealFileSource
PASS
```

### Task 4: Wire the Cobra `install` command without moving domain logic into `cmd/tbboot`

**Files:**

- Create: `cmd/tbboot/install.go`
- Modify: `cmd/tbboot/root.go`
- Modify: `cmd/tbboot/root_test.go`

- [x] **Step 1: Write the CLI smoke tests first**

Add these tests to `cmd/tbboot/root_test.go`:

```go
func TestInstallCommandRequiresSourceArgument(t *testing.T) {
 var stderr bytes.Buffer
 code := execute(context.Background(), []string{"install"}, &bytes.Buffer{}, &stderr)
 if code != int(app.ExitOperationalOrValidationError) {
  t.Fatalf("exit code = %d, want 1", code)
 }
 if got := strings.TrimSpace(stderr.String()); got != "accepts 1 arg(s), received 0" {
  t.Fatalf("stderr = %q", got)
 }
}

func TestInstallCommandAcceptsArtifactFlag(t *testing.T) {
 rootDir := t.TempDir()
 writeInstallFixture(t, rootDir)

 var stdout bytes.Buffer
 code := execute(context.Background(), []string{"install", "file:" + rootDir, "--artifact", "base-readme"}, &stdout, &bytes.Buffer{})
 if code != int(app.ExitSuccess) {
  t.Fatalf("exit code = %d, want 0", code)
 }
 if got := strings.TrimSpace(stdout.String()); !strings.Contains(got, "base-readme") {
  t.Fatalf("stdout = %q, want selected artifact name", got)
 }
}
```

- [x] **Step 2: Run the targeted CLI tests to verify they fail**

Run:

```bash
go test ./cmd/tbboot -run TestInstallCommand -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/cmd/tbboot [build failed]
```

- [x] **Step 3: Implement `cmd/tbboot/install.go`**

```go
package tbboot

import (
 "context"
 "encoding/json"
 "fmt"
 "io"
 "strings"

 "github.com/spf13/cobra"
 "github.com/talby/talby-bootstrap/internal/install"
 "github.com/talby/talby-bootstrap/internal/source"
 sourcefile "github.com/talby/talby-bootstrap/internal/source/file"
)

func installCommand(ctx context.Context, opts *options, stdout io.Writer) *cobra.Command {
 var artifact string

 cmd := &cobra.Command{
  Use:     "install <source>",
  Aliases: []string{"i"},
  Short:   "Install or sync artifacts",
  Args:    cobra.ExactArgs(1),
  RunE: func(cmd *cobra.Command, args []string) error {
   ref, err := parseSourceRef(args[0])
   if err != nil {
    return err
   }

   service := install.NewService(source.NewStaticRegistry(map[string]source.Source{
    "file": sourcefile.New(),
   }))
   result, err := service.Install(ctx, install.Request{
    Source:   ref,
    Artifact: artifact,
   })
   if err != nil {
    return err
   }

   if opts.output == outputJSON {
    return json.NewEncoder(stdout).Encode(result)
   }

   _, err = fmt.Fprintf(stdout, "selected artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
   return err
  },
 }

 cmd.Flags().StringVar(&artifact, "artifact", "", "install exactly one named artifact")
 return cmd
}

func parseSourceRef(raw string) (source.Ref, error) {
 sourceType, locator, ok := strings.Cut(raw, ":")
 if !ok || sourceType == "" || locator == "" {
  return source.Ref{}, fmt.Errorf("source must be formatted as <type>:<locator>")
 }
 return source.Ref{Type: sourceType, Locator: locator}, nil
}
```

- [x] **Step 4: Replace the placeholder install command in `root.go`**

Change:

```go
placeholderCommand(ctx, opts, stdout, "install", []string{"i"}, "Install or sync artifacts"),
```

to:

```go
installCommand(ctx, opts, stdout),
```

- [x] **Step 5: Add a local fixture helper to `cmd/tbboot/root_test.go`**

Create a helper that writes the same minimal source files used in the file-source tests so CLI smoke tests do not depend on `testdata/examples/` staging conventions:

```go
func writeInstallFixture(t *testing.T, root string) {
 t.Helper()
 mustWrite := func(path string, content string) {
  t.Helper()
  if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
   t.Fatalf("MkdirAll(%q) error = %v", path, err)
  }
  if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
   t.Fatalf("WriteFile(%q) error = %v", path, err)
  }
 }

 mustWrite(filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
 mustWrite(filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\nsteps:\n  - type: file\n    path: README.md\n    source: README.md\n")
}
```

- [x] **Step 6: Run the CLI package tests**

Run:

```bash
go test ./cmd/tbboot
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/cmd/tbboot
```

### Task 5: Run repository verification and close the slice with explicit gaps

**Files:**

- Modify: none unless a test reveals a missing import or naming mismatch

- [x] **Step 1: Format the touched Go files**

Run:

```bash
just fmt-go
```

Expected:

```text
<no output>
```

- [x] **Step 2: Run focused Go verification for the new packages**

Run:

```bash
go test ./internal/source ./internal/source/file ./internal/install ./cmd/tbboot
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/source
ok   github.com/talby/talby-bootstrap/internal/source/file
ok   github.com/talby/talby-bootstrap/internal/install
ok   github.com/talby/talby-bootstrap/cmd/tbboot
```

- [x] **Step 3: Run the repository Go checks**

Run:

```bash
just check-go
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/...
```

- [x] **Step 4: Record the intentional gaps in the handoff note or PR summary**

State explicitly that this slice still defers:

```text
- manifest and lockfile writes
- materialization of artifact steps
- trust-policy enforcement
- real git source resolution
- example-runner process execution
```

- [x] **Step 5: Stop for user review instead of committing**

Report the diff, the exact verification commands run, and any remaining risk. Do not create a commit; the repository instructions require the user to review and commit manually.

## Self-review

### Spec coverage

- `InstallRequest` and a command-independent service are covered in Task 2.
- Environment isolation for validation-heavy tests is covered by fake registry and fake source tests in Task 2.
- `SourceRegistry`, typed `Source`, and capability model are covered in Task 1.
- Real `file` source support is covered in Task 3.
- Thin Cobra wiring and output mapping are covered in Task 4.
- Deferred end-to-end example execution remains intentionally out of scope and is listed again in Task 5.

### Placeholder scan

No `TODO`, `TBD`, or "implement later" placeholders are left in the executable steps. Deferred work is called out only in the explicit gap list.

### Type consistency

The plan consistently uses `source.Ref`, `source.Registry`, `source.Source`, `install.Request`, `install.Result`, `source.ResolveRequest`, and `source.ResolvedSource`. The only implementation detail to watch during execution is whether `install.NewService` returns a value or pointer; keep tests and command wiring aligned either way.
