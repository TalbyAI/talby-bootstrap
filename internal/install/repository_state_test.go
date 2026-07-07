package install

import (
	"reflect"
	"testing"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

func TestRepositoryStateDeclarationFromInstallResult(t *testing.T) {
	req := Request{
		Source:   source.Ref{Type: "file", Locator: "/tmp/example", Version: "v1.2.3"},
		Artifact: "base-readme",
	}
	result := Result{
		Source: source.Identity{
			Type:    "file",
			Name:    "local-example-source",
			Version: "local-snapshot-001",
		},
		Artifact: source.ArtifactDescriptor{
			Name:    "base-readme",
			Version: "1.0.0",
			Path:    "artifacts/base-readme",
		},
	}

	got := ManifestDeclaration(req, result)
	want := repositorystate.Declaration{
		Source: repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
		Target: repositorystate.DeclarationTarget{
			Scope:    repositorystate.DeclarationScopeArtifact,
			Artifact: "base-readme",
		},
		Input: &repositorystate.SourceInput{
			Locator: "/tmp/example",
			Version: "v1.2.3",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ManifestDeclaration() = %#v, want %#v", got, want)
	}
}

func TestRepositoryStateResolutionFromInstallResult(t *testing.T) {
	result := Result{
		Source: source.Identity{
			Type:    "file",
			Name:    "local-example-source",
			Version: "local-snapshot-001",
		},
		Artifact: source.ArtifactDescriptor{
			Name:    "base-readme",
			Version: "1.0.0",
		},
	}

	got := LockfileResolution(result)
	want := repositorystate.Resolution{
		Source:          repositorystate.SourceIdentity{Type: "file", Name: "local-example-source"},
		ResolvedVersion: "local-snapshot-001",
		Artifact: repositorystate.ArtifactResolution{
			Name:    "base-readme",
			Version: "1.0.0",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LockfileResolution() = %#v, want %#v", got, want)
	}
}
