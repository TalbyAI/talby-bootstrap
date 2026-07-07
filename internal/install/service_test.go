package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
	"github.com/talby/talby-bootstrap/internal/source/file"
)

type fakeRegistry struct {
	source           source.Source
	err              error
	lookupSourceType string
	lookupCallCount  int
}

func (r *fakeRegistry) Lookup(sourceType string) (source.Source, error) {
	r.lookupSourceType = sourceType
	r.lookupCallCount++
	return r.source, r.err
}

type fakeSource struct {
	resolved         source.ResolvedSource
	err              error
	resolveRequest   source.ResolveRequest
	resolveCallCount int
}

type failingManagedStateStore struct {
	repositorystate.Store
	writeMaterializationRecordErr error
}

func (s failingManagedStateStore) WriteMaterializationRecord(_ context.Context, _ string, _ repositorystate.MaterializationRecord) error {
	return s.writeMaterializationRecordErr
}

func (f *fakeSource) Capabilities() source.Capabilities { return source.Capabilities{} }

func (f *fakeSource) Resolve(_ context.Context, req source.ResolveRequest) (source.ResolvedSource, error) {
	f.resolveRequest = req
	f.resolveCallCount++
	return f.resolved, f.err
}

func TestInstallReturnsSelectedArtifact(t *testing.T) {
	sourceImpl := &fakeSource{
		resolved: source.ResolvedSource{
			Identity: source.Identity{Type: "file", Name: "local-example-source", Version: "local-snapshot-001"},
			Artifacts: []source.ArtifactDescriptor{
				{Name: "base-readme", Version: "1.0.0", Path: "artifacts/base-readme"},
			},
		},
	}
	registry := &fakeRegistry{
		source: sourceImpl,
	}
	svc := NewService(registry, repositorystate.NewStore())
	req := Request{
		Source:   source.Ref{Type: "file", Locator: "/tmp/example", Version: "v1.2.3"},
		Artifact: "base-readme",
	}

	got, err := svc.Install(context.Background(), req)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got.Source.Name != "local-example-source" {
		t.Fatalf("Source.Name = %q, want local-example-source", got.Source.Name)
	}
	if got.Artifact.Name != "base-readme" {
		t.Fatalf("Artifact.Name = %q, want base-readme", got.Artifact.Name)
	}
	if got, want := registry.lookupCallCount, 1; got != want {
		t.Fatalf("Lookup() call count = %d, want %d", got, want)
	}
	if got, want := registry.lookupSourceType, req.Source.Type; got != want {
		t.Fatalf("Lookup() source type = %q, want %q", got, want)
	}
	if got, want := sourceImpl.resolveCallCount, 1; got != want {
		t.Fatalf("Resolve() call count = %d, want %d", got, want)
	}
	if got, want := sourceImpl.resolveRequest.Ref, req.Source; got != want {
		t.Fatalf("Resolve() request ref = %#v, want %#v", got, want)
	}
}

func TestInstallSurfacesResolveError(t *testing.T) {
	sourceImpl := &fakeSource{
		err: fmt.Errorf("resolve failed"),
	}
	registry := &fakeRegistry{
		source: sourceImpl,
	}
	svc := NewService(registry, repositorystate.NewStore())
	req := Request{
		Source:   source.Ref{Type: "file", Locator: "/tmp/example", Version: "v1.2.3"},
		Artifact: "base-readme",
	}

	_, err := svc.Install(context.Background(), req)
	if err == nil {
		t.Fatal("Install() error = nil, want resolve error")
	}
	if got, want := err.Error(), "resolve failed"; got != want {
		t.Fatalf("Install() error = %q, want %q", got, want)
	}
	if got, want := registry.lookupSourceType, req.Source.Type; got != want {
		t.Fatalf("Lookup() source type = %q, want %q", got, want)
	}
	if got, want := sourceImpl.resolveRequest.Ref, req.Source; got != want {
		t.Fatalf("Resolve() request ref = %#v, want %#v", got, want)
	}
}

