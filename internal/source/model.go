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
	Steps   []MaterializationStep
}

type MaterializationStep struct {
	Type        string
	TargetPath  string
	SourceBytes []byte
}

type Identity struct {
	Type    string
	Name    string
	Version string
}

type ResolvedSource struct {
	Identity   Identity
	Artifacts  []ArtifactDescriptor
	InputPaths []string
}

type Source interface {
	Capabilities() Capabilities
	Resolve(context.Context, ResolveRequest) (ResolvedSource, error)
}
