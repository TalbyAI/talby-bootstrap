package repositorystate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateManifestRejectsDuplicateAndMixedScopes(t *testing.T) {
	root := t.TempDir()
	source := SourceIdentity{Type: "file", Locator: "sources/tools"}
	err := ValidateManifest(root, Manifest{Declarations: []Declaration{{Source: source, Target: DeclarationTarget{Scope: DeclarationScopeSource}}, {Source: source, Target: DeclarationTarget{Scope: DeclarationScopeArtifact, Artifact: "lint"}}}})
	if err == nil || !strings.Contains(err.Error(), "mixes source and artifact scopes") {
		t.Fatalf("ValidateManifest() error = %v, want mixed-scope error", err)
	}
}
func TestNormalizeSourceIdentityStoresRootRelativeAndExternalAbsoluteLocators(t *testing.T) {
	root := t.TempDir()
	in, err := NormalizeSourceIdentity(root, SourceIdentity{Type: "file", Locator: "x"})
	if err != nil || in.Locator != "x" {
		t.Fatalf("in root = %#v, %v", in, err)
	}
	out, err := NormalizeSourceIdentity(root, SourceIdentity{Type: "file", Locator: "/tmp/outside"})
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
	got, err := NormalizeSourceIdentity(root, SourceIdentity{Type: "file", Locator: "linked"})
	if err != nil || got.Locator != filepath.ToSlash(external) {
		t.Fatalf("NormalizeSourceIdentity() = %#v, %v, want external canonical locator", got, err)
	}
}
func TestValidateManifestRejectsMismatchedPreservedLocator(t *testing.T) {
	root := t.TempDir()
	err := ValidateManifest(root, Manifest{Declarations: []Declaration{{Source: SourceIdentity{Type: "file", Locator: "x"}, Target: DeclarationTarget{Scope: DeclarationScopeSource}, Input: &SourceInput{Locator: "y"}}}})
	if err == nil {
		t.Fatal("expected mismatch")
	}
}
