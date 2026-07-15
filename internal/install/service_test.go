package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestInstallValidatesRequest(t *testing.T) {
	service, _ := testService(testResolved())
	if _, err := service.Install(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "repository root") {
		t.Fatalf("root error = %v", err)
	}
	root := t.TempDir()
	if _, err := service.Install(context.Background(), Request{Root: root}); err == nil || !strings.Contains(err.Error(), "source type") {
		t.Fatalf("type error = %v", err)
	}
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file"}}); err == nil || !strings.Contains(err.Error(), "source locator") {
		t.Fatalf("locator error = %v", err)
	}
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source", Version: "v1"}}); err == nil || !strings.Contains(err.Error(), "requested source versions") {
		t.Fatalf("version error = %v", err)
	}
}

func TestInstallErrorMessagesAreStableAndSorted(t *testing.T) {
	conflict := UserActionError{Result: Result{Conflicts: []Conflict{{}, {}}}}
	if got := conflict.Error(); got != "operation has 2 conflict(s)" {
		t.Fatalf("UserActionError.Error() = %q", got)
	}
	denied := TrustPolicyError{Denied: []repositorystate.SourceIdentity{{Type: "file", Locator: "z"}, {Type: "file", Locator: "a"}}}
	if got := denied.Error(); got != "unapproved sources: file:a, file:z" {
		t.Fatalf("TrustPolicyError.Error() = %q", got)
	}
}

func TestSelectedArtifactsHandlesSourceAndArtifactScopes(t *testing.T) {
	resolved := testResolved(testArtifact("a", "a"), testArtifact("b", "b"))
	selected, err := selectedArtifacts(resolved, repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource})
	if err != nil || len(selected) != 2 {
		t.Fatalf("source selection = %#v, %v", selected, err)
	}
	selected, err = selectedArtifacts(resolved, repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: "b"})
	if err != nil || len(selected) != 1 || selected[0].Name != "b" {
		t.Fatalf("artifact selection = %#v, %v", selected, err)
	}
	if _, err := selectedArtifacts(testResolved(), repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}); err == nil {
		t.Fatal("expected empty source rejection")
	}
	if _, err := selectedArtifacts(resolved, repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: "missing"}); err == nil {
		t.Fatal("expected missing artifact rejection")
	}
}

func TestTrustApprovalAndRootClassification(t *testing.T) {
	identity := repositorystate.SourceIdentity{Type: "file", Locator: "source"}
	if approved(repositorystate.TrustPolicy{}, identity) {
		t.Fatal("unexpected approval")
	}
	if !approved(repositorystate.TrustPolicy{ApprovedSources: []repositorystate.SourceIdentity{identity}}, identity) {
		t.Fatal("expected approval")
	}
	if outsideRoot(identity) || !outsideRoot(repositorystate.SourceIdentity{Type: "file", Locator: filepath.ToSlash(filepath.Join(t.TempDir(), "source"))}) {
		t.Fatal("unexpected root classification")
	}
}

func TestInstallRejectsMissingArtifactAndResolutionFailure(t *testing.T) {
	root := t.TempDir()
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "missing"}); err == nil {
		t.Fatal("expected missing artifact")
	}
	impl.resolve = func(source.ResolveRequest) (source.ResolvedSource, error) {
		return source.ResolvedSource{}, errors.New("resolve failed")
	}
	if _, err := service.Install(context.Background(), Request{Root: t.TempDir(), Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a"}); err == nil || err.Error() != "resolve failed" {
		t.Fatalf("resolve error = %v", err)
	}
}

func TestInstallEnforcesAndAcceptsExternalSourceTrust(t *testing.T) {
	external := t.TempDir()
	service, impl := testService(testResolved(testArtifact("a", "a")))
	root := t.TempDir()
	request := Request{Root: root, Source: source.Ref{Type: "file", Locator: external}, Artifact: "a", DeclareOnly: true}
	if _, err := service.Install(context.Background(), request); err == nil {
		t.Fatal("expected trust denial")
	}
	identity := repositorystate.SourceIdentity{Type: "file", Locator: filepath.ToSlash(external)}
	if err := repositorystate.NewStore().WriteManifest(context.Background(), root, repositorystate.Manifest{TrustPolicy: repositorystate.TrustPolicy{ApprovedSources: []repositorystate.SourceIdentity{identity}}}); err != nil {
		t.Fatal(err)
	}
	result, err := service.Install(context.Background(), request)
	if err != nil || result.Outcome != OutcomeApplied || impl.calls != 1 {
		t.Fatalf("trusted install = %#v, %v, calls=%d", result, err, impl.calls)
	}
}

