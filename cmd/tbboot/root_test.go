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

func TestInstallCommandWithoutSourceRunsSyncShape(t *testing.T) {
	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"install"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "sync not implemented" {
		t.Fatalf("stdout = %q, want sync placeholder message", got)
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
	var got struct {
		Code     int               `json:"code"`
		Message  string            `json:"message"`
		Details  map[string]any    `json:"details"`
		Warnings []string          `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal(stdout) error = %v", err)
	}
	if got.Code != int(app.ExitSuccess) {
		t.Fatalf("code = %d, want 0", got.Code)
	}
	if got.Message != "install succeeded" {
		t.Fatalf("message = %q, want install succeeded", got.Message)
	}
	sourceDetails, ok := got.Details["source"].(map[string]any)
	if !ok {
		t.Fatalf("details.source = %#v, want object", got.Details["source"])
	}
	artifactDetails, ok := got.Details["artifact"].(map[string]any)
	if !ok {
		t.Fatalf("details.artifact = %#v, want object", got.Details["artifact"])
	}
	if sourceDetails["name"] != "local-example-source" {
		t.Fatalf("details.source.name = %#v, want local-example-source", sourceDetails["name"])
	}
	if artifactDetails["name"] != "base-readme" {
		t.Fatalf("details.artifact.name = %#v, want base-readme", artifactDetails["name"])
	}
	if _, ok := got.Details["code"]; ok {
		t.Fatalf("details.code present in details: %#v", got.Details)
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
