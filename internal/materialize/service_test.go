package materialize

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

func TestApplyWritesManagedFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	result, err := Apply(context.Background(), Request{
		Root: root,
		Key: repositorystate.ManagedArtifactKey{
			Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "base-readme",
		},
		Artifact: source.ArtifactDescriptor{
			Name: "base-readme",
			Steps: []source.MaterializationStep{{
				Type:       "file",
				TargetPath: "README.md",
				SourcePath: sourcePath,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Action != "created" {
		t.Fatalf("Changes = %#v, want one created change", result.Changes)
	}
	if bytes, err := os.ReadFile(filepath.Join(root, "README.md")); err != nil || string(bytes) != "hello\n" {
		t.Fatalf("ReadFile(README.md) = %q, %v, want hello", bytes, err)
	}
}

func TestApplyLeavesIdenticalFileUnchanged(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	result, err := Apply(context.Background(), Request{
		Root: root,
		Key: repositorystate.ManagedArtifactKey{
			Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "base-readme",
		},
		Artifact: source.ArtifactDescriptor{
			Name: "base-readme",
			Steps: []source.MaterializationStep{{
				Type:       "file",
				TargetPath: "README.md",
				SourcePath: sourcePath,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Action != "unchanged" {
		t.Fatalf("Changes = %#v, want one unchanged change", result.Changes)
	}
}

func TestApplyStopsWhenAnotherManagedArtifactOwnsTargetFile(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	_, err := Apply(context.Background(), Request{
		Root: root,
		Key: repositorystate.ManagedArtifactKey{
			Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "base-readme",
		},
		Record: repositorystate.MaterializationRecord{
			Artifacts: []repositorystate.ManagedArtifactRecord{{
				Key: repositorystate.ManagedArtifactKey{
					Source:          repositorystate.SourceIdentity{Type: "file", Name: "other-source"},
					ResolvedVersion: "local-snapshot-999",
					Artifact:        "other-artifact",
				},
				Files: []repositorystate.ManagedFileRecord{{Path: "README.md", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			}},
		},
		Artifact: source.ArtifactDescriptor{
			Name: "base-readme",
			Steps: []source.MaterializationStep{{
				Type:       "file",
				TargetPath: "README.md",
				SourcePath: sourcePath,
			}},
		},
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want ownership conflict")
	}

	var conflict OwnershipConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T, want OwnershipConflictError", err)
	}
}

func TestApplyStopsWhenTrackedFileHasDrifted(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("user edit\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	_, err := Apply(context.Background(), Request{
		Root: root,
		Key: repositorystate.ManagedArtifactKey{
			Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "base-readme",
		},
		Record: repositorystate.MaterializationRecord{
			Artifacts: []repositorystate.ManagedArtifactRecord{{
				Key: repositorystate.ManagedArtifactKey{
					Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
					ResolvedVersion: "local-snapshot-001",
					Artifact:        "base-readme",
				},
				Files: []repositorystate.ManagedFileRecord{{
					Path:   "README.md",
					Digest: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2fcbbfc4017",
				}},
			}},
		},
		Artifact: source.ArtifactDescriptor{
			Name: "base-readme",
			Steps: []source.MaterializationStep{{
				Type:       "file",
				TargetPath: "README.md",
				SourcePath: sourcePath,
			}},
		},
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want drift conflict")
	}

	var drift DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("error = %T, want DriftError", err)
	}
}

func TestApplyRejectsTargetPathOutsideOperationRoot(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	_, err := Apply(context.Background(), Request{
		Root: root,
		Key: repositorystate.ManagedArtifactKey{
			Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "base-readme",
		},
		Artifact: source.ArtifactDescriptor{
			Name: "base-readme",
			Steps: []source.MaterializationStep{{
				Type:       "file",
				TargetPath: "../README.md",
				SourcePath: sourcePath,
			}},
		},
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want target path escape error")
	}
	if got, want := err.Error(), "file target path must stay within operation root"; got != want {
		t.Fatalf("Apply() error = %q, want %q", got, want)
	}
}

func TestApplyRejectsUnsupportedStepTypes(t *testing.T) {
	root := t.TempDir()

	_, err := Apply(context.Background(), Request{
		Root: root,
		Key: repositorystate.ManagedArtifactKey{
			Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "base-readme",
		},
		Artifact: source.ArtifactDescriptor{
			Name: "base-readme",
			Steps: []source.MaterializationStep{{
				Type:       "script",
				TargetPath: "README.md",
			}},
		},
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want unsupported step type error")
	}
	if got, want := err.Error(), `unsupported step type "script"`; got != want {
		t.Fatalf("Apply() error = %q, want %q", got, want)
	}
}
