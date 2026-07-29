package install

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

func failAfter(kind mutationKind, path string, failure error) mutationHook {
	return func(gotKind mutationKind, gotPath string, apply func() error) error {
		if err := apply(); err != nil {
			return err
		}
		if gotKind == kind && gotPath == path {
			return failure
		}
		return nil
	}
}

func TestInstallRollbackRestoresAbsenceAndDirectories(t *testing.T) {
	root := t.TempDir()
	artifact := source.ArtifactDescriptor{Name: "a", Version: "1.0.0", Steps: []source.MaterializationStep{
		{Type: "file", TargetPath: "nested/new", SourceBytes: []byte("new")},
	}}
	service, _ := testService(testResolved(artifact))
	failure := errors.New("controlled write failure")
	service.mutationHook = failAfter(mutationWrite, repositorystate.MaterializationRecordFileName, failure)

	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a"})
	if !errors.Is(err, failure) {
		t.Fatalf("Install() error = %v", err)
	}
	for _, path := range []string{"nested/new", "nested", repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName, repositorystate.RecoveryStateFileName} {
		if _, statErr := os.Stat(filepath.Join(root, path)); !os.IsNotExist(statErr) {
			t.Fatalf("%s exists after rollback: %v", path, statErr)
		}
	}
}

func TestSyncRollbackRestoresFileBytesAndMode(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	before := testArtifact("a", "a")
	before.Steps[0].SourceBytes = []byte("before")
	service, impl := testService(testResolved(before))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "a"), 0o600); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Steps = slices.Clone(before.Steps)
	after.Steps[0].SourceBytes = []byte("after")
	impl.resolved = testResolved(after)
	failure := errors.New("controlled write failure")
	service.mutationHook = failAfter(mutationWrite, repositorystate.MaterializationRecordFileName, failure)

	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	if !errors.Is(err, failure) {
		t.Fatalf("Sync() error = %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(root, "a"))
	info, statErr := os.Stat(filepath.Join(root, "a"))
	if readErr != nil || statErr != nil || string(data) != "before" || info.Mode().Perm() != 0o600 {
		t.Fatalf("a = %q mode %v, read %v stat %v", data, info.Mode(), readErr, statErr)
	}
}

func TestInstallStateWriteFailureRollsBackTargetsAndEarlierState(t *testing.T) {
	root := t.TempDir()
	service, impl := testService(testResolved(testArtifact("a", "a"), testArtifact("b", "b")))
	if _, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "a"}); err != nil {
		t.Fatal(err)
	}
	names := []string{repositorystate.ManifestFileName, repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName}
	before := map[string][]byte{}
	modes := map[string]os.FileMode{}
	for _, name := range names {
		before[name], _ = os.ReadFile(filepath.Join(root, name))
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		modes[name] = info.Mode().Perm()
	}
	impl.resolved = testResolved(testArtifact("a", "a"), testArtifact("b", "b"))
	failure := errors.New("manifest write failed")
	service.mutationHook = failAfter(mutationWrite, repositorystate.ManifestFileName, failure)
	_, err := service.Install(context.Background(), Request{Root: root, Source: source.Ref{Type: "file", Locator: "./source"}, Artifact: "b"})
	if !errors.Is(err, failure) {
		t.Fatalf("Install() error = %v", err)
	}
	for _, name := range names {
		data, readErr := os.ReadFile(filepath.Join(root, name))
		info, statErr := os.Stat(filepath.Join(root, name))
		if readErr != nil || statErr != nil || !bytes.Equal(data, before[name]) || info.Mode().Perm() != modes[name] {
			t.Fatalf("%s was not restored", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "b")); !os.IsNotExist(err) {
		t.Fatalf("b exists after rollback: %v", err)
	}
}

