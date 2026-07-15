package repositorystate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	for _, file := range []StateFile{StateFileManifest, StateFileLockfile, StateFileMaterializationRecord} {
		var err error
		switch file {
		case StateFileManifest:
			_, err = store.LoadManifest(context.Background(), t.TempDir())
		case StateFileLockfile:
			_, err = store.LoadLockfile(context.Background(), t.TempDir())
		default:
			_, err = store.LoadMaterializationRecord(context.Background(), t.TempDir())
		}
		var state StateFileError
		if !errors.As(err, &state) || state.File != file || state.Kind != StateFileErrorNotFound {
			t.Fatalf("%s error = %v, want typed not-found", file, err)
		}
	}
}

func TestStoreRoundTripsGroupedStateSchema(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	source := SourceIdentity{Type: SourceTypeFile, Locator: "source"}
	manifest := Manifest{TrustPolicy: TrustPolicy{ApprovedSources: []SourceIdentity{source}}, Declarations: []Declaration{{Source: source, Target: DeclarationTarget{Scope: DeclarationScopeSource}}}}
	lockfile := Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: "snapshot", Artifacts: []ArtifactResolution{{Name: "a", Version: "1"}, {Name: "b", Version: "2"}}}}}
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{Source: source, ResolvedVersion: "snapshot", Artifact: "a", ArtifactVersion: "1", Files: []ManagedFileRecord{{Path: "a.txt", Digest: strings.Repeat("a", 64)}}}}}
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
		for _, want := range []string{"type: file", "locator: source"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q: %s", name, want, text)
			}
		}
	}
	lockData, _ := os.ReadFile(filepath.Join(root, LockfileFileName))
	if !strings.Contains(string(lockData), "artifacts:") {
		t.Fatalf("lockfile = %s, want grouped artifacts", lockData)
	}
	recordData, _ := os.ReadFile(filepath.Join(root, MaterializationRecordFileName))
	if !strings.Contains(string(recordData), "artifact_version:") {
		t.Fatalf("record = %s, want artifact_version", recordData)
	}
	if got, err := store.LoadLockfile(context.Background(), root); err != nil || len(got.Resolutions) != 1 || len(got.Resolutions[0].Artifacts) != 2 {
		t.Fatalf("LoadLockfile() = %#v, %v", got, err)
	}
	if got, err := store.LoadMaterializationRecord(context.Background(), root); err != nil || got.Artifacts[0].ArtifactVersion != "1" {
		t.Fatalf("LoadMaterializationRecord() = %#v, %v", got, err)
	}
}

func TestStoreRejectsInvalidStateFiles(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	for _, name := range []string{ManifestFileName, LockfileFileName, MaterializationRecordFileName} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("schema_version: 2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		var err error
		switch name {
		case ManifestFileName:
			_, err = store.LoadManifest(context.Background(), root)
		case LockfileFileName:
			_, err = store.LoadLockfile(context.Background(), root)
		default:
			_, err = store.LoadMaterializationRecord(context.Background(), root)
		}
		var state StateFileError
		if !errors.As(err, &state) || state.Kind != StateFileErrorInvalidFormat {
			t.Fatalf("%s error = %v", name, err)
		}
	}
}
