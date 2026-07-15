package repositorystate

import "testing"

func TestValidateLockfileRejectsEmptyAndDuplicateSnapshotState(t *testing.T) {
	if ValidateLockfile(Lockfile{Resolutions: []Resolution{{Source: SourceIdentity{Type: "file", Locator: "x"}, ResolvedVersion: "v"}}}) == nil {
		t.Fatal("expected empty artifact rejection")
	}
	source := SourceIdentity{Type: "file", Locator: "x"}
	if ValidateLockfile(Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "a", Version: "1"}}}, {Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "b", Version: "1"}}}}}) == nil {
		t.Fatal("expected duplicate snapshot rejection")
	}
}
func TestLockfileLookupsAndSnapshotMerge(t *testing.T) {
	source := SourceIdentity{Type: "file", Locator: "x"}
	lock, _, err := Lockfile{}.UpsertResolution(Resolution{Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "a", Version: "1"}}})
	if err != nil {
		t.Fatal(err)
	}
	lock, _, err = lock.UpsertResolution(Resolution{Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "b", Version: "1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, a, ok := lock.Artifact(ArtifactKey{Source: source, Name: "b"}); !ok || a.Version != "1" {
		t.Fatal("missing merged artifact")
	}
}
