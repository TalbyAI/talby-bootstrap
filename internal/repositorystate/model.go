package repositorystate

type Manifest struct {
	TrustPolicy  TrustPolicy
	Declarations []Declaration
}

type TrustPolicy struct {
	ApprovedSources []SourceIdentity
}

type Declaration struct {
	Source SourceIdentity
	Target DeclarationTarget
	Input  *SourceInput
}

type DeclarationTarget struct {
	Scope    DeclarationScope
	Artifact string
}

type DeclarationScope string

const (
	DeclarationScopeArtifact DeclarationScope = "artifact"
	DeclarationScopeSource   DeclarationScope = "source"
)

type SourceIdentity struct {
	Type string
	Name string
}

const (
	SourceTypeFile = "file"
	SourceTypeGit  = "git"
)

type SourceInput struct {
	Locator string
	Version string
}

type Lockfile struct {
	Resolutions []Resolution
}

type Resolution struct {
	Source          SourceIdentity
	ResolvedVersion string
	Artifact        ArtifactResolution
}

type ArtifactResolution struct {
	Name    string
	Version string
}

type ChangeKind string

const (
	ChangeKindInserted  ChangeKind = "inserted"
	ChangeKindReplaced  ChangeKind = "replaced"
	ChangeKindUnchanged ChangeKind = "unchanged"
)

type StateFile string

const (
	StateFileManifest StateFile = "manifest"
	StateFileLockfile StateFile = "lockfile"
)

type StateFileErrorKind string

const (
	StateFileErrorNotFound      StateFileErrorKind = "not_found"
	StateFileErrorInvalidFormat StateFileErrorKind = "invalid_format"
)

type StateFileError struct {
	File StateFile
	Kind StateFileErrorKind
	Err  error
}

func (e StateFileError) Error() string {
	if e.Err == nil {
		return string(e.File) + " " + string(e.Kind)
	}
	return string(e.File) + " " + string(e.Kind) + ": " + e.Err.Error()
}

func (e StateFileError) Unwrap() error {
	return e.Err
}
