package install

import (
	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
)

func ManifestDeclaration(req Request, result Result) repositorystate.Declaration {
	return repositorystate.Declaration{
		Source: repositorystate.SourceIdentity{
			Type: result.Source.Type,
			Name: result.Source.Name,
		},
		Target: repositorystate.DeclarationTarget{
			Scope:    repositorystate.DeclarationScopeArtifact,
			Artifact: result.Artifact.Name,
		},
		Input: &repositorystate.SourceInput{
			Locator: req.Source.Locator,
			Version: req.Source.Version,
		},
	}
}

func LockfileResolution(result Result) repositorystate.Resolution {
	return repositorystate.Resolution{
		Source: repositorystate.SourceIdentity{
			Type: result.Source.Type,
			Name: result.Source.Name,
		},
		ResolvedVersion: result.Source.Version,
		Artifact: repositorystate.ArtifactResolution{
			Name:    result.Artifact.Name,
			Version: result.Artifact.Version,
		},
	}
}

func ManagedArtifactKeyFor(result Result) repositorystate.ManagedArtifactKey {
	return repositorystate.ManagedArtifactKey{
		Source: repositorystate.SourceIdentity{
			Type: result.Source.Type,
			Name: result.Source.Name,
		},
		ResolvedVersion: result.Source.Version,
		Artifact:        result.Artifact.Name,
	}
}

func ManagedArtifactRecordFor(result Result, matResult materialize.Result) repositorystate.ManagedArtifactRecord {
	files := make([]repositorystate.ManagedFileRecord, 0, len(matResult.Changes))
	for _, change := range matResult.Changes {
		files = append(files, repositorystate.ManagedFileRecord{
			Path:   change.Path,
			Digest: change.Digest,
		})
	}
	return repositorystate.ManagedArtifactRecord{
		Key:   ManagedArtifactKeyFor(result),
		Files: files,
	}
}
