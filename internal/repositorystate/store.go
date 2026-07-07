package repositorystate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	supportedSchemaVersion        = 1
	ManifestFileName              = "talby-artifacts.yaml"
	LockfileFileName              = "talby-artifacts.lock.yaml"
	MaterializationRecordFileName = "talby-artifacts.managed.yaml"
)

type Store interface {
	LoadManifest(context.Context, string) (Manifest, error)
	WriteManifest(context.Context, string, Manifest) error
	LoadLockfile(context.Context, string) (Lockfile, error)
	WriteLockfile(context.Context, string, Lockfile) error
	LoadMaterializationRecord(context.Context, string) (MaterializationRecord, error)
	WriteMaterializationRecord(context.Context, string, MaterializationRecord) error
}

type fileStore struct{}

func NewStore() Store {
	return fileStore{}
}

type manifestDocument struct {
	SchemaVersion int                      `yaml:"schema_version"`
	TrustPolicy   manifestTrustPolicyDTO   `yaml:"trust_policy,omitempty"`
	Declarations  []manifestDeclarationDTO `yaml:"declarations,omitempty"`
}

type manifestTrustPolicyDTO struct {
	ApprovedSources []SourceIdentity `yaml:"approved_sources,omitempty"`
}

type manifestDeclarationDTO struct {
	Source SourceIdentity    `yaml:"source"`
	Target manifestTargetDTO `yaml:"target"`
	Input  *manifestInputDTO `yaml:"input,omitempty"`
}

type manifestTargetDTO struct {
	Scope    DeclarationScope `yaml:"scope"`
	Artifact string           `yaml:"artifact,omitempty"`
}

type manifestInputDTO struct {
	Locator string `yaml:"locator,omitempty"`
	Version string `yaml:"version,omitempty"`
}

type lockfileDocument struct {
	SchemaVersion int                     `yaml:"schema_version"`
	Resolutions   []lockfileResolutionDTO `yaml:"resolutions,omitempty"`
}

type lockfileResolutionDTO struct {
	Source          SourceIdentity      `yaml:"source"`
	ResolvedVersion string              `yaml:"resolved_version"`
	Artifact        lockfileArtifactDTO `yaml:"artifact"`
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
	Key   materializationArtifactKeyDTO   `yaml:"key"`
	Files []materializationManagedFileDTO `yaml:"files,omitempty"`
}

type materializationArtifactKeyDTO struct {
	Source          SourceIdentity `yaml:"source"`
	ResolvedVersion string         `yaml:"resolved_version"`
	Artifact        string         `yaml:"artifact"`
}

type materializationManagedFileDTO struct {
	Path   string `yaml:"path"`
	Digest string `yaml:"digest"`
}

func (fileStore) LoadManifest(_ context.Context, root string) (Manifest, error) {
	bytes, err := os.ReadFile(filepath.Join(root, ManifestFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorNotFound, Err: err}
		}
		return Manifest{}, err
	}
	if strings.TrimSpace(string(bytes)) == "" {
		return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("file is empty")}
	}

	var doc manifestDocument
	if err := yaml.Unmarshal(bytes, &doc); err != nil {
		return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: err}
	}
	if doc.SchemaVersion != supportedSchemaVersion {
		return Manifest{}, StateFileError{
			File: StateFileManifest,
			Kind: StateFileErrorInvalidFormat,
			Err:  fmt.Errorf("schema_version must be %d", supportedSchemaVersion),
		}
	}

	manifest := Manifest{
		TrustPolicy:  TrustPolicy{ApprovedSources: append([]SourceIdentity(nil), doc.TrustPolicy.ApprovedSources...)},
		Declarations: make([]Declaration, 0, len(doc.Declarations)),
	}
	for _, dto := range doc.Declarations {
		manifest.Declarations = append(manifest.Declarations, Declaration{
			Source: dto.Source,
			Target: DeclarationTarget{Scope: dto.Target.Scope, Artifact: dto.Target.Artifact},
			Input:  sourceInputFromDTO(dto.Input),
		})
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat, Err: err}
	}

	return manifest, nil
}

