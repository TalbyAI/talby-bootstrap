package repositorystate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateManifestRejectsDuplicateAndMixedScopes(t *testing.T) {
	root := t.TempDir()
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./sources/tools"}
	err := ValidateManifest(root, Manifest{Declarations: []Declaration{{Source: source, Target: DeclarationTarget{Scope: DeclarationScopeSource}}, {Source: source, Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "lint"}}}})
	if err == nil || !strings.Contains(err.Error(), "mixes source and artifact scopes") {
		t.Fatalf("ValidateManifest() error = %v, want mixed-scope error", err)
	}
}

func TestNormalizeSourceIdentityStoresRootRelativeAndExternalAbsoluteLocators(t *testing.T) {
	root := t.TempDir()
	in, err := NormalizeSourceIdentity(root, SourceIdentity{Type: SourceTypeFile, Locator: "x"})
	if err != nil || in.Locator != "./x" {
		t.Fatalf("in root = %#v, %v", in, err)
	}
	out, err := NormalizeSourceIdentity(root, SourceIdentity{Type: SourceTypeFile, Locator: "/tmp/outside"})
	if err != nil || out.Locator != "/tmp/outside" {
		t.Fatalf("external = %#v, %v", out, err)
	}
}

func TestNormalizeSourceIdentityCanonicalizesSymlinkContainment(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, "linked")); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	got, err := NormalizeSourceIdentity(root, SourceIdentity{Type: SourceTypeFile, Locator: "linked"})
	if err != nil || got.Locator != filepath.ToSlash(external) {
		t.Fatalf("NormalizeSourceIdentity() = %#v, %v, want external canonical locator", got, err)
	}
}

func TestValidateManifestRejectsInvalidSourceVersion(t *testing.T) {
	root := t.TempDir()
	declaration := Declaration{Source: SourceIdentity{Type: SourceTypeFile, Locator: "./source"}, Target: DeclarationTarget{Scope: DeclarationScopeSource}, SourceVersion: "1.0.0"}
	if err := ValidateManifest(root, Manifest{Declarations: []Declaration{declaration}}); err == nil {
		t.Fatal("expected file source version rejection")
	}
	declaration.Source = SourceIdentity{Type: SourceTypeGit, Locator: "https://example.com/source.git"}
	declaration.SourceVersion = "v1"
	if err := ValidateManifest(root, Manifest{Declarations: []Declaration{declaration}}); err == nil {
		t.Fatal("expected non-canonical Git source version rejection")
	}
	declaration.SourceVersion = "1.0.0"
	if err := ValidateManifest(root, Manifest{Declarations: []Declaration{declaration}}); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeSourceIdentityRejectsIncompleteAndAcceptsGitSources(t *testing.T) {
	root := t.TempDir()
	if _, err := NormalizeSourceIdentity(root, SourceIdentity{}); err == nil {
		t.Fatal("expected incomplete source rejection")
	}
	if _, err := NormalizeSourceIdentity(root, SourceIdentity{Type: SourceTypeGit, Locator: "https://example.com/source.git"}); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeSourceIdentity(filepath.Join(root, "missing"), SourceIdentity{Type: SourceTypeFile, Locator: "x"}); err == nil {
		t.Fatal("expected missing root rejection")
	}
}

func TestAcquisitionLocatorRequiresNormalizedSource(t *testing.T) {
	root := t.TempDir()
	if _, err := AcquisitionLocator(root, SourceIdentity{Type: SourceTypeFile, Locator: "source"}); err == nil {
		t.Fatal("expected unnormalized locator rejection")
	}
	got, err := AcquisitionLocator(root, SourceIdentity{Type: SourceTypeFile, Locator: "./source"})
	if err != nil || got != filepath.Join(root, "source") {
		t.Fatalf("AcquisitionLocator() = %q, %v", got, err)
	}
	got, err = AcquisitionLocator(root, SourceIdentity{Type: SourceTypeGit, Locator: "https://example.com/source.git"})
	if err != nil || got != "https://example.com/source.git" {
		t.Fatalf("Git AcquisitionLocator() = %q, %v", got, err)
	}
}

func TestManifestAddDeclarationKindsAndConflicts(t *testing.T) {
	root := t.TempDir()
	declaration := Declaration{Source: SourceIdentity{Type: SourceTypeFile, Locator: "source"}, Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "a"}}
	manifest, kind, err := (Manifest{}).AddDeclaration(root, declaration)
	if err != nil || kind != ChangeKindInserted || len(manifest.Declarations) != 1 {
		t.Fatalf("insert = %#v, %q, %v", manifest, kind, err)
	}
	_, kind, err = manifest.AddDeclaration(root, declaration)
	if err != nil || kind != ChangeKindUnchanged {
		t.Fatalf("same kind = %q, error = %v", kind, err)
	}
	if _, _, err := manifest.AddDeclaration(root, Declaration{Source: SourceIdentity{Type: SourceTypeFile, Locator: "./source"}, Target: DeclarationTarget{Scope: DeclarationScopeSource}}); err == nil {
		t.Fatal("expected conflicting scope declaration")
	}
	if _, _, err := manifest.AddDeclaration(root, Declaration{Source: SourceIdentity{Type: SourceTypeFile, Locator: "./source"}}); err == nil {
		t.Fatal("expected invalid target")
	}
}

func TestValidateManifestTargetsTrustAndDuplicates(t *testing.T) {
	root := t.TempDir()
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./source"}
	if err := ValidateManifest(root, Manifest{TrustPolicy: TrustPolicy{ApprovedSources: []SourceIdentity{{Type: SourceTypeFile, Locator: "source"}}}}); err == nil {
		t.Fatal("expected unnormalized approved source")
	}
	if err := ValidateManifest(root, Manifest{Declarations: []Declaration{{Source: source}}}); err == nil {
		t.Fatal("expected missing scope")
	}
	if err := ValidateManifest(root, Manifest{Declarations: []Declaration{{Source: source, Target: DeclarationTarget{Scope: DeclarationScopeArtifact}}}}); err == nil {
		t.Fatal("expected missing artifact")
	}
	if err := ValidateManifest(root, Manifest{Declarations: []Declaration{{Source: source, Target: DeclarationTarget{Scope: DeclarationScopeSource, Artifact: "a"}}}}); err == nil {
		t.Fatal("expected source artifact rejection")
	}
	declaration := Declaration{Source: source, Target: DeclarationTarget{Scope: DeclarationScopeSource}}
	if err := ValidateManifest(root, Manifest{Declarations: []Declaration{declaration, declaration}}); err == nil {
		t.Fatal("expected duplicate declaration")
	}
}
