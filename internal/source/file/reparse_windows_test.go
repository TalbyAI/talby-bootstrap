//go:build windows

package file

import (
	"os"
	"syscall"
	"testing"
)

type reparsePointInfo struct{ os.FileInfo }

func (reparsePointInfo) Sys() any {
	return &syscall.Win32FileAttributeData{FileAttributes: syscall.FILE_ATTRIBUTE_REPARSE_POINT}
}

func TestIsWindowsReparsePoint(t *testing.T) {
	if !isWindowsReparsePoint(reparsePointInfo{}) {
		t.Fatal("isWindowsReparsePoint() = false, want true")
	}
}
