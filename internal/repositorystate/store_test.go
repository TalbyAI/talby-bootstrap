package repositorystate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func requireStateFileError(t *testing.T, err error, file StateFile, kind StateFileErrorKind) {
	t.Helper()

	var stateErr StateFileError
	if !errors.As(err, &stateErr) {
		t.Fatalf("error = %T, want StateFileError", err)
	}
	if stateErr.File != file || stateErr.Kind != kind {
		t.Fatalf("StateFileError = %#v, want %s/%s", stateErr, file, kind)
	}
}

func TestStateFileErrorFormatsWithoutCause(t *testing.T) {
	err := StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat}
	if got := err.Error(); got != "manifest invalid_format" {
		t.Fatalf("Error() = %q, want nil-safe fallback", got)
	}
}

func TestStoreLoadManifestReturnsNotFoundForMissingFile(t *testing.T) {
	store := NewStore()

	_, err := store.LoadManifest(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want StateFileError")
	}
	requireStateFileError(t, err, StateFileManifest, StateFileErrorNotFound)
}

func TestStoreManifestRoundTripPreservesSchemaAndSortOrder(t *testing.T) {
	root := t.TempDir()
	store := NewStore()

	manifest := Manifest{
		TrustPolicy: TrustPolicy{
			ApprovedSources: []SourceIdentity{
				{Type: "git", Name: "company/platform"},
				{Type: "file", Name: "local-example-source"},
			},
		},
		Declarations: []Declaration{
			{
				Source: SourceIdentity{Type: "file", Name: "local-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "base-readme"},
				Input:  &SourceInput{Locator: "/tmp/example"},
			},
			{
				Source: SourceIdentity{Type: "git", Name: "company/platform"},
				Target: DeclarationTarget{Scope: DeclarationScopeSource},
			},
		},
	}

	if err := store.WriteManifest(context.Background(), root, manifest); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}

	bytes, err := os.ReadFile(filepath.Join(root, ManifestFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "" +
		"schema_version: 1\n" +
		"trust_policy:\n" +
		"  approved_sources:\n" +
		"    - type: file\n" +
		"      name: local-example-source\n" +
		"    - type: git\n" +
		"      name: company/platform\n" +
		"declarations:\n" +
		"  - source:\n" +
		"      type: file\n" +
		"      name: local-example-source\n" +
		"    target:\n" +
		"      scope: artifact\n" +
		"      artifact: base-readme\n" +
		"    input:\n" +
		"      locator: /tmp/example\n" +
		"  - source:\n" +
		"      type: git\n" +
		"      name: company/platform\n" +
		"    target:\n" +
		"      scope: source\n"
	if got := string(bytes); got != want {
		t.Fatalf("manifest file = %q, want %q", got, want)
	}

	loaded, err := store.LoadManifest(context.Background(), root)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	wantManifest := Manifest{
		TrustPolicy: TrustPolicy{
			ApprovedSources: []SourceIdentity{
				{Type: "file", Name: "local-example-source"},
				{Type: "git", Name: "company/platform"},
			},
		},
		Declarations: []Declaration{
			{
				Source: SourceIdentity{Type: "file", Name: "local-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "base-readme"},
				Input:  &SourceInput{Locator: "/tmp/example"},
			},
			{
				Source: SourceIdentity{Type: "git", Name: "company/platform"},
				Target: DeclarationTarget{Scope: DeclarationScopeSource},
			},
		},
	}
	if !reflect.DeepEqual(loaded, wantManifest) {
		t.Fatalf("LoadManifest() = %#v, want %#v", loaded, wantManifest)
	}
}

func TestStoreLoadManifestTreatsEmptyAndInvalidFilesAsInvalidFormat(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	path := filepath.Join(root, ManifestFileName)

	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := store.LoadManifest(context.Background(), root)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)

	if err := os.WriteFile(path, []byte("schema_version: 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = store.LoadManifest(context.Background(), root)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)

	if err := os.WriteFile(path, []byte("schema_version: [\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = store.LoadManifest(context.Background(), root)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)
}

func TestStoreLoadManifestRejectsDuplicateOnDiskState(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	path := filepath.Join(root, ManifestFileName)

	content := "" +
		"schema_version: 1\n" +
		"declarations:\n" +
		"  - source:\n" +
		"      type: file\n" +
		"      name: local-example-source\n" +
		"    target:\n" +
		"      scope: artifact\n" +
		"      artifact: base-readme\n" +
		"  - source:\n" +
		"      type: file\n" +
		"      name: local-example-source\n" +
		"    target:\n" +
		"      scope: artifact\n" +
		"      artifact: base-readme\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := store.LoadManifest(context.Background(), root)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileManifest, StateFileErrorInvalidFormat)
}

func TestStoreLoadLockfileReturnsNotFoundForMissingFile(t *testing.T) {
	store := NewStore()

	_, err := store.LoadLockfile(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("LoadLockfile() error = nil, want StateFileError")
	}
	requireStateFileError(t, err, StateFileLockfile, StateFileErrorNotFound)
}

func TestStoreLockfileRoundTripPreservesSchemaAndSortOrder(t *testing.T) {
	root := t.TempDir()
	store := NewStore()

	lockfile := Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "git", Name: "company/platform"},
				ResolvedVersion: "git-sha-abc123",
				Artifact:        ArtifactResolution{Name: "ci-github", Version: "2.4.0"},
			},
			{
				Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "local-snapshot-001",
				Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
			},
		},
	}

	if err := store.WriteLockfile(context.Background(), root, lockfile); err != nil {
		t.Fatalf("WriteLockfile() error = %v", err)
	}

	bytes, err := os.ReadFile(filepath.Join(root, LockfileFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "" +
		"schema_version: 1\n" +
		"resolutions:\n" +
		"  - source:\n" +
		"      type: file\n" +
		"      name: local-example-source\n" +
		"    resolved_version: local-snapshot-001\n" +
		"    artifact:\n" +
		"      name: base-readme\n" +
		"      version: 1.0.0\n" +
		"  - source:\n" +
		"      type: git\n" +
		"      name: company/platform\n" +
		"    resolved_version: git-sha-abc123\n" +
		"    artifact:\n" +
		"      name: ci-github\n" +
		"      version: 2.4.0\n"
	if got := string(bytes); got != want {
		t.Fatalf("lockfile = %q, want %q", got, want)
	}

	loaded, err := store.LoadLockfile(context.Background(), root)
	if err != nil {
		t.Fatalf("LoadLockfile() error = %v", err)
	}
	wantLockfile := Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "local-snapshot-001",
				Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
			},
			{
				Source:          SourceIdentity{Type: "git", Name: "company/platform"},
				ResolvedVersion: "git-sha-abc123",
				Artifact:        ArtifactResolution{Name: "ci-github", Version: "2.4.0"},
			},
		},
	}
	if !reflect.DeepEqual(loaded, wantLockfile) {
		t.Fatalf("LoadLockfile() = %#v, want %#v", loaded, wantLockfile)
	}
}

func TestStoreLoadLockfileTreatsEmptyAndInvalidFilesAsInvalidFormat(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	path := filepath.Join(root, LockfileFileName)

	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := store.LoadLockfile(context.Background(), root)
	if err == nil {
		t.Fatal("LoadLockfile() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)

	if err := os.WriteFile(path, []byte("schema_version: 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = store.LoadLockfile(context.Background(), root)
	if err == nil {
		t.Fatal("LoadLockfile() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)

	if err := os.WriteFile(path, []byte("schema_version: [\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = store.LoadLockfile(context.Background(), root)
	if err == nil {
		t.Fatal("LoadLockfile() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)
}

func TestStoreLoadLockfileRejectsDuplicateOnDiskState(t *testing.T) {
	root := t.TempDir()
	store := NewStore()
	path := filepath.Join(root, LockfileFileName)

	content := "" +
		"schema_version: 1\n" +
		"resolutions:\n" +
		"  - source:\n" +
		"      type: file\n" +
		"      name: local-example-source\n" +
		"    resolved_version: local-snapshot-001\n" +
		"    artifact:\n" +
		"      name: base-readme\n" +
		"      version: 1.0.0\n" +
		"  - source:\n" +
		"      type: file\n" +
		"      name: local-example-source\n" +
		"    resolved_version: local-snapshot-002\n" +
		"    artifact:\n" +
		"      name: base-readme\n" +
		"      version: 1.1.0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := store.LoadLockfile(context.Background(), root)
	if err == nil {
		t.Fatal("LoadLockfile() error = nil, want invalid_format error")
	}
	requireStateFileError(t, err, StateFileLockfile, StateFileErrorInvalidFormat)
}
