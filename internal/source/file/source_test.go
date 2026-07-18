package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/talby/talby-bootstrap/internal/source"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T) string {
	root := t.TempDir()
	write(t, filepath.Join(root, "tbboot-source.yaml"), "schema_version: 1\nartifacts:\n  - name: a\n    path: a\n")
	write(t, filepath.Join(root, "a", "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps:\n  - type: file\n    path: out\n    source: in\n")
	write(t, filepath.Join(root, "a", "in"), "captured\n")
	return root
}
func TestResolveRejectsRequestedVersion(t *testing.T) {
	_, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: fixture(t), Version: "v"}})
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestResolveCapturesFileBytesAndEveryInputPath(t *testing.T) {
	root := fixture(t)
	got, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Type: "file", Locator: root}})
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Artifacts[0].Steps[0].SourceBytes) != "captured\n" {
		t.Fatalf("SourceBytes = %q, want captured bytes", got.Artifacts[0].Steps[0].SourceBytes)
	}
	if len(got.InputPaths) != 3 {
		t.Fatalf("InputPaths count = %d, want source descriptor, artifact descriptor, and step input", len(got.InputPaths))
	}
}
func TestResolveFramesSnapshotPayloads(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "a", "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps:\n  - type: file\n    path: a\n    source: one\n  - type: file\n    path: b\n    source: two\n")
	write(t, filepath.Join(root, "a", "one"), "x")
	write(t, filepath.Join(root, "a", "two"), "b\x00z")
	first, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}})
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(root, "a", "one"), "xb\x00")
	write(t, filepath.Join(root, "a", "two"), "z")
	second, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity.Version == second.Identity.Version {
		t.Fatalf("snapshot versions match for different payloads: %q", first.Identity.Version)
	}
}
func TestResolveRejectsEmptySourceArtifactList(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "tbboot-source.yaml"), "schema_version: 1\nartifacts: []\n")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRejectsArtifactWithoutSteps(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "a", "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps: []\n")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want empty-step rejection")
	}
}

func TestResolveRejectsUnsupportedStep(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "a", "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps:\n  - type: prompt\n    path: ignored\n")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("expected unsupported step rejection")
	}
}

func TestResolveRejectsArtifactSymlinkEscapingSourceRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write(t, filepath.Join(root, "tbboot-source.yaml"), "schema_version: 1\nartifacts:\n  - name: a\n    path: a\n")
	if err := os.Symlink(outside, filepath.Join(root, "a")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want containment rejection")
	}
}

func TestResolveRejectsStepInputSymlinkEscapingArtifactRoot(t *testing.T) {
	root, outside := fixture(t), t.TempDir()
	write(t, filepath.Join(outside, "input"), "outside")
	if err := os.Remove(filepath.Join(root, "a", "in")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "input"), filepath.Join(root, "a", "in")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want containment rejection")
	}
}

func TestResolveRejectsSourceDescriptorSymlinkEscapingSourceRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write(t, filepath.Join(outside, "source.yaml"), "schema_version: 1\nartifacts: []\n")
	if err := os.Symlink(filepath.Join(outside, "source.yaml"), filepath.Join(root, "tbboot-source.yaml")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want containment rejection")
	}
}

