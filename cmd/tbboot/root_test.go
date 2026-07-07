package tbboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/app"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
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
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"i", "file:" + sourceRoot, "--artifact", "base-readme"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "installed artifact base-readme from local-example-source" {
		t.Fatalf("stdout = %q, want installed artifact message", got)
	}
}

func TestInstallCommandWithoutSourceRunsSyncShape(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	if code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); code != int(app.ExitSuccess) {
		t.Fatalf("first install exit code = %d, want 0", code)
	}

	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"install"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "sync: no changes" {
		t.Fatalf("stdout = %q, want sync noop message", got)
	}
}

func TestInstallCommandWithoutSourceStopsOnDrift(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	if code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); code != int(app.ExitSuccess) {
		t.Fatalf("first install exit code = %d, want 0", code)
	}
	writeTestFile(t, filepath.Join(repoRoot, "README.md"), "user edit\n")

	var stderr bytes.Buffer
	code := execute(context.Background(), []string{"install"}, &bytes.Buffer{}, &stderr)
	if code != int(app.ExitUserActionConflict) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
	}
	if got := strings.TrimSpace(stderr.String()); !strings.Contains(got, "has drifted") {
		t.Fatalf("stderr = %q, want drift text", got)
	}
}

func TestInstallCommandWithoutSourceStopsWhenSourceSnapshotChanges(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	if code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); code != int(app.ExitSuccess) {
		t.Fatalf("first install exit code = %d, want 0", code)
	}
	writeTestFile(t, filepath.Join(sourceRoot, "artifacts", "base-readme", "README.md"), "changed at source\n")

	var stderr bytes.Buffer
	code := execute(context.Background(), []string{"install"}, &bytes.Buffer{}, &stderr)
	if code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitOperationalOrValidationError)
	}
	if got := strings.TrimSpace(stderr.String()); !strings.Contains(got, "locked source version") {
		t.Fatalf("stderr = %q, want locked source version text", got)
	}
}

func TestInstallTrustDenialReturnsTrustOrPolicyExitCode(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := t.TempDir()
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&stderr,
	)
	if code != int(app.ExitTrustOrPolicyDenial) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitTrustOrPolicyDenial)
	}
	if got := strings.TrimSpace(stderr.String()); !strings.Contains(got, "outside the operation root") {
		t.Fatalf("stderr = %q, want trust denial text", got)
	}
}

func TestInstallTrustDenialReturnsTrustOrPolicyExitCodeAsJSON(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := t.TempDir()
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"--output", "json", "install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&stdout,
		&stderr,
	)
	if code != int(app.ExitTrustOrPolicyDenial) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitTrustOrPolicyDenial)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}

	var result app.Result
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal(stderr) error = %v", err)
	}
	if result.Code != app.ExitTrustOrPolicyDenial {
		t.Fatalf("result.code = %d, want %d", result.Code, app.ExitTrustOrPolicyDenial)
	}
	if !strings.Contains(result.Message, "outside the operation root") {
		t.Fatalf("result.message = %q, want trust denial text", result.Message)
	}
}

func TestInstallCommandAcceptsArtifactFlag(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&stdout,
		&bytes.Buffer{},
	)
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "installed artifact base-readme from local-example-source" {
		t.Fatalf("stdout = %q, want installed artifact message", got)
	}
}

func TestJSONOutputEnvelope(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	code := execute(context.Background(), []string{"--output", "json", "install", "file:" + sourceRoot, "--artifact", "base-readme"}, &stdout, &bytes.Buffer{})
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var got struct {
		Code     int            `json:"code"`
		Message  string         `json:"message"`
		Details  map[string]any `json:"details"`
		Warnings []string       `json:"warnings"`
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
	if got.Details["change"] != "installed" {
		t.Fatalf("details.change = %#v, want installed", got.Details["change"])
	}
	files, ok := got.Details["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("details.files = %#v, want one file", got.Details["files"])
	}
	if _, ok := got.Details["code"]; ok {
		t.Fatalf("details.code present in details: %#v", got.Details)
	}
}

func TestInstallOwnershipConflictReturnsUserActionConflictExitCode(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)
	writeTestFile(t, filepath.Join(repoRoot, repositorystate.MaterializationRecordFileName), ""+
		"schema_version: 1\n"+
		"artifacts:\n"+
		"  - key:\n"+
		"      source:\n"+
		"        type: file\n"+
		"        name: other-source\n"+
		"      resolved_version: local-snapshot-999\n"+
		"      artifact: other-artifact\n"+
		"    files:\n"+
		"      - path: README.md\n"+
		"        digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&stderr,
	)
	if code != int(app.ExitUserActionConflict) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
	}
	if got := strings.TrimSpace(stderr.String()); !strings.Contains(got, "already owned by another artifact") {
		t.Fatalf("stderr = %q, want ownership conflict text", got)
	}
}

