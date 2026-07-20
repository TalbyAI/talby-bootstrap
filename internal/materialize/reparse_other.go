//go:build !windows

package materialize

import "os"

func isWindowsReparsePoint(os.FileInfo) bool { return false }
