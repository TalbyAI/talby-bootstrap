package repositorystate

import (
	"strings"
	"testing"
)

func recordDigest(hexDigit string) string { return "sha256:" + strings.Repeat(hexDigit, 64) }

func TestValidateMaterializationRecordAcceptsCanonicalNestedPath(t *testing.T) {
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{
		Source:          SourceIdentity{Type: SourceTypeFile, Locator: "./source"},
		ResolvedVersion: recordDigest("a"),
		Artifact:        "a",
		ArtifactVersion: "1.0.0",
		Files:           []ManagedFileRecord{{Path: "dir/file", Digest: recordDigest("a")}},
	}}}
	if err := ValidateMaterializationRecord(record); err != nil {
		t.Fatalf("ValidateMaterializationRecord() error = %v", err)
	}
}

func TestValidateMaterializationRecordRequiresExactVersionsAndUniquePaths(t *testing.T) {
	r := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{Source: SourceIdentity{Type: SourceTypeFile, Locator: "./x"}, ResolvedVersion: recordDigest("a"), Artifact: "a", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "x", Digest: recordDigest("a")}}}}}
	if err := ValidateMaterializationRecord(r); err != nil {
		t.Fatal(err)
	}
	r.Artifacts[0].ArtifactVersion = ""
	if ValidateMaterializationRecord(r) == nil {
		t.Fatal("expected missing artifact version rejection")
	}
	r.Artifacts[0].ArtifactVersion = "1.0.0"
	r.Artifacts = append(r.Artifacts, ManagedArtifactRecord{Source: SourceIdentity{Type: SourceTypeFile, Locator: "./y"}, ResolvedVersion: recordDigest("b"), Artifact: "b", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "x", Digest: recordDigest("b")}}})
	if ValidateMaterializationRecord(r) == nil {
		t.Fatal("expected duplicate path rejection")
	}
}

func TestMaterializationRecordArtifactLookupUsesSourceAndArtifactName(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./x"}
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{Source: source, ResolvedVersion: recordDigest("a"), Artifact: "a", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "x", Digest: recordDigest("a")}}}}}
	if _, ok := record.Artifact(ArtifactKey{Source: source, Name: "a"}); !ok {
		t.Fatal("missing artifact")
	}
}

func TestUpsertManagedArtifactInsertsReplacesAndSorts(t *testing.T) {
	digest := recordDigest("a")
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./source"}
	b := ManagedArtifactRecord{Source: source, ResolvedVersion: digest, Artifact: "b", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "z", Digest: digest}, {Path: "a", Digest: digest}}}
	a := ManagedArtifactRecord{Source: source, ResolvedVersion: digest, Artifact: "a", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "a", Digest: digest}}}
	record := UpsertManagedArtifact(MaterializationRecord{}, b)
	record = UpsertManagedArtifact(record, a)
	if record.Artifacts[0].Artifact != "a" || record.Artifacts[1].Files[0].Path != "a" {
		t.Fatalf("inserted record not sorted: %#v", record)
	}
	b.ArtifactVersion = "2.0.0"
	record = UpsertManagedArtifact(record, b)
	if got, ok := record.Artifact(ArtifactKey{Source: source, Name: "b"}); !ok || got.ArtifactVersion != "2.0.0" {
		t.Fatalf("replaced artifact = %#v, %v", got, ok)
	}
}

func TestValidateMaterializationRecordRejectsOwnerFilesPathsAndDigests(t *testing.T) {
	digest := recordDigest("a")
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./source"}
	valid := ManagedArtifactRecord{Source: source, ResolvedVersion: digest, Artifact: "a", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "a", Digest: digest}}}
	if ValidateMaterializationRecord(MaterializationRecord{Artifacts: []ManagedArtifactRecord{valid, valid}}) == nil {
		t.Fatal("expected duplicate owner rejection")
	}
	missingFiles := valid
	missingFiles.Files = nil
	if ValidateMaterializationRecord(MaterializationRecord{Artifacts: []ManagedArtifactRecord{missingFiles}}) == nil {
		t.Fatal("expected missing files rejection")
	}
	badPath := valid
	badPath.Files = []ManagedFileRecord{{Path: "a/../b", Digest: digest}}
	if ValidateMaterializationRecord(MaterializationRecord{Artifacts: []ManagedArtifactRecord{badPath}}) == nil {
		t.Fatal("expected non-canonical path rejection")
	}
	badDigest := valid
	badDigest.Files = []ManagedFileRecord{{Path: "a", Digest: "sha256:" + strings.Repeat("A", 64)}}
	if ValidateMaterializationRecord(MaterializationRecord{Artifacts: []ManagedArtifactRecord{badDigest}}) == nil {
		t.Fatal("expected digest rejection")
	}
}
