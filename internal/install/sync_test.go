package install

import (
	"bytes"
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
)

type countingStore struct {
	repositorystate.Store
	loadManifestCalls int
	loadLockfileCalls int
	loadRecordCalls   int
	lockWrites        int
	recordWrites      int
}

func (s *countingStore) LoadManifest(ctx context.Context, root string) (repositorystate.Manifest, error) {
	s.loadManifestCalls++
	return s.Store.LoadManifest(ctx, root)
}

func (s *countingStore) LoadLockfile(ctx context.Context, root string) (repositorystate.Lockfile, error) {
	s.loadLockfileCalls++
	return s.Store.LoadLockfile(ctx, root)
}

func (s *countingStore) LoadMaterializationRecord(ctx context.Context, root string) (repositorystate.MaterializationRecord, error) {
	s.loadRecordCalls++
	return s.Store.LoadMaterializationRecord(ctx, root)
}

func (s *countingStore) WriteLockfile(ctx context.Context, root string, lock repositorystate.Lockfile) error {
	s.lockWrites++
	return s.Store.WriteLockfile(ctx, root, lock)
}

func (s *countingStore) WriteMaterializationRecord(ctx context.Context, root string, record repositorystate.MaterializationRecord) error {
	s.recordWrites++
	return s.Store.WriteMaterializationRecord(ctx, root, record)
}

func syncManifest(t *testing.T, root string, declarations ...repositorystate.Declaration) {
	t.Helper()
	if err := repositorystate.NewStore().WriteManifest(context.Background(), root, repositorystate.Manifest{Declarations: declarations}); err != nil {
		t.Fatal(err)
	}
}
func artifactDeclaration(name string) repositorystate.Declaration {
	return repositorystate.Declaration{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: name}}
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

func TestSyncDryRunWritesNothingAndDoesNotCreateLock(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	beforeTarget, err := os.ReadFile(filepath.Join(root, "a"))
	if err != nil {
		t.Fatal(err)
	}
	beforeLock, err := os.ReadFile(filepath.Join(root, repositorystate.LockfileFileName))
	if err != nil {
		t.Fatal(err)
	}
	beforeRecord, err := os.ReadFile(filepath.Join(root, repositorystate.MaterializationRecordFileName))
	if err != nil {
		t.Fatal(err)
	}
	impl.resolved = testResolved(source.ArtifactDescriptor{Name: "a", Version: "1.0.0", Steps: []source.MaterializationStep{{Type: "file", TargetPath: "a", SourceBytes: []byte("new")}}})

	result, err := service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomePlanned || !result.DryRun || !hasChange(result, ChangeFileUpdated) {
		t.Fatalf("result = %#v, want planned dry run", result)
	}
	for _, check := range []struct {
		name string
		want []byte
	}{
		{name: "a", want: beforeTarget},
		{name: repositorystate.LockfileFileName, want: beforeLock},
		{name: repositorystate.MaterializationRecordFileName, want: beforeRecord},
	} {
		got, err := os.ReadFile(filepath.Join(root, check.name))
		if err != nil || !bytes.Equal(got, check.want) {
			t.Fatalf("%s after dry run = %q, %v; want unchanged", check.name, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, operationLockName)); !os.IsNotExist(err) {
		t.Fatalf("operation lock exists after dry run: %v", err)
	}
}

func TestSyncDryRunConflictReportsDryRunAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	beforeManifest, err := os.ReadFile(filepath.Join(root, repositorystate.ManifestFileName))
	if err != nil {
		t.Fatal(err)
	}
	beforeTarget := []byte("other")
	if err := os.WriteFile(filepath.Join(root, "a"), beforeTarget, 0o644); err != nil {
		t.Fatal(err)
	}
	service, _ := testService(testResolved(testArtifact("a", "a")))

	_, err = service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true})
	var conflict UserActionError
	if !errors.As(err, &conflict) || !conflict.Result.DryRun {
		t.Fatalf("Sync() error = %T %v, want dry-run conflict", err, err)
	}
	afterManifest, err := os.ReadFile(filepath.Join(root, repositorystate.ManifestFileName))
	if err != nil || !bytes.Equal(afterManifest, beforeManifest) {
		t.Fatalf("manifest after dry-run conflict = %q, %v; want unchanged", afterManifest, err)
	}
	afterTarget, err := os.ReadFile(filepath.Join(root, "a"))
	if err != nil || !bytes.Equal(afterTarget, beforeTarget) {
		t.Fatalf("target after dry-run conflict = %q, %v; want unchanged", afterTarget, err)
	}
	for _, name := range []string{repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("%s after dry-run conflict = %v, want absent", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, operationLockName)); !os.IsNotExist(err) {
		t.Fatalf("operation lock exists after dry-run conflict: %v", err)
	}
}