func TestInstallRejectsZeroArtifactTarget(t *testing.T) {
	svc := NewService(&fakeRegistry{
		source: &fakeSource{
			resolved: source.ResolvedSource{
				Artifacts: []source.ArtifactDescriptor{},
			},
		},
	}, repositorystate.NewStore())

	_, err := svc.Install(context.Background(), Request{
		Source:   source.Ref{Type: "file", Locator: "/tmp/example"},
		Artifact: "",
	})
	if err == nil {
		t.Fatal("Install() error = nil, want zero-artifact error")
	}
	if got, want := err.Error(), "install target must resolve to exactly one artifact"; got != want {
		t.Fatalf("Install() error = %q, want %q", got, want)
	}
}

func TestInstallRejectsAmbiguousArtifactTarget(t *testing.T) {
	svc := NewService(&fakeRegistry{
		source: &fakeSource{
			resolved: source.ResolvedSource{
				Artifacts: []source.ArtifactDescriptor{
					{Name: "base-readme"},
					{Name: "ci-github"},
				},
			},
		},
	}, repositorystate.NewStore())

	_, err := svc.Install(context.Background(), Request{
		Source:   source.Ref{Type: "file", Locator: "/tmp/example"},
		Artifact: "",
	})
	if err == nil {
		t.Fatal("Install() error = nil, want ambiguity error")
	}
	if got, want := err.Error(), "install target must resolve to exactly one artifact"; got != want {
		t.Fatalf("Install() error = %q, want %q", got, want)
	}
}

func TestInstallRejectsUnknownSourceType(t *testing.T) {
	registry := &fakeRegistry{err: fmt.Errorf("unsupported source type %q", "git")}
	svc := NewService(registry, repositorystate.NewStore())

	req := Request{
		Source:   source.Ref{Type: "git", Locator: "github.com/example/library"},
		Artifact: "base-readme",
	}
	_, err := svc.Install(context.Background(), req)
	if err == nil {
		t.Fatal("Install() error = nil, want source lookup error")
	}
	if !strings.Contains(err.Error(), `unsupported source type "git"`) {
		t.Fatalf("error = %q, want unsupported source type", err)
	}
	if got, want := registry.lookupSourceType, req.Source.Type; got != want {
		t.Fatalf("Lookup() source type = %q, want %q", got, want)
	}
}

func TestInstallRejectsEmptySourceType(t *testing.T) {
	svc := NewService(&fakeRegistry{}, repositorystate.NewStore())

	_, err := svc.Install(context.Background(), Request{
		Source: source.Ref{Locator: "/tmp/example"},
	})
	if err == nil {
		t.Fatal("Install() error = nil, want missing source type error")
	}
	if got, want := err.Error(), "source type is required"; got != want {
		t.Fatalf("Install() error = %q, want %q", got, want)
	}
}

func TestInstallRejectsEmptySourceLocator(t *testing.T) {
	svc := NewService(&fakeRegistry{}, repositorystate.NewStore())

	_, err := svc.Install(context.Background(), Request{
		Source: source.Ref{Type: "file"},
	})
	if err == nil {
		t.Fatal("Install() error = nil, want missing source locator error")
	}
	if got, want := err.Error(), "source locator is required"; got != want {
		t.Fatalf("Install() error = %q, want %q", got, want)
	}
}

func TestInstallRejectsMissingNamedArtifact(t *testing.T) {
	svc := NewService(&fakeRegistry{
		source: &fakeSource{
			resolved: source.ResolvedSource{
				Artifacts: []source.ArtifactDescriptor{
					{Name: "base-readme"},
				},
			},
		},
	}, repositorystate.NewStore())

	_, err := svc.Install(context.Background(), Request{
		Source:   source.Ref{Type: "file", Locator: "/tmp/example"},
		Artifact: "ci-github",
	})
	if err == nil {
		t.Fatal("Install() error = nil, want missing artifact error")
	}
	if got, want := err.Error(), `artifact "ci-github" was not found`; got != want {
		t.Fatalf("Install() error = %q, want %q", got, want)
	}
}

func TestInstallWithRealFileSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\nsteps:\n  - type: file\n    path: README.md\n    source: README.md\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "README.md"), "hello\n")

	svc := NewService(source.NewStaticRegistry(map[string]source.Source{
		"file": file.New(),
	}), repositorystate.NewStore())

	got, err := svc.Install(context.Background(), Request{
		Source:   source.Ref{Type: "file", Locator: root},
		Artifact: "base-readme",
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got.Source.Type != "file" {
		t.Fatalf("Source.Type = %q, want file", got.Source.Type)
	}
	if got.Source.Name != "local-example-source" {
		t.Fatalf("Source.Name = %q, want local-example-source", got.Source.Name)
	}
	if !strings.HasPrefix(got.Source.Version, "local-snapshot-") {
		t.Fatalf("Source.Version = %q, want local-snapshot-*", got.Source.Version)
	}
	if got.Artifact.Name != "base-readme" {
		t.Fatalf("Artifact.Name = %q, want base-readme", got.Artifact.Name)
	}
	if got.Artifact.Version != "1.0.0" {
		t.Fatalf("Artifact.Version = %q, want 1.0.0", got.Artifact.Version)
	}
	if got.Artifact.Path != "artifacts/base-readme" {
		t.Fatalf("Artifact.Path = %q, want artifacts/base-readme", got.Artifact.Path)
	}
}

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

func TestDeclareOnlyInstallWritesManifestAtRepositoryRoot(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "examples")
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
	sourceRoot := filepath.Join(repoRoot, "examples")
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
	writeFile(t, filepath.Join(repoRoot, repositorystate.LockfileFileName), ""+
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

func TestInstallWritesLockfileManagedStateAndManagedFiles(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		repositorystate.NewStore(),
	)

	got, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got.Change != ChangeInstalled {
		t.Fatalf("Change = %q, want %q", got.Change, ChangeInstalled)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "README.md" {
		t.Fatalf("Files = %#v, want README.md change", got.Files)
	}
	if _, err := os.Stat(filepath.Join(root, repositorystate.LockfileFileName)); err != nil {
		t.Fatalf("lockfile stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, repositorystate.ManifestFileName)); err != nil {
		t.Fatalf("manifest stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, repositorystate.MaterializationRecordFileName)); err != nil {
		t.Fatalf("managed-state stat error = %v", err)
	}
	gotReadme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile(README.md) error = %v", err)
	}
	if string(gotReadme) != "hello\n" {
		t.Fatalf("README.md = %q, want hello", gotReadme)
	}
}

func TestInstallRemovesNewManagedFilesWhenManagedStateWriteFails(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		failingManagedStateStore{
			Store:                         repositorystate.NewStore(),
			writeMaterializationRecordErr: fmt.Errorf("write managed state failed"),
		},
	)

	_, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	})
	if err == nil {
		t.Fatal("Install() error = nil, want managed state write error")
	}
	if _, statErr := os.Stat(filepath.Join(root, "README.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("README.md stat error = %v, want not exist", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, repositorystate.MaterializationRecordFileName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("managed-state stat error = %v, want not exist", statErr)
	}
}

func TestInstallStopsWhenManagedFileHasDrifted(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		repositorystate.NewStore(),
	)

	if _, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	writeFile(t, filepath.Join(root, "README.md"), "user edit\n")

	_, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	})
	if err == nil {
		t.Fatal("Install() error = nil, want drift conflict")
	}

	var driftErr materialize.DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("error = %T, want DriftError", err)
	}
}

