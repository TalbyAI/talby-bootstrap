package install

import (
	"os"
	"path/filepath"
	"testing"
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
