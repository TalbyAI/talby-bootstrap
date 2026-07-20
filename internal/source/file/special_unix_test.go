//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package file

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/talby/talby-bootstrap/internal/source"
)

func TestResolveRejectsSpecialStepInput(t *testing.T) {
	root := fixture(t)
	input := filepath.Join(root, "a", "in")
	if err := os.Remove(input); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(input, 0644); err != nil {
		t.Skipf("Mkfifo: %v", err)
	}
	if _, err := New().Resolve(context.Background(), source.ResolveRequest{Ref: source.Ref{Type: "file", Locator: root}}); err == nil {
		t.Fatal("Resolve() accepted special source input")
	}
}