func TestInstallReusesManagedStateForUnchangedReapply(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		repositorystate.NewStore(),
	)

	if _, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	before, err := os.ReadFile(filepath.Join(root, repositorystate.MaterializationRecordFileName))
	if err != nil {
		t.Fatalf("ReadFile(before managed state) error = %v", err)
	}

	got, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	})
	if err != nil {
		t.Fatalf("second Install() error = %v", err)
	}
	if got.Change != ChangeNoOp {
		t.Fatalf("Change = %q, want %q", got.Change, ChangeNoOp)
	}

	after, err := os.ReadFile(filepath.Join(root, repositorystate.MaterializationRecordFileName))
	if err != nil {
		t.Fatalf("ReadFile(after managed state) error = %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("managed state changed on noop:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
	}
}

func TestSyncReappliesManagedFilesFromPersistedState(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		repositorystate.NewStore(),
	)

	if _, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	writeFile(t, filepath.Join(root, "README.md"), "user edit\n")

	_, err := svc.Sync(context.Background(), SyncRequest{Root: root})
	if err == nil {
		t.Fatal("Sync() error = nil, want drift conflict")
	}

	var driftErr materialize.DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("error = %T, want DriftError", err)
	}
}

func TestSyncReturnsNoOpWhenRepositoryAlreadyMatchesPersistedState(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		repositorystate.NewStore(),
	)

	if _, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{Root: root})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if got.Change != ChangeNoOp {
		t.Fatalf("Change = %q, want %q", got.Change, ChangeNoOp)
	}
}

func TestSyncUsesResolvedVersionFromLockfile(t *testing.T) {
	root := t.TempDir()
	store := repositorystate.NewStore()
	if err := store.WriteManifest(context.Background(), root, repositorystate.Manifest{
		Declarations: []repositorystate.Declaration{{
			Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			Target: repositorystate.DeclarationTarget{
				Scope:    repositorystate.DeclarationScopeArtifact,
				Artifact: "base-readme",
			},
			Input: &repositorystate.SourceInput{
				Locator: filepath.Join(root, "source"),
				Version: "manifest-version",
			},
		}},
	}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	if err := store.WriteLockfile(context.Background(), root, repositorystate.Lockfile{
		Resolutions: []repositorystate.Resolution{{
			Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "lock-version",
			Artifact:        repositorystate.ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
		}},
	}); err != nil {
		t.Fatalf("WriteLockfile() error = %v", err)
	}
	if err := store.WriteMaterializationRecord(context.Background(), root, repositorystate.MaterializationRecord{
		Artifacts: []repositorystate.ManagedArtifactRecord{{
			Key: repositorystate.ManagedArtifactKey{
				Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "lock-version",
				Artifact:        "base-readme",
			},
			Files: []repositorystate.ManagedFileRecord{{Path: "README.md", Digest: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2fcbbfc4017"}},
		}},
	}); err != nil {
		t.Fatalf("WriteMaterializationRecord() error = %v", err)
	}

	sourceImpl := &fakeSource{
		resolved: source.ResolvedSource{
			Identity: source.Identity{Type: "file", Name: "local-example-source", Version: "lock-version"},
			Artifacts: []source.ArtifactDescriptor{{
				Name:    "base-readme",
				Version: "1.0.0",
				Steps: []source.MaterializationStep{{
					Type:       "file",
					TargetPath: "README.md",
					SourcePath: filepath.Join(root, "missing-source.txt"),
				}},
			}},
		},
	}
	svc := NewService(&fakeRegistry{source: sourceImpl}, store)

	_, err := svc.Sync(context.Background(), SyncRequest{Root: root})
	if err == nil {
		t.Fatal("Sync() error = nil, want source read error after resolve")
	}
	if got, want := sourceImpl.resolveRequest.Ref.Version, "lock-version"; got != want {
		t.Fatalf("Resolve() request version = %q, want %q", got, want)
	}
}

func TestSyncAllowsApprovedFileSourceOutsideOperationRoot(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := t.TempDir()
	writeInstallFixture(t, sourceRoot)

	store := repositorystate.NewStore()
	resolved, err := file.New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: sourceRoot},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	result := Result{
		Source:   resolved.Identity,
		Artifact: resolved.Artifacts[0],
	}
	if err := store.WriteManifest(context.Background(), repoRoot, repositorystate.Manifest{
		TrustPolicy: repositorystate.TrustPolicy{
			ApprovedSources: []repositorystate.SourceIdentity{{
				Type: resolved.Identity.Type,
				Name: resolved.Identity.Name,
			}},
		},
		Declarations: []repositorystate.Declaration{ManifestDeclaration(Request{
			Source:   source.Ref{Type: "file", Locator: sourceRoot},
			Artifact: "base-readme",
		}, result)},
	}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	if err := store.WriteLockfile(context.Background(), repoRoot, repositorystate.Lockfile{
		Resolutions: []repositorystate.Resolution{LockfileResolution(result)},
	}); err != nil {
		t.Fatalf("WriteLockfile() error = %v", err)
	}
	if err := store.WriteMaterializationRecord(context.Background(), repoRoot, repositorystate.MaterializationRecord{
		Artifacts: []repositorystate.ManagedArtifactRecord{ManagedArtifactRecordFor(result, materialize.Result{
			Changes: []materialize.FileChange{{
				Path:   "README.md",
				Digest: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2fcbbfc4017",
			}},
		})},
	}); err != nil {
		t.Fatalf("WriteMaterializationRecord() error = %v", err)
	}

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		store,
	)

	got, err := svc.Sync(context.Background(), SyncRequest{Root: repoRoot})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if got.Change != ChangeInstalled {
		t.Fatalf("Change = %q, want %q", got.Change, ChangeInstalled)
	}
}

func TestSyncRejectsMissingMaterializationRecordAfterBootstrap(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		repositorystate.NewStore(),
	)

	if _, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, repositorystate.MaterializationRecordFileName)); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	_, err := svc.Sync(context.Background(), SyncRequest{Root: root})
	if err == nil {
		t.Fatal("Sync() error = nil, want missing materialization record error")
	}
	if got, want := err.Error(), "sync requires existing materialization record"; got != want {
		t.Fatalf("Sync() error = %q, want %q", got, want)
	}
}

