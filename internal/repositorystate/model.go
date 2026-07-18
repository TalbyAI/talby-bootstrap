package repositorystate

import "encoding/json"

type Manifest struct {
	TrustPolicy  TrustPolicy
	Declarations []Declaration
}
type TrustPolicy struct{ ApprovedSources []SourceIdentity }
type Declaration struct {
	Source        SourceIdentity
	Target        DeclarationTarget
	SourceVersion string
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
	Type    string `yaml:"type" json:"type"`
	Locator string `yaml:"locator" json:"locator"`
}

func (source SourceIdentity) MarshalJSON() ([]byte, error) {
	return json.Marshal(FormatSourceReference(source))
}

const (
	SourceTypeFile = "file"
	SourceTypeGit  = "git"
)

type ArtifactKey struct {
	Source SourceIdentity
	Name   string
}
type SnapshotKey struct {
	Source          SourceIdentity
	ResolvedVersion string
}
type Lockfile struct{ Resolutions []Resolution }
type Resolution struct {
	Source          SourceIdentity
	ResolvedVersion string
	Commit          string
	Artifacts       []ArtifactResolution
}
type ArtifactResolution struct {
	Name    string
	Version string
}
type MaterializationRecord struct{ Artifacts []ManagedArtifactRecord }
type ManagedArtifactRecord struct {
	Source          SourceIdentity
	ResolvedVersion string
	Commit          string
	Artifact        string
	ArtifactVersion string
	Files           []ManagedFileRecord
}
type ManagedFileRecord struct {
	Path   string
	Digest string
}

type RecoveryState struct {
	Code         string
	Summary      string
	Observations []RecoveryObservation
}

type RecoveryObservation struct {
	Path          string
	Result        string
	ExpectedState string
	Digest        string
	Mode          uint32
	Owner         *RecoveryOwner
}

type RecoveryOwner struct {
	Source          SourceIdentity
	ResolvedVersion string
	Artifact        string
}
type ChangeKind string

const (
	ChangeKindInserted  ChangeKind = "inserted"
	ChangeKindReplaced  ChangeKind = "replaced"
	ChangeKindUnchanged ChangeKind = "unchanged"
)

type StateFile string

const (
	StateFileManifest              StateFile = "manifest"
	StateFileLockfile              StateFile = "lockfile"
	StateFileMaterializationRecord StateFile = "materialization_record"
	StateFileRecovery              StateFile = "recovery"
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
func (e StateFileError) Unwrap() error { return e.Err }
