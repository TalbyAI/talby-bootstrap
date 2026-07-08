package file

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/talby/talby-bootstrap/internal/source"
	"gopkg.in/yaml.v3"
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

type Source struct{}

type artifactStep struct {
	Type   string `yaml:"type"`
	Path   string `yaml:"path"`
	Source string `yaml:"source"`
}

func New() Source {
	return Source{}
}

func (Source) Capabilities() source.Capabilities {
	return source.Capabilities{
		SupportsVersions:   false,
		ProvidesIdentity:   true,
		ProvidesTimestamp:  false,
		EnumeratesVersions: false,
	}
}

func (Source) Resolve(_ context.Context, req source.ResolveRequest) (source.ResolvedSource, error) {
	sourcePath := filepath.Join(req.Ref.Locator, sourceDescriptorName)
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

	resolved := source.ResolvedSource{
		Identity: source.Identity{
			Type: "file",
			Name: descriptor.Source.Name,
		},
		SourcePath: req.Ref.Locator,
		Artifacts:  make([]source.ArtifactDescriptor, 0, len(descriptor.Artifacts)),
	}
	snapshot := sha256.New()
	snapshot.Write(sourceBytes)

	for _, artifactRef := range descriptor.Artifacts {
		artifactDir, err := resolveArtifactDir(req.Ref.Locator, artifactRef.Path)
		if err != nil {
			return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", sourcePath, err)
		}

		artifactPath := filepath.Join(artifactDir, artifactDescriptorName)
		artifactBytes, err := os.ReadFile(artifactPath)
		if err != nil {
			return source.ResolvedSource{}, fmt.Errorf("read %s: %w", artifactPath, err)
		}

		var descriptor artifactDescriptor
		if err := yaml.Unmarshal(artifactBytes, &descriptor); err != nil {
			return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, err)
		}
		if err := validateArtifactDescriptor(artifactRef, descriptor); err != nil {
			return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, err)
		}
		snapshot.Write([]byte(artifactRef.Name))
		snapshot.Write([]byte{0})
		snapshot.Write([]byte(artifactRef.Path))
		snapshot.Write([]byte{0})
		snapshot.Write(artifactBytes)

		steps := make([]source.MaterializationStep, 0, len(descriptor.Steps))
		for _, step := range descriptor.Steps {
			if step.Type != "file" {
				return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, fmt.Errorf("unsupported step type %q", step.Type))
			}
			if step.Path == "" {
				return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, fmt.Errorf("file step path is required"))
			}
			if step.Source == "" {
				return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, fmt.Errorf("file step source is required"))
			}

			stepSourcePath, err := resolveRelativePath(artifactDir, step.Source, "file step source must stay within artifact directory")
			if err != nil {
				return source.ResolvedSource{}, fmt.Errorf("parse %s: %w", artifactPath, err)
			}
			stepBytes, err := os.ReadFile(stepSourcePath)
			if err != nil {
				return source.ResolvedSource{}, fmt.Errorf("read %s: %w", stepSourcePath, err)
			}
			snapshot.Write([]byte(step.Path))
			snapshot.Write([]byte{0})
			snapshot.Write(stepBytes)

			steps = append(steps, source.MaterializationStep{
				Type:       step.Type,
				TargetPath: step.Path,
				SourcePath: stepSourcePath,
			})
		}

		resolved.Artifacts = append(resolved.Artifacts, source.ArtifactDescriptor{
			Name:    artifactRef.Name,
			Version: descriptor.Artifact.Version,
			Path:    artifactRef.Path,
			Steps:   steps,
		})
	}
	resolved.Identity.Version = "local-snapshot-" + hex.EncodeToString(snapshot.Sum(nil))

	return resolved, nil
}

func validateSourceDescriptor(descriptor sourceDescriptor) error {
	if descriptor.SchemaVersion != supportedSchemaVersion {
		return fmt.Errorf("schema_version must be %d", supportedSchemaVersion)
	}
	if descriptor.Source.Name == "" {
		return fmt.Errorf("source name is required")
	}
	seen := make(map[string]struct{}, len(descriptor.Artifacts))
	for _, artifact := range descriptor.Artifacts {
		if artifact.Name == "" {
			return fmt.Errorf("artifact name is required")
		}
		if artifact.Path == "" {
			return fmt.Errorf("artifact path is required")
		}
		if _, ok := seen[artifact.Name]; ok {
			return fmt.Errorf("duplicate artifact name %q", artifact.Name)
		}
		seen[artifact.Name] = struct{}{}
	}

	return nil
}

func resolveArtifactDir(root string, artifactPath string) (string, error) {
	return resolveRelativePath(root, artifactPath, "artifact path must stay within source root")
}

func resolveRelativePath(root string, relativePath string, errText string) (string, error) {
	cleanPath := filepath.Clean(relativePath)
	if cleanPath == "." || cleanPath == ".." || filepath.IsAbs(cleanPath) {
		return "", errors.New(errText)
	}
	if len(cleanPath) >= 3 && cleanPath[:3] == ".."+string(filepath.Separator) {
		return "", errors.New(errText)
	}

	return filepath.Join(root, cleanPath), nil
}

func validateArtifactDescriptor(ref artifactRef, descriptor artifactDescriptor) error {
	if descriptor.SchemaVersion != supportedSchemaVersion {
		return fmt.Errorf("schema_version must be %d", supportedSchemaVersion)
	}
	if descriptor.Artifact.Name == "" {
		return fmt.Errorf("artifact name is required")
	}
	if descriptor.Artifact.Name != ref.Name {
		return fmt.Errorf("artifact name %q does not match source descriptor name %q", descriptor.Artifact.Name, ref.Name)
	}
	if descriptor.Artifact.Version == "" {
		return fmt.Errorf("artifact version is required")
	}

	return nil
}
