package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
}
