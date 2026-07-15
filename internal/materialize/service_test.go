package materialize

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestObserveCanonicalizesAbsentAndRegularTargets(t *testing.T) {
	root := t.TempDir()
	got, err := Observe(root, "a/b")
	if err != nil || got.Kind != EntryAbsent {
		t.Fatalf("%#v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err = Observe(root, "a")
	if err != nil || got.Kind != EntryRegular {
		t.Fatalf("%#v %v", got, err)
	}
}
func TestWriteRejectsChangedTargetSinceObservation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(root, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	err = Write(observed, []byte("desired"))
	var changed ChangedSincePreflightError
	if !errors.As(err, &changed) {
		t.Fatalf("Write() error = %T %v, want ChangedSincePreflightError", err, err)
	}
}

func TestWriteClassifiesParentTopologyRaceAsChanged(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	observed, err := Observe(root, "parent/target")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "parent")); err != nil {
		t.Skipf("symlink: %v", err)
	}

	err = Write(observed, []byte("desired"))
	var changed ChangedSincePreflightError
	if !errors.As(err, &changed) {
		t.Fatalf("Write() error = %T %v, want ChangedSincePreflightError", err, err)
	}
}

func TestObserveRejectsExistingSymlinkPathComponent(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if _, err := Observe(root, "link/file"); err == nil {
		t.Fatal("Observe() error = nil, want symlink parent rejection")
	}
}

func TestObserveReportsFinalSymlinkAndNonRegularTargets(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("target", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if got, err := Observe(root, "link"); err != nil || got.Kind != EntrySymlink {
		t.Fatalf("Observe(link) = %#v, %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if got, err := Observe(root, "dir"); err != nil || got.Kind != EntryOther {
		t.Fatalf("Observe(dir) = %#v, %v", got, err)
	}
}

func TestWriteCreatesWithMode0644(t *testing.T) {
	root := t.TempDir()
	observed, err := Observe(root, "new")
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(observed, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "new"))
	if err != nil || info.Mode().Perm() != 0644 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
}

func TestWritePreservesExistingMode(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "old")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(root, "old")
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(observed, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
}

func TestWriteReplacesThroughTemporaryFileInTargetDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(root, "nested/file")
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(observed, []byte("content")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "nested", "file")); err != nil || string(got) != "content" {
		t.Fatalf("file = %q, %v", got, err)
	}
	matches, err := filepath.Glob(filepath.Join(root, "nested", ".file.*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, %v", matches, err)
	}
}
