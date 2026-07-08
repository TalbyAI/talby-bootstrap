package repositorystate

import (
	"fmt"
	"slices"
	"strings"
)

func ValidateMaterializationRecord(record MaterializationRecord) error {
	seenArtifacts := map[string]struct{}{}
	seenFiles := map[string]struct{}{}

	for _, artifact := range record.Artifacts {
		if err := validateSourceIdentity(artifact.Key.Source); err != nil {
			return fmt.Errorf("managed artifact source: %w", err)
		}
		if artifact.Key.ResolvedVersion == "" {
			return fmt.Errorf("managed artifact resolved version is required")
		}
		if artifact.Key.Artifact == "" {
			return fmt.Errorf("managed artifact name is required")
		}
		key := managedArtifactKeyString(artifact.Key)
		if _, ok := seenArtifacts[key]; ok {
			return fmt.Errorf("duplicate managed artifact %s", key)
		}
		seenArtifacts[key] = struct{}{}

		for _, file := range artifact.Files {
			if file.Path == "" {
				return fmt.Errorf("managed file path is required")
			}
			if len(file.Digest) != 64 || strings.Trim(file.Digest, "0123456789abcdef") != "" {
				return fmt.Errorf("managed file digest must be a lowercase hex sha256")
			}
			if _, ok := seenFiles[file.Path]; ok {
				return fmt.Errorf("managed file path %q has multiple owners", file.Path)
			}
			seenFiles[file.Path] = struct{}{}
		}
	}

	return nil
}

func UpsertManagedArtifact(record MaterializationRecord, next ManagedArtifactRecord) MaterializationRecord {
	artifacts := append([]ManagedArtifactRecord(nil), record.Artifacts...)
	key := managedArtifactKeyString(next.Key)
	for i, artifact := range artifacts {
		if managedArtifactKeyString(artifact.Key) == key {
			artifacts[i] = next
			return MaterializationRecord{Artifacts: artifacts}
		}
	}
	artifacts = append(artifacts, next)
	slices.SortFunc(artifacts, func(a, b ManagedArtifactRecord) int {
		return strings.Compare(managedArtifactKeyString(a.Key), managedArtifactKeyString(b.Key))
	})
	return MaterializationRecord{Artifacts: artifacts}
}

func RemoveManagedArtifact(record MaterializationRecord, key ManagedArtifactKey) MaterializationRecord {
	filtered := make([]ManagedArtifactRecord, 0, len(record.Artifacts))
	target := managedArtifactKeyString(key)
	for _, artifact := range record.Artifacts {
		if managedArtifactKeyString(artifact.Key) == target {
			continue
		}
		filtered = append(filtered, artifact)
	}
	return MaterializationRecord{Artifacts: filtered}
}

func managedArtifactKeyString(key ManagedArtifactKey) string {
	return key.Source.Type + "\x00" + key.Source.Name + "\x00" + key.ResolvedVersion + "\x00" + key.Artifact
}