func (fileStore) WriteManifest(_ context.Context, root string, manifest Manifest) error {
	if err := ValidateManifest(manifest); err != nil {
		return err
	}

	doc := manifestDocument{
		SchemaVersion: supportedSchemaVersion,
		TrustPolicy: manifestTrustPolicyDTO{
			ApprovedSources: append([]SourceIdentity(nil), manifest.TrustPolicy.ApprovedSources...),
		},
		Declarations: make([]manifestDeclarationDTO, 0, len(manifest.Declarations)),
	}
	sort.Slice(doc.TrustPolicy.ApprovedSources, func(i, j int) bool {
		left := doc.TrustPolicy.ApprovedSources[i]
		right := doc.TrustPolicy.ApprovedSources[j]
		if left.Type != right.Type {
			return left.Type < right.Type
		}
		return left.Name < right.Name
	})

	declarations := append([]Declaration(nil), manifest.Declarations...)
	sort.Slice(declarations, func(i, j int) bool {
		return declarationKey(declarations[i]) < declarationKey(declarations[j])
	})
	for _, decl := range declarations {
		doc.Declarations = append(doc.Declarations, manifestDeclarationDTO{
			Source: decl.Source,
			Target: manifestTargetDTO{Scope: decl.Target.Scope, Artifact: decl.Target.Artifact},
			Input:  manifestInputFromDomain(decl.Input),
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
	if strings.TrimSpace(string(bytes)) == "" {
		return Lockfile{}, StateFileError{File: StateFileLockfile, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("file is empty")}
	}

	var doc lockfileDocument
	if err := yaml.Unmarshal(bytes, &doc); err != nil {
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
		lockfile.Resolutions = append(lockfile.Resolutions, Resolution{
			Source:          dto.Source,
			ResolvedVersion: dto.ResolvedVersion,
			Artifact: ArtifactResolution{
				Name:    dto.Artifact.Name,
				Version: dto.Artifact.Version,
			},
		})
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

	doc := lockfileDocument{
		SchemaVersion: supportedSchemaVersion,
		Resolutions:   make([]lockfileResolutionDTO, 0, len(lockfile.Resolutions)),
	}

	resolutions := append([]Resolution(nil), lockfile.Resolutions...)
	sort.Slice(resolutions, func(i, j int) bool {
		return resolutionKey(resolutions[i]) < resolutionKey(resolutions[j])
	})
	for _, res := range resolutions {
		doc.Resolutions = append(doc.Resolutions, lockfileResolutionDTO{
			Source:          res.Source,
			ResolvedVersion: res.ResolvedVersion,
			Artifact: lockfileArtifactDTO{
				Name:    res.Artifact.Name,
				Version: res.Artifact.Version,
			},
		})
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
	if strings.TrimSpace(string(bytes)) == "" {
		return MaterializationRecord{}, StateFileError{File: StateFileMaterializationRecord, Kind: StateFileErrorInvalidFormat, Err: fmt.Errorf("file is empty")}
	}

	var doc materializationRecordDocument
	if err := yaml.Unmarshal(bytes, &doc); err != nil {
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
		files := make([]ManagedFileRecord, 0, len(dto.Files))
		for _, file := range dto.Files {
			files = append(files, ManagedFileRecord{
				Path:   file.Path,
				Digest: file.Digest,
			})
		}
		record.Artifacts = append(record.Artifacts, ManagedArtifactRecord{
			Key: ManagedArtifactKey{
				Source:          dto.Key.Source,
				ResolvedVersion: dto.Key.ResolvedVersion,
				Artifact:        dto.Key.Artifact,
			},
			Files: files,
		})
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

	doc := materializationRecordDocument{
		SchemaVersion: supportedSchemaVersion,
		Artifacts:     make([]materializationArtifactRecordDTO, 0, len(record.Artifacts)),
	}
	artifacts := append([]ManagedArtifactRecord(nil), record.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool {
		return managedArtifactKeyString(artifacts[i].Key) < managedArtifactKeyString(artifacts[j].Key)
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
		doc.Artifacts = append(doc.Artifacts, materializationArtifactRecordDTO{
			Key: materializationArtifactKeyDTO{
				Source:          artifact.Key.Source,
				ResolvedVersion: artifact.Key.ResolvedVersion,
				Artifact:        artifact.Key.Artifact,
			},
			Files: fileDTOs,
		})
	}

	bytes, err := encodeYAML(doc)
	if err != nil {
		return err
	}
	return writeFileAtomically(filepath.Join(root, MaterializationRecordFileName), bytes, 0o644)
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

func encodeYAML(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func sourceInputFromDTO(input *manifestInputDTO) *SourceInput {
	if input == nil {
		return nil
	}
	return &SourceInput{Locator: input.Locator, Version: input.Version}
}

func manifestInputFromDomain(input *SourceInput) *manifestInputDTO {
	if input == nil {
		return nil
	}
	return &manifestInputDTO{Locator: input.Locator, Version: input.Version}
}
