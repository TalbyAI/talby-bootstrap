package install

import (
	"context"
	"fmt"

	"github.com/talby/talby-bootstrap/internal/source"
)

type Request struct {
	Source   source.Ref
	Artifact string
}

type Result struct {
	Source   source.Identity
	Artifact source.ArtifactDescriptor
}

type Service struct {
	registry source.Registry
}

func NewService(registry source.Registry) Service {
	return Service{registry: registry}
}

func (s Service) Install(ctx context.Context, req Request) (Result, error) {
	if req.Source.Type == "" {
		return Result{}, fmt.Errorf("source type is required")
	}
	if req.Source.Locator == "" {
		return Result{}, fmt.Errorf("source locator is required")
	}

	sourceImpl, err := s.registry.Lookup(req.Source.Type)
	if err != nil {
		return Result{}, err
	}

	resolved, err := sourceImpl.Resolve(ctx, source.ResolveRequest{Ref: req.Source})
	if err != nil {
		return Result{}, err
	}

	artifact, err := selectArtifact(resolved.Artifacts, req.Artifact)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Source:   resolved.Identity,
		Artifact: artifact,
	}, nil
}

func selectArtifact(artifacts []source.ArtifactDescriptor, wanted string) (source.ArtifactDescriptor, error) {
	if wanted != "" {
		for _, artifact := range artifacts {
			if artifact.Name == wanted {
				return artifact, nil
			}
		}

		return source.ArtifactDescriptor{}, fmt.Errorf("artifact %q was not found", wanted)
	}

	if len(artifacts) != 1 {
		return source.ArtifactDescriptor{}, fmt.Errorf("install target must resolve to exactly one artifact")
	}

	return artifacts[0], nil
}
