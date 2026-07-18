package repositorystate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	supportedSchemaVersion        = 1
	SourceDescriptorFileName      = "tbboot-source.yaml"
	ArtifactDescriptorFileName    = "tbboot-artifact.yaml"
	ManifestFileName              = "tbboot-artifacts.yaml"
	LockfileFileName              = "tbboot-artifacts.lock.yaml"
	MaterializationRecordFileName = "tbboot-artifacts.managed.yaml"
	RecoveryStateFileName         = "tbboot-artifacts.recovery.yaml"
)

type Store interface {
	LoadManifest(context.Context, string) (Manifest, error)
	WriteManifest(context.Context, string, Manifest) error
	LoadLockfile(context.Context, string) (Lockfile, error)
	WriteLockfile(context.Context, string, Lockfile) error
	LoadMaterializationRecord(context.Context, string) (MaterializationRecord, error)
	WriteMaterializationRecord(context.Context, string, MaterializationRecord) error
	LoadRecoveryState(context.Context, string) (RecoveryState, error)
	WriteRecoveryState(context.Context, string, RecoveryState) error
}

type fileStore struct{}

func NewStore() Store {
	return fileStore{}
}

type manifestDocument struct {
	SchemaVersion int                      `yaml:"schema_version"`
	TrustPolicy   *manifestTrustPolicyDTO  `yaml:"trust_policy,omitempty"`
	Declarations  []manifestDeclarationDTO `yaml:"declarations,omitempty"`
}

type manifestTrustPolicyDTO struct {
	ApprovedSources []string `yaml:"approved_sources,omitempty"`
}

type manifestDeclarationDTO struct {
	Source        string           `yaml:"source"`
	Scope         DeclarationScope `yaml:"scope"`
	Artifact      string           `yaml:"artifact,omitempty"`
	SourceVersion string           `yaml:"source_version,omitempty"`
}

type lockfileDocument struct {
	SchemaVersion int                     `yaml:"schema_version"`
	Resolutions   []lockfileResolutionDTO `yaml:"resolutions,omitempty"`
}

type lockfileResolutionDTO struct {
	Source        string                `yaml:"source"`
	SourceVersion string                `yaml:"source_version"`
	Commit        string                `yaml:"commit,omitempty"`
	Artifacts     []lockfileArtifactDTO `yaml:"artifacts"`
}

type lockfileArtifactDTO struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type materializationRecordDocument struct {
	SchemaVersion int                                `yaml:"schema_version"`
	Artifacts     []materializationArtifactRecordDTO `yaml:"artifacts,omitempty"`
}

type materializationArtifactRecordDTO struct {
	Source          string                          `yaml:"source"`
	SourceVersion   string                          `yaml:"source_version"`
	Commit          string                          `yaml:"commit,omitempty"`
	Artifact        string                          `yaml:"artifact"`
	ArtifactVersion string                          `yaml:"artifact_version"`
	Files           []materializationManagedFileDTO `yaml:"files,omitempty"`
}

type materializationManagedFileDTO struct {
	Path   string `yaml:"path"`
	Digest string `yaml:"digest"`
}

type recoveryDocument struct {
	SchemaVersion int                      `yaml:"schema_version"`
	Code          string                   `yaml:"code"`
	Summary       string                   `yaml:"summary"`
	Observations  []recoveryObservationDTO `yaml:"observations"`
}

type recoveryObservationDTO struct {
	Path     string              `yaml:"path"`
	Result   string              `yaml:"result"`
	Expected recoveryExpectedDTO `yaml:"expected"`
	Owner    *recoveryOwnerDTO   `yaml:"owner,omitempty"`
}

type recoveryExpectedDTO struct {
	State  string `yaml:"state"`
	Digest string `yaml:"digest,omitempty"`
	Mode   uint32 `yaml:"mode,omitempty"`
}

type recoveryOwnerDTO struct {
	Source        string `yaml:"source"`
	SourceVersion string `yaml:"source_version"`
	Artifact      string `yaml:"artifact"`
}

