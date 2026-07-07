# Repository State Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `internal/repositorystate` so the repository has a real persisted Manifest and Lockfile model with stable YAML contracts, validation, typed load errors, and install-adjacent mapping helpers, without changing current CLI behavior.

**Architecture:** Keep repository persistence isolated in `internal/repositorystate` and keep install orchestration in `internal/install`. The new package owns domain types, validation, upsert helpers, file names, YAML DTOs, and filesystem reads and writes; `internal/install` only maps resolved install results into repository-state values for later slices.

**Tech Stack:** Go 1.26.4, `gopkg.in/yaml.v3`, `gofmt`, `go test`, `just`.

---

## Constraints

- Do not change `cmd/tbboot` behavior in this slice.
- Do not make `internal/install.Service` write `talby-artifacts.yaml` or `talby-artifacts.lock.yaml`.
- Do not add trust enforcement, materialization ownership, drift detection, or sync behavior.
- Do not write commits from an agent. `AGENTS.md` requires explicit approval, so use review checkpoints instead of commit steps.

## File structure

- Create: `internal/repositorystate/model.go` - domain types, enums, constants, and typed state-file errors.
- Create: `internal/repositorystate/manifest.go` - manifest validation, declaration normalization, and `UpsertDeclaration`.
- Create: `internal/repositorystate/lockfile.go` - lockfile validation, resolution normalization, and `UpsertResolution`.
- Create: `internal/repositorystate/store.go` - filesystem-backed store, YAML DTOs, read/write helpers, and stable sorting before writes.
- Create: `internal/repositorystate/manifest_test.go` - focused tests for manifest validation and upsert behavior.
- Create: `internal/repositorystate/lockfile_test.go` - focused tests for lockfile validation and upsert behavior.
- Create: `internal/repositorystate/store_test.go` - temp-directory tests for round-trips, `not_found`, and `invalid_format`.
- Create: `internal/install/repository_state.go` - helpers that map install request/result data into `repositorystate.Declaration` and `repositorystate.Resolution`.
- Create: `internal/install/repository_state_test.go` - cross-module contract tests proving a resolved `file:` source maps cleanly into repository state.

The package should use one store implementation instead of separate manifest and lockfile stores so callers get a single persistence seam without a generic repository object.

## Tasks

### Task 1: Define the repository-state domain and validation rules

**Files:**

- Create: `internal/repositorystate/model.go`
- Create: `internal/repositorystate/manifest.go`
- Create: `internal/repositorystate/lockfile.go`
- Create: `internal/repositorystate/manifest_test.go`
- Create: `internal/repositorystate/lockfile_test.go`

- [ ] **Step 1: Write the failing manifest and lockfile rule tests**

