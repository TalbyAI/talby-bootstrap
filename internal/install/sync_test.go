package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

type countingStore struct {
	repositorystate.Store
	lockWrites int
}

func (s *countingStore) WriteLockfile(ctx context.Context, root string, lock repositorystate.Lockfile) error {
	s.lockWrites++
	return s.Store.WriteLockfile(ctx, root, lock)
}

func syncManifest(t *testing.T, root string, declarations ...repositorystate.Declaration) {
	t.Helper()
	if err := repositorystate.NewStore().WriteManifest(context.Background(), root, repositorystate.Manifest{Declarations: declarations}); err != nil {
		t.Fatal(err)
	}
}
func artifactDeclaration(name string) repositorystate.Declaration {
	return repositorystate.Declaration{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source"}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: name}}
}
func hasChange(result Result, kind ChangeKind) bool {
	for _, change := range result.Changes {
		if change.Kind == kind {
			return true
		}
	}
	return false
}

func TestSyncMissingManifestIsOperationalError(t *testing.T) {
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: t.TempDir()}); err == nil {
		t.Fatal("Sync() error = nil")
	}
}
func TestSyncEmptyManifestNoOp(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root)
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || got.Outcome != OutcomeNoOp {
		t.Fatalf("Sync() = %#v, %v", got, err)
	}
}
func TestSyncFirstResolutionLocksDeclaration(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || !hasChange(got, ChangeResolutionLocked) {
		t.Fatalf("Sync() = %#v, %v", got, err)
	}
}
func TestSyncReplaysExactArtifactResolution(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || got.Outcome != OutcomeNoOp {
		t.Fatalf("replay = %#v, %v", got, err)
	}
}
func TestSyncReplaysExactSourceArtifactSetWithoutAddingPublishedArtifacts(t *testing.T) {
	root := t.TempDir()
	decl := repositorystate.Declaration{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source"}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}}
	syncManifest(t, root, decl)
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	impl.resolved = testResolved(testArtifact("a", "a"), testArtifact("b", "b"))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil {
		t.Fatal("Sync() error = nil, want locked-set mismatch")
	}
}
func TestSyncRejectsIdentitySourceVersionArtifactVersionAndSetMismatch(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	impl.resolved = testResolved(source.ArtifactDescriptor{Name: "a", Version: "2", Steps: []source.MaterializationStep{{Type: "file", TargetPath: "a", SourceBytes: []byte("a")}}})
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil {
		t.Fatal("Sync() error = nil, want exact lock mismatch")
	}
}
func TestSyncSupportsMultipleDeclarationsInDeterministicOrder(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("b"), artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a"), testArtifact("b", "b")))
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || got.ArtifactCount != 2 {
		t.Fatalf("Sync() = %#v, %v", got, err)
	}
}
func TestSyncAggregatesTrustDenialsBeforeResolution(t *testing.T) {
	root := t.TempDir()
	decls := []repositorystate.Declaration{
		{Source: repositorystate.SourceIdentity{Type: "file", Locator: filepath.Join(t.TempDir(), "one")}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}},
		{Source: repositorystate.SourceIdentity{Type: "file", Locator: filepath.Join(t.TempDir(), "two")}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}},
	}
	syncManifest(t, root, decls...)
	service, impl := testService(testResolved(testArtifact("a", "a")))
	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	var denied TrustPolicyError
	if !errors.As(err, &denied) || len(denied.Denied) != 2 || impl.calls != 0 {
		t.Fatalf("Sync() error = %T %v, calls=%d", err, err, impl.calls)
	}
}
func TestSyncRejectsDuplicateDesiredTargetWithinArtifact(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	artifact := testArtifact("a", "same")
	artifact.Steps = append(artifact.Steps, source.MaterializationStep{Type: "file", TargetPath: "same", SourceBytes: []byte("a")})
	service, _ := testService(testResolved(artifact))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil {
		t.Fatal("Sync() error = nil")
	}
}
func TestSyncAggregatesOwnershipDriftAndRemovalWithoutWrites(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	syncManifest(t, root)
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("drift"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, repositorystate.MaterializationRecordFileName))
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Sync(context.Background(), SyncRequest{Root: root})
	var conflict UserActionError
	if !errors.As(err, &conflict) {
		t.Fatalf("Sync() error = %T %v", err, err)
	}
	if !hasConflict(conflict.Result, ConflictDrift) || !hasConflict(conflict.Result, ConflictRemovalRequired) {
		t.Fatalf("conflicts = %#v, want drift and removal", conflict.Result.Conflicts)
	}
	after, _ := os.ReadFile(filepath.Join(root, repositorystate.MaterializationRecordFileName))
	if !bytes.Equal(before, after) {
		t.Fatal("managed state changed after conflict")
	}
}

