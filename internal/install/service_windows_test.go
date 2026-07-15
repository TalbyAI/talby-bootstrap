//go:build windows

package install

import (
	"testing"

	"github.com/talby/talby-bootstrap/internal/repositorystate"
)

func TestOutsideRootRecognizesWindowsAbsoluteLocator(t *testing.T) {
	if !outsideRoot(repositorystate.SourceIdentity{Type: "file", Locator: "C:/external/source"}) {
		t.Fatal("Windows absolute locator treated as inside Operation Root")
	}
}
