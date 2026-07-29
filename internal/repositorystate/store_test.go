package repositorystate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testSnapshot(hexDigit string) string { return "sha256:" + strings.Repeat(hexDigit, 64) }

func TestStateFileErrorFormatsWithoutCause(t *testing.T) {
	if got := (StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat}).Error(); got != "manifest invalid_format" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestStateFileErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("boom")
	if !errors.Is(StateFileError{Err: cause}, cause) {
		t.Fatal("StateFileError did not unwrap cause")
	}
}

func TestStoreMissingFilesAreTyped(t *testing.T) {
	store := NewStore()
	for _, file := range []StateFile{StateFileManifest, StateFileLockfile, StateFileMaterializationRecord, StateFileRecovery} {
		var err error
		switch file {
		case StateFileManifest:
			_, err = store.LoadManifest(context.Background(), t.TempDir())
		case StateFileLockfile:
			_, err = store.LoadLockfile(context.Background(), t.TempDir())
		case StateFileMaterializationRecord:
			_, err = store.LoadMaterializationRecord(context.Background(), t.TempDir())
		default:
			_, err = store.LoadRecoveryState(context.Background(), t.TempDir())
		}
		var state StateFileError
		if !errors.As(err, &state) || state.File != file || state.Kind != StateFileErrorNotFound {
			t.Fatalf("%s error = %v, want typed not-found", file, err)
		}
	}
}

func TestStoreRoundTripsCanonicalStateSchema(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./source"}
	manifest := Manifest{TrustPolicy: TrustPolicy{ApprovedSources: []SourceIdentity{source}}, Declarations: []Declaration{{Source: source, Target: DeclarationTarget{Scope: DeclarationScopeSource}}}}
	lockfile := Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: testSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "a", Version: "1.0.0"}, {Name: "b", Version: "2.0.0"}}}}}
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{Source: source, ResolvedVersion: testSnapshot("a"), Artifact: "a", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "a.txt", Digest: testSnapshot("b")}}}}}
	if err := store.WriteManifest(context.Background(), root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteLockfile(context.Background(), root, lockfile); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteMaterializationRecord(context.Background(), root, record); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{ManifestFileName, LockfileFileName, MaterializationRecordFileName} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "source: file:./source") || strings.Contains(text, "type: file") || strings.Contains(text, "locator:") {
			t.Fatalf("%s has non-canonical source reference: %s", name, text)
		}
	}
	if got, err := store.LoadManifest(context.Background(), root); err != nil || !reflect.DeepEqual(got, manifest) {
		t.Fatalf("LoadManifest() = %#v, %v, want %#v", got, err, manifest)
	}
	if got, err := store.LoadLockfile(context.Background(), root); err != nil || !reflect.DeepEqual(got, lockfile) {
		t.Fatalf("LoadLockfile() = %#v, %v, want %#v", got, err, lockfile)
	}
	if got, err := store.LoadMaterializationRecord(context.Background(), root); err != nil || !reflect.DeepEqual(got, record) {
		t.Fatalf("LoadMaterializationRecord() = %#v, %v, want %#v", got, err, record)
	}
}

func TestStoreSortsLockfileArtifacts(t *testing.T) {
	root := t.TempDir()
	lockfile := Lockfile{Resolutions: []Resolution{{
		Source:          SourceIdentity{Type: SourceTypeFile, Locator: "./source"},
		ResolvedVersion: testSnapshot("a"),
		Artifacts:       []ArtifactResolution{{Name: "z", Version: "1.0.0"}, {Name: "a", Version: "1.0.0"}},
	}}}
	if err := NewStore().WriteLockfile(context.Background(), root, lockfile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, LockfileFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(data), "name: a") > strings.Index(string(data), "name: z") {
		t.Fatalf("lockfile artifacts are not sorted: %s", data)
	}
}