func TestInstallOwnershipConflictReturnsUserActionConflictExitCodeAsJSON(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)
	writeTestFile(t, filepath.Join(repoRoot, repositorystate.MaterializationRecordFileName), ""+
		"schema_version: 1\n"+
		"artifacts:\n"+
		"  - key:\n"+
		"      source:\n"+
		"        type: file\n"+
		"        name: other-source\n"+
		"      resolved_version: local-snapshot-999\n"+
		"      artifact: other-artifact\n"+
		"    files:\n"+
		"      - path: README.md\n"+
		"        digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"--output", "json", "install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&stdout,
		&stderr,
	)
	if code != int(app.ExitUserActionConflict) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}

	var result app.Result
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal(stderr) error = %v", err)
	}
	if result.Code != app.ExitUserActionConflict {
		t.Fatalf("result.code = %d, want %d", result.Code, app.ExitUserActionConflict)
	}
	if !strings.Contains(result.Message, "already owned by another artifact") {
		t.Fatalf("result.message = %q, want ownership conflict text", result.Message)
	}
}

func TestInstallDriftReturnsUserActionConflictExitCode(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	if code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); code != int(app.ExitSuccess) {
		t.Fatalf("first install exit code = %d, want 0", code)
	}
	writeTestFile(t, filepath.Join(repoRoot, "README.md"), "user edit\n")

	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&stderr,
	)
	if code != int(app.ExitUserActionConflict) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
	}
	if got := strings.TrimSpace(stderr.String()); !strings.Contains(got, "has drifted") {
		t.Fatalf("stderr = %q, want drift text", got)
	}
}

func TestInstallDriftReturnsUserActionConflictExitCodeAsJSON(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	if code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); code != int(app.ExitSuccess) {
		t.Fatalf("first install exit code = %d, want 0", code)
	}
	writeTestFile(t, filepath.Join(repoRoot, "README.md"), "user edit\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"--output", "json", "install", "file:" + sourceRoot, "--artifact", "base-readme"},
		&stdout,
		&stderr,
	)
	if code != int(app.ExitUserActionConflict) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}

	var result app.Result
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal(stderr) error = %v", err)
	}
	if result.Code != app.ExitUserActionConflict {
		t.Fatalf("result.code = %d, want %d", result.Code, app.ExitUserActionConflict)
	}
	if !strings.Contains(result.Message, "has drifted") {
		t.Fatalf("result.message = %q, want drift text", result.Message)
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

func TestDeclareOnlyConflictReturnsUserActionConflictExitCode(t *testing.T) {
	repoRoot := t.TempDir()
	writeInstallFixture(t, repoRoot)
	writeTestFile(t, filepath.Join(repoRoot, "talby-artifacts.yaml"), ""+
		"schema_version: 1\n"+
		"declarations:\n"+
		"  - source:\n"+
		"      type: file\n"+
		"      name: local-example-source\n"+
		"    target:\n"+
		"      scope: artifact\n"+
		"      artifact: base-readme\n"+
		"    input:\n"+
		"      locator: /tmp/other\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
		&bytes.Buffer{},
		&stderr,
	)
	if code != int(app.ExitUserActionConflict) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
	}
	if got := strings.TrimSpace(stderr.String()); got != `artifact "base-readme" from source "local-example-source" is already declared with different input; use upgrade` {
		t.Fatalf("stderr = %q, want conflict message", got)
	}
}