```go
package repositorystate

import "testing"

func TestManifestUpsertDeclarationInsertReplaceAndUnchanged(t *testing.T) {
 base := Manifest{}

 decl := Declaration{
  Source: SourceIdentity{Type: "file", Name: "local-example-source"},
  Target: DeclarationTarget{
   Scope:    DeclarationScopeArtifact,
   Artifact: "base-readme",
  },
  Input: &SourceInput{Locator: "/tmp/example", Version: "v1.2.3"},
 }

 inserted, change := base.UpsertDeclaration(decl)
 if change != ChangeKindInserted {
  t.Fatalf("change = %q, want %q", change, ChangeKindInserted)
 }

 replaced, change := inserted.UpsertDeclaration(Declaration{
  Source: decl.Source,
  Target: decl.Target,
  Input:  &SourceInput{Locator: "/tmp/other"},
 })
 if change != ChangeKindReplaced {
  t.Fatalf("change = %q, want %q", change, ChangeKindReplaced)
 }

 unchanged, change := replaced.UpsertDeclaration(Declaration{
  Source: decl.Source,
  Target: decl.Target,
  Input:  &SourceInput{Locator: "/tmp/other"},
 })
 if change != ChangeKindUnchanged {
  t.Fatalf("change = %q, want %q", change, ChangeKindUnchanged)
 }
 if len(unchanged.Declarations) != 1 {
  t.Fatalf("len(Declarations) = %d, want 1", len(unchanged.Declarations))
 }
}

func TestValidateManifestRejectsInvalidTargetsAndDuplicates(t *testing.T) {
 err := ValidateManifest(Manifest{
  Declarations: []Declaration{
   {
    Source: SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: DeclarationTarget{Scope: DeclarationScopeArtifact},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateManifest() error = nil, want missing artifact name error")
 }

 err = ValidateManifest(Manifest{
  Declarations: []Declaration{
   {
    Source: SourceIdentity{Name: "local-example-source"},
    Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "base-readme"},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateManifest() error = nil, want missing source type error")
 }

 err = ValidateManifest(Manifest{
  Declarations: []Declaration{
   {
    Source: SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: DeclarationTarget{Scope: DeclarationScopeSource, Artifact: "base-readme"},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateManifest() error = nil, want source-scoped artifact error")
 }

 err = ValidateManifest(Manifest{
  Declarations: []Declaration{
   {
    Source: SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: DeclarationTarget{Scope: DeclarationScopeSource},
   },
   {
    Source: SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: DeclarationTarget{Scope: DeclarationScopeSource},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateManifest() error = nil, want duplicate source-scope declaration error")
 }
}

func TestLockfileUpsertResolutionInsertReplaceAndUnchanged(t *testing.T) {
 base := Lockfile{}

 res := Resolution{
  Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
  ResolvedVersion: "local-snapshot-001",
  Artifact: ArtifactResolution{
   Name:    "base-readme",
   Version: "1.0.0",
  },
 }

 inserted, change := base.UpsertResolution(res)
 if change != ChangeKindInserted {
  t.Fatalf("change = %q, want %q", change, ChangeKindInserted)
 }

 replaced, change := inserted.UpsertResolution(Resolution{
  Source:          res.Source,
  ResolvedVersion: "local-snapshot-002",
  Artifact: ArtifactResolution{Name: "base-readme", Version: "1.1.0"},
 })
 if change != ChangeKindReplaced {
  t.Fatalf("change = %q, want %q", change, ChangeKindReplaced)
 }

 unchanged, change := replaced.UpsertResolution(replaced.Resolutions[0])
 if change != ChangeKindUnchanged {
  t.Fatalf("change = %q, want %q", change, ChangeKindUnchanged)
 }
 if len(unchanged.Resolutions) != 1 {
  t.Fatalf("len(Resolutions) = %d, want 1", len(unchanged.Resolutions))
 }
}

func TestValidateLockfileRejectsMissingFieldsAndDuplicates(t *testing.T) {
 err := ValidateLockfile(Lockfile{
  Resolutions: []Resolution{
   {
    Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
    ResolvedVersion: "",
    Artifact: ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateLockfile() error = nil, want missing resolved version error")
 }

 err = ValidateLockfile(Lockfile{
  Resolutions: []Resolution{
   {
    Source:          SourceIdentity{Type: "file"},
    ResolvedVersion: "local-snapshot-001",
    Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateLockfile() error = nil, want missing source name error")
 }

 err = ValidateLockfile(Lockfile{
  Resolutions: []Resolution{
   {
    Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
    ResolvedVersion: "local-snapshot-001",
    Artifact:        ArtifactResolution{Version: "1.0.0"},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateLockfile() error = nil, want missing artifact name error")
 }

 err = ValidateLockfile(Lockfile{
  Resolutions: []Resolution{
   {
    Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
    ResolvedVersion: "local-snapshot-001",
    Artifact:        ArtifactResolution{Name: "base-readme"},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateLockfile() error = nil, want missing artifact version error")
 }

 err = ValidateLockfile(Lockfile{
  Resolutions: []Resolution{
   {
    Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
    ResolvedVersion: "local-snapshot-001",
    Artifact: ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
   },
   {
    Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
    ResolvedVersion: "local-snapshot-002",
    Artifact: ArtifactResolution{Name: "base-readme", Version: "1.1.0"},
   },
  },
 })
 if err == nil {
  t.Fatal("ValidateLockfile() error = nil, want duplicate resolution error")
 }
}
```

- [ ] **Step 2: Run the repository-state tests to verify they fail**

Run:

```bash
go test ./internal/repositorystate -run 'Test(Manifest|Lockfile)' -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/repositorystate [build failed]
```

- [ ] **Step 3: Add the domain model and package constants**

```go
package repositorystate

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
 Type string
 Name string
}

type SourceInput struct {
 Locator string
 Version string
}

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

type ChangeKind string

const (
 ChangeKindInserted  ChangeKind = "inserted"
 ChangeKindReplaced  ChangeKind = "replaced"
 ChangeKindUnchanged ChangeKind = "unchanged"
)

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

type StateFileError struct {
 File StateFile
 Kind StateFileErrorKind
 Err  error
}

func (e StateFileError) Error() string {
 return string(e.File) + " " + string(e.Kind) + ": " + e.Err.Error()
}

func (e StateFileError) Unwrap() error {
 return e.Err
}
```

- [ ] **Step 4: Implement manifest validation and upsert behavior**

