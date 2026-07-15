package file

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/talby/talby-bootstrap/internal/source"
	"gopkg.in/yaml.v3"
	"hash"
	"os"
	"path/filepath"
	"strings"
)

const (
	sourceDescriptorName   = "talby-source.yaml"
	artifactDescriptorName = "talby-artifact.yaml"
	supportedSchemaVersion = 1
)

type sourceDescriptor struct {
	SchemaVersion int `yaml:"schema_version"`
	Source        struct {
		Name string `yaml:"name"`
	} `yaml:"source"`
	Artifacts []artifactRef `yaml:"artifacts"`
}
type artifactRef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}
type artifactDescriptor struct {
	SchemaVersion int `yaml:"schema_version"`
	Artifact      struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"artifact"`
	Steps []artifactStep `yaml:"steps"`
}
type artifactStep struct {
	Type   string `yaml:"type"`
	Path   string `yaml:"path"`
	Source string `yaml:"source"`
}
type Source struct{}

func New() Source                                { return Source{} }
func (Source) Capabilities() source.Capabilities { return source.Capabilities{ProvidesIdentity: true} }
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
func canonicalContained(root, path, message string) (string, error) {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, canonical)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New(message)
	}
	return canonical, nil
}
func (Source) Resolve(_ context.Context, req source.ResolveRequest) (source.ResolvedSource, error) {
	if req.Ref.Version != "" {
		return source.ResolvedSource{}, fmt.Errorf("file source does not support requested versions")
	}
	root, err := canonicalExistingDir(req.Ref.Locator)
	if err != nil {
		return source.ResolvedSource{}, err
	}
	sourcePath, err := canonicalContained(root, filepath.Join(root, sourceDescriptorName), "source descriptor must stay within source root")
	if err != nil {
		return source.ResolvedSource{}, err
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return source.ResolvedSource{}, fmt.Errorf("read %s: %w", sourcePath, err)
	}
	var descriptor sourceDescriptor
	if err := yaml.Unmarshal(sourceBytes, &descriptor); err != nil {
		return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", sourcePath, err)
	}
	if err := validateSourceDescriptor(descriptor); err != nil {
		return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", sourcePath, err)
	}
	resolved := source.ResolvedSource{Identity: source.Identity{Type: "file", Name: descriptor.Source.Name}, Artifacts: make([]source.ArtifactDescriptor, 0, len(descriptor.Artifacts)), InputPaths: []string{sourcePath}}
	snapshot := sha256.New()
	writeSnapshotField(snapshot, sourceBytes)
	for _, ref := range descriptor.Artifacts {
		dir, err := canonicalContained(root, filepath.Join(root, ref.Path), "artifact path must stay within source root")
		if err != nil {
			return source.ResolvedSource{}, err
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			if err != nil {
				return source.ResolvedSource{}, err
			}
			return source.ResolvedSource{}, fmt.Errorf("artifact path is not a directory")
		}
		artifactPath, err := canonicalContained(dir, filepath.Join(dir, artifactDescriptorName), "artifact descriptor must stay within artifact directory")
		if err != nil {
			return source.ResolvedSource{}, err
		}
		data, err := os.ReadFile(artifactPath)
		if err != nil {
			return source.ResolvedSource{}, fmt.Errorf("read %s: %w", artifactPath, err)
		}
		var artifact artifactDescriptor
		if err := yaml.Unmarshal(data, &artifact); err != nil {
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
			if step.Path == "" {
				return source.ResolvedSource{}, fmt.Errorf("parse %s: file step path is required", artifactPath)
			}
			out := source.MaterializationStep{Type: step.Type, TargetPath: step.Path}
			if step.Type == "file" {
				if step.Source == "" {
					return source.ResolvedSource{}, fmt.Errorf("parse %s: file step source is required", artifactPath)
				}
				input, err := canonicalContained(dir, filepath.Join(dir, step.Source), "file step source must stay within artifact directory")
				if err != nil {
					return source.ResolvedSource{}, err
				}
				bytes, err := os.ReadFile(input)
				if err != nil {
					return source.ResolvedSource{}, fmt.Errorf("read %s: %w", input, err)
				}
				out.SourceBytes = bytes
				resolved.InputPaths = append(resolved.InputPaths, input)
				writeSnapshotField(snapshot, []byte(step.Path))
				writeSnapshotField(snapshot, bytes)
			}
			steps = append(steps, out)
		}
		resolved.Artifacts = append(resolved.Artifacts, source.ArtifactDescriptor{Name: ref.Name, Version: artifact.Artifact.Version, Path: ref.Path, Steps: steps})
	}
	resolved.Identity.Version = "local-snapshot-" + hex.EncodeToString(snapshot.Sum(nil))
	return resolved, nil
}
func validateSourceDescriptor(d sourceDescriptor) error {
	if d.SchemaVersion != supportedSchemaVersion {
		return fmt.Errorf("schema_version must be %d", supportedSchemaVersion)
	}
	if d.Source.Name == "" {
		return fmt.Errorf("source name is required")
	}
	if len(d.Artifacts) == 0 {
		return fmt.Errorf("source must contain at least one artifact")
	}
	seen := map[string]struct{}{}
	for _, a := range d.Artifacts {
		if a.Name == "" || a.Path == "" {
			return fmt.Errorf("artifact name and path are required")
		}
		if _, ok := seen[a.Name]; ok {
			return fmt.Errorf("duplicate artifact name %q", a.Name)
		}
		seen[a.Name] = struct{}{}
	}
	return nil
}
func validateArtifactDescriptor(ref artifactRef, d artifactDescriptor) error {
	if d.SchemaVersion != supportedSchemaVersion {
		return fmt.Errorf("schema_version must be %d", supportedSchemaVersion)
	}
	if d.Artifact.Name == "" || d.Artifact.Name != ref.Name {
		return fmt.Errorf("artifact name does not match source descriptor")
	}
	if d.Artifact.Version == "" {
		return fmt.Errorf("artifact version is required")
	}
	if len(d.Steps) == 0 {
		return fmt.Errorf("artifact must contain at least one materialization step")
	}
	return nil
}