func TestStoreRoundTripsRecoveryState(t *testing.T) {
	root := t.TempDir()
	state := RecoveryState{Code: RecoveryCodeRollbackIncomplete, Summary: "rollback incomplete", Observations: []RecoveryObservation{{Path: "file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}}
	store := NewStore()
	if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadRecoveryState(context.Background(), root)
	if err != nil || !reflect.DeepEqual(got, state) {
		t.Fatalf("LoadRecoveryState() = %#v, %v, want %#v", got, err, state)
	}
	info, err := os.Stat(filepath.Join(root, RecoveryStateFileName))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("recovery mode = %v, %v", info.Mode(), err)
	}
}

func TestStoreRemovesRecoveryState(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	state := RecoveryState{Code: RecoveryCodeRollbackIncomplete, Summary: "rollback incomplete", Observations: []RecoveryObservation{{Path: "file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}}
	if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveRecoveryState(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadRecoveryState(context.Background(), root); !stateFileNotFoundForTest(err, StateFileRecovery) {
		t.Fatalf("LoadRecoveryState() error = %v, want typed not-found", err)
	}
	if err := store.RemoveRecoveryState(context.Background(), root); err == nil {
		t.Fatal("second RemoveRecoveryState() error = nil")
	}
}

func stateFileNotFoundForTest(err error, file StateFile) bool {
	var state StateFileError
	return errors.As(err, &state) && state.File == file && state.Kind == StateFileErrorNotFound
}

func TestStoreRejectsInvalidStateFiles(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	for _, name := range []string{ManifestFileName, LockfileFileName, MaterializationRecordFileName, RecoveryStateFileName} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("schema_version: 2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var err error
		switch name {
		case ManifestFileName:
			_, err = store.LoadManifest(context.Background(), root)
		case LockfileFileName:
			_, err = store.LoadLockfile(context.Background(), root)
		case MaterializationRecordFileName:
			_, err = store.LoadMaterializationRecord(context.Background(), root)
		default:
			_, err = store.LoadRecoveryState(context.Background(), root)
		}
		var state StateFileError
		if !errors.As(err, &state) || state.Kind != StateFileErrorInvalidFormat {
			t.Fatalf("%s error = %v", name, err)
		}
		_ = os.Remove(filepath.Join(root, name))
	}
}

func TestLoadManifestRejectsUnknownAndDeprecatedFields(t *testing.T) {
	root := t.TempDir()
	data := "schema_version: 1\ndeclarations:\n  - source: file:./source\n    target:\n      scope: source\n"
	if err := os.WriteFile(filepath.Join(root, ManifestFileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore().LoadManifest(context.Background(), root)
	var state StateFileError
	if !errors.As(err, &state) || state.File != StateFileManifest || state.Kind != StateFileErrorInvalidFormat {
		t.Fatalf("LoadManifest() error = %T %v, want invalid-format Manifest", err, err)
	}
}

func TestStoreRejectsEmptyAndMalformedStateFiles(t *testing.T) {
	store := NewStore()
	for _, contents := range []string{"", "schema_version: ["} {
		root := t.TempDir()
		for _, name := range []string{ManifestFileName, LockfileFileName, MaterializationRecordFileName, RecoveryStateFileName} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		loads := []func() error{
			func() error { _, err := store.LoadManifest(context.Background(), root); return err },
			func() error { _, err := store.LoadLockfile(context.Background(), root); return err },
			func() error { _, err := store.LoadMaterializationRecord(context.Background(), root); return err },
			func() error { _, err := store.LoadRecoveryState(context.Background(), root); return err },
		}
		for _, load := range loads {
			var state StateFileError
			if err := load(); !errors.As(err, &state) || state.Kind != StateFileErrorInvalidFormat {
				t.Fatalf("load error = %T %v, want invalid format", err, err)
			}
		}
	}
}

func TestStoreRejectsInvalidDomainValuesBeforeWriting(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	if err := store.WriteManifest(context.Background(), root, Manifest{Declarations: []Declaration{{}}}); err == nil {
		t.Fatal("expected invalid manifest")
	}
	if err := store.WriteLockfile(context.Background(), root, Lockfile{Resolutions: []Resolution{{}}}); err == nil {
		t.Fatal("expected invalid lockfile")
	}
	if err := store.WriteMaterializationRecord(context.Background(), root, MaterializationRecord{Artifacts: []ManagedArtifactRecord{{}}}); err == nil {
		t.Fatal("expected invalid materialization record")
	}
	if err := store.WriteRecoveryState(context.Background(), root, RecoveryState{}); err == nil {
		t.Fatal("expected invalid recovery state")
	}
}

func TestStoreRejectsNonCanonicalStateSourceLocators(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	source := SourceIdentity{Type: SourceTypeFile, Locator: "source"}
	if err := store.WriteLockfile(context.Background(), root, Lockfile{Resolutions: []Resolution{{
		Source:          source,
		ResolvedVersion: testSnapshot("a"),
		Artifacts:       []ArtifactResolution{{Name: "a", Version: "1.0.0"}},
	}}}); err == nil {
		t.Fatal("expected non-canonical lockfile source rejection")
	}
	if err := store.WriteMaterializationRecord(context.Background(), root, MaterializationRecord{Artifacts: []ManagedArtifactRecord{{
		Source:          source,
		ResolvedVersion: testSnapshot("a"),
		Artifact:        "a",
		ArtifactVersion: "1.0.0",
		Files:           []ManagedFileRecord{{Path: "a", Digest: testSnapshot("b")}},
	}}}); err == nil {
		t.Fatal("expected non-canonical materialization source rejection")
	}
}

func TestStoreWriteReportsMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	store := NewStore()
	if err := store.WriteManifest(context.Background(), root, Manifest{}); err == nil {
		t.Fatal("expected manifest write failure")
	}
	if err := store.WriteLockfile(context.Background(), root, Lockfile{}); err == nil {
		t.Fatal("expected lockfile write failure")
	}
	if err := store.WriteMaterializationRecord(context.Background(), root, MaterializationRecord{}); err == nil {
		t.Fatal("expected materialization record write failure")
	}
	state := RecoveryState{
		Code:         RecoveryCodeRollbackIncomplete,
		Summary:      "rollback incomplete",
		Observations: []RecoveryObservation{{Path: "file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}},
	}
	if err := store.WriteRecoveryState(context.Background(), root, state); err == nil {
		t.Fatal("expected recovery write failure")
	}
}
