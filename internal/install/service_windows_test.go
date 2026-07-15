//go:build windows

package install

import (
	"testing"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
)

func TestOutsideRootRecognizesWindowsAbsoluteLocator(t *testing.T) {
	if !outsideRoot(repositorystate.SourceIdentity{Type: "file", Locator: "C:/external/source"}) {
		t.Fatal("Windows absolute locator treated as inside Operation Root")
	}
}

func TestPreflightRecognizesCaseInsensitiveManagedOwner(t *testing.T) {
	sourceA := repositorystate.SourceIdentity{Type: "file", Locator: "source-a"}
	sourceB := repositorystate.SourceIdentity{Type: "file", Locator: "source-b"}
	record := repositorystate.MaterializationRecord{Artifacts: []repositorystate.ManagedArtifactRecord{{
		Source:          sourceA,
		ResolvedVersion: "snapshot",
		Artifact:        "a",
		ArtifactVersion: "1",
		Files:           []repositorystate.ManagedFileRecord{{Path: "Folder/File", Digest: "unused"}},
	}}}
	desired := []desiredArtifact{{
		Key:        repositorystate.ArtifactKey{Source: sourceB, Name: "b"},
		Descriptor: testArtifact("b", "folder/file"),
	}}

	_, conflicts, err := preflightFiles(t.TempDir(), desired, record)
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || conflicts[0].Kind != ConflictOwnership {
		t.Fatalf("conflicts = %#v, want case-insensitive ownership conflict", conflicts)
	}
}
