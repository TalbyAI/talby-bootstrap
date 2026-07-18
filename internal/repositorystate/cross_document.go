package repositorystate

import "fmt"

func ValidateCrossDocumentState(lock Lockfile, record MaterializationRecord) error {
	for _, artifact := range record.Artifacts {
		resolution, locked, ok := lock.Artifact(ArtifactKey{Source: artifact.Source, Name: artifact.Artifact})
		if !ok || resolution.ResolvedVersion != artifact.ResolvedVersion || resolution.Commit != artifact.Commit || locked.Version != artifact.ArtifactVersion {
			return fmt.Errorf("materialized artifact %q does not match a lockfile resolution", artifact.Artifact)
		}
	}
	return nil
}
