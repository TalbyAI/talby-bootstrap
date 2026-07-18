package repositorystate

import (
	"fmt"
	"slices"
	"strings"
)

func ValidateLockfile(lockfile Lockfile) error {
	snapshots, artifacts := map[SnapshotKey]struct{}{}, map[ArtifactKey]struct{}{}
	for _, resolution := range lockfile.Resolutions {
		if resolution.Source.Type != SourceTypeFile && resolution.Source.Type != SourceTypeGit {
			return fmt.Errorf("unsupported source type %q", resolution.Source.Type)
		}
		if resolution.Source.Locator == "" || resolution.ResolvedVersion == "" {
			return fmt.Errorf("complete snapshot fields are required")
		}
		if resolution.Source.Type == SourceTypeFile && !isSHA256Digest(resolution.ResolvedVersion) {
			return fmt.Errorf("file source version must be a sha256 digest")
		}
		if resolution.Source.Type == SourceTypeGit && (!isCanonicalSemVer(resolution.ResolvedVersion) || !isGitCommit(resolution.Commit)) {
			return fmt.Errorf("Git resolution requires canonical SemVer and full commit")
		}
		sk := SnapshotKey{resolution.Source, resolution.ResolvedVersion}
		if _, ok := snapshots[sk]; ok {
			return fmt.Errorf("duplicate snapshot")
		}
		snapshots[sk] = struct{}{}
		if len(resolution.Artifacts) == 0 {
			return fmt.Errorf("snapshot requires artifacts")
		}
		for _, artifact := range resolution.Artifacts {
			if artifact.Name == "" || !isCanonicalSemVer(artifact.Version) {
				return fmt.Errorf("complete artifact fields are required")
			}
			key := ArtifactKey{resolution.Source, artifact.Name}
			if _, ok := artifacts[key]; ok {
				return fmt.Errorf("artifact belongs to multiple snapshots")
			}
			artifacts[key] = struct{}{}
		}
	}
	return nil
}
func (lockfile Lockfile) Snapshot(key SnapshotKey) (Resolution, bool) {
	for _, r := range lockfile.Resolutions {
		if r.Source == key.Source && r.ResolvedVersion == key.ResolvedVersion {
			return r, true
		}
	}
	return Resolution{}, false
}
func (lockfile Lockfile) Artifact(key ArtifactKey) (Resolution, ArtifactResolution, bool) {
	for _, r := range lockfile.Resolutions {
		if r.Source != key.Source {
			continue
		}
		for _, a := range r.Artifacts {
			if a.Name == key.Name {
				return r, a, true
			}
		}
	}
	return Resolution{}, ArtifactResolution{}, false
}
func (lockfile Lockfile) UpsertResolution(resolution Resolution) (Lockfile, ChangeKind, error) {
	if err := ValidateLockfile(Lockfile{Resolutions: []Resolution{resolution}}); err != nil {
		return Lockfile{}, "", err
	}
	next := Lockfile{Resolutions: make([]Resolution, len(lockfile.Resolutions))}
	copy(next.Resolutions, lockfile.Resolutions)
	for i := range next.Resolutions {
		next.Resolutions[i].Artifacts = append([]ArtifactResolution(nil), next.Resolutions[i].Artifacts...)
	}
	for _, a := range resolution.Artifacts {
		if r, _, ok := lockfile.Artifact(ArtifactKey{resolution.Source, a.Name}); ok && !(r.Source == resolution.Source && r.ResolvedVersion == resolution.ResolvedVersion) {
			return Lockfile{}, "", fmt.Errorf("artifact already belongs to another snapshot")
		}
	}
	for i, r := range next.Resolutions {
		if r.Source == resolution.Source && r.ResolvedVersion == resolution.ResolvedVersion {
			existing := map[string]ArtifactResolution{}
			for _, a := range r.Artifacts {
				existing[a.Name] = a
			}
			changed := false
			for _, a := range resolution.Artifacts {
				if _, ok := existing[a.Name]; !ok {
					r.Artifacts = append(r.Artifacts, a)
					changed = true
				}
			}
			slices.SortFunc(r.Artifacts, func(a, b ArtifactResolution) int { return strings.Compare(a.Name, b.Name) })
			next.Resolutions[i] = r
			if !changed {
				return next, ChangeKindUnchanged, nil
			}
			return next, ChangeKindReplaced, nil
		}
	}
	resolution.Artifacts = append([]ArtifactResolution(nil), resolution.Artifacts...)
	slices.SortFunc(resolution.Artifacts, func(a, b ArtifactResolution) int { return strings.Compare(a.Name, b.Name) })
	next.Resolutions = append(next.Resolutions, resolution)
	sortResolutions(next.Resolutions)
	return next, ChangeKindInserted, nil
}
func (lockfile Lockfile) KeepArtifacts(keys map[ArtifactKey]struct{}) (Lockfile, []ArtifactKey) {
	var next Lockfile
	var removed []ArtifactKey
	for _, r := range lockfile.Resolutions {
		nr := Resolution{Source: r.Source, ResolvedVersion: r.ResolvedVersion, Commit: r.Commit}
		for _, a := range r.Artifacts {
			k := ArtifactKey{r.Source, a.Name}
			if _, ok := keys[k]; ok {
				nr.Artifacts = append(nr.Artifacts, a)
			} else {
				removed = append(removed, k)
			}
		}
		if len(nr.Artifacts) > 0 {
			next.Resolutions = append(next.Resolutions, nr)
		}
	}
	sortResolutions(next.Resolutions)
	slices.SortFunc(removed, func(a, b ArtifactKey) int {
		return strings.Compare(SourceIdentityKey(a.Source)+"\x00"+a.Name, SourceIdentityKey(b.Source)+"\x00"+b.Name)
	})
	return next, removed
}
func sortResolutions(values []Resolution) {
	slices.SortFunc(values, func(a, b Resolution) int {
		return strings.Compare(SourceIdentityKey(a.Source)+"\x00"+a.ResolvedVersion, SourceIdentityKey(b.Source)+"\x00"+b.ResolvedVersion)
	})
}
