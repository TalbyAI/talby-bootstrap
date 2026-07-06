package tbboot

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/app"
)

func TestHelpIncludesV1CommandSurfaces(t *testing.T) {
	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"--help"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	out := stdout.String()
	for _, want := range []string{"install", "upgrade", "search", "logs", "catalog"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output missing %q:\n%s", want, out)
		}
	}
}

func TestInstallAlias(t *testing.T) {
	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"i"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "not implemented" {
		t.Fatalf("stdout = %q, want not implemented", got)
	}
}

func TestJSONOutputEnvelope(t *testing.T) {
	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"--output", "json", "install"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); !strings.Contains(got, `"message":"not implemented"`) {
		t.Fatalf("stdout = %q, want JSON result", got)
	}
}

func TestInvalidOutputModeFailsAsValidationError(t *testing.T) {
	code := execute(context.Background(), []string{"--output", "xml", "install"}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
