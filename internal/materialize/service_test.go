package materialize

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestWriteRootedStaysWithOpenedOperationRoot(t *testing.T) {
	root := t.TempDir()
	observed, err := Observe(root, "target")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Skipf("rename opened root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(moved) })
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeRooted(opened, observed, []byte("content")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(moved, "target")); err != nil || string(got) != "content" {
		t.Fatalf("opened-root target = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "target")); !os.IsNotExist(err) {
		t.Fatalf("replacement-root target error = %v, want not exist", err)
	}
}

func TestWriteRejectsReplacedOperationRoot(t *testing.T) {
	root := t.TempDir()
	observed, err := Observe(root, "target")
	if err != nil {
		t.Fatal(err)
	}
	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Skipf("rename operation root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(moved) })
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	err = Write(observed, []byte("content"))
	var changed ChangedSincePreflightError
	if !errors.As(err, &changed) {
		t.Fatalf("Write() error = %T %v, want ChangedSincePreflightError", err, err)
	}
	if _, err := os.Stat(filepath.Join(root, "target")); !os.IsNotExist(err) {
		t.Fatalf("replacement-root target error = %v, want not exist", err)
	}
}

func TestObserveRejectsTargetsOutsideRootAndInvalidRoot(t *testing.T) {
	root := t.TempDir()
	for _, target := range []string{".", "..", "../outside", filepath.Join(root, "absolute")} {
		if _, err := Observe(root, target); err == nil {
			t.Fatalf("Observe(%q) error = nil", target)
		}
	}
	if _, err := Observe(filepath.Join(root, "missing"), "file"); err == nil {
		t.Fatal("expected invalid root error")
	}
}

func TestObserveRejectsRegularFileAsParent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "parent"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Observe(root, "parent/file"); err == nil || err.Error() != "target parent must be a real directory" {
		t.Fatalf("Observe() error = %v", err)
	}
}

func TestPathKeyDigestAndErrorText(t *testing.T) {
	if got := PathKey("a/../b"); got != "b" {
		t.Fatalf("PathKey() = %q", got)
	}
	if got := Digest([]byte("content")); len(got) != 64 || strings.Trim(got, "0123456789abcdef") != "" {
		t.Fatalf("Digest() = %q", got)
	}
	if got := (ChangedSincePreflightError{Path: "a"}).Error(); got != `target "a" changed after preflight` {
		t.Fatalf("Error() = %q", got)
	}
}

func TestRevalidateAcceptsSameObservationAndRejectsChanges(t *testing.T) {
	root := t.TempDir()
	observed, err := Observe(root, "file")
	if err != nil {
		t.Fatal(err)
	}
	if err := Revalidate(observed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	var changed ChangedSincePreflightError
	if err := Revalidate(observed); !errors.As(err, &changed) {
		t.Fatalf("Revalidate() error = %T %v", err, err)
	}
}

func TestSameObservationComparesAllStableFields(t *testing.T) {
	base := Observation{Root: "root", Path: "path", AbsolutePath: "absolute", Kind: EntryRegular, Mode: 0o644, Digest: "digest"}
	if !SameObservation(base, base) {
		t.Fatal("identical observations differ")
	}
	changed := base
	changed.Digest = "other"
	if SameObservation(base, changed) {
		t.Fatal("different observations match")
	}
}
