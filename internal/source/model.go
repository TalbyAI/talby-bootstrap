package source

import "context"

type Ref struct {
	Type    string
	Locator string
	Version string
}

type Capabilities struct {
	SupportsVersions   bool
	ProvidesIdentity   bool
	ProvidesTimestamp  bool
	EnumeratesVersions bool
}

type ResolveRequest struct {
	Ref Ref
}

type ArtifactDescriptor struct {
	Name    string
	Version string
	Path    string
}

type Identity struct {
	Type    string
	Name    string
	Version string
}

type ResolvedSource struct {
	Identity   Identity
	Artifacts  []ArtifactDescriptor
	SourcePath string
}

type Source interface {
	Capabilities() Capabilities
	Resolve(context.Context, ResolveRequest) (ResolvedSource, error)
}
