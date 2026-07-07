package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	svc := NewService(registry)
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
	svc := NewService(registry)
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
	})

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
	})

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
	svc := NewService(registry)

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
	svc := NewService(&fakeRegistry{})

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
	svc := NewService(&fakeRegistry{})

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
	})

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
	}))

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
	if got.Source.Version != "local-snapshot-001" {
		t.Fatalf("Source.Version = %q, want local-snapshot-001", got.Source.Version)
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

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
