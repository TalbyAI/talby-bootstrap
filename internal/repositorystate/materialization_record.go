package repositorystate

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

func managedPathKey(path string) string {
	path = filepath.Clean(filepath.FromSlash(path))
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func ValidateMaterializationRecord(record MaterializationRecord) error {
	owners := map[ArtifactKey]struct{}{}
	paths := map[string]struct{}{}
	for _, a := range record.Artifacts {
		if a.Source.Type == "" || a.Source.Locator == "" || a.ResolvedVersion == "" || a.Artifact == "" || a.ArtifactVersion == "" {
			return fmt.Errorf("complete managed artifact fields are required")
		}
		k := ManagedArtifactKey(a)
		if _, ok := owners[k]; ok {
			return fmt.Errorf("duplicate managed artifact")
		}
		owners[k] = struct{}{}
		if len(a.Files) == 0 {
			return fmt.Errorf("managed artifact requires files")
		}
		for _, f := range a.Files {
			native := filepath.FromSlash(f.Path)
			if f.Path == "" || filepath.IsAbs(native) || filepath.ToSlash(filepath.Clean(native)) != f.Path {
				return fmt.Errorf("managed file path must be canonical")
			}
			if len(f.Digest) != 64 || strings.Trim(f.Digest, "0123456789abcdef") != "" {
				return fmt.Errorf("managed file digest must be a lowercase hex sha256")
			}
			key := managedPathKey(f.Path)
			if _, ok := paths[key]; ok {
				return fmt.Errorf("managed file path has multiple owners")
			}
			paths[key] = struct{}{}
		}
	}
	return nil
}
func (record MaterializationRecord) Artifact(key ArtifactKey) (ManagedArtifactRecord, bool) {
	for _, a := range record.Artifacts {
		if ManagedArtifactKey(a) == key {
			return a, true
		}
	}
	return ManagedArtifactRecord{}, false
}
func UpsertManagedArtifact(record MaterializationRecord, next ManagedArtifactRecord) MaterializationRecord {
	values := append([]ManagedArtifactRecord(nil), record.Artifacts...)
	key := ManagedArtifactKey(next)
	for i, a := range values {
		if ManagedArtifactKey(a) == key {
			values[i] = next
			sortManaged(values)
			return MaterializationRecord{Artifacts: values}
		}
	}
	values = append(values, next)
	sortManaged(values)
	return MaterializationRecord{Artifacts: values}
}
func ManagedArtifactKey(record ManagedArtifactRecord) ArtifactKey {
	return ArtifactKey{Source: record.Source, Name: record.Artifact}
}
func sortManaged(values []ManagedArtifactRecord) {
	for i := range values {
		slices.SortFunc(values[i].Files, func(a, b ManagedFileRecord) int { return strings.Compare(a.Path, b.Path) })
	}
	slices.SortFunc(values, func(a, b ManagedArtifactRecord) int {
		return strings.Compare(SourceIdentityKey(a.Source)+"\x00"+a.Artifact, SourceIdentityKey(b.Source)+"\x00"+b.Artifact)
	})
}
