//go:build windows

package repositorystate

import (
	"strings"
	"testing"
)

func TestValidateMaterializationRecordRejectsCaseAliasOwners(t *testing.T) {
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{
		{Source: SourceIdentity{Type: "file", Locator: "source"}, ResolvedVersion: "snapshot", Artifact: "a", ArtifactVersion: "1", Files: []ManagedFileRecord{{Path: "Folder/File", Digest: strings.Repeat("a", 64)}}},
		{Source: SourceIdentity{Type: "file", Locator: "source"}, ResolvedVersion: "snapshot", Artifact: "b", ArtifactVersion: "1", Files: []ManagedFileRecord{{Path: "folder/file", Digest: strings.Repeat("b", 64)}}},
	}}
	if err := ValidateMaterializationRecord(record); err == nil {
		t.Fatal("ValidateMaterializationRecord() error = nil, want case-alias rejection")
	}
}
