package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/talby/talby-bootstrap/internal/materialize"
)

func TestAcquireOperationLockRejectsExistingPathAndReleases(t *testing.T) {
	root := t.TempDir()
	release, err := acquireOperationLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, operationLockName)); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireOperationLock(root); err == nil {
		t.Fatal("second acquire succeeded")
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, operationLockName)); !os.IsNotExist(err) {
		t.Fatalf("lock after release = %v, want not exist", err)
	}
}

func TestAcquireOperationLockDoesNotTakeOverFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, operationLockName)
	if err := os.WriteFile(path, []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireOperationLock(root); err == nil {
		t.Fatal("acquire succeeded over existing file")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "owned" {
		t.Fatalf("existing lock content = %q, %v", data, err)
	}
}

func TestAcquireOperationLockReleaseReturnsRemovalError(t *testing.T) {
	root := t.TempDir()
	release, err := acquireOperationLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, operationLockName, "held"), []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := release(); err == nil {
		t.Fatal("release succeeded for non-empty lock")
	}
}

func TestOpenOperationRootCanonicalizesAndDetectsReplacement(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlink: %v", err)
	}
	operation, release, err := openOperationRoot(alias, false)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if operation.path != want {
		t.Fatalf("operation root = %q, want %q", operation.path, want)
	}
	if err := release(); err != nil {
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
	var changed materialize.ChangedSincePreflightError
	if err := operation.validate(); !errors.As(err, &changed) || changed.Path != "." {
		t.Fatalf("validate() error = %T %v, want root drift", err, err)
	}
}
