package repositorystate

import "testing"

func TestValidateMaterializationRecordRequiresExactVersionsAndUniquePaths(t *testing.T) {
	r := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{Source: SourceIdentity{Type: "file", Locator: "x"}, ResolvedVersion: "v", Artifact: "a", ArtifactVersion: "1", Files: []ManagedFileRecord{{Path: "x", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}}}
	if err := ValidateMaterializationRecord(r); err != nil {
		t.Fatal(err)
	}
	r.Artifacts[0].ArtifactVersion = ""
	if ValidateMaterializationRecord(r) == nil {
		t.Fatal("expected missing artifact version rejection")
	}
	r.Artifacts[0].ArtifactVersion = "1"
	r.Artifacts = append(r.Artifacts, ManagedArtifactRecord{Source: SourceIdentity{Type: "file", Locator: "y"}, ResolvedVersion: "v", Artifact: "b", ArtifactVersion: "1", Files: []ManagedFileRecord{{Path: "x", Digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}})
	if ValidateMaterializationRecord(r) == nil {
		t.Fatal("expected duplicate path rejection")
	}
}
func TestMaterializationRecordArtifactLookupUsesSourceAndArtifactName(t *testing.T) {
	source := SourceIdentity{Type: "file", Locator: "x"}
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{Source: source, ResolvedVersion: "v", Artifact: "a", ArtifactVersion: "1", Files: []ManagedFileRecord{{Path: "x", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}}}
	if _, ok := record.Artifact(ArtifactKey{Source: source, Name: "a"}); !ok {
		t.Fatal("missing artifact")
	}
}