```go
package repositorystate

import (
 "fmt"
 "reflect"
)

func ValidateManifest(m Manifest) error {
 seen := map[string]struct{}{}

 for _, approved := range m.TrustPolicy.ApprovedSources {
  if err := validateSourceIdentity(approved); err != nil {
   return fmt.Errorf("trust policy approved source: %w", err)
  }
 }

 for _, decl := range m.Declarations {
  if err := validateSourceIdentity(decl.Source); err != nil {
   return fmt.Errorf("declaration source: %w", err)
  }
  if err := validateDeclarationTarget(decl.Target); err != nil {
   return err
  }

  key := declarationKey(decl)
  if _, ok := seen[key]; ok {
   return fmt.Errorf("duplicate declaration for %s", key)
  }
  seen[key] = struct{}{}
 }

 return nil
}

func (m Manifest) UpsertDeclaration(decl Declaration) (Manifest, ChangeKind) {
 next := Manifest{
  TrustPolicy: TrustPolicy{
   ApprovedSources: append([]SourceIdentity(nil), m.TrustPolicy.ApprovedSources...),
  },
  Declarations: append([]Declaration(nil), m.Declarations...),
 }

 key := declarationKey(decl)
 for i, existing := range next.Declarations {
  if declarationKey(existing) != key {
   continue
  }
  if reflect.DeepEqual(existing, decl) {
   return next, ChangeKindUnchanged
  }
  next.Declarations[i] = decl
  return next, ChangeKindReplaced
 }

 next.Declarations = append(next.Declarations, decl)
 return next, ChangeKindInserted
}

func validateSourceIdentity(source SourceIdentity) error {
 if source.Type == "" {
  return fmt.Errorf("source type is required")
 }
 if source.Name == "" {
  return fmt.Errorf("source name is required")
 }
 return nil
}

func validateDeclarationTarget(target DeclarationTarget) error {
 switch target.Scope {
 case DeclarationScopeArtifact:
  if target.Artifact == "" {
   return fmt.Errorf("artifact target requires artifact name")
  }
 case DeclarationScopeSource:
  if target.Artifact != "" {
   return fmt.Errorf("source target must not include artifact name")
  }
 default:
  return fmt.Errorf("declaration scope is required")
 }
 return nil
}

func declarationKey(decl Declaration) string {
 return decl.Source.Type + "\x00" + decl.Source.Name + "\x00" + string(decl.Target.Scope) + "\x00" + decl.Target.Artifact
}
```

- [ ] **Step 5: Implement lockfile validation and upsert behavior**

```go
package repositorystate

import "fmt"

func ValidateLockfile(l Lockfile) error {
 seen := map[string]struct{}{}

 for _, res := range l.Resolutions {
  if err := validateSourceIdentity(res.Source); err != nil {
   return fmt.Errorf("resolution source: %w", err)
  }
  if res.ResolvedVersion == "" {
   return fmt.Errorf("resolved source version is required")
  }
  if res.Artifact.Name == "" {
   return fmt.Errorf("artifact name is required")
  }
  if res.Artifact.Version == "" {
   return fmt.Errorf("artifact version is required")
  }

  key := resolutionKey(res)
  if _, ok := seen[key]; ok {
   return fmt.Errorf("duplicate resolution for %s", key)
  }
  seen[key] = struct{}{}
 }

 return nil
}

func (l Lockfile) UpsertResolution(res Resolution) (Lockfile, ChangeKind) {
 next := Lockfile{
  Resolutions: append([]Resolution(nil), l.Resolutions...),
 }

 key := resolutionKey(res)
 for i, existing := range next.Resolutions {
  if resolutionKey(existing) != key {
   continue
  }
  if existing == res {
   return next, ChangeKindUnchanged
  }
  next.Resolutions[i] = res
  return next, ChangeKindReplaced
 }

 next.Resolutions = append(next.Resolutions, res)
 return next, ChangeKindInserted
}

func resolutionKey(res Resolution) string {
 return res.Source.Type + "\x00" + res.Source.Name + "\x00" + res.Artifact.Name
}
```

- [ ] **Step 6: Run the repository-state rule tests**

Run:

```bash
go test ./internal/repositorystate -run 'Test(Manifest|Lockfile)' -v
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/repositorystate
```

- [ ] **Step 7: Review the schema choices before file IO**

Confirm these decisions are still intentional before moving on:

- Manifest and lockfile keep separate domain types.
- Target scope is explicit in memory and will stay explicit in YAML.
- Duplicate replacement happens only through `Upsert...`; duplicate on-disk state remains invalid.

### Task 2: Add the filesystem store and manifest YAML contract

**Files:**

- Create: `internal/repositorystate/store.go`
- Create: `internal/repositorystate/store_test.go`

- [ ] **Step 1: Write the failing manifest store tests**

