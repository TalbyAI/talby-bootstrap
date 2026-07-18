package repositorystate

import (
	"strings"
	"testing"
)

func lockSnapshot(hexDigit string) string { return "sha256:" + strings.Repeat(hexDigit, 64) }

func TestValidateLockfileRejectsEmptyAndDuplicateSnapshotState(t *testing.T) {
	if ValidateLockfile(Lockfile{Resolutions: []Resolution{{Source: SourceIdentity{Type: SourceTypeFile, Locator: "./x"}, ResolvedVersion: lockSnapshot("a")}}}) == nil {
		t.Fatal("expected empty artifact rejection")
	}
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./x"}
	if ValidateLockfile(Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "a", Version: "1.0.0"}}}, {Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "b", Version: "1.0.0"}}}}}) == nil {
		t.Fatal("expected duplicate snapshot rejection")
	}
}
func TestLockfileLookupsAndSnapshotMerge(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./x"}
	lock, _, err := Lockfile{}.UpsertResolution(Resolution{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "a", Version: "1.0.0"}}})
	if err != nil {
		t.Fatal(err)
	}
	lock, _, err = lock.UpsertResolution(Resolution{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "b", Version: "1.0.0"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, a, ok := lock.Artifact(ArtifactKey{Source: source, Name: "b"}); !ok || a.Version != "1.0.0" {
		t.Fatal("missing merged artifact")
	}
}

func TestLockfileSnapshotAndArtifactMisses(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./x"}
	lock := Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "a", Version: "1.0.0"}}}}}
	if _, ok := lock.Snapshot(SnapshotKey{Source: source, ResolvedVersion: lockSnapshot("a")}); !ok {
		t.Fatal("expected snapshot")
	}
	if _, ok := lock.Snapshot(SnapshotKey{Source: source, ResolvedVersion: lockSnapshot("b")}); ok {
		t.Fatal("unexpected snapshot")
	}
	if _, _, ok := lock.Artifact(ArtifactKey{Source: SourceIdentity{Type: SourceTypeFile, Locator: "./other"}, Name: "a"}); ok {
		t.Fatal("unexpected artifact for other source")
	}
	if _, _, ok := lock.Artifact(ArtifactKey{Source: source, Name: "missing"}); ok {
		t.Fatal("unexpected artifact")
	}
}

func TestLockfileUpsertKindsConflictsAndSorting(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./x"}
	first := Resolution{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "b", Version: "1.0.0"}}}
	lock, kind, err := (Lockfile{}).UpsertResolution(first)
	if err != nil || kind != ChangeKindInserted {
		t.Fatalf("insert = %#v, %q, %v", lock, kind, err)
	}
	lock, kind, err = lock.UpsertResolution(first)
	if err != nil || kind != ChangeKindUnchanged {
		t.Fatalf("same = %#v, %q, %v", lock, kind, err)
	}
	lock, kind, err = lock.UpsertResolution(Resolution{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "a", Version: "2.0.0"}}})
	if err != nil || kind != ChangeKindReplaced || lock.Resolutions[0].Artifacts[0].Name != "a" {
		t.Fatalf("merge = %#v, %q, %v", lock, kind, err)
	}
	if _, _, err := lock.UpsertResolution(Resolution{Source: source, ResolvedVersion: lockSnapshot("b"), Artifacts: []ArtifactResolution{{Name: "a", Version: "2.0.0"}}}); err == nil {
		t.Fatal("expected snapshot ownership conflict")
	}
	if _, _, err := lock.UpsertResolution(Resolution{}); err == nil {
		t.Fatal("expected invalid resolution")
	}
}

func TestLockfileKeepArtifactsRemovesEmptySnapshotsAndSorts(t *testing.T) {
	a := SourceIdentity{Type: SourceTypeFile, Locator: "./a"}
	b := SourceIdentity{Type: SourceTypeFile, Locator: "./b"}
	lock := Lockfile{Resolutions: []Resolution{
		{Source: b, ResolvedVersion: lockSnapshot("b"), Artifacts: []ArtifactResolution{{Name: "drop", Version: "1.0.0"}}},
		{Source: a, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "keep", Version: "1.0.0"}, {Name: "drop", Version: "1.0.0"}}},
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
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./x"}
	if ValidateLockfile(Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "", Version: "1.0.0"}}}}}) == nil {
		t.Fatal("expected incomplete artifact rejection")
	}
	lock := Lockfile{Resolutions: []Resolution{
		{Source: source, ResolvedVersion: lockSnapshot("a"), Artifacts: []ArtifactResolution{{Name: "a", Version: "1.0.0"}}},
		{Source: source, ResolvedVersion: lockSnapshot("b"), Artifacts: []ArtifactResolution{{Name: "a", Version: "2.0.0"}}},
	}}
	if ValidateLockfile(lock) == nil {
		t.Fatal("expected multiply owned artifact rejection")
	}
}

func TestValidateLockfileRejectsCommitsOnFileSources(t *testing.T) {
	lock := Lockfile{Resolutions: []Resolution{{
		Source:          SourceIdentity{Type: SourceTypeFile, Locator: "./x"},
		ResolvedVersion: lockSnapshot("a"),
		Commit:          strings.Repeat("a", 40),
		Artifacts:       []ArtifactResolution{{Name: "a", Version: "1.0.0"}},
	}}}
	if ValidateLockfile(lock) == nil {
		t.Fatal("expected file source commit rejection")
	}
}
