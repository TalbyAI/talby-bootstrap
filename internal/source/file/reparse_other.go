//go:build !windows

package file

import "os"

func isWindowsReparsePoint(os.FileInfo) bool { return false }