func TestSyncRollbackRunsReverseAttemptsEveryRestoreAndWritesSanitizedRecovery(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	prior := []byte("prior secret contents")
	if err := os.WriteFile(filepath.Join(root, "a"), prior, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "a"), 0o640); err != nil {
		t.Fatal(err)
	}
	record, err := repositorystate.NewStore().LoadMaterializationRecord(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	record.Artifacts[0].Files[0].Digest = materialize.Digest(prior)
	if err := repositorystate.NewStore().WriteMaterializationRecord(context.Background(), root, record); err != nil {
		t.Fatal(err)
	}
	changed := testArtifact("a", "a")
	changed.Steps[0].SourceBytes = []byte("after")
	impl.resolved = testResolved(changed)
	operationFailure := errors.New("raw operation secret")
	restoreFailure := errors.New("raw restoration secret")
	var mutations, restorations []string
	service.mutationHook = func(kind mutationKind, path string, apply func() error) error {
		switch kind {
		case mutationWrite, mutationRemove:
			if err := apply(); err != nil {
				return err
			}
			mutations = append(mutations, path)
			if path == repositorystate.MaterializationRecordFileName {
				return operationFailure
			}
			return nil
		case mutationRestore:
			restorations = append(restorations, path)
			if path == "a" {
				return restoreFailure
			}
			return apply()
		default:
			return apply()
		}
	}
	_, err = service.Sync(context.Background(), SyncRequest{Root: root})
	if !errors.Is(err, operationFailure) {
		t.Fatalf("Sync() error = %v", err)
	}
	wantOrder := slices.Clone(mutations)
	slices.Reverse(wantOrder)
	if !reflect.DeepEqual(restorations, wantOrder) {
		t.Fatalf("restore order = %#v, want %#v", restorations, wantOrder)
	}
	state, err := repositorystate.NewStore().LoadRecoveryState(context.Background(), root)
	if err != nil || state.Code != repositorystate.RecoveryCodeRollbackIncomplete || state.Summary != recoverySummary || len(state.Observations) != 1 {
		t.Fatalf("Recovery State = %#v, %v", state, err)
	}
	observation := state.Observations[0]
	if observation.Path != "a" || observation.Result != repositorystate.RecoveryResultRestoreFailed || observation.ExpectedState != repositorystate.RecoveryExpectedFile || observation.Digest != materialize.Digest(prior) || observation.Mode != 0o640 || observation.Owner == nil || observation.Owner.Artifact != "a" {
		t.Fatalf("observation = %#v", observation)
	}
	data, err := os.ReadFile(filepath.Join(root, repositorystate.RecoveryStateFileName))
	info, statErr := os.Stat(filepath.Join(root, repositorystate.RecoveryStateFileName))
	if err != nil || statErr != nil || info.Mode().Perm() != 0o600 || bytes.Contains(data, prior) || bytes.Contains(data, []byte("raw operation secret")) || bytes.Contains(data, []byte("raw restoration secret")) || bytes.Contains(data, []byte(root)) {
		t.Fatalf("unsafe Recovery State: %s, read %v stat %v", data, err, statErr)
	}
}

func TestSyncJoinsRecoveryModeVerificationFailureWithOriginalFailure(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	changed := testArtifact("a", "a")
	changed.Steps[0].SourceBytes = []byte("after")
	impl.resolved = testResolved(changed)
	operationFailure := errors.New("operation failed")
	service.mutationHook = func(kind mutationKind, path string, apply func() error) error {
		switch {
		case kind == mutationWrite && path == repositorystate.MaterializationRecordFileName:
			if err := apply(); err != nil {
				return err
			}
			return operationFailure
		case kind == mutationRestore && path == "a":
			return errors.New("restore failed")
		case kind == mutationRecovery:
			if err := apply(); err != nil {
				return err
			}
			return os.Chmod(filepath.Join(root, repositorystate.RecoveryStateFileName), 0o644)
		default:
			return apply()
		}
	}
	_, err := service.Sync(context.Background(), SyncRequest{Root: root})
	if !errors.Is(err, operationFailure) || !strings.Contains(err.Error(), "write recovery state") {
		t.Fatalf("Sync() error = %v", err)
	}
	info, statErr := os.Stat(filepath.Join(root, repositorystate.RecoveryStateFileName))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("Recovery State mode = %v", info.Mode())
	}
}

func TestSyncAcceptsVerifiedRestorationDespiteReportedActionError(t *testing.T) {
	root := t.TempDir()
	syncManifest(t, root, artifactDeclaration("a"))
	service, impl := testService(testResolved(testArtifact("a", "a")))
	if _, err := service.Sync(context.Background(), SyncRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	prior, err := os.ReadFile(filepath.Join(root, "a"))
	if err != nil {
		t.Fatal(err)
	}
	changed := testArtifact("a", "a")
	changed.Steps[0].SourceBytes = []byte("after")
	impl.resolved = testResolved(changed)
	operationFailure := errors.New("operation failed")
	restorationReported := errors.New("restoration reported an error after success")
	service.mutationHook = func(kind mutationKind, path string, apply func() error) error {
		switch {
		case kind == mutationWrite && path == repositorystate.MaterializationRecordFileName:
			if err := apply(); err != nil {
				return err
			}
			return operationFailure
		case kind == mutationRestore && path == "a":
			if err := apply(); err != nil {
				return err
			}
			return restorationReported
		default:
			return apply()
		}
	}
	_, err = service.Sync(context.Background(), SyncRequest{Root: root})
	if !errors.Is(err, operationFailure) || errors.Is(err, restorationReported) {
		t.Fatalf("Sync() error = %v", err)
	}
	restored, readErr := os.ReadFile(filepath.Join(root, "a"))
	if readErr != nil || !bytes.Equal(restored, prior) {
		t.Fatalf("restored = %q, %v", restored, readErr)
	}
	if _, loadErr := repositorystate.NewStore().LoadRecoveryState(context.Background(), root); !stateNotFound(loadErr, repositorystate.StateFileRecovery) {
		t.Fatalf("Recovery State exists after verified rollback: %v", loadErr)
	}
}
