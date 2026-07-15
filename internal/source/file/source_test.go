package file

import (
	"context"
	"github.com/talby/talby-bootstrap/internal/source"
	"os"
	"path/filepath"
	"testing"
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
	write(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: test\nartifacts:\n  - name: a\n    path: a\n")
	write(t, filepath.Join(root, "a", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1\nsteps:\n  - type: file\n    path: out\n    source: in\n")
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
	write(t, filepath.Join(root, "a", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1\nsteps:\n  - type: file\n    path: a\n    source: one\n  - type: file\n    path: b\n    source: two\n")
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
	write(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: test\nartifacts: []\n")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRejectsArtifactWithoutSteps(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "a", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1\nsteps: []\n")
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want empty-step rejection")
	}
}

func TestResolveParsesUnsupportedStepWithoutRejectingSource(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "a", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1\nsteps:\n  - type: prompt\n    path: ignored\n")
	got, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}})
	if err != nil || got.Artifacts[0].Steps[0].Type != "prompt" {
		t.Fatalf("Resolve() = %#v, %v", got, err)
	}
}

func TestResolveRejectsArtifactSymlinkEscapingSourceRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: test\nartifacts:\n  - name: a\n    path: a\n")
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
	write(t, filepath.Join(outside, "source.yaml"), "schema_version: 1\nsource:\n  name: test\nartifacts: []\n")
	if err := os.Symlink(filepath.Join(outside, "source.yaml"), filepath.Join(root, "talby-source.yaml")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want containment rejection")
	}
}

func TestResolveRejectsArtifactDescriptorSymlinkEscapingArtifactRoot(t *testing.T) {
	root, outside := fixture(t), t.TempDir()
	write(t, filepath.Join(outside, "artifact.yaml"), "schema_version: 1\nartifact:\n  name: a\n  version: 1\nsteps: []\n")
	if err := os.Remove(filepath.Join(root, "a", "talby-artifact.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "artifact.yaml"), filepath.Join(root, "a", "talby-artifact.yaml")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Locator: root}}); err == nil {
		t.Fatal("Resolve() error = nil, want containment rejection")
	}
}