```go
package repositorystate

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "reflect"
 "testing"
)

func requireStateFileError(t *testing.T, err error, file StateFile, kind StateFileErrorKind) {
 t.Helper()

 var stateErr StateFileError
 if !errors.As(err, &stateErr) {
  t.Fatalf("error = %T, want StateFileError", err)
 }
 if stateErr.File != file || stateErr.Kind != kind {
  t.Fatalf("StateFileError = %#v, want %s/%s", stateErr, file, kind)
 }
}

func TestStoreLoadManifestReturnsNotFoundForMissingFile(t *testing.T) {
 store := NewStore()

 _, err := store.LoadManifest(context.Background(), t.TempDir())
 if err == nil {
  t.Fatal("LoadManifest() error = nil, want StateFileError")
 }
 requireStateFileError(t, err, StateFileManifest, StateFileErrorNotFound)
}

func TestStoreManifestRoundTripPreservesSchemaAndSortOrder(t *testing.T) {
 root := t.TempDir()
 store := NewStore()

 manifest := Manifest{
  TrustPolicy: TrustPolicy{
   ApprovedSources: []SourceIdentity{
    {Type: "git", Name: "company/platform"},
    {Type: "file", Name: "local-example-source"},
   },
  },
  Declarations: []Declaration{
   {
    Source: SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "base-readme"},
    Input:  &SourceInput{Locator: "/tmp/example"},
   },
   {
    Source: SourceIdentity{Type: "git", Name: "company/platform"},
    Target: DeclarationTarget{Scope: DeclarationScopeSource},
   },
  },
 }

 if err := store.WriteManifest(context.Background(), root, manifest); err != nil {
  t.Fatalf("WriteManifest() error = %v", err)
 }

 bytes, err := os.ReadFile(filepath.Join(root, ManifestFileName))
 if err != nil {
  t.Fatalf("ReadFile() error = %v", err)
 }
 want := "" +
  "schema_version: 1\n" +
  "trust_policy:\n" +
  "  approved_sources:\n" +
  "    - type: file\n" +
  "      name: local-example-source\n" +
  "    - type: git\n" +
  "      name: company/platform\n" +
  "declarations:\n" +
  "  - source:\n" +
  "      type: file\n" +
  "      name: local-example-source\n" +
  "    target:\n" +
  "      scope: artifact\n" +
  "      artifact: base-readme\n" +
  "    input:\n" +
  "      locator: /tmp/example\n" +
  "  - source:\n" +
  "      type: git\n" +
  "      name: company/platform\n" +
  "    target:\n" +
  "      scope: source\n"
 if got := string(bytes); got != want {
  t.Fatalf("manifest file = %q, want %q", got, want)
 }

 loaded, err := store.LoadManifest(context.Background(), root)
 if err != nil {
  t.Fatalf("LoadManifest() error = %v", err)
 }
 wantManifest := Manifest{
  TrustPolicy: TrustPolicy{
   ApprovedSources: []SourceIdentity{
    {Type: "file", Name: "local-example-source"},
    {Type: "git", Name: "company/platform"},
   },
  },
  Declarations: []Declaration{
   {
    Source: SourceIdentity{Type: "file", Name: "local-example-source"},
    Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "base-readme"},
    Input:  &SourceInput{Locator: "/tmp/example"},
   },
   {
    Source: SourceIdentity{Type: "git", Name: "company/platform"},
    Target: DeclarationTarget{Scope: DeclarationScopeSource},
   },
  },
 }
 if !reflect.DeepEqual(loaded, wantManifest) {
  t.Fatalf("LoadManifest() = %#v, want %#v", loaded, wantManifest)
 }
}

func TestStoreLoadManifestTreatsEmptyAndInvalidFilesAsInvalidFormat(t *testing.T) {
 root := t.TempDir()
 store := NewStore()
 path := filepath.Join(root, ManifestFileName)

 if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err := store.LoadManifest(context.Background(), root)
 if err == nil {
  t.Fatal("LoadManifest() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)

 if err := os.WriteFile(path, []byte("schema_version: 2\n"), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err = store.LoadManifest(context.Background(), root)
 if err == nil {
  t.Fatal("LoadManifest() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)

 if err := os.WriteFile(path, []byte("schema_version: [\n"), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err = store.LoadManifest(context.Background(), root)
 if err == nil {
  t.Fatal("LoadManifest() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)
}

func TestStoreLoadManifestRejectsDuplicateOnDiskState(t *testing.T) {
 root := t.TempDir()
 store := NewStore()
 path := filepath.Join(root, ManifestFileName)

 content := "" +
  "schema_version: 1\n" +
  "declarations:\n" +
  "  - source:\n" +
  "      type: file\n" +
  "      name: local-example-source\n" +
  "    target:\n" +
  "      scope: artifact\n" +
  "      artifact: base-readme\n" +
  "  - source:\n" +
  "      type: file\n" +
  "      name: local-example-source\n" +
  "    target:\n" +
  "      scope: artifact\n" +
  "      artifact: base-readme\n"
 if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err := store.LoadManifest(context.Background(), root)
 if err == nil {
  t.Fatal("LoadManifest() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)
}
```

- [ ] **Step 2: Run the manifest store tests to verify they fail**

Run:

```bash
go test ./internal/repositorystate -run 'TestStore.*Manifest' -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/repositorystate [build failed]
```

- [ ] **Step 3: Implement the store type, filenames, and manifest DTO conversion**

