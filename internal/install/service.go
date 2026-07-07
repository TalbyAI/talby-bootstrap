package install

import (
	"context"
	"errors"
	"fmt"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

type Request struct {
	Root        string
	Source      source.Ref
	Artifact    string
	DeclareOnly bool
}

type ChangeKind string

const (
	ChangeDeclared ChangeKind = "declared"
	ChangeNoOp     ChangeKind = "noop"
)

type Result struct {
	Source   source.Identity
	Artifact source.ArtifactDescriptor
	Change   ChangeKind
}

type ConflictError struct {
	SourceName string
	Artifact   string
}

func (e ConflictError) Error() string {
	return fmt.Sprintf(
		`artifact %q from source %q is already declared with different input; use upgrade`,
		e.Artifact,
		e.SourceName,
	)
}

type Service struct {
	registry source.Registry
	store    repositorystate.Store
}

func NewService(registry source.Registry, store repositorystate.Store) Service {
	return Service{
		registry: registry,
		store:    store,
	}
}

func (s Service) Install(ctx context.Context, req Request) (Result, error) {
	if req.Source.Type == "" {
		return Result{}, fmt.Errorf("source type is required")
	}
	if req.Source.Locator == "" {
		return Result{}, fmt.Errorf("source locator is required")
	}
	if req.DeclareOnly && req.Root == "" {
		return Result{}, fmt.Errorf("repository root is required for declare-only install")
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

	result := Result{
		Source:   resolved.Identity,
		Artifact: artifact,
	}
	if !req.DeclareOnly {
		return result, nil
	}

	manifest, err := s.loadManifestOrEmpty(ctx, req.Root)
	if err != nil {
		return Result{}, err
	}

	decl := ManifestDeclaration(req, result)
	next, change := manifest.UpsertDeclaration(decl)
	switch change {
	case repositorystate.ChangeKindInserted:
		if err := s.store.WriteManifest(ctx, req.Root, next); err != nil {
			return Result{}, err
		}
		result.Change = ChangeDeclared
		return result, nil
	case repositorystate.ChangeKindUnchanged:
		result.Change = ChangeNoOp
		return result, nil
	default:
		return Result{}, ConflictError{
			SourceName: decl.Source.Name,
			Artifact:   decl.Target.Artifact,
		}
	}
}

func (s Service) loadManifestOrEmpty(ctx context.Context, root string) (repositorystate.Manifest, error) {
	manifest, err := s.store.LoadManifest(ctx, root)
	if err == nil {
		return manifest, nil
	}

	var stateErr repositorystate.StateFileError
	if errors.As(err, &stateErr) &&
		stateErr.File == repositorystate.StateFileManifest &&
		stateErr.Kind == repositorystate.StateFileErrorNotFound {
		return repositorystate.Manifest{}, nil
	}

	return repositorystate.Manifest{}, err
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
