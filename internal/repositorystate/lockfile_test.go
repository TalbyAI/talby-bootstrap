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

func TestLockfileSnapshotAndArtifactMisses(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "x"}
	lock := Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "a", Version: "1"}}}}}
	if _, ok := lock.Snapshot(SnapshotKey{Source: source, ResolvedVersion: "v"}); !ok {
		t.Fatal("expected snapshot")
	}
	if _, ok := lock.Snapshot(SnapshotKey{Source: source, ResolvedVersion: "missing"}); ok {
		t.Fatal("unexpected snapshot")
	}
	if _, _, ok := lock.Artifact(ArtifactKey{Source: SourceIdentity{Type: SourceTypeFile, Locator: "other"}, Name: "a"}); ok {
		t.Fatal("unexpected artifact for other source")
	}
	if _, _, ok := lock.Artifact(ArtifactKey{Source: source, Name: "missing"}); ok {
		t.Fatal("unexpected artifact")
	}
}

func TestLockfileUpsertKindsConflictsAndSorting(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "x"}
	first := Resolution{Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "b", Version: "1"}}}
	lock, kind, err := (Lockfile{}).UpsertResolution(first)
	if err != nil || kind != ChangeKindInserted {
		t.Fatalf("insert = %#v, %q, %v", lock, kind, err)
	}
	lock, kind, err = lock.UpsertResolution(first)
	if err != nil || kind != ChangeKindUnchanged {
		t.Fatalf("same = %#v, %q, %v", lock, kind, err)
	}
	lock, kind, err = lock.UpsertResolution(Resolution{Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "a", Version: "2"}}})
	if err != nil || kind != ChangeKindReplaced || lock.Resolutions[0].Artifacts[0].Name != "a" {
		t.Fatalf("merge = %#v, %q, %v", lock, kind, err)
	}
	if _, _, err := lock.UpsertResolution(Resolution{Source: source, ResolvedVersion: "other", Artifacts: []ArtifactResolution{{Name: "a", Version: "2"}}}); err == nil {
		t.Fatal("expected snapshot ownership conflict")
	}
	if _, _, err := lock.UpsertResolution(Resolution{}); err == nil {
		t.Fatal("expected invalid resolution")
	}
}

func TestLockfileKeepArtifactsRemovesEmptySnapshotsAndSorts(t *testing.T) {
	a := SourceIdentity{Type: SourceTypeFile, Locator: "a"}
	b := SourceIdentity{Type: SourceTypeFile, Locator: "b"}
	lock := Lockfile{Resolutions: []Resolution{
		{Source: b, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "drop", Version: "1"}}},
		{Source: a, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "keep", Version: "1"}, {Name: "drop", Version: "1"}}},
	}}
	kept, removed := lock.KeepArtifacts(map[ArtifactKey]struct{}{{Source: a, Name: "keep"}: {}})
	if len(kept.Resolutions) != 1 || len(kept.Resolutions[0].Artifacts) != 1 || kept.Resolutions[0].Artifacts[0].Name != "keep" {
		t.Fatalf("kept = %#v", kept)
	}
	if len(removed) != 2 || removed[0].Source != a || removed[1].Source != b {
		t.Fatalf("removed = %#v", removed)
	}
}

func TestValidateLockfileRejectsIncompleteAndMultiplyOwnedArtifacts(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "x"}
	if ValidateLockfile(Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: "v", Artifacts: []ArtifactResolution{{Name: "", Version: "1"}}}}}) == nil {
		t.Fatal("expected incomplete artifact rejection")
	}
	lock := Lockfile{Resolutions: []Resolution{
		{Source: source, ResolvedVersion: "v1", Artifacts: []ArtifactResolution{{Name: "a", Version: "1"}}},
		{Source: source, ResolvedVersion: "v2", Artifacts: []ArtifactResolution{{Name: "a", Version: "2"}}},
	}}
	if ValidateLockfile(lock) == nil {
		t.Fatal("expected multiply owned artifact rejection")
	}
}
