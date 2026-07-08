package materialize

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

type Request struct {
	Root     string
	Key      repositorystate.ManagedArtifactKey
	Record   repositorystate.MaterializationRecord
	Artifact source.ArtifactDescriptor
}

type FileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Digest string `json:"digest"`
}

type Result struct {
	Changes      []FileChange
	CreatedPaths []string
}

type OwnershipConflictError struct {
	Path string
}

func (e OwnershipConflictError) Error() string {
	return fmt.Sprintf("managed file %q is already owned by another artifact", e.Path)
}

type DriftError struct {
	Path string
}

func (e DriftError) Error() string {
	return fmt.Sprintf("managed file %q has drifted from the recorded state", e.Path)
}

func Apply(_ context.Context, req Request) (Result, error) {
	owned := indexOwnedFiles(req.Record)
	result := Result{}

	for _, step := range req.Artifact.Steps {
		if step.Type != "file" {
			return Result{}, fmt.Errorf("unsupported step type %q", step.Type)
		}
		if owner, ok := owned[step.TargetPath]; ok && owner != req.Key {
			return Result{}, OwnershipConflictError{Path: step.TargetPath}
		}

		sourceBytes, err := os.ReadFile(step.SourcePath)
		if err != nil {
			return Result{}, err
		}

		targetPath, err := resolveTargetPath(req.Root, step.TargetPath)
		if err != nil {
			return Result{}, err
		}
		action := "created"
		currentBytes, err := os.ReadFile(targetPath)
		switch {
		case err == nil && bytes.Equal(currentBytes, sourceBytes):
			action = "unchanged"
		case err == nil:
			if priorDigest, ok := digestFor(req.Record, req.Key, step.TargetPath); ok && sha256Hex(currentBytes) != priorDigest {
				return Result{}, DriftError{Path: step.TargetPath}
			}
			action = "updated"
		case !os.IsNotExist(err):
			return Result{}, err
		}

		if action != "unchanged" {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return Result{}, err
			}
			if err := os.WriteFile(targetPath, sourceBytes, 0o644); err != nil {
				return Result{}, err
			}
			if action == "created" {
				result.CreatedPaths = append(result.CreatedPaths, targetPath)
			}
		}

		result.Changes = append(result.Changes, FileChange{
			Path:   step.TargetPath,
			Action: action,
			Digest: sha256Hex(sourceBytes),
		})
	}

	return result, nil
}

func resolveTargetPath(root string, targetPath string) (string, error) {
	cleanPath := filepath.Clean(targetPath)
	if cleanPath == "." || cleanPath == ".." || filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("file target path must stay within operation root")
	}
	if len(cleanPath) >= 3 && cleanPath[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("file target path must stay within operation root")
	}

	return filepath.Join(root, cleanPath), nil
}

func sha256Hex(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func indexOwnedFiles(record repositorystate.MaterializationRecord) map[string]repositorystate.ManagedArtifactKey {
	owned := make(map[string]repositorystate.ManagedArtifactKey)
	for _, artifact := range record.Artifacts {
		for _, file := range artifact.Files {
			owned[file.Path] = artifact.Key
		}
	}
	return owned
}

func digestFor(record repositorystate.MaterializationRecord, key repositorystate.ManagedArtifactKey, path string) (string, bool) {
	for _, artifact := range record.Artifacts {
		if artifact.Key != key {
			continue
		}
		for _, file := range artifact.Files {
			if file.Path == path {
				return file.Digest, true
			}
		}
	}
	return "", false
}