func TestSyncJoinsLockReleaseError(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	impl := &testSource{resolve: func(source.ResolveRequest) (source.ResolvedSource, error) {
		if err := os.RemoveAll(filepath.Join(root, operationLockName)); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(root, operationLockName), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, operationLockName, "block-removal"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return source.ResolvedSource{}, errors.New("resolve failed")
	}}
	service := NewService(source.NewStaticRegistry(map[string]source.Source{"file": impl}), repositorystate.NewStore())

	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	if err == nil || !strings.Contains(err.Error(), "resolve failed") || !strings.Contains(err.Error(), "release operation lock") {
		t.Fatalf("Sync() error = %v, want operation and release errors", err)
	}
}

func TestSyncRejectsLockedOperationBeforeDependencies(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, operationLockName), 0o700); err != nil {
		t.Fatal(err)
	}
	service, impl := testService(testResolved(testArtifact("a", "a")))
	store := &countingStore{Store: repositorystate.NewStore()}
	service.store = store

	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	if err == nil || !strings.Contains(err.Error(), "already locked") {
		t.Fatalf("Sync() error = %v, want operation lock conflict", err)
	}
	if impl.calls != 0 || store.loadManifestCalls != 0 || store.loadLockfileCalls != 0 || store.loadRecordCalls != 0 {
		t.Fatalf("dependencies reached: resolve=%d manifest=%d lock=%d record=%d", impl.calls, store.loadManifestCalls, store.loadLockfileCalls, store.loadRecordCalls)
	}
}

