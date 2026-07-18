//go:build windows

package repositorystate

import "testing"

func TestValidateMaterializationRecordRejectsCaseAliasOwners(t *testing.T) {
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{
		{Source: SourceIdentity{Type: "file", Locator: "./source"}, ResolvedVersion: recordDigest("a"), Artifact: "a", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "Folder/File", Digest: recordDigest("a")}}},
		{Source: SourceIdentity{Type: "file", Locator: "./source"}, ResolvedVersion: recordDigest("a"), Artifact: "b", ArtifactVersion: "1.0.0", Files: []ManagedFileRecord{{Path: "folder/file", Digest: recordDigest("b")}}},
	}}
	if err := ValidateMaterializationRecord(record); err == nil {
		t.Fatal("ValidateMaterializationRecord() error = nil, want case-alias rejection")
	}
}
