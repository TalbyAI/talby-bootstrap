package install

import (
	"testing"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

func TestRepositoryStateDeclarationFromInstallRequest(t *testing.T) {
	identity := repositorystate.SourceIdentity{Type: "file", Locator: "./source"}
	if got := declarationFor(Request{Artifact: "a"}, identity); got.Target != (repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: "a"}) {
		t.Fatalf("declarationFor() = %#v", got)
	}
	if got := declarationFor(Request{}, identity); got.Target != (repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}) {
		t.Fatalf("source declaration = %#v", got)
	}
}

func TestRepositoryStateResolutionGroupsSelectedArtifacts(t *testing.T) {
	identity := repositorystate.SourceIdentity{Type: "file", Locator: "./source"}
	got := resolutionFor(identity, source.ResolvedSource{Identity: source.Identity{Version: testSnapshotVersion}}, []source.ArtifactDescriptor{{Name: "a", Version: "1.0.0"}, {Name: "b", Version: "2.0.0"}})
	if got.ResolvedVersion != testSnapshotVersion || len(got.Artifacts) != 2 {
		t.Fatalf("resolutionFor() = %#v", got)
	}
}

func TestRepositoryStateManagedRecordPreservesArtifactVersion(t *testing.T) {
	got := managedRecordFor(repositorystate.SourceIdentity{Type: "file", Locator: "./source"}, source.ResolvedSource{Identity: source.Identity{Version: testSnapshotVersion}}, source.ArtifactDescriptor{Name: "a", Version: "1.0.0"}, nil)
	if got.ArtifactVersion != "1.0.0" || got.ResolvedVersion != testSnapshotVersion {
		t.Fatalf("managedRecordFor() = %#v", got)
	}
}
