package file

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

const supportedSchemaVersion = 1

var (
	artifactNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	semVerPattern       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

type sourceDescriptor struct {
	SchemaVersion int           `yaml:"schema_version"`
	Artifacts     []artifactRef `yaml:"artifacts"`
}

type artifactRef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type artifactDescriptor struct {
	SchemaVersion int `yaml:"schema_version"`
	Artifact      struct {
		Name        string `yaml:"name"`
		Version     string `yaml:"version"`
		Description string `yaml:"description,omitempty"`
	} `yaml:"artifact"`
	Steps []artifactStep `yaml:"steps"`
}

type artifactStep struct {
	Type   string `yaml:"type"`
	Path   string `yaml:"path"`
	Source string `yaml:"source"`
}

type SourceDescriptor struct {
	Artifacts []ArtifactReference
}

type ArtifactReference struct {
	Name string
	Path string
}

type ArtifactDescriptor struct {
	Name        string
	Version     string
	Description string
	Steps       []ArtifactStep
}

type ArtifactStep struct {
	Type   string
	Path   string
	Source string
}

type Source struct{}

func New() Source                                { return Source{} }
func (Source) Capabilities() source.Capabilities { return source.Capabilities{ProvidesIdentity: true} }

func EncodeSourceDescriptor(value SourceDescriptor) ([]byte, error) {
	artifacts := append([]ArtifactReference(nil), value.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].Name != artifacts[j].Name {
			return artifacts[i].Name < artifacts[j].Name
		}
		return artifacts[i].Path < artifacts[j].Path
	})
	descriptor := sourceDescriptor{SchemaVersion: supportedSchemaVersion, Artifacts: make([]artifactRef, 0, len(artifacts))}
	for _, artifact := range artifacts {
		descriptor.Artifacts = append(descriptor.Artifacts, artifactRef{Name: artifact.Name, Path: artifact.Path})
	}
	if err := validateSourceDescriptor(descriptor); err != nil {
		return nil, err
	}
	return repositorystate.EncodeYAML(descriptor)
}

func EncodeArtifactDescriptor(value ArtifactDescriptor) ([]byte, error) {
	descriptor := artifactDescriptor{SchemaVersion: supportedSchemaVersion}
	descriptor.Artifact.Name = value.Name
	descriptor.Artifact.Version = value.Version
	descriptor.Artifact.Description = value.Description
	descriptor.Steps = make([]artifactStep, 0, len(value.Steps))
	for _, step := range value.Steps {
		descriptor.Steps = append(descriptor.Steps, artifactStep{Type: step.Type, Path: step.Path, Source: step.Source})
	}
	if err := validateArtifactDescriptor(artifactRef{Name: value.Name}, descriptor); err != nil {
		return nil, err
	}
	return repositorystate.EncodeYAML(descriptor)
}

func writeSnapshotField(snapshot hash.Hash, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = snapshot.Write(size[:])
	_, _ = snapshot.Write(value)
}

func canonicalExistingDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func realParentDirs(root *os.Root, relative string) error {
	for parent := filepath.Dir(relative); parent != "."; parent = filepath.Dir(parent) {
		info, err := root.Lstat(parent)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || isWindowsReparsePoint(info) || !info.IsDir() {
			return fmt.Errorf("source path must be a real directory")
		}
	}
	return nil
}

func lstatSourceEntry(root *os.Root, relative string) (os.FileInfo, error) {
	if err := realParentDirs(root, relative); err != nil {
		return nil, err
	}
	return root.Lstat(relative)
}

func realDir(root *os.Root, relative string) error {
	info, err := lstatSourceEntry(root, relative)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || isWindowsReparsePoint(info) || !info.IsDir() {
		return fmt.Errorf("source path must be a real directory")
	}
	return nil
}

func readRealFile(root *os.Root, relative string) ([]byte, error) {
	info, err := lstatSourceEntry(root, relative)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || isWindowsReparsePoint(info) || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("source input must be a regular file")
	}
	return root.ReadFile(relative)
}

