package repositorystate

import "testing"

func TestManifestUpsertDeclarationInsertReplaceAndUnchanged(t *testing.T) {
	base := Manifest{}

	decl := Declaration{
		Source: SourceIdentity{Type: "file", Name: "local-example-source"},
		Target: DeclarationTarget{
			Scope:    DeclarationScopeArtifact,
			Artifact: "base-readme",
		},
		Input: &SourceInput{Locator: "/tmp/example", Version: "v1.2.3"},
	}

	inserted, change := base.UpsertDeclaration(decl)
	if change != ChangeKindInserted {
		t.Fatalf("change = %q, want %q", change, ChangeKindInserted)
	}

	replaced, change := inserted.UpsertDeclaration(Declaration{
		Source: decl.Source,
		Target: decl.Target,
		Input:  &SourceInput{Locator: "/tmp/other"},
	})
	if change != ChangeKindReplaced {
		t.Fatalf("change = %q, want %q", change, ChangeKindReplaced)
	}

	unchanged, change := replaced.UpsertDeclaration(Declaration{
		Source: decl.Source,
		Target: decl.Target,
		Input:  &SourceInput{Locator: "/tmp/other"},
	})
	if change != ChangeKindUnchanged {
		t.Fatalf("change = %q, want %q", change, ChangeKindUnchanged)
	}
	if len(unchanged.Declarations) != 1 {
		t.Fatalf("len(Declarations) = %d, want 1", len(unchanged.Declarations))
	}
}

func TestValidateManifestRejectsInvalidTargetsAndDuplicates(t *testing.T) {
	err := ValidateManifest(Manifest{
		Declarations: []Declaration{
			{
				Source: SourceIdentity{Type: "file", Name: "local-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeArtifact},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateManifest() error = nil, want missing artifact name error")
	}

	err = ValidateManifest(Manifest{
		Declarations: []Declaration{
			{
				Source: SourceIdentity{Name: "local-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "base-readme"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateManifest() error = nil, want missing source type error")
	}

	err = ValidateManifest(Manifest{
		Declarations: []Declaration{
			{
				Source: SourceIdentity{Type: "http", Name: "remote-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "base-readme"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateManifest() error = nil, want unsupported source type error")
	}

	err = ValidateManifest(Manifest{
		Declarations: []Declaration{
			{
				Source: SourceIdentity{Type: "file", Name: "local-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeSource, Artifact: "base-readme"},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateManifest() error = nil, want source-scoped artifact error")
	}

	err = ValidateManifest(Manifest{
		Declarations: []Declaration{
			{
				Source: SourceIdentity{Type: "file", Name: "local-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeSource},
			},
			{
				Source: SourceIdentity{Type: "file", Name: "local-example-source"},
				Target: DeclarationTarget{Scope: DeclarationScopeSource},
			},
		},
	})
	if err == nil {
		t.Fatal("ValidateManifest() error = nil, want duplicate source-scope declaration error")
	}
}
