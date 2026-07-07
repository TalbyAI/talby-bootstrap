package repositorystate

import (
	"strings"
	"testing"
)

func TestLockfileUpsertResolutionInsertReplaceAndUnchanged(t *testing.T) {
	base := Lockfile{}

	res := Resolution{
		Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
		ResolvedVersion: "local-snapshot-001",
		Artifact: ArtifactResolution{
			Name:    "base-readme",
			Version: "1.0.0",
		},
	}

	inserted, change := base.UpsertResolution(res)
	if change != ChangeKindInserted {
		t.Fatalf("change = %q, want %q", change, ChangeKindInserted)
	}

	replaced, change := inserted.UpsertResolution(Resolution{
		Source:          res.Source,
		ResolvedVersion: "local-snapshot-002",
		Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.1.0"},
	})
	if change != ChangeKindReplaced {
		t.Fatalf("change = %q, want %q", change, ChangeKindReplaced)
	}

	unchanged, change := replaced.UpsertResolution(replaced.Resolutions[0])
	if change != ChangeKindUnchanged {
		t.Fatalf("change = %q, want %q", change, ChangeKindUnchanged)
	}
	if len(unchanged.Resolutions) != 1 {
		t.Fatalf("len(Resolutions) = %d, want 1", len(unchanged.Resolutions))
	}
}

func TestValidateLockfileRejectsMissingFieldsAndDuplicates(t *testing.T) {
	err := ValidateLockfile(Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "",
				Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateLockfile() error = nil, want missing resolved version error")
	}

	err = ValidateLockfile(Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "file"},
				ResolvedVersion: "local-snapshot-001",
				Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateLockfile() error = nil, want missing source name error")
	}

	err = ValidateLockfile(Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "http", Name: "remote-example-source"},
				ResolvedVersion: "remote-snapshot-001",
				Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateLockfile() error = nil, want unsupported source type error")
	}

	err = ValidateLockfile(Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "local-snapshot-001",
				Artifact:        ArtifactResolution{Version: "1.0.0"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateLockfile() error = nil, want missing artifact name error")
	}

	err = ValidateLockfile(Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "local-snapshot-001",
				Artifact:        ArtifactResolution{Name: "base-readme"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateLockfile() error = nil, want missing artifact version error")
	}

	err = ValidateLockfile(Lockfile{
		Resolutions: []Resolution{
			{
				Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "local-snapshot-001",
				Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.0.0"},
			},
			{
				Source:          SourceIdentity{Type: "file", Name: "local-example-source"},
				ResolvedVersion: "local-snapshot-002",
				Artifact:        ArtifactResolution{Name: "base-readme", Version: "1.1.0"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateLockfile() error = nil, want duplicate resolution error")
	}
	if strings.Contains(err.Error(), "\x00") {
		t.Fatalf("ValidateLockfile() error = %q, want readable duplicate error", err)
	}
	if !strings.Contains(err.Error(), "source file/local-example-source artifact base-readme") {
		t.Fatalf("ValidateLockfile() error = %q, want source and artifact identity", err)
	}
}