func TestResolveRejectsArtifactDescriptorSymlinkEscapingArtifactRoot(t *testing.T) {
	root, outside := fixture(t), t.TempDir()
	write(t, filepath.Join(outside, "artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps: []\n")
	if err := os.Remove(filepath.Join(root, "a", "tbboot-artifact.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "artifact.yaml"), filepath.Join(root, "a", "tbboot-artifact.yaml")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want containment rejection")
	}
}

func TestEncodeDescriptorsIsDeterministicAndCanonical(t *testing.T) {
	sourceBytes, err := EncodeSourceDescriptor(SourceDescriptor{Artifacts: []ArtifactReference{
		{Name: "z", Path: "z"},
		{Name: "a", Path: "a"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(sourceBytes), "schema_version: 1\nartifacts:\n  - name: a\n    path: a\n  - name: z\n    path: z\n"; got != want {
		t.Fatalf("source descriptor = %q, want %q", got, want)
	}
	artifactBytes, err := EncodeArtifactDescriptor(ArtifactDescriptor{
		Name: "a", Version: "1.0.0", Description: "example",
		Steps: []ArtifactStep{{Type: "file", Path: "out", Source: "in"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(artifactBytes), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\n  description: example\nsteps:\n  - type: file\n    path: out\n    source: in\n"; got != want {
		t.Fatalf("artifact descriptor = %q, want %q", got, want)
	}
}

func TestCapabilitiesProvideIdentity(t *testing.T) {
	if !New().Capabilities().ProvidesIdentity {
		t.Fatal("file source must provide identity")
	}
}

func TestValidateSourceDescriptorRejectsInvalidFieldsAndDuplicates(t *testing.T) {
	valid := sourceDescriptor{SchemaVersion: supportedSchemaVersion, Artifacts: []artifactRef{{Name: "a", Path: "a"}}}
	invalidSchema := valid
	invalidSchema.SchemaVersion = 2
	if validateSourceDescriptor(invalidSchema) == nil {
		t.Fatal("expected schema rejection")
	}
	invalidName := valid
	invalidName.Artifacts = append([]artifactRef(nil), valid.Artifacts...)
	invalidName.Artifacts[0].Name = "A"
	if validateSourceDescriptor(invalidName) == nil {
		t.Fatal("expected lowercase artifact name rejection")
	}
	incomplete := valid
	incomplete.Artifacts = append([]artifactRef(nil), valid.Artifacts...)
	incomplete.Artifacts[0].Path = ""
	if validateSourceDescriptor(incomplete) == nil {
		t.Fatal("expected incomplete artifact rejection")
	}
	duplicate := valid
	duplicate.Artifacts = append(duplicate.Artifacts, artifactRef{Name: "a", Path: "other"})
	if err := validateSourceDescriptor(duplicate); err == nil || err.Error() != `duplicate artifact name "a"` {
		t.Fatalf("validateSourceDescriptor() error = %v, want duplicate artifact rejection", err)
	}
}

func TestValidateArtifactDescriptorRejectsSchemaNameAndVersion(t *testing.T) {
	ref := artifactRef{Name: "a", Path: "a"}
	valid := artifactDescriptor{SchemaVersion: supportedSchemaVersion, Steps: []artifactStep{{Type: "file", Path: "out", Source: "in"}}}
	valid.Artifact.Name, valid.Artifact.Version = "a", "1.0.0"
	invalidSchema := valid
	invalidSchema.SchemaVersion = 2
	if validateArtifactDescriptor(ref, invalidSchema) == nil {
		t.Fatal("expected schema rejection")
	}
	wrongName := valid
	wrongName.Artifact.Name = "b"
	if validateArtifactDescriptor(ref, wrongName) == nil {
		t.Fatal("expected name rejection")
	}
	invalidVersion := valid
	invalidVersion.Artifact.Version = "1"
	if validateArtifactDescriptor(ref, invalidVersion) == nil {
		t.Fatal("expected canonical version rejection")
	}
}

func TestResolveRejectsMalformedDescriptorsAndFileSteps(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "tbboot-source.yaml"), "schema_version: [")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("expected malformed source descriptor")
	}

	root = fixture(t)
	write(t, filepath.Join(root, "a", "tbboot-artifact.yaml"), "schema_version: [")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("expected malformed artifact descriptor")
	}

	root = fixture(t)
	write(t, filepath.Join(root, "a", "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps:\n  - type: file\n    source: in\n")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("expected missing target path rejection")
	}

	root = fixture(t)
	write(t, filepath.Join(root, "a", "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1.0.0\nsteps:\n  - type: file\n    path: out\n")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("expected missing source rejection")
	}
}
