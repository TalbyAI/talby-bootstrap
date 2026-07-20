//go:build windows

package materialize

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestObserveRejectsReparsePointTargetParent(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	parent := filepath.Join(root, "parent")
	if err := os.Symlink(outside, parent); err != nil {
		t.Skipf("symlink: %v", err)
	}
	info, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		t.Fatal("parent lacks FILE_ATTRIBUTE_REPARSE_POINT")
	}
	if _, err := Observe(root, "parent/file"); err == nil || err.Error() != "target parent must be a real directory" {
		t.Fatalf("Observe() error = %v, want real-directory rejection", err)
	}
}
