package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/source"
)

func TestResolveLoadsSourceIdentityAndArtifacts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\nsteps:\n  - type: file\n    path: README.md\n    source: README.md\n")

	resolved, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Identity.Type != "file" {
		t.Fatalf("Identity.Type = %q, want file", resolved.Identity.Type)
	}
	if resolved.Identity.Name != "local-example-source" {
		t.Fatalf("Identity.Name = %q, want local-example-source", resolved.Identity.Name)
	}
	if resolved.Identity.Version != "local-snapshot-001" {
		t.Fatalf("Identity.Version = %q, want local-snapshot-001", resolved.Identity.Version)
	}
	if resolved.SourcePath != root {
		t.Fatalf("SourcePath = %q, want %q", resolved.SourcePath, root)
	}
	if len(resolved.Artifacts) != 1 || resolved.Artifacts[0].Version != "1.0.0" {
		t.Fatalf("Artifacts = %#v, want one versioned artifact", resolved.Artifacts)
	}
	if resolved.Artifacts[0].Name != "base-readme" {
		t.Fatalf("Artifacts[0].Name = %q, want base-readme", resolved.Artifacts[0].Name)
	}
	if resolved.Artifacts[0].Path != "artifacts/base-readme" {
		t.Fatalf("Artifacts[0].Path = %q, want artifacts/base-readme", resolved.Artifacts[0].Path)
	}
}

func TestResolveRejectsMissingArtifactDescriptor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want missing talby-artifact.yaml error")
	}
	if !strings.Contains(err.Error(), "read ") || !strings.Contains(err.Error(), "talby-artifact.yaml") {
		t.Fatalf("Resolve() error = %q, want wrapped read talby-artifact.yaml error", err)
	}
}

func TestResolveRejectsInvalidSourceDescriptor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource: [\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want parse talby-source.yaml error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "talby-source.yaml") {
		t.Fatalf("Resolve() error = %q, want wrapped parse talby-source.yaml error", err)
	}
}

func TestResolveRejectsArtifactPathOutsideSourceRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "outside-artifact")
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: ../outside-artifact\n")
	writeFile(t, filepath.Join(outside, "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want artifact path escape error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "artifact path") {
		t.Fatalf("Resolve() error = %q, want parse artifact path error", err)
	}
}

func TestResolveRejectsMismatchedArtifactNames(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: different-name\n  version: 1.0.0\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want artifact name mismatch error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "artifact name") {
		t.Fatalf("Resolve() error = %q, want parse artifact name mismatch error", err)
	}
}

func TestResolveRejectsUnsupportedSchemaVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 2\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want unsupported schema version error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("Resolve() error = %q, want parse schema_version error", err)
	}
}

func TestResolveRejectsMissingRequiredFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: \nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want missing required field error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "source name") {
		t.Fatalf("Resolve() error = %q, want parse source name error", err)
	}
}

func TestResolveRejectsMissingArtifactPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: \"\"\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want missing artifact path error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "artifact path") {
		t.Fatalf("Resolve() error = %q, want parse artifact path error", err)
	}
}

func TestResolveRejectsMissingArtifactName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: \"\"\n  version: 1.0.0\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want missing artifact name error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "artifact name") {
		t.Fatalf("Resolve() error = %q, want parse artifact name error", err)
	}
}

func TestResolveRejectsMissingArtifactVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: \"\"\n")

	_, err := New().Resolve(context.Background(), source.ResolveRequest{
		Ref: source.Ref{Type: "file", Locator: root},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want missing artifact version error")
	}
	if !strings.Contains(err.Error(), "parse ") || !strings.Contains(err.Error(), "artifact version") {
		t.Fatalf("Resolve() error = %q, want parse artifact version error", err)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
