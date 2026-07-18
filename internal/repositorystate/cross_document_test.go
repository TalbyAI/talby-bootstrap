package repositorystate

import (
	"strings"
	"testing"
)

func TestValidateCrossDocumentStateRequiresMatchingDeclarationsAndResolutions(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./source"}
	version := "sha256:" + strings.Repeat("a", 64)
	manifest := Manifest{Declarations: []Declaration{{
		Source: source,
		Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "a"},
	}}}
	lock := Lockfile{Resolutions: []Resolution{{
		Source:          source,
		ResolvedVersion: version,
		Artifacts:       []ArtifactResolution{{Name: "a", Version: "1.0.0"}},
	}}}
	record := MaterializationRecord{Artifacts: []ManagedArtifactRecord{{
		Source:          source,
		ResolvedVersion: version,
		Artifact:        "a",
		ArtifactVersion: "1.0.0",
		Files:           []ManagedFileRecord{{Path: "a", Digest: version}},
	}}}
	if err := ValidateCrossDocumentState(lock, record); err != nil {
		t.Fatal(err)
	}

	for name, invalid := range map[string]struct {
		manifest Manifest
		lock     Lockfile
		record   MaterializationRecord
	}{
		"record version mismatch": {
			manifest: manifest,
			lock:     lock,
			record: MaterializationRecord{Artifacts: []ManagedArtifactRecord{{
				Source: source, ResolvedVersion: "sha256:" + strings.Repeat("b", 64), Artifact: "a", ArtifactVersion: "1.0.0",
				Files: []ManagedFileRecord{{Path: "a", Digest: version}},
			}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateCrossDocumentState(invalid.lock, invalid.record); err == nil {
				t.Fatal("expected cross-document validation error")
			}
		})
	}
}