func (fileStore) LoadManifest(_ context.Context, root string) (Manifest, error) {
	bytes, err := os.ReadFile(filepath.Join(root, ManifestFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorNotFound, Err: err}
		}
		return Manifest{}, err
	}
	var doc manifestDocument
	if err := decodeStrictYAML(bytes, &doc); err != nil {
		return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: err}
	}
	if doc.SchemaVersion != supportedSchemaVersion {
		return Manifest{}, StateFileError{
			File: StateFileManifest,
			Kind: StateFileErrorInvalidFormat,
			Err:  fmt.Errorf("schema_version must be %d", supportedSchemaVersion),
		}
	}

	manifest := Manifest{Declarations: make([]Declaration, 0, len(doc.Declarations))}
	if doc.TrustPolicy != nil {
		manifest.TrustPolicy.ApprovedSources = make([]SourceIdentity, 0, len(doc.TrustPolicy.ApprovedSources))
		for _, raw := range doc.TrustPolicy.ApprovedSources {
			source, err := canonicalSourceReference(root, raw)
			if err != nil {
				return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("approved source: %w", err)}
			}
			manifest.TrustPolicy.ApprovedSources = append(manifest.TrustPolicy.ApprovedSources, source)
		}
	}
	for _, dto := range doc.Declarations {
		source, err := canonicalSourceReference(root, dto.Source)
		if err != nil {
			return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("declaration source: %w", err)}
		}
		manifest.Declarations = append(manifest.Declarations, Declaration{
			Source:        source,
			Target:        DeclarationTarget{Scope: dto.Scope, Artifact: dto.Artifact},
			SourceVersion: dto.SourceVersion,
		})
	}
	if err := ValidateManifest(root, manifest); err != nil {
		return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: err}
	}

	return manifest, nil
}

func (fileStore) WriteManifest(_ context.Context, root string, manifest Manifest) error {
	if err := ValidateManifest(root, manifest); err != nil {
		return err
	}

	doc := manifestDocument{SchemaVersion: supportedSchemaVersion, Declarations: make([]manifestDeclarationDTO, 0, len(manifest.Declarations))}
	if len(manifest.TrustPolicy.ApprovedSources) > 0 {
		approved := append([]SourceIdentity(nil), manifest.TrustPolicy.ApprovedSources...)
		sort.Slice(approved, func(i, j int) bool {
			return FormatSourceReference(approved[i]) < FormatSourceReference(approved[j])
		})
		doc.TrustPolicy = &manifestTrustPolicyDTO{ApprovedSources: make([]string, 0, len(approved))}
		for _, source := range approved {
			doc.TrustPolicy.ApprovedSources = append(doc.TrustPolicy.ApprovedSources, FormatSourceReference(source))
		}
	}

	declarations := append([]Declaration(nil), manifest.Declarations...)
	sort.Slice(declarations, func(i, j int) bool {
		return DeclarationKey(declarations[i]) < DeclarationKey(declarations[j])
	})
	for _, decl := range declarations {
		doc.Declarations = append(doc.Declarations, manifestDeclarationDTO{
			Source:        FormatSourceReference(decl.Source),
			Scope:         decl.Target.Scope,
			Artifact:      decl.Target.Artifact,
			SourceVersion: decl.SourceVersion,
		})
	}

	bytes, err := encodeYAML(doc)
	if err != nil {
		return err
	}
	return writeFileAtomically(filepath.Join(root, ManifestFileName), bytes, 0o644)
}

func (fileStore) LoadLockfile(_ context.Context, root string) (Lockfile, error) {
	bytes, err := os.ReadFile(filepath.Join(root, LockfileFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorNotFound, Err: err}
		}
		return Lockfile{}, err
	}
	var doc lockfileDocument
	if err := decodeStrictYAML(bytes, &doc); err != nil {
		return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: err}
	}
	if doc.SchemaVersion != supportedSchemaVersion {
		return Lockfile{}, StateFileError{
			File: StateFileLockfile,
			Kind: StateFileErrorInvalidFormat,
			Err:  fmt.Errorf("schema_version must be %d", supportedSchemaVersion),
		}
	}

	lockfile := Lockfile{
		Resolutions: make([]Resolution, 0, len(doc.Resolutions)),
	}
	for _, dto := range doc.Resolutions {
		source, err := canonicalSourceReference(root, dto.Source)
		if err != nil {
			return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("resolution source: %w", err)}
		}
		artifacts := make([]ArtifactResolution, 0, len(dto.Artifacts))
		for _, artifact := range dto.Artifacts {
			artifacts = append(artifacts, ArtifactResolution{Name: artifact.Name, Version: artifact.Version})
		}
		lockfile.Resolutions = append(lockfile.Resolutions, Resolution{Source: source, ResolvedVersion: dto.SourceVersion, Commit: dto.Commit, Artifacts: artifacts})
	}
	if err := ValidateLockfile(lockfile); err != nil {
		return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: err}
	}

	return lockfile, nil
}

