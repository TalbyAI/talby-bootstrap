package repositorystate

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStoreMaterializationRecordRoundTripPreservesSchemaAndSortOrder(t *testing.T) {
	root := t.TempDir()
	store := NewStore()

	record := MaterializationRecord{
		Artifacts: []ManagedArtifactRecord{
			{
				Key: ManagedArtifactKey{
					Source:          SourceIdentity{Type: "git", Name: "company/platform"},
					ResolvedVersion: "git-sha-abc123",
					Artifact:        "ci-github",
				},
				Files: []ManagedFileRecord{{Path: ".github/workflows/ci.yml", Digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
			},
			{
				Key: ManagedArtifactKey{
					Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
					ResolvedVersion: "local-snapshot-001",
					Artifact:        "base-readme",
				},
				Files: []ManagedFileRecord{{Path: "README.md", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			},
		},
	}

	if err := store.WriteMaterializationRecord(context.Background(), root, record); err != nil {
		t.Fatalf("WriteMaterializationRecord() error = %v", err)
	}

	bytes, err := os.ReadFile(filepath.Join(root, MaterializationRecordFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "" +
		"schema_version: 1\n" +
		"artifacts:\n" +
		"  - key:\n" +
		"      source:\n" +
		"        type: file\n" +
		"        name: local-example-source\n" +
		"      resolved_version: local-snapshot-001\n" +
		"      artifact: base-readme\n" +
		"    files:\n" +
		"      - path: README.md\n" +
		"        digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n" +
		"  - key:\n" +
		"      source:\n" +
		"        type: git\n" +
		"        name: company/platform\n" +
		"      resolved_version: git-sha-abc123\n" +
		"      artifact: ci-github\n" +
		"    files:\n" +
		"      - path: .github/workflows/ci.yml\n" +
		"        digest: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"
	if got := string(bytes); got != want {
		t.Fatalf("materialization record = %q, want %q", got, want)
	}

	loaded, err := store.LoadMaterializationRecord(context.Background(), root)
	if err != nil {
		t.Fatalf("LoadMaterializationRecord() error = %v", err)
	}
	wantRecord := MaterializationRecord{
		Artifacts: []ManagedArtifactRecord{
			{
				Key: ManagedArtifactKey{
					Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
					ResolvedVersion: "local-snapshot-001",
					Artifact:        "base-readme",
				},
				Files: []ManagedFileRecord{{Path: "README.md", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			},
			{
				Key: ManagedArtifactKey{
					Source:          SourceIdentity{Type: "git", Name: "company/platform"},
					ResolvedVersion: "git-sha-abc123",
					Artifact:        "ci-github",
				},
				Files: []ManagedFileRecord{{Path: ".github/workflows/ci.yml", Digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
			},
		},
	}
	if !reflect.DeepEqual(loaded, wantRecord) {
		t.Fatalf("LoadMaterializationRecord() = %#v, want %#v", loaded, wantRecord)
	}
}

func TestUpsertManagedArtifactInsertReplaceAndUnchanged(t *testing.T) {
	base := MaterializationRecord{}
	record := ManagedArtifactRecord{
		Key: ManagedArtifactKey{
			Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "base-readme",
		},
		Files: []ManagedFileRecord{{Path: "README.md", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
	}

	inserted := UpsertManagedArtifact(base, record)
	if len(inserted.Artifacts) != 1 {
		t.Fatalf("len(Artifacts) = %d, want 1", len(inserted.Artifacts))
	}

	replaced := UpsertManagedArtifact(inserted, ManagedArtifactRecord{
		Key:   record.Key,
		Files: []ManagedFileRecord{{Path: "README.md", Digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
	})
	if got := replaced.Artifacts[0].Files[0].Digest; got != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("digest = %q, want replacement", got)
	}

	unchanged := UpsertManagedArtifact(replaced, replaced.Artifacts[0])
	if !reflect.DeepEqual(unchanged, replaced) {
		t.Fatalf("UpsertManagedArtifact() changed equivalent record")
	}
}

func TestValidateMaterializationRecordRejectsDuplicateOwnersAndInvalidDigests(t *testing.T) {
	err := ValidateMaterializationRecord(MaterializationRecord{
		Artifacts: []ManagedArtifactRecord{
			{
				Key: ManagedArtifactKey{
					Source:          SourceIdentity{Type: "file", Name: "one"},
					ResolvedVersion: "local-snapshot-001",
					Artifact:        "a",
				},
				Files: []ManagedFileRecord{{Path: "README.md", Digest: strings.Repeat("a", 64)}},
			},
			{
				Key: ManagedArtifactKey{
					Source:          SourceIdentity{Type: "file", Name: "two"},
					ResolvedVersion: "local-snapshot-002",
					Artifact:        "b",
				},
				Files: []ManagedFileRecord{{Path: "README.md", Digest: strings.Repeat("b", 64)}},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateMaterializationRecord() error = nil, want duplicate owner error")
	}

	err = ValidateMaterializationRecord(MaterializationRecord{
		Artifacts: []ManagedArtifactRecord{{
			Key: ManagedArtifactKey{
				Source:          SourceIdentity{Type: "file", Name: "one"},
				ResolvedVersion: "local-snapshot-001",
				Artifact:        "a",
			},
			Files: []ManagedFileRecord{{Path: "README.md", Digest: "not-hex"}},
		}},
	})
	if err == nil {
		t.Fatal("ValidateMaterializationRecord() error = nil, want invalid digest error")
	}
}

func TestRemoveManagedArtifactDropsOnlyMatchingKey(t *testing.T) {
	keep := ManagedArtifactRecord{
		Key: ManagedArtifactKey{
			Source:          SourceIdentity{Type: "file", Name: "two"},
			ResolvedVersion: "local-snapshot-002",
			Artifact:        "b",
		},
	}
	remove := ManagedArtifactRecord{
		Key: ManagedArtifactKey{
			Source:          SourceIdentity{Type: "file", Name: "one"},
			ResolvedVersion: "local-snapshot-001",
			Artifact:        "a",
		},
	}

	got := RemoveManagedArtifact(MaterializationRecord{
		Artifacts: []ManagedArtifactRecord{remove, keep},
	}, remove.Key)

	want := MaterializationRecord{Artifacts: []ManagedArtifactRecord{keep}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RemoveManagedArtifact() = %#v, want %#v", got, want)
	}
}