func TestSyncRejectsManagedArtifactRemoval(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	writeFile(t, filepath.Join(sourceRoot, "artifacts", "extra", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: extra\n  version: 1.0.0\nsteps:\n  - type: file\n    path: extra.txt\n    source: extra.txt\n")
	writeFile(t, filepath.Join(sourceRoot, "artifacts", "extra", "extra.txt"), "extra\n")
	writeFile(t, filepath.Join(sourceRoot, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n  - name: extra\n    path: artifacts/extra\n")

	svc := NewService(
		source.NewStaticRegistry(map[string]source.Source{"file": file.New()}),
		repositorystate.NewStore(),
	)

	if _, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "base-readme",
	}); err != nil {
		t.Fatalf("Install(base-readme) error = %v", err)
	}
	if _, err := svc.Install(context.Background(), Request{
		Root:     root,
		Source:   source.Ref{Type: "file", Locator: sourceRoot},
		Artifact: "extra",
	}); err != nil {
		t.Fatalf("Install(extra) error = %v", err)
	}

	if err := repositorystate.NewStore().WriteManifest(context.Background(), root, repositorystate.Manifest{
		Declarations: []repositorystate.Declaration{{
			Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			Target: repositorystate.DeclarationTarget{
				Scope:    repositorystate.DeclarationScopeArtifact,
				Artifact: "base-readme",
			},
			Input: &repositorystate.SourceInput{Locator: sourceRoot},
		}},
	}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}

	_, err := svc.Sync(context.Background(), SyncRequest{Root: root})
	if err == nil {
		t.Fatal("Sync() error = nil, want managed artifact removal conflict")
	}

	var removalErr ManagedArtifactRemovalError
	if !errors.As(err, &removalErr) {
		t.Fatalf("error = %T, want ManagedArtifactRemovalError", err)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writeInstallFixture(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\nsteps:\n  - type: file\n    path: README.md\n    source: README.md\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "README.md"), "hello\n")
}