```go
package repositorystate

import (
 "bytes"
 "context"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "sort"
 "strings"

 "gopkg.in/yaml.v3"
)

const (
 supportedSchemaVersion = 1
 ManifestFileName       = "talby-artifacts.yaml"
 LockfileFileName       = "talby-artifacts.lock.yaml"
)

type Store interface {
 LoadManifest(context.Context, string) (Manifest, error)
 WriteManifest(context.Context, string, Manifest) error
 LoadLockfile(context.Context, string) (Lockfile, error)
 WriteLockfile(context.Context, string, Lockfile) error
}

type fileStore struct{}

func NewStore() fileStore {
 return fileStore{}
}

type manifestDocument struct {
 SchemaVersion int                      `yaml:"schema_version"`
 TrustPolicy   manifestTrustPolicyDTO   `yaml:"trust_policy,omitempty"`
 Declarations  []manifestDeclarationDTO `yaml:"declarations,omitempty"`
}

type manifestTrustPolicyDTO struct {
 ApprovedSources []SourceIdentity `yaml:"approved_sources,omitempty"`
}

type manifestDeclarationDTO struct {
 Source SourceIdentity     `yaml:"source"`
 Target manifestTargetDTO  `yaml:"target"`
 Input  *manifestInputDTO  `yaml:"input,omitempty"`
}

type manifestTargetDTO struct {
 Scope    DeclarationScope `yaml:"scope"`
 Artifact string           `yaml:"artifact,omitempty"`
}

type manifestInputDTO struct {
 Locator string `yaml:"locator,omitempty"`
 Version string `yaml:"version,omitempty"`
}

func (fileStore) LoadManifest(_ context.Context, root string) (Manifest, error) {
 bytes, err := os.ReadFile(filepath.Join(root, ManifestFileName))
 if err != nil {
  if errors.Is(err, os.ErrNotExist) {
   return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorNotFound, Err: err}
  }
  return Manifest{}, err
 }
 if strings.TrimSpace(string(bytes)) == "" {
  return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("file is empty")}
 }

 var doc manifestDocument
 if err := yaml.Unmarshal(bytes, &doc); err != nil {
  return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: err}
 }
 if doc.SchemaVersion != supportedSchemaVersion {
  return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("schema_version must be %d", supportedSchemaVersion)}
 }

 manifest := Manifest{
  TrustPolicy:  TrustPolicy{ApprovedSources: append([]SourceIdentity(nil), doc.TrustPolicy.ApprovedSources...)},
  Declarations: make([]Declaration, 0, len(doc.Declarations)),
 }
 for _, dto := range doc.Declarations {
  manifest.Declarations = append(manifest.Declarations, Declaration{
   Source: dto.Source,
   Target: DeclarationTarget{Scope: dto.Target.Scope, Artifact: dto.Target.Artifact},
   Input:  sourceInputFromDTO(dto.Input),
  })
 }
 if err := ValidateManifest(manifest); err != nil {
  return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: err}
 }

 return manifest, nil
}

func (fileStore) WriteManifest(_ context.Context, root string, manifest Manifest) error {
 if err := ValidateManifest(manifest); err != nil {
  return err
 }

 doc := manifestDocument{
  SchemaVersion: supportedSchemaVersion,
  TrustPolicy: manifestTrustPolicyDTO{
   ApprovedSources: append([]SourceIdentity(nil), manifest.TrustPolicy.ApprovedSources...),
  },
  Declarations: make([]manifestDeclarationDTO, 0, len(manifest.Declarations)),
 }
 sort.Slice(doc.TrustPolicy.ApprovedSources, func(i, j int) bool {
  left := doc.TrustPolicy.ApprovedSources[i]
  right := doc.TrustPolicy.ApprovedSources[j]
  if left.Type != right.Type {
   return left.Type < right.Type
  }
  return left.Name < right.Name
 })

 declarations := append([]Declaration(nil), manifest.Declarations...)
 sort.Slice(declarations, func(i, j int) bool {
  return declarationKey(declarations[i]) < declarationKey(declarations[j])
 })
 for _, decl := range declarations {
 doc.Declarations = append(doc.Declarations, manifestDeclarationDTO{
  Source: decl.Source,
  Target: manifestTargetDTO{Scope: decl.Target.Scope, Artifact: decl.Target.Artifact},
  Input:  manifestInputFromDomain(decl.Input),
 })
}

 bytes, err := encodeYAML(doc)
 if err != nil {
  return err
 }
 return os.WriteFile(filepath.Join(root, ManifestFileName), bytes, 0o644)
}

func encodeYAML(value any) ([]byte, error) {
 var buffer bytes.Buffer
 encoder := yaml.NewEncoder(&buffer)
 encoder.SetIndent(2)
 if err := encoder.Encode(value); err != nil {
  _ = encoder.Close()
  return nil, err
 }
 if err := encoder.Close(); err != nil {
  return nil, err
 }
 return buffer.Bytes(), nil
}

func sourceInputFromDTO(input *manifestInputDTO) *SourceInput {
 if input == nil {
  return nil
 }
 return &SourceInput{Locator: input.Locator, Version: input.Version}
}

func manifestInputFromDomain(input *SourceInput) *manifestInputDTO {
 if input == nil {
  return nil
 }
 return &manifestInputDTO{Locator: input.Locator, Version: input.Version}
}
```