func (fileStore) WriteLockfile(_ context.Context, root string, lockfile Lockfile) error {
	if err := ValidateLockfile(lockfile); err != nil {
		return err
	}
	for _, resolution := range lockfile.Resolutions {
		if err := validateCanonicalStateSource(root, resolution.Source); err != nil {
			return fmt.Errorf("lockfile resolution source: %w", err)
		}
	}

	doc := lockfileDocument{
		SchemaVersion: supportedSchemaVersion,
		Resolutions:   make([]lockfileResolutionDTO, 0, len(lockfile.Resolutions)),
	}

	resolutions := append([]Resolution(nil), lockfile.Resolutions...)
	sort.Slice(resolutions, func(i, j int) bool {
		return SourceIdentityKey(resolutions[i].Source)+"\x00"+resolutions[i].ResolvedVersion < SourceIdentityKey(resolutions[j].Source)+"\x00"+resolutions[j].ResolvedVersion
	})
	for _, res := range resolutions {
		artifacts := append([]ArtifactResolution(nil), res.Artifacts...)
		sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Name < artifacts[j].Name })
		artifactDTOs := make([]lockfileArtifactDTO, 0, len(artifacts))
		for _, artifact := range artifacts {
			artifactDTOs = append(artifactDTOs, lockfileArtifactDTO{Name: artifact.Name, Version: artifact.Version})
		}
		doc.Resolutions = append(doc.Resolutions, lockfileResolutionDTO{Source: FormatSourceReference(res.Source), SourceVersion: res.ResolvedVersion, Commit: res.Commit, Artifacts: artifactDTOs})
	}

	bytes, err := encodeYAML(doc)
	if err != nil {
		return err
	}
	return writeFileAtomically(filepath.Join(root, LockfileFileName), bytes, 0o644)
}

func (fileStore) LoadMaterializationRecord(_ context.Context, root string) (MaterializationRecord, error) {
	bytes, err := os.ReadFile(filepath.Join(root, MaterializationRecordFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MaterializationRecord{}, StateFileError{File: StateFileMaterializationRecord, Kind: StateFileErrorNotFound, Err: err}
		}
		return MaterializationRecord{}, err
	}
	var doc materializationRecordDocument
	if err := decodeStrictYAML(bytes, &doc); err != nil {
		return MaterializationRecord{}, StateFileError{File: StateFileMaterializationRecord, Kind: StateFileErrorInvalidFormat, Err: err}
	}
	if doc.SchemaVersion != supportedSchemaVersion {
		return MaterializationRecord{}, StateFileError{
			File: StateFileMaterializationRecord,
			Kind: StateFileErrorInvalidFormat,
			Err:  fmt.Errorf("schema_version must be %d", supportedSchemaVersion),
		}
	}

	record := MaterializationRecord{Artifacts: make([]ManagedArtifactRecord, 0, len(doc.Artifacts))}
	for _, dto := range doc.Artifacts {
		source, err := canonicalSourceReference(root, dto.Source)
		if err != nil {
			return MaterializationRecord{}, StateFileError{File: StateFileMaterializationRecord, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("managed artifact source: %w", err)}
		}
		files := make([]ManagedFileRecord, 0, len(dto.Files))
		for _, file := range dto.Files {
			files = append(files, ManagedFileRecord{
				Path:   file.Path,
				Digest: file.Digest,
			})
		}
		record.Artifacts = append(record.Artifacts, ManagedArtifactRecord{Source: source, ResolvedVersion: dto.SourceVersion, Commit: dto.Commit, Artifact: dto.Artifact, ArtifactVersion: dto.ArtifactVersion, Files: files})
	}
	if err := ValidateMaterializationRecord(record); err != nil {
		return MaterializationRecord{}, StateFileError{File: StateFileMaterializationRecord, Kind: StateFileErrorInvalidFormat, Err: err}
	}

	return record, nil
}

func (fileStore) WriteMaterializationRecord(_ context.Context, root string, record MaterializationRecord) error {
	if err := ValidateMaterializationRecord(record); err != nil {
		return err
	}
	for _, artifact := range record.Artifacts {
		if err := validateCanonicalStateSource(root, artifact.Source); err != nil {
			return fmt.Errorf("managed artifact source: %w", err)
		}
	}

	doc := materializationRecordDocument{
		SchemaVersion: supportedSchemaVersion,
		Artifacts:     make([]materializationArtifactRecordDTO, 0, len(record.Artifacts)),
	}
	artifacts := append([]ManagedArtifactRecord(nil), record.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool {
		return SourceIdentityKey(artifacts[i].Source)+"\x00"+artifacts[i].ResolvedVersion+"\x00"+artifacts[i].Artifact < SourceIdentityKey(artifacts[j].Source)+"\x00"+artifacts[j].ResolvedVersion+"\x00"+artifacts[j].Artifact
	})
	for _, artifact := range artifacts {
		files := append([]ManagedFileRecord(nil), artifact.Files...)
		sort.Slice(files, func(i, j int) bool {
			return files[i].Path < files[j].Path
		})
		fileDTOs := make([]materializationManagedFileDTO, 0, len(files))
		for _, file := range files {
			fileDTOs = append(fileDTOs, materializationManagedFileDTO{
				Path:   file.Path,
				Digest: file.Digest,
			})
		}
		doc.Artifacts = append(doc.Artifacts, materializationArtifactRecordDTO{Source: FormatSourceReference(artifact.Source), SourceVersion: artifact.ResolvedVersion, Commit: artifact.Commit, Artifact: artifact.Artifact, ArtifactVersion: artifact.ArtifactVersion, Files: fileDTOs})
	}

	bytes, err := encodeYAML(doc)
	if err != nil {
		return err
	}
	return writeFileAtomically(filepath.Join(root, MaterializationRecordFileName), bytes, 0o644)
}

