package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

type removeFailingStore struct {
	repositorystate.Store
	err error
}

func (store removeFailingStore) RemoveRecoveryState(context.Context, string) error {
	return store.err
}

func recoveryState(observation repositorystate.RecoveryObservation) repositorystate.RecoveryState {
	return repositorystate.RecoveryState{Code: repositorystate.RecoveryCodeRollbackIncomplete, Summary: recoverySummary, Observations: []repositorystate.RecoveryObservation{observation}}
}

func TestInstallMismatchingRecoveryBlocksBeforeSourceResolution(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := repositorystate.NewStore()
	state := recoveryState(repositorystate.RecoveryObservation{Path: "blocked", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
	if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a"})
	var conflict RecoveryConflictError
	if !errors.As(err, &conflict) || sourceImpl.calls != 0 || !reflect.DeepEqual(conflict.Observations, state.Observations) {
		t.Fatalf("Install() error = %T %v, calls = %d", err, err, sourceImpl.calls)
	}
	if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
		t.Fatalf("Recovery State was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, repositorystate.ManifestFileName)); !os.IsNotExist(err) {
		t.Fatalf("Manifest exists: %v", err)
	}
}

func TestSyncMatchingRecoveryClearsBeforeNormalOperation(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	marker := []byte("fixed")
	if err := os.WriteFile(filepath.Join(root, "marker"), marker, 0o640); err != nil {
		t.Fatal(err)
	}
	store := repositorystate.NewStore()
	state := recoveryState(repositorystate.RecoveryObservation{Path: "marker", Result: repositorystate.RecoveryResultVerificationFailed, ExpectedState: repositorystate.RecoveryExpectedFile, Digest: materialize.Digest(marker), Mode: 0o640})
	if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	sourceImpl := &testSource{resolve: func(source.ResolveRequest) (source.ResolvedSource, error) {
		_, err := store.LoadRecoveryState(context.Background(), root)
		if !stateNotFound(err, repositorystate.StateFileRecovery) {
			return source.ResolvedSource{}, fmt.Errorf("Recovery State not cleared before resolve: %v", err)
		}
		return testResolved(testArtifact("a", "a")), nil
	}}
	service := NewService(source.NewStaticRegistry(map[string]source.Source{"file": sourceImpl}), store)
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if sourceImpl.calls != 1 {
		t.Fatalf("Resolve calls = %d", sourceImpl.calls)
	}
}

func TestSyncDryRunInspectsButNeverClearsRecovery(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	store := repositorystate.NewStore()
	state := recoveryState(repositorystate.RecoveryObservation{Path: "missing", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
	if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if sourceImpl.calls != 1 {
		t.Fatalf("Resolve calls = %d", sourceImpl.calls)
	}
	if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
		t.Fatalf("Dry Run cleared Recovery State: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "missing"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceImpl.calls = 0
	_, err := service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true})
	var conflict RecoveryConflictError
	if !errors.As(err, &conflict) || sourceImpl.calls != 0 {
		t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
	}
	if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
		t.Fatalf("blocked Dry Run cleared Recovery State: %v", err)
	}
}

func TestRecoveryClearFailureLeavesOperationBlocked(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	base := repositorystate.NewStore()
	state := recoveryState(repositorystate.RecoveryObservation{Path: "missing", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
	if err := base.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("remove failed")
	store := removeFailingStore{Store: base, err: failure}
	sourceImpl := &testSource{resolved: testResolved(testArtifact("a", "a"))}
	service := NewService(source.NewStaticRegistry(map[string]source.Source{"file": sourceImpl}), store)
	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	var conflict RecoveryConflictError
	if !errors.Is(err, failure) || errors.As(err, &conflict) || sourceImpl.calls != 0 || err.Error() != "rollback_incomplete: recovery state could not be cleared" {
		t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
	}
	if strings.Contains(err.Error(), failure.Error()) || strings.Contains(err.Error(), root) {
		t.Fatalf("clear error leaked details: %q", err.Error())
	}
	if _, err := base.LoadRecoveryState(context.Background(), root); err != nil {
		t.Fatalf("Recovery State missing: %v", err)
	}
}

func TestRecoveryParentChangeDuringClearReturnsSanitizedConflict(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	if err := os.Mkdir(filepath.Join(root, "parent"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := repositorystate.NewStore()
	state := recoveryState(repositorystate.RecoveryObservation{Path: "parent/missing", Result: repositorystate.RecoveryResultRestoreFailed, ExpectedState: repositorystate.RecoveryExpectedAbsent})
	if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
	service.mutationHook = func(kind mutationKind, _ string, apply func() error) error {
		if kind == mutationRecoveryClear {
			if err := os.Rename(filepath.Join(root, "parent"), filepath.Join(root, "old-parent")); err != nil {
				return err
			}
			if err := os.Mkdir(filepath.Join(root, "parent"), 0o755); err != nil {
				return err
			}
		}
		return apply()
	}
	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	var conflict RecoveryConflictError
	if !errors.As(err, &conflict) || sourceImpl.calls != 0 || !reflect.DeepEqual(conflict.Observations, state.Observations) {
		t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
	}
	if _, err := store.LoadRecoveryState(context.Background(), root); err != nil {
		t.Fatalf("Recovery State was cleared after topology change: %v", err)
	}
}

func TestRecoveryExpectedFileRejectsTypeDigestModeAndUnsafeTopology(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		setUp func(*testing.T, string)
	}{
		{name: "non-regular", path: "target", setUp: func(t *testing.T, root string) {
			t.Helper()
			if err := os.Mkdir(filepath.Join(root, "target"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "digest", path: "target", setUp: func(t *testing.T, root string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, "target"), []byte("other"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "mode", path: "target", setUp: func(t *testing.T, root string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, "target"), []byte("expected"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unsafe topology", path: "parent/target", setUp: func(t *testing.T, root string) {
			t.Helper()
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, "parent")); err != nil {
				t.Skipf("symlink: %v", err)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			syncManifest(t, root, artifactDeclaration("a"))
			test.setUp(t, root)
			store := repositorystate.NewStore()
			state := recoveryState(repositorystate.RecoveryObservation{Path: test.path, Result: repositorystate.RecoveryResultVerificationFailed, ExpectedState: repositorystate.RecoveryExpectedFile, Digest: materialize.Digest([]byte("expected")), Mode: 0o644})
			if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
				t.Fatal(err)
			}
			service, sourceImpl := testService(testResolved(testArtifact("a", "a")))
			_, err := service.Sync(context.Background(), SyncRequest{Root: root, DryRun: true})
			var conflict RecoveryConflictError
			if !errors.As(err, &conflict) || sourceImpl.calls != 0 || conflict.Observations[0].Path != test.path {
				t.Fatalf("Sync() error = %T %v, calls = %d", err, err, sourceImpl.calls)
			}
		})
	}
}