func TestDeclareOnlyConflictReturnsUserActionConflictExitCodeAsJSON(t *testing.T) {
	repoRoot := t.TempDir()
	writeInstallFixture(t, repoRoot)
	writeTestFile(t, filepath.Join(repoRoot, "talby-artifacts.yaml"), ""+
		"schema_version: 1\n"+
		"declarations:\n"+
		"  - source:\n"+
		"      type: file\n"+
		"      name: local-example-source\n"+
		"    target:\n"+
		"      scope: artifact\n"+
		"      artifact: base-readme\n"+
		"    input:\n"+
		"      locator: /tmp/other\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"--output", "json", "install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
		&stdout,
		&stderr,
	)
	if code != int(app.ExitUserActionConflict) {
		t.Fatalf("exit code = %d, want %d", code, app.ExitUserActionConflict)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}

	var result app.Result
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("Unmarshal(stderr) error = %v", err)
	}
	if result.Code != app.ExitUserActionConflict {
		t.Fatalf("result.code = %d, want %d", result.Code, app.ExitUserActionConflict)
	}
	if result.Message != `artifact "base-readme" from source "local-example-source" is already declared with different input; use upgrade` {
		t.Fatalf("result.message = %q, want conflict message", result.Message)
	}
}

func TestDeclareOnlyInstallCommandWritesHumanSuccessMessage(t *testing.T) {
	repoRoot := t.TempDir()
	writeInstallFixture(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
		&stdout,
		&bytes.Buffer{},
	)
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "declared artifact base-readme from local-example-source" {
		t.Fatalf("stdout = %q, want declare-only message", got)
	}
}

func TestDeclareOnlyInstallCommandJSONIncludesChange(t *testing.T) {
	repoRoot := t.TempDir()
	writeInstallFixture(t, repoRoot)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"--output", "json", "install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
		&stdout,
		&bytes.Buffer{},
	)
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var got struct {
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal(stdout) error = %v", err)
	}
	if got.Message != "declare-only succeeded" {
		t.Fatalf("message = %q, want declare-only succeeded", got.Message)
	}
	if got.Details["change"] != "declared" {
		t.Fatalf("details.change = %#v, want declared", got.Details["change"])
	}
}

func TestDeclareOnlyInstallCommandWritesNoOpHumanMessage(t *testing.T) {
	repoRoot := t.TempDir()
	writeInstallFixture(t, repoRoot)
	writeTestFile(t, filepath.Join(repoRoot, "talby-artifacts.yaml"), ""+
		"schema_version: 1\n"+
		"declarations:\n"+
		"  - source:\n"+
		"      type: file\n"+
		"      name: local-example-source\n"+
		"    target:\n"+
		"      scope: artifact\n"+
		"      artifact: base-readme\n"+
		"    input:\n"+
		"      locator: "+repoRoot+"\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	var stdout bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "file:" + repoRoot, "--artifact", "base-readme", "--declare-only"},
		&stdout,
		&bytes.Buffer{},
	)
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "artifact base-readme from local-example-source is already declared" {
		t.Fatalf("stdout = %q, want noop message", got)
	}
}

func TestDeclareOnlyInstallCommandWritesManifestAtGitRepositoryRoot(t *testing.T) {
	repoRoot := t.TempDir()
	sourceRoot := filepath.Join(repoRoot, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, repoRoot)
	subdir := filepath.Join(repoRoot, "nested", "work")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", subdir, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	code := execute(
		context.Background(),
		[]string{"install", "file:" + sourceRoot, "--artifact", "base-readme", "--declare-only"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, repositorystate.ManifestFileName)); err != nil {
		t.Fatalf("repo root manifest stat error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(subdir, repositorystate.ManifestFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("subdir manifest state error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(sourceRoot, repositorystate.ManifestFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source root manifest state error = %v, want not exist", err)
	}
}

func TestDeclareOnlyInstallCommandRejectsMissingSource(t *testing.T) {
	var stderr bytes.Buffer
	code := execute(
		context.Background(),
		[]string{"install", "--declare-only"},
		&bytes.Buffer{},
		&stderr,
	)
	if code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if got := strings.TrimSpace(stderr.String()); got != "declare-only install requires an explicit <source>" {
		t.Fatalf("stderr = %q, want missing source message", got)
	}
}

func writeInstallFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n")
	writeTestFile(t, filepath.Join(root, "artifacts", "base-readme", "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: base-readme\n  version: 1.0.0\nsteps:\n  - type: file\n    path: README.md\n    source: README.md\n")
	writeTestFile(t, filepath.Join(root, "artifacts", "base-readme", "README.md"), "hello\n")
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

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "init", "-q", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %q error = %v, output = %s", root, err, output)
	}
}