- [ ] **Step 4: Run the manifest store tests**

Run:

```bash
go test ./internal/repositorystate -run 'TestStore.*Manifest' -v
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/repositorystate
```

### Task 3: Add lockfile YAML persistence and duplicate-on-disk rejection

**Files:**

- Modify: `internal/repositorystate/store.go`
- Modify: `internal/repositorystate/store_test.go`

- [ ] **Step 1: Write the failing lockfile store tests**

```go
func TestStoreLoadLockfileReturnsNotFoundForMissingFile(t *testing.T) {
 store := NewStore()

 _, err := store.LoadLockfile(context.Background(), t.TempDir())
 if err == nil {
  t.Fatal("LoadLockfile() error = nil, want StateFileError")
 }
 requireStateFileError(t, err, StateFileLockfile, StateFileErrorNotFound)
}

func TestStoreLockfileRoundTripPreservesSchemaAndSortOrder(t *testing.T) {
 root := t.TempDir()
 store := NewStore()

 lockfile := Lockfile{
  Resolutions: []Resolution{
   {
    Source:          SourceIdentity{Type: "git", Name: "company/platform"},
    ResolvedVersion: "git-sha-abc123",
    Artifact:        ArtifactResolution{Name: "ci-github", Version: "2.4.0"},
   },
   {
    Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
    ResolvedVersion: "local-snapshot-001",
    Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
   },
  },
 }

 if err := store.WriteLockfile(context.Background(), root, lockfile); err != nil {
  t.Fatalf("WriteLockfile() error = %v", err)
 }

 bytes, err := os.ReadFile(filepath.Join(root, LockfileFileName))
 if err != nil {
  t.Fatalf("ReadFile() error = %v", err)
 }
 want := "" +
  "schema_version: 1\n" +
  "resolutions:\n" +
  "  - source:\n" +
  "      type: file\n" +
  "      name: local-example-source\n" +
  "    resolved_version: local-snapshot-001\n" +
  "    artifact:\n" +
  "      name: base-readme\n" +
  "      version: 1.0.0\n" +
  "  - source:\n" +
  "      type: git\n" +
  "      name: company/platform\n" +
  "    resolved_version: git-sha-abc123\n" +
  "    artifact:\n" +
  "      name: ci-github\n" +
  "      version: 2.4.0\n"
 if got := string(bytes); got != want {
  t.Fatalf("lockfile = %q, want %q", got, want)
 }

 loaded, err := store.LoadLockfile(context.Background(), root)
 if err != nil {
  t.Fatalf("LoadLockfile() error = %v", err)
 }
 wantLockfile := Lockfile{
  Resolutions: []Resolution{
   {
    Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
    ResolvedVersion: "local-snapshot-001",
    Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
   },
   {
    Source:          SourceIdentity{Type: "git", Name: "company/platform"},
    ResolvedVersion: "git-sha-abc123",
    Artifact:        ArtifactResolution{Name: "ci-github", Version: "2.4.0"},
   },
  },
 }
 if !reflect.DeepEqual(loaded, wantLockfile) {
  t.Fatalf("LoadLockfile() = %#v, want %#v", loaded, wantLockfile)
 }
}

func TestStoreLoadLockfileTreatsEmptyAndInvalidFilesAsInvalidFormat(t *testing.T) {
 root := t.TempDir()
 store := NewStore()
 path := filepath.Join(root, LockfileFileName)

 if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err := store.LoadLockfile(context.Background(), root)
 if err == nil {
  t.Fatal("LoadLockfile() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)

 if err := os.WriteFile(path, []byte("schema_version: 2\n"), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err = store.LoadLockfile(context.Background(), root)
 if err == nil {
  t.Fatal("LoadLockfile() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)

 if err := os.WriteFile(path, []byte("schema_version: [\n"), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err = store.LoadLockfile(context.Background(), root)
 if err == nil {
  t.Fatal("LoadLockfile() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)
}

func TestStoreLoadLockfileRejectsDuplicateOnDiskState(t *testing.T) {
 root := t.TempDir()
 store := NewStore()
 path := filepath.Join(root, LockfileFileName)

 content := "" +
  "schema_version: 1\n" +
  "resolutions:\n" +
  "  - source:\n" +
  "      type: file\n" +
  "      name: local-example-source\n" +
  "    resolved_version: local-snapshot-001\n" +
  "    artifact:\n" +
  "      name: base-readme\n" +
  "      version: 1.0.0\n" +
  "  - source:\n" +
  "      type: file\n" +
  "      name: local-example-source\n" +
  "    resolved_version: local-snapshot-002\n" +
  "    artifact:\n" +
  "      name: base-readme\n" +
  "      version: 1.1.0\n"
 if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
  t.Fatalf("WriteFile() error = %v", err)
 }

 _, err := store.LoadLockfile(context.Background(), root)
 if err == nil {
  t.Fatal("LoadLockfile() error = nil, want invalid_format error")
 }
 requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)
}
```

