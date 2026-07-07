package tbboot

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	root := t.TempDir()
	writeInstallFixture(t, root)

	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"i", "file:" + root, "--artifact", "base-readme"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "selected artifact base-readme from local-example-source" {
		t.Fatalf("stdout = %q, want selected artifact message", got)
	}
}

func TestInstallCommandRequiresSourceArgument(t *testing.T) {
	var stderr bytes.Buffer
	code := execute(context.Background(), []string{"install"}, &bytes.Buffer{}, &stderr)
	if code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if got := strings.TrimSpace(stderr.String()); got != "accepts 1 arg(s), received 0" {
		t.Fatalf("stderr = %q, want exact-args error", got)
	}
}

func TestInstallCommandAcceptsArtifactFlag(t *testing.T) {
	root := t.TempDir()
	writeInstallFixture(t, root)

	var stdout bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + root, "--artifact", "base-readme"},
		&stdout,
		&bytes.Buffer{},
	)
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "selected artifact base-readme from local-example-source" {
		t.Fatalf("stdout = %q, want selected artifact message", got)
	}
}

func TestJSONOutputEnvelope(t *testing.T) {
	root := t.TempDir()
	writeInstallFixture(t, root)

	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"--output", "json", "install", "file:" + root, "--artifact", "base-readme"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	raw := stdout.String()
	if !strings.Contains(raw, `"source":`) || strings.Contains(raw, `"Source":`) {
		t.Fatalf("stdout = %q, want CLI JSON field names", raw)
	}
	if !strings.Contains(raw, `"artifact":`) || strings.Contains(raw, `"Artifact":`) {
		t.Fatalf("stdout = %q, want CLI JSON field names", raw)
	}
	var got struct {
		Source struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"source"`
		Artifact struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Path    string `json:"path"`
		} `json:"artifact"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal(stdout) error = %v", err)
	}
	if got.Source.Name != "local-example-source" {
		t.Fatalf("source.name = %q, want local-example-source", got.Source.Name)
	}
	if got.Artifact.Name != "base-readme" {
		t.Fatalf("artifact.name = %q, want base-readme", got.Artifact.Name)
	}
}

func TestInvalidOutputModeFailsAsValidationError(t *testing.T) {
	var stderr bytes.Buffer
	code := execute(context.Background(), []string{"--output", "xml", "upgrade"}, &bytes.Buffer{}, &stderr)
	if code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if got := strings.TrimSpace(stderr.String()); got != `unsupported output mode "xml"` {
		t.Fatalf("stderr = %q, want unsupported output message", got)
	}
}

func TestJSONOutputErrorsGoToStderrAsJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := execute(context.Background(), []string{"--output", "json", "install", "invalid"}, &stdout, &stderr)
	if code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}

	var result app.Result
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal(stderr) error = %v", err)
	}
	if result.Code != app.ExitOperationalOrValidationError {
		t.Fatalf("result.code = %d, want %d", result.Code, app.ExitOperationalOrValidationError)
	}
	if result.Message != "source must be formatted as <type>:<locator>" {
		t.Fatalf("result.message = %q, want source parse error", result.Message)
	}
}

func writeInstallFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeTestFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\nsteps:\n  - type: file\n    path: README.md\n    source: README.md\n")
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