func (fileStore) LoadRecoveryState(_ context.Context, root string) (RecoveryState, error) {
	bytes, err := os.ReadFile(filepath.Join(root, RecoveryStateFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RecoveryState{}, StateFileError{File: StateFileRecovery, Kind: StateFileErrorNotFound, Err: err}
		}
		return RecoveryState{}, err
	}
	var doc recoveryDocument
	if err := decodeStrictYAML(bytes, &doc); err != nil {
		return RecoveryState{}, StateFileError{File: StateFileRecovery, Kind: StateFileErrorInvalidFormat, Err: err}
	}
	if doc.SchemaVersion != supportedSchemaVersion {
		return RecoveryState{}, StateFileError{File: StateFileRecovery, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("schema_version must be %d", supportedSchemaVersion)}
	}
	state := RecoveryState{Code: doc.Code, Summary: doc.Summary, Observations: make([]RecoveryObservation, 0, len(doc.Observations))}
	for _, dto := range doc.Observations {
		observation := RecoveryObservation{Path: dto.Path, Result: dto.Result, ExpectedState: dto.Expected.State, Digest: dto.Expected.Digest, Mode: dto.Expected.Mode}
		if dto.Owner != nil {
			source, err := canonicalSourceReference(root, dto.Owner.Source)
			if err != nil {
				return RecoveryState{}, StateFileError{File: StateFileRecovery, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("recovery owner source: %w", err)}
			}
			observation.Owner = &RecoveryOwner{Source: source, ResolvedVersion: dto.Owner.SourceVersion, Artifact: dto.Owner.Artifact}
		}
		state.Observations = append(state.Observations, observation)
	}
	if err := ValidateRecoveryState(root, state); err != nil {
		return RecoveryState{}, StateFileError{File: StateFileRecovery, Kind: StateFileErrorInvalidFormat, Err: err}
	}
	return state, nil
}

func (fileStore) WriteRecoveryState(_ context.Context, root string, state RecoveryState) error {
	if err := ValidateRecoveryState(root, state); err != nil {
		return err
	}
	doc := recoveryDocument{SchemaVersion: supportedSchemaVersion, Code: state.Code, Summary: state.Summary, Observations: make([]recoveryObservationDTO, 0, len(state.Observations))}
	observations := append([]RecoveryObservation(nil), state.Observations...)
	sort.Slice(observations, func(i, j int) bool { return observations[i].Path < observations[j].Path })
	for _, observation := range observations {
		dto := recoveryObservationDTO{
			Path:     observation.Path,
			Result:   observation.Result,
			Expected: recoveryExpectedDTO{State: observation.ExpectedState, Digest: observation.Digest, Mode: observation.Mode},
		}
		if observation.Owner != nil {
			dto.Owner = &recoveryOwnerDTO{Source: FormatSourceReference(observation.Owner.Source), SourceVersion: observation.Owner.ResolvedVersion, Artifact: observation.Owner.Artifact}
		}
		doc.Observations = append(doc.Observations, dto)
	}
	bytes, err := encodeYAML(doc)
	if err != nil {
		return err
	}
	return writeFileAtomically(filepath.Join(root, RecoveryStateFileName), bytes, 0o600)
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	n, err := file.Write(data)
	if err != nil {
		_ = file.Close()
		return err
	}
	if n != len(data) {
		_ = file.Close()
		return io.ErrShortWrite
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func canonicalSourceReference(root, raw string) (SourceIdentity, error) {
	source, err := ParseSourceReference(raw)
	if err != nil {
		return SourceIdentity{}, err
	}
	normalized, err := NormalizeSourceIdentity(root, source)
	if err != nil {
		return SourceIdentity{}, err
	}
	if FormatSourceReference(normalized) != raw {
		return SourceIdentity{}, fmt.Errorf("source reference is not canonical")
	}
	return normalized, nil
}

func validateCanonicalStateSource(root string, source SourceIdentity) error {
	normalized, err := NormalizeSourceIdentity(root, source)
	if err != nil {
		return err
	}
	if normalized != source {
		return fmt.Errorf("source locator is not normalized")
	}
	return nil
}