- [ ] **Step 2: Run the lockfile store tests to verify they fail**

Run:

```bash
go test ./internal/repositorystate -run 'TestStore.*Lockfile' -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/repositorystate [build failed]
```

- [ ] **Step 3: Extend the store with lockfile DTOs and read/write methods**

Replace the Task 2 `NewStore` function with the final interface-returning signature now that `fileStore` will implement all store methods.

```go
func NewStore() Store {
 return fileStore{}
}

type lockfileDocument struct {
 SchemaVersion int                     `yaml:"schema_version"`
 Resolutions   []lockfileResolutionDTO `yaml:"resolutions,omitempty"`
}

type lockfileResolutionDTO struct {
 Source          SourceIdentity         `yaml:"source"`
 ResolvedVersion string                 `yaml:"resolved_version"`
 Artifact        lockfileArtifactDTO    `yaml:"artifact"`
}

type lockfileArtifactDTO struct {
 Name    string `yaml:"name"`
 Version string `yaml:"version"`
}

func (fileStore) LoadLockfile(_ context.Context, root string) (Lockfile, error) {
 bytes, err := os.ReadFile(filepath.Join(root, LockfileFileName))
 if err != nil {
  if errors.Is(err, os.ErrNotExist) {
   return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorNotFound, Err: err}
  }
  return Lockfile{}, err
 }
 if strings.TrimSpace(string(bytes)) == "" {
  return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("file is empty")}
 }

 var doc lockfileDocument
 if err := yaml.Unmarshal(bytes, &doc); err != nil {
  return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: err}
 }
 if doc.SchemaVersion != supportedSchemaVersion {
  return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("schema_version must be %d", supportedSchemaVersion)}
 }

 lockfile := Lockfile{
  Resolutions: make([]Resolution, 0, len(doc.Resolutions)),
 }
 for _, dto := range doc.Resolutions {
  lockfile.Resolutions = append(lockfile.Resolutions, Resolution{
   Source:          dto.Source,
   ResolvedVersion: dto.ResolvedVersion,
   Artifact: ArtifactResolution{
    Name:    dto.Artifact.Name,
    Version: dto.Artifact.Version,
   },
  })
 }
 if err := ValidateLockfile(lockfile); err != nil {
  return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: err}
 }

 return lockfile, nil
}

func (fileStore) WriteLockfile(_ context.Context, root string, lockfile Lockfile) error {
 if err := ValidateLockfile(lockfile); err != nil {
  return err
 }

 doc := lockfileDocument{
  SchemaVersion: supportedSchemaVersion,
  Resolutions:   make([]lockfileResolutionDTO, 0, len(lockfile.Resolutions)),
 }

 resolutions := append([]Resolution(nil), lockfile.Resolutions...)
 sort.Slice(resolutions, func(i, j int) bool {
  return resolutionKey(resolutions[i]) < resolutionKey(resolutions[j])
 })
 for _, res := range resolutions {
  doc.Resolutions = append(doc.Resolutions, lockfileResolutionDTO{
   Source:          res.Source,
   ResolvedVersion: res.ResolvedVersion,
   Artifact: lockfileArtifactDTO{
    Name:    res.Artifact.Name,
    Version: res.Artifact.Version,
   },
  })
 }

 bytes, err := encodeYAML(doc)
 if err != nil {
  return err
 }
 return os.WriteFile(filepath.Join(root, LockfileFileName), bytes, 0o644)
}
```

- [ ] **Step 4: Run the full repository-state package tests**

Run:

```bash
go test ./internal/repositorystate -v
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/repositorystate
```

### Task 4: Add install-adjacent mapping helpers and the cross-module contract test

**Files:**

- Create: `internal/install/repository_state.go`
- Create: `internal/install/repository_state_test.go`

- [ ] **Step 1: Write the failing install mapping tests**