func TestInstallPropagatesStateLoadErrors(t *testing.T) {
	service, _ := testService(testResolved(testArtifact("a", "a")))
	request := func(root string) Request {
		return Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a"}
	}
	manifestRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(manifestRoot, repositorystate.ManifestFileName), []byte("schema_version: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Install(context.Background(), request(manifestRoot)); err == nil {
		t.Fatal("expected manifest load error")
	}
	lockRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(lockRoot, repositorystate.LockfileFileName), []byte("schema_version: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Install(context.Background(), request(lockRoot)); err == nil {
		t.Fatal("expected lockfile load error")
	}
	recordRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(recordRoot, repositorystate.MaterializationRecordFileName), []byte("schema_version: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Install(context.Background(), request(recordRoot)); err == nil {
		t.Fatal("expected materialization record load error")
	}
}

func TestInstallRejectsMultipleLockedSnapshotsForSourceScope(t *testing.T) {
	root := t.TempDir()
	identity := repositorystate.SourceIdentity{Type: "file", Locator: "source"}
	lock := repositorystate.Lockfile{Resolutions: []repositorystate.Resolution{
		{Source: identity, ResolvedVersion: "v1", Artifacts: []repositorystate.ArtifactResolution{{Name: "a", Version: "1"}}},
		{Source: identity, ResolvedVersion: "v2", Artifacts: []repositorystate.ArtifactResolution{{Name: "b", Version: "1"}}},
	}}
	if err := repositorystate.NewStore().WriteLockfile(context.Background(), root, lock); err != nil {
		t.Fatal(err)
	}
	service, _ := testService(testResolved(testArtifact("a", "a"), testArtifact("b", "b")))
	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}})
	if err == nil || !strings.Contains(err.Error(), "multiple locked snapshots") {
		t.Fatalf("Install() error = %v", err)
	}
}

func TestInstallRejectsUnsupportedAndUnregisteredSources(t *testing.T) {
	root := t.TempDir()
	service, _ := testService(testResolved())
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "git", Locator: "source"}}); err == nil {
		t.Fatal("expected unsupported source")
	}
	service = NewService(source.NewStaticRegistry(nil), repositorystate.NewStore())
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}}); err == nil {
		t.Fatal("expected unregistered source")
	}
}

func TestInstallRejectsLockedMismatchUnsupportedStepAndFileConflict(t *testing.T) {
	root := t.TempDir()
	service, impl := testService(testResolved(testArtifact("a", "a")))
	request := Request{Root: root, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a"}
	if _, err := service.Install(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	impl.resolved = testResolved(source.ArtifactDescriptor{Name: "a", Version: "2", Steps: []source.MaterializationStep{{Type: "file", TargetPath: "a", SourceBytes: []byte("a")}}})
	if _, err := service.Install(context.Background(), request); err == nil {
		t.Fatal("expected locked mismatch")
	}

	badRoot := t.TempDir()
	badService, _ := testService(testResolved(source.ArtifactDescriptor{Name: "a", Version: "1", Steps: []source.MaterializationStep{{Type: "prompt", TargetPath: "a"}}}))
	if _, err := badService.Install(context.Background(), Request{Root: badRoot, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a"}); err == nil {
		t.Fatal("expected unsupported step")
	}

	conflictRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(conflictRoot, "a"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflictService, _ := testService(testResolved(testArtifact("a", "a")))
	_, err := conflictService.Install(context.Background(), Request{Root: conflictRoot, Source: source.Ref{Type: "file", Locator: "source"}, Artifact: "a"})
	var conflict UserActionError
	if !errors.As(err, &conflict) || conflict.Result.Outcome != OutcomeConflict {
		t.Fatalf("conflict error = %T %v", err, err)
	}
}
