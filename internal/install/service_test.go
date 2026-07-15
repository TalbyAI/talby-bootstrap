package install

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

type testSource struct {
	resolved source.ResolvedSource
	calls    int
	requests []source.ResolveRequest
	resolve  func(source.ResolveRequest) (source.ResolvedSource, error)
}

func (s *testSource) Capabilities() source.Capabilities { return source.Capabilities{} }
func (s *testSource) Resolve(_ context.Context, request source.ResolveRequest) (source.ResolvedSource, error) {
	s.calls++
	s.requests = append(s.requests, request)
	if s.resolve != nil {
		return s.resolve(request)
	}
	return s.resolved, nil
}

func testService(resolved source.ResolvedSource) (Service, *testSource) {
	impl := &testSource{resolved: resolved}
	return NewService(source.NewStaticRegistry(map[string]source.Source{"file": impl}), repositorystate.NewStore()), impl
}

func testResolved(artifacts ...source.ArtifactDescriptor) source.ResolvedSource {
	return source.ResolvedSource{Identity: source.Identity{Version: "snapshot"}, Artifacts: artifacts}
}

func testArtifact(name, target string) source.ArtifactDescriptor {
	return source.ArtifactDescriptor{Name: name, Version: "1", Steps: []source.MaterializationStep{{Type: "file", TargetPath: target, SourceBytes: []byte(name)}}}
}

func TestInstallMixedScopeReturnsTypedIntentConflict(t *testing.T) {
	root := t.TempDir()
	store := repositorystate.NewStore()
	manifest := repositorystate.Manifest{Declarations: []repositorystate.Declaration{{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source"}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}}}}
	if err := store.WriteManifest(context.Background(), root, manifest); err != nil {
		t.Fatal(err)
	}
	service, _ := testService(testResolved(testArtifact("a", "a")))
	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a"})
	var conflict UserActionError
	if !errors.As(err, &conflict) || conflict.Result.Conflicts[0].Kind != ConflictIntent {
		t.Fatalf("Install() error = %T %v", err, err)
	}
}

func TestInstallChangedInputReturnsTypedIntentConflict(t *testing.T) {
	root := t.TempDir()
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, DeclareOnly: true}); err != nil {
		t.Fatal(err)
	}
	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, DeclareOnly: true})
	var conflict UserActionError
	if !errors.As(err, &conflict) || conflict.Result.Conflicts[0].Kind != ConflictIntent {
		t.Fatalf("Install() error = %T %v", err, err)
	}
}

func TestInstallPreservesOtherLockedAndManagedDeclarations(t *testing.T) {
	root := t.TempDir()
	service, _ := testService(testResolved(testArtifact("a", "a"), testArtifact("b", "b")))
	for _, artifact := range []string{"a", "b"} {
		if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: artifact}); err != nil {
			t.Fatalf("Install(%s) error = %v", artifact, err)
		}
	}
	store := repositorystate.NewStore()
	lock, err := store.LoadLockfile(context.Background(), root)
	if err != nil || len(lock.Resolutions) != 1 || len(lock.Resolutions[0].Artifacts) != 2 {
		t.Fatalf("lock = %#v, %v", lock, err)
	}
	record, err := store.LoadMaterializationRecord(context.Background(), root)
	if err != nil || len(record.Artifacts) != 2 {
		t.Fatalf("record = %#v, %v", record, err)
	}
	if _, err := filepath.Abs(root); err != nil {
		t.Fatal(err)
	}
}

func TestDeclareOnlyUnsupportedStepAcceptanceAndNoOpWithoutResolve(t *testing.T) {
	root := t.TempDir()
	service, impl := testService(testResolved(source.ArtifactDescriptor{Name: "a", Version: "1", Steps: []source.MaterializationStep{{Type: "prompt", TargetPath: "ignored"}}}))
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a", DeclareOnly: true}); err != nil {
		t.Fatal(err)
	}
	impl.calls = 0
	result, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a", DeclareOnly: true})
	if err != nil || result.Outcome != OutcomeNoOp || impl.calls != 0 {
		t.Fatalf("repeat = %#v, %v, calls=%d", result, err, impl.calls)
	}
}