func TestSyncValidatesRootAndPropagatesStateLoadErrors(t *testing.T) {
	service, _ := testService(testResolved())
	if _, err := service.Sync(context.Background(), SyncRequest{}); err == nil {
		t.Fatal("expected missing root error")
	}
	lockRoot := t.TempDir()
	syncManifest(t, lockRoot)
	if err := os.WriteFile(filepath.Join(lockRoot, repositorystate.LockfileFileName), []byte("schema_version: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background(), SyncRequest{Root: lockRoot}); err == nil {
		t.Fatal("expected lockfile load error")
	}
	recordRoot := t.TempDir()
	syncManifest(t, recordRoot)
	if err := os.WriteFile(filepath.Join(recordRoot, repositorystate.MaterializationRecordFileName), []byte("schema_version: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sync(context.Background(), SyncRequest{Root: recordRoot}); err == nil {
		t.Fatal("expected materialization record load error")
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
func TestSyncChangesIncludeProvenance(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	result, err := service.Sync(context.Background(), SyncRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range result.Changes {
		if change.Kind == ChangeFileCreated {
			if change.SourceVersion != testSnapshotVersion || change.OwnershipKind != OwnershipWholeFile {
				t.Fatalf("file change = %#v, want complete provenance", change)
			}
			return
		}
	}
	t.Fatalf("changes = %#v, want file_created", result.Changes)
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
func TestSyncReplaysArtifactDeclarationsFromMergedSnapshot(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"), artifactDeclaration("b"))
	service, _ := testService(testResolved(testArtifact("a", "a"), testArtifact("b", "b")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || got.Outcome != OutcomeNoOp {
		t.Fatalf("replay = %#v, %v", got, err)
	}
}
func TestSyncReplaysCanonicallyEquivalentTargetPath(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "./foo")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil || got.Outcome != OutcomeNoOp {
		t.Fatalf("replay = %#v, %v", got, err)
	}
}
func TestSyncReplaysExactSourceArtifactSetWithoutAddingPublishedArtifacts(t *testing.T) {
	root := t.TempDir()
	decl := repositorystate.Declaration{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}}
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
	impl.resolved = testResolved(source.ArtifactDescriptor{Name: "a", Version: "2.0.0", Steps: []source.MaterializationStep{{Type: "file", TargetPath: "a", SourceBytes: []byte("a")}}})
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

func TestSyncDeduplicatesTrustDenialsBySourceIdentity(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	identity := repositorystate.SourceIdentity{Type: "file", Locator: external}
	syncManifest(t, root,
		repositorystate.Declaration{Source: identity, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: "a"}},
		repositorystate.Declaration{Source: identity, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: "b"}},
	)
	service, impl := testService(testResolved(testArtifact("a", "a")))
	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	var denied TrustPolicyError
	if !errors.As(err, &denied) || len(denied.Denied) != 1 || denied.Denied[0].Locator != filepath.ToSlash(external) || impl.calls != 0 {
		t.Fatalf("Sync() error = %T %v, denied=%#v, calls=%d", err, err, denied.Denied, impl.calls)
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

func TestSyncAggregatesInaccessibleManagedPath(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	syncManifest(t, root)

	target := filepath.Join(root, "a")
	t.Cleanup(func() {
		if err := os.Chmod(target, 0o644); err != nil {
			t.Errorf("restore target permissions: %v", err)
		}
	})
	if err := os.Chmod(target, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(target); err == nil {
		t.Skip("runner bypasses file permissions")
	}

	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	var conflict UserActionError
	if !errors.As(err, &conflict) {
		t.Fatalf("Sync() error = %T %v, want user-action conflict", err, err)
	}
	if !hasConflict(conflict.Result, ConflictRemovalRequired) || !hasConflict(conflict.Result, ConflictTopology) {
		t.Fatalf("conflicts = %#v, want removal-required and topology", conflict.Result.Conflicts)
	}
}

func TestSyncPruneRemovesUnchangedManagedArtifact(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	syncManifest(t, root)

	result, err := service.Sync(context.Background(), SyncRequest{Root: root, Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeApplied || !hasChange(result, ChangeFileRemoved) || !hasChange(result, ChangeLockPruned) {
		t.Fatalf("Sync() = %#v, want applied removal", result)
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Fatalf("removed target stat = %v, want not exist", err)
	}
	lock, err := repositorystate.NewStore().LoadLockfile(context.Background(), root)
	if err != nil || len(lock.Resolutions) != 0 {
		t.Fatalf("lockfile = %#v, %v, want empty", lock, err)
	}
	record, err := repositorystate.NewStore().LoadMaterializationRecord(context.Background(), root)
	if err != nil || len(record.Artifacts) != 0 {
		t.Fatalf("materialization record = %#v, %v, want empty", record, err)
	}
}

func TestSyncPruneBlocksOnDriftWithoutWrites(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	syncManifest(t, root)
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, name := range []string{repositorystate.ManifestFileName, repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName, "a"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = data
	}

	_, err := service.Sync(context.Background(), SyncRequest{Root: root, Prune: true})
	var conflict UserActionError
	if !errors.As(err, &conflict) || !hasConflict(conflict.Result, ConflictDrift) {
		t.Fatalf("Sync() error = %T %v, want drift conflict", err, err)
	}
	for name, want := range before {
		got, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s after conflicted prune = %q, %v; want unchanged", name, got, readErr)
		}
	}
}

func TestSyncPruneDryRunPlansRemovalWithoutWrites(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, _ := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	syncManifest(t, root)
	before, err := os.ReadFile(filepath.Join(root, "a"))
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Sync(context.Background(), SyncRequest{Root: root, Prune: true, DryRun: true})
	if err != nil || result.Outcome != OutcomePlanned || !hasChange(result, ChangeFileRemoved) {
		t.Fatalf("Sync() = %#v, %v, want planned removal", result, err)
	}
	if after, err := os.ReadFile(filepath.Join(root, "a")); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("target after dry-run prune = %q, %v; want unchanged", after, err)
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
	for _, target := range []string{repositorystate.ManifestFileName, repositorystate.RecoveryStateFileName, "input"} {
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

func TestSyncAggregatesUnsafeTopologyAsConflict(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "nested")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	syncManifest(t, root, artifactDeclaration("a"), artifactDeclaration("b"))
	service, _ := testService(testResolved(testArtifact("a", "nested/a"), testArtifact("b", "b")))

	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	var conflict UserActionError
	if !errors.As(err, &conflict) || !hasConflict(conflict.Result, ConflictTopology) {
		t.Fatalf("Sync() error = %T %v, want unsafe-topology conflict", err, err)
	}
	if _, err := os.Stat(filepath.Join(root, "b")); !os.IsNotExist(err) {
		t.Fatalf("unrelated target stat = %v, want no writes", err)
	}
}
func TestPreflightRejectsTargetOverlappingAnotherSourceInput(t *testing.T) {
	root := t.TempDir()
	desired := []desiredArtifact{
		{
			Key:        repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source-a"}, Name: "a"},
			Descriptor: testArtifact("a", "input"),
		},
		{
			Key:        repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source-b"}, Name: "b"},
			Descriptor: testArtifact("b", "output"),
			InputPaths: []string{filepath.Join(root, "input")},
		},
	}

	if _, _, err := preflightFiles(root, desired, repositorystate.MaterializationRecord{}); err == nil || !strings.Contains(err.Error(), "overlaps source input") {
		t.Fatalf("preflightFiles() error = %v, want cross-Source input overlap", err)
	}
}

func TestPreflightRejectsOperationLockDescendant(t *testing.T) {
	desired := []desiredArtifact{{
		Key:        repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source"}, Name: "a"},
		Descriptor: testArtifact("a", filepath.Join(operationLockName, "nested")),
	}}

	if _, _, err := preflightFiles(t.TempDir(), desired, repositorystate.MaterializationRecord{}); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("preflightFiles() error = %v, want operation-lock reservation", err)
	}
}

func TestPreflightIgnoresMissingUnrelatedManagedFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "target"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := repositorystate.MaterializationRecord{Artifacts: []repositorystate.ManagedArtifactRecord{{
		Source:          repositorystate.SourceIdentity{Type: "file", Locator: "other"},
		ResolvedVersion: testSnapshotVersion,
		Artifact:        "other",
		ArtifactVersion: "1.0.0",
		Files:           []repositorystate.ManagedFileRecord{{Path: "missing", Digest: "unused"}},
	}}}
	desired := []desiredArtifact{{
		Key:        repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "source"}, Name: "a"},
		Descriptor: testArtifact("a", "target"),
	}}
	desired[0].Descriptor.Steps[0].SourceBytes = []byte("same")

	files, conflicts, err := preflightFiles(root, desired, record)
	if err != nil || len(conflicts) != 0 || len(files) != 1 {
		t.Fatalf("preflightFiles() = files %d, conflicts %#v, error %v; want adoption", len(files), conflicts, err)
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

func TestPersistPreparedRevalidatesAdoptionAfterApply(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "target"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	adopted, err := materialize.Observe(root, "target")
	if err != nil {
		t.Fatal(err)
	}
	createdArtifact := desiredArtifact{
		Key:             repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, Name: "created"},
		Resolution:      repositorystate.ArtifactResolution{Name: "created", Version: "1.0.0"},
		ResolvedVersion: testSnapshotVersion,
		Descriptor:      testArtifact("created", "target"),
	}
	createdArtifact.Descriptor.Steps[0].SourceBytes = []byte("new")
	adoptedArtifact := desiredArtifact{
		Key:             repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, Name: "adopted"},
		Resolution:      repositorystate.ArtifactResolution{Name: "adopted", Version: "1.0.0"},
		ResolvedVersion: testSnapshotVersion,
		Descriptor:      testArtifact("adopted", "target"),
	}
	prepared := preparedOperation{
		Desired: []desiredArtifact{createdArtifact},
		Files: []plannedFile{
			{Artifact: adoptedArtifact, Step: adoptedArtifact.Descriptor.Steps[0], Observed: adopted, Digest: materialize.Digest([]byte("old")), Change: ChangeOwnershipAdopted},
			{Artifact: createdArtifact, Step: createdArtifact.Descriptor.Steps[0], Observed: adopted, Digest: materialize.Digest([]byte("new")), Change: ChangeFileCreated},
		},
	}
	store := &countingStore{Store: repositorystate.NewStore()}

	_, err = (Service{store: store}).persistPrepared(context.Background(), root, "sync", prepared, nil, false)
	var conflict UserActionError
	if !errors.As(err, &conflict) || len(conflict.Result.Conflicts) != 1 || conflict.Result.Conflicts[0].Kind != ConflictDrift {
		t.Fatalf("persistPrepared() error = %T %v, want adoption drift", err, err)
	}
	if store.recordWrites != 0 {
		t.Fatalf("WriteMaterializationRecord calls = %d, want 0", store.recordWrites)
	}
	if _, err := os.Stat(filepath.Join(root, repositorystate.MaterializationRecordFileName)); !os.IsNotExist(err) {
		t.Fatalf("materialization record after drift = %v, want absent", err)
	}
}

func TestPersistPreparedClassifiesWriteRaceAsDrift(t *testing.T) {
	root := t.TempDir()
	observed, err := materialize.Observe(root, "target")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "target"), []byte("raced"), 0644); err != nil {
		t.Fatal(err)
	}
	artifact := desiredArtifact{
		Key:             repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, Name: "a"},
		Resolution:      repositorystate.ArtifactResolution{Name: "a", Version: "1.0.0"},
		ResolvedVersion: testSnapshotVersion,
		Descriptor:      testArtifact("a", "target"),
	}
	prepared := preparedOperation{
		Desired: []desiredArtifact{artifact},
		Files: []plannedFile{{
			Artifact: artifact,
			Step:     artifact.Descriptor.Steps[0],
			Observed: observed,
			Digest:   materialize.Digest([]byte("a")),
			Change:   ChangeFileCreated,
		}},
	}
	_, err = (Service{}).persistPrepared(context.Background(), root, "sync", prepared, nil, false)
	var conflict UserActionError
	if !errors.As(err, &conflict) || len(conflict.Result.Conflicts) != 1 || conflict.Result.Conflicts[0].Kind != ConflictDrift || conflict.Result.Conflicts[0].Source != artifact.Key.Source || conflict.Result.Conflicts[0].Artifact != artifact.Key.Name || conflict.Result.Conflicts[0].Paths[0] != "target" {
		t.Fatalf("persistPrepared() error = %T %v, want target drift", err, err)
	}
}
func TestPersistPreparedClassifiesAdoptionParentRaceAsDrift(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(root, "parent", "target")
	if err := os.Mkdir(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	observed, err := materialize.Observe(root, "parent/target")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Dir(target)); err != nil {
		t.Skipf("symlink: %v", err)
	}
	artifact := desiredArtifact{
		Key:             repositorystate.ArtifactKey{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, Name: "a"},
		Resolution:      repositorystate.ArtifactResolution{Name: "a", Version: "1.0.0"},
		ResolvedVersion: testSnapshotVersion,
		Descriptor:      testArtifact("a", "parent/target"),
	}
	prepared := preparedOperation{
		Desired: []desiredArtifact{artifact},
		Files: []plannedFile{{
			Artifact: artifact,
			Step:     artifact.Descriptor.Steps[0],
			Observed: observed,
			Digest:   materialize.Digest([]byte("a")),
			Change:   ChangeOwnershipAdopted,
		}},
	}

	_, err = (Service{}).persistPrepared(context.Background(), root, "sync", prepared, nil, false)
	var conflict UserActionError
	if !errors.As(err, &conflict) || len(conflict.Result.Conflicts) != 1 || conflict.Result.Conflicts[0].Kind != ConflictDrift {
		t.Fatalf("persistPrepared() error = %T %v, want adoption drift", err, err)
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
func TestSyncRejectsMissingLockWithManagedRecord(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, repositorystate.LockfileFileName)); err != nil {
		t.Fatal(err)
	}
	impl.calls = 0
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil || !strings.Contains(err.Error(), "does not match a lockfile resolution") {
		t.Fatalf("Sync() error = %v, want cross-document validation", err)
	}
	if impl.calls != 0 {
		t.Fatalf("Resolve calls = %d, want 0", impl.calls)
	}
}

func TestSyncRejectsMaterializationRecordThatDiffersFromLockfile(t *testing.T) {
	root := t.TempDir()
	sourceID := repositorystate.SourceIdentity{Type: "file", Locator: "./source"}
	syncManifest(t, root, artifactDeclaration("a"))
	store := repositorystate.NewStore()
	if err := store.WriteLockfile(context.Background(), root, repositorystate.Lockfile{Resolutions: []repositorystate.Resolution{{
		Source:          sourceID,
		ResolvedVersion: testSnapshotVersion,
		Artifacts:       []repositorystate.ArtifactResolution{{Name: "a", Version: "1.0.0"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteMaterializationRecord(context.Background(), root, repositorystate.MaterializationRecord{Artifacts: []repositorystate.ManagedArtifactRecord{{
		Source:          sourceID,
		ResolvedVersion: testSnapshotVersionB,
		Artifact:        "a",
		ArtifactVersion: "1.0.0",
		Files:           []repositorystate.ManagedFileRecord{{Path: "a", Digest: testSnapshotVersion}},
	}}}); err != nil {
		t.Fatal(err)
	}
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err == nil || !strings.Contains(err.Error(), "does not match a lockfile resolution") {
		t.Fatalf("Sync() error = %v, want cross-document validation", err)
	}
	if impl.calls != 0 {
		t.Fatalf("Resolve calls = %d, want 0", impl.calls)
	}
}

func TestSyncPrunesStaleUnmanagedLockState(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root)
	store := repositorystate.NewStore()
	if err := store.WriteLockfile(context.Background(), root, repositorystate.Lockfile{Resolutions: []repositorystate.Resolution{{Source: repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, ResolvedVersion: testSnapshotVersion, Artifacts: []repositorystate.ArtifactResolution{{Name: "a", Version: "1.0.0"}}}}}); err != nil {
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
	bad := source.ArtifactDescriptor{Name: "b", Version: "1.0.0", Steps: []source.MaterializationStep{{Type: "prompt", TargetPath: "b"}}}
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
	syncManifest(t, root, declaration("./b"), declaration("./a"))
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
	impl.resolved = testResolved(source.ArtifactDescriptor{Name: "a", Version: "1.0.0", Steps: []source.MaterializationStep{{Type: "file", TargetPath: "a", SourceBytes: []byte("new")}}})
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if store.lockWrites != 0 {
		t.Fatalf("WriteLockfile calls = %d, want 0", store.lockWrites)
	}
}
