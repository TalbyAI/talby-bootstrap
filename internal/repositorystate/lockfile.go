package repositorystate

import "fmt"

func ValidateLockfile(l Lockfile) error {
	seen := map[string]struct{}{}

	for _, res := range l.Resolutions {
		if err := validateSourceIdentity(res.Source); err != nil {
			return fmt.Errorf("resolution source: %w", err)
		}
		if res.ResolvedVersion == "" {
			return fmt.Errorf("resolved source version is required")
		}
		if res.Artifact.Name == "" {
			return fmt.Errorf("artifact name is required")
		}
		if res.Artifact.Version == "" {
			return fmt.Errorf("artifact version is required")
		}

		key := resolutionKey(res)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate resolution for %s", key)
		}
		seen[key] = struct{}{}
	}

	return nil
}

func (l Lockfile) UpsertResolution(res Resolution) (Lockfile, ChangeKind) {
	next := Lockfile{
		Resolutions: append([]Resolution(nil), l.Resolutions...),
	}

	key := resolutionKey(res)
	for i, existing := range next.Resolutions {
		if resolutionKey(existing) != key {
			continue
		}
		if existing == res {
			return next, ChangeKindUnchanged
		}
		next.Resolutions[i] = res
		return next, ChangeKindReplaced
	}

	next.Resolutions = append(next.Resolutions, res)
	return next, ChangeKindInserted
}

func resolutionKey(res Resolution) string {
	return res.Source.Type + "\x00" + res.Source.Name + "\x00" + res.Artifact.Name
}