func (Source) Resolve(_ context.Context, req source.ResolveRequest) (source.ResolvedSource, error) {
	if req.Ref.Version != "" {
		return source.ResolvedSource{}, fmt.Errorf("file source does not support requested versions")
	}
	canonicalRoot, err := canonicalExistingDir(req.Ref.Locator)
	if err != nil {
		return source.ResolvedSource{}, err
	}
	root, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return source.ResolvedSource{}, err
	}
	defer func() { _ = root.Close() }()

	sourcePath := filepath.Join(canonicalRoot, repositorystate.SourceDescriptorFileName)
	sourceBytes, err := readRealFile(root, repositorystate.SourceDescriptorFileName)
	if err != nil {
		return source.ResolvedSource{}, fmt.Errorf("read %s: %w", sourcePath, err)
	}
	var descriptor sourceDescriptor
	if err := repositorystate.DecodeStrictYAML(sourceBytes, &descriptor); err != nil {
		return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", sourcePath, err)
	}
	if err := validateSourceDescriptor(descriptor); err != nil {
		return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", sourcePath, err)
	}

	resolved := source.ResolvedSource{Identity: source.Identity{Type: "file", Name: filepath.Base(canonicalRoot)}, Artifacts: make([]source.ArtifactDescriptor, 0, len(descriptor.Artifacts)), InputPaths: []string{sourcePath}}
	snapshot := sha256.New()
	writeSnapshotField(snapshot, sourceBytes)
	for _, ref := range descriptor.Artifacts {
		artifactDir := filepath.FromSlash(ref.Path)
		if err := realDir(root, artifactDir); err != nil {
			return source.ResolvedSource{}, err
		}
		artifactRoot, err := root.OpenRoot(artifactDir)
		if err != nil {
			return source.ResolvedSource{}, err
		}
		defer func() { _ = artifactRoot.Close() }()
		artifactPath := filepath.Join(canonicalRoot, artifactDir, repositorystate.ArtifactDescriptorFileName)
		data, err := readRealFile(artifactRoot, repositorystate.ArtifactDescriptorFileName)
		if err != nil {
			return source.ResolvedSource{}, fmt.Errorf("read %s: %w", artifactPath, err)
		}
		var artifact artifactDescriptor
		if err := repositorystate.DecodeStrictYAML(data, &artifact); err != nil {
			return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, err)
		}
		if err := validateArtifactDescriptor(ref, artifact); err != nil {
			return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, err)
		}
		resolved.InputPaths = append(resolved.InputPaths, artifactPath)
		writeSnapshotField(snapshot, []byte(ref.Name))
		writeSnapshotField(snapshot, []byte(ref.Path))
		writeSnapshotField(snapshot, data)
		steps := make([]source.MaterializationStep, 0, len(artifact.Steps))
		for _, step := range artifact.Steps {
			inputRelative := filepath.FromSlash(step.Source)
			input := filepath.Join(canonicalRoot, artifactDir, inputRelative)
			bytes, err := readRealFile(artifactRoot, inputRelative)
			if err != nil {
				return source.ResolvedSource{}, fmt.Errorf("read %s: %w", input, err)
			}
			resolved.InputPaths = append(resolved.InputPaths, input)
			writeSnapshotField(snapshot, []byte(step.Path))
			writeSnapshotField(snapshot, bytes)
			steps = append(steps, source.MaterializationStep{Type: step.Type, TargetPath: step.Path, SourceBytes: bytes})
		}
		resolved.Artifacts = append(resolved.Artifacts, source.ArtifactDescriptor{Name: ref.Name, Version: artifact.Artifact.Version, Path: ref.Path, Steps: steps})
	}
	resolved.Identity.Version = "sha256:" + hex.EncodeToString(snapshot.Sum(nil))
	return resolved, nil
}

func validateSourceDescriptor(d sourceDescriptor) error {
	if d.SchemaVersion != supportedSchemaVersion {
		return fmt.Errorf("schema_version must be %d", supportedSchemaVersion)
	}
	if len(d.Artifacts) == 0 {
		return fmt.Errorf("source must contain at least one artifact")
	}
	names, paths := map[string]struct{}{}, map[string]struct{}{}
	for _, artifact := range d.Artifacts {
		if !artifactNamePattern.MatchString(artifact.Name) {
			return fmt.Errorf("artifact name must be lowercase ASCII hyphenated")
		}
		if err := validateRelativePath(artifact.Path); err != nil {
			return fmt.Errorf("artifact path: %w", err)
		}
		if _, ok := names[artifact.Name]; ok {
			return fmt.Errorf("duplicate artifact name %q", artifact.Name)
		}
		if _, ok := paths[artifact.Path]; ok {
			return fmt.Errorf("duplicate artifact path %q", artifact.Path)
		}
		names[artifact.Name], paths[artifact.Path] = struct{}{}, struct{}{}
	}
	return nil
}

func validateArtifactDescriptor(ref artifactRef, d artifactDescriptor) error {
	if d.SchemaVersion != supportedSchemaVersion {
		return fmt.Errorf("schema_version must be %d", supportedSchemaVersion)
	}
	if d.Artifact.Name != ref.Name {
		return fmt.Errorf("artifact name does not match source descriptor")
	}
	if !artifactNamePattern.MatchString(d.Artifact.Name) {
		return fmt.Errorf("artifact name must be lowercase ASCII hyphenated")
	}
	if !semVerPattern.MatchString(d.Artifact.Version) {
		return fmt.Errorf("artifact version must be canonical SemVer")
	}
	if d.Artifact.Description != "" && strings.TrimSpace(d.Artifact.Description) == "" {
		return fmt.Errorf("artifact description must be non-empty")
	}
	if len(d.Steps) == 0 {
		return fmt.Errorf("artifact must contain at least one materialization step")
	}
	targets := map[string]struct{}{}
	for _, step := range d.Steps {
		if step.Type != "file" {
			return fmt.Errorf("unsupported materialization step type %q", step.Type)
		}
		if err := validateRelativePath(step.Source); err != nil {
			return fmt.Errorf("file step source: %w", err)
		}
		if err := validateRelativePath(step.Path); err != nil {
			return fmt.Errorf("file step target: %w", err)
		}
		if step.Path == repositorystate.ArtifactDescriptorFileName || step.Path == repositorystate.SourceDescriptorFileName {
			return fmt.Errorf("file step target must not be a descriptor")
		}
		if _, ok := targets[step.Path]; ok {
			return fmt.Errorf("duplicate file step target %q", step.Path)
		}
		targets[step.Path] = struct{}{}
	}
	return nil
}

func validateRelativePath(value string) error {
	if value == "" ||
		strings.Contains(value, "\\") ||
		path.IsAbs(value) ||
		(len(value) >= 2 && value[1] == ':' &&
			((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z'))) {
		return fmt.Errorf("path must be clean and relative")
	}
	clean := path.Clean(value)
	if clean != value || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path must be clean and relative")
	}
	return nil
}