func hasConflict(result Result, kind ConflictKind) bool {
	for _, conflict := range result.Conflicts {
		if conflict.Kind == kind {
			return true
		}
	}
	return false
}
func TestSyncRejectsReservedAndActiveSourceInputTargets(t *testing.T) {
	for _, target := range []string{repositorystate.ManifestFileName, "input"} {
		root := t.TempDir()
		syncManifest(t, root, artifactDeclaration("a"))
		resolved := testResolved(testArtifact("a", target))
		resolved.InputPaths = []string{filepath.Join(root, "input")}
		service, _ := testService(resolved)
		if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil {
			t.Fatalf("Sync(%q) error = nil", target)
		}
	}
}
func TestSyncRejectsNonRegularUnownedTarget(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	if err := os.Mkdir(filepath.Join(root, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil {
		t.Fatal("Sync() error = nil")
	}
}
func TestSyncAdoptsIdenticalUnownedFile(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || !hasChange(got, ChangeOwnershipAdopted) {
		t.Fatalf("Sync() = %#v, %v", got, err)
	}
}
func TestSyncRevalidatesAdoptedFileBeforePersistence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	observed, err := materialize.Observe(root, "a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = revalidateAdoptions([]plannedFile{{Observed: observed, Change: ChangeOwnershipAdopted}})
	var changed materialize.ChangedSincePreflightError
	if !errors.As(err, &changed) {
		t.Fatalf("revalidateAdoptions() error = %T %v", err, err)
	}
}
func TestSyncReportsMissingManagedFileAsDrift(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil {
		t.Fatal("Sync() error = nil")
	}
}
func TestSyncRejectsManagedVersionAndPathSetMismatch(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	impl.resolved = testResolved(testArtifact("a", "different"))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil {
		t.Fatal("Sync() error = nil")
	}
}
func TestSyncReconstructsMissingLockOnlyOnExactManagedMatch(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, repositorystate.LockfileFileName)); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || !hasChange(got, ChangeResolutionLocked) {
		t.Fatalf("Sync() = %#v, %v", got, err)
	}
}
func TestSyncPrunesStaleUnmanagedLockState(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root)
	store := repositorystate.NewStore()
	if err := store.WriteLockfile(context.Background(), root, repositorystate.Lockfile{Resolutions: []repositorystate.Resolution{{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source"}, ResolvedVersion: "v", Artifacts: []repositorystate.ArtifactResolution{{Name: "a", Version: "1"}}}}}); err != nil {
		t.Fatal(err)
	}
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || !hasChange(got, ChangeLockPruned) {
		t.Fatalf("Sync() = %#v, %v", got, err)
	}
}
func TestSyncEmptyManifestPrunesOrRequiresRemoval(t *testing.T) {
	TestSyncPrunesStaleUnmanagedLockState(t)
}
func TestSyncRejectsUnsupportedStepOnlyWhenSelected(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	bad := source.ArtifactDescriptor{Name: "b", Version: "1", Steps: []source.MaterializationStep{{Type: "prompt", TargetPath: "b"}}}
	service, _ := testService(testResolved(testArtifact("a", "a"), bad))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
}
func TestSyncFailsAtFirstResolutionErrorInDeclarationOrder(t *testing.T) {
	root := t.TempDir()
	declaration := func(locator string) repositorystate.Declaration {
		return repositorystate.Declaration{Source: repositorystate.SourceIdentity{Type: "file", Locator: locator}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: "a"}}
	}
	syncManifest(t, root, declaration("b"), declaration("a"))
	impl := &testSource{resolve: func(request source.ResolveRequest) (source.ResolvedSource, error) {
		return source.ResolvedSource{}, fmt.Errorf("resolve %s", filepath.Base(request.Ref.Locator))
	}}
	service := NewService(source.NewStaticRegistry(map[string]source.Source{"file": impl}), repositorystate.NewStore())
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil || err.Error() != "resolve a" {
		t.Fatalf("Sync() error = %v, want first sorted declaration failure", err)
	}
	if impl.calls != 1 {
		t.Fatalf("Resolve calls = %d, want 1", impl.calls)
	}
}
func TestSyncUsesCapturedBytesAndRevalidatesTarget(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	artifact := testArtifact("a", "a")
	artifact.Steps[0].SourceBytes = []byte("captured")
	service, _ := testService(testResolved(artifact))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "a")); err != nil || string(got) != "captured" {
		t.Fatalf("file = %q, %v", got, err)
	}
}
func TestSyncWritesNoStateOnAnyPreflightConflict(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, repositorystate.ManifestFileName))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	var conflict UserActionError
	if !errors.As(err, &conflict) {
		t.Fatalf("Sync() error = %T %v", err, err)
	}
	after, _ := os.ReadFile(filepath.Join(root, repositorystate.ManifestFileName))
	if !bytes.Equal(before, after) {
		t.Fatal("manifest changed after failed preflight")
	}
}

func TestSyncUpdateDoesNotRewriteUnchangedLockfile(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	impl := &testSource{resolved: testResolved(testArtifact("a", "a"))}
	store := &countingStore{Store: repositorystate.NewStore()}
	service := NewService(source.NewStaticRegistry(map[string]source.Source{"file": impl}), store)
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	store.lockWrites = 0
	impl.resolved = testResolved(source.ArtifactDescriptor{Name: "a", Version: "1", Steps: []source.MaterializationStep{{Type: "file", TargetPath: "a", SourceBytes: []byte("new")}}})
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if store.lockWrites != 0 {
		t.Fatalf("WriteLockfile calls = %d, want 0", store.lockWrites)
	}
}