```go
package install

import (
 "reflect"
 "testing"

 "github.com/talby/talby-bootstrap/internal/repositorystate"
 "github.com/talby/talby-bootstrap/internal/source"
)

func TestRepositoryStateDeclarationFromInstallResult(t *testing.T) {
 req := Request{
  Source:   source.Ref{Type: "file", Locator: "/tmp/example", Version: "v1.2.3"},
  Artifact: "base-readme",
 }
 result := Result{
  Source: source.Identity{
   Type:    "file",
   Name:    "local-example-source",
   Version: "local-snapshot-001",
  },
  Artifact: source.ArtifactDescriptor{
   Name:    "base-readme",
   Version: "1.0.0",
   Path:    "artifacts/base-readme",
  },
 }

 got := ManifestDeclaration(req, result)
 want := repositorystate.Declaration{
  Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
  Target: repositorystate.DeclarationTarget{
   Scope:    repositorystate.DeclarationScopeArtifact,
   Artifact: "base-readme",
  },
  Input: &repositorystate.SourceInput{
   Locator: "/tmp/example",
   Version: "v1.2.3",
  },
 }
 if !reflect.DeepEqual(got, want) {
  t.Fatalf("ManifestDeclaration() = %#v, want %#v", got, want)
 }
}

func TestRepositoryStateResolutionFromInstallResult(t *testing.T) {
 result := Result{
  Source: source.Identity{
   Type:    "file",
   Name:    "local-example-source",
   Version: "local-snapshot-001",
  },
  Artifact: source.ArtifactDescriptor{
   Name:    "base-readme",
   Version: "1.0.0",
  },
 }

 got := LockfileResolution(result)
 want := repositorystate.Resolution{
  Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
  ResolvedVersion: "local-snapshot-001",
  Artifact: repositorystate.ArtifactResolution{
   Name:    "base-readme",
   Version: "1.0.0",
  },
 }
 if !reflect.DeepEqual(got, want) {
  t.Fatalf("LockfileResolution() = %#v, want %#v", got, want)
 }
}
```

- [ ] **Step 2: Run the install mapping tests to verify they fail**

Run:

```bash
go test ./internal/install -run 'TestRepositoryState' -v
```

Expected:

```text
FAIL github.com/talby/talby-bootstrap/internal/install [build failed]
```

- [ ] **Step 3: Implement the mapping helpers without changing install behavior**

```go
package install

import "github.com/talby/talby-bootstrap/internal/repositorystate"

func ManifestDeclaration(req Request, result Result) repositorystate.Declaration {
 return repositorystate.Declaration{
  Source: repositorystate.SourceIdentity{
   Type: result.Source.Type,
   Name: result.Source.Name,
  },
  Target: repositorystate.DeclarationTarget{
   Scope:    repositorystate.DeclarationScopeArtifact,
   Artifact: result.Artifact.Name,
  },
  Input: &repositorystate.SourceInput{
   Locator: req.Source.Locator,
   Version: req.Source.Version,
  },
 }
}

func LockfileResolution(result Result) repositorystate.Resolution {
 return repositorystate.Resolution{
  Source: repositorystate.SourceIdentity{
   Type: result.Source.Type,
   Name: result.Source.Name,
  },
  ResolvedVersion: result.Source.Version,
  Artifact: repositorystate.ArtifactResolution{
   Name:    result.Artifact.Name,
   Version: result.Artifact.Version,
  },
 }
}
```

- [ ] **Step 4: Run the repository-state contract tests**

Run:

```bash
go test ./internal/install -run 'TestRepositoryState' -v
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/install
```

- [ ] **Step 5: Check the boundary stays one-way**

Verify `internal/repositorystate` imports no package from `internal/install` or `internal/source`. The dependency direction must stay:

```text
internal/install -> internal/source
internal/install -> internal/repositorystate
internal/repositorystate -> bytes/context/errors/fmt/os/path/filepath/reflect/sort/strings/yaml
```

### Task 5: Format, verify, and review the slice

**Files:**

- Modify: all files created in Tasks 1-4

- [ ] **Step 1: Format the touched Go files**

Run:

```bash
gofmt -w internal/repositorystate/*.go internal/install/repository_state*.go
```

Expected:

```text
```

- [ ] **Step 2: Run the focused Go test suites**

Run:

```bash
go test ./internal/repositorystate ./internal/install -v
```

Expected:

```text
ok   github.com/talby/talby-bootstrap/internal/repositorystate
ok   github.com/talby/talby-bootstrap/internal/install
```

- [ ] **Step 3: Run the repository Go checks**

Run:

```bash
just check-go
```

Expected:

```text
go test ./...
...
```

- [ ] **Step 4: Review against the spec before handing off**

Confirm all of these are true:

- `internal/repositorystate` owns only persisted state and pure helpers.
- Missing files return `StateFileError` with `not_found`.
- Empty files, malformed YAML, unsupported schema versions, and validation failures return `invalid_format`.
- Upserts distinguish `inserted`, `replaced`, and `unchanged`.
- The package validates source-scoped versus artifact-scoped manifest declarations.
- The package rejects duplicate on-disk state while allowing deterministic replacement through upsert helpers.
- The only install-adjacent code is mapping helpers and tests; no write behavior is activated.

## Self-review

- Spec coverage: the plan covers domain types, typed errors, YAML contracts, validation, not-found versus invalid-format load semantics, upsert behavior, deterministic sorting, and the one required cross-module mapping test.
- Placeholder scan: no executable step uses placeholder wording or vague validation instructions without exact tests or code.
- Type consistency: `SourceIdentity`, `DeclarationTarget`, `Resolution`, `StateFileError`, `ManifestDeclaration`, and `LockfileResolution` are named consistently across all tasks.
