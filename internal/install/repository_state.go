package install

import (
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

func declarationFor(request Request, identity repositorystate.SourceIdentity) repositorystate.Declaration {
	target := repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}
	if request.Artifact != "" {
		target = repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeArtifact, Artifact: request.Artifact}
	}
	return repositorystate.Declaration{Source: identity, Target: target}
}
func resolutionFor(identity repositorystate.SourceIdentity, resolved source.ResolvedSource, artifacts []source.ArtifactDescriptor) repositorystate.Resolution {
	out := repositorystate.Resolution{Source: identity, ResolvedVersion: resolved.Identity.Version, Artifacts: make([]repositorystate.ArtifactResolution, 0, len(artifacts))}
	for _, a := range artifacts {
		out.Artifacts = append(out.Artifacts, repositorystate.ArtifactResolution{Name: a.Name, Version: a.Version})
	}
	return out
}
func managedRecordFor(identity repositorystate.SourceIdentity, resolved source.ResolvedSource, artifact source.ArtifactDescriptor, files []repositorystate.ManagedFileRecord) repositorystate.ManagedArtifactRecord {
	return repositorystate.ManagedArtifactRecord{Source: identity, ResolvedVersion: resolved.Identity.Version, Artifact: artifact.Name, ArtifactVersion: artifact.Version, Files: files}
}
