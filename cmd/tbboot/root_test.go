package tbboot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/app"
	installsvc "github.com/talby/talby-bootstrap/internal/install"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
)

func TestRepositoryRootAtCanonicalizesGitOutput(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlink: %v", err)
	}

	got, err := repositoryRootAt(context.Background(), alias, func(context.Context, string) ([]byte, []byte, error) {
		return []byte(alias + "\n"), nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("repositoryRootAt() = %q, want %q", got, want)
	}
}

func TestRepositoryRootAtRejectsMalformedGitOutput(t *testing.T) {
	cwd := t.TempDir()
	for name, stdout := range map[string][]byte{
		"leading newline":        []byte("\n" + cwd + "\n"),
		"interior newline":       []byte(cwd + "\nchild\n"),
		"extra trailing newline": []byte(cwd + "\n\n"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := repositoryRootAt(context.Background(), cwd, func(context.Context, string) ([]byte, []byte, error) {
				return stdout, nil, nil
			})
			if err == nil {
				t.Fatal("repositoryRootAt() error = nil")
			}
		})
	}
}

func TestRepositoryRootAtPreservesPathWhitespace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root ")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := repositoryRootAt(context.Background(), root, func(context.Context, string) ([]byte, []byte, error) {
		return []byte(root + "\n"), nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("repositoryRootAt() = %q, want %q", got, root)
	}
}

func TestRepositoryRootAtFallsBackOnlyForExplicitNonRepository(t *testing.T) {
	cwd := t.TempDir()
	got, err := repositoryRootAt(context.Background(), cwd, func(context.Context, string) ([]byte, []byte, error) {
		return nil, []byte("fatal: not a git repository (or any of the parent directories): .git\n"), errors.New("exit status 128")
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("fallback root = %q, want %q", got, want)
	}
}

func TestRepositoryRootAtPropagatesNonRepositoryFailures(t *testing.T) {
	cases := []struct {
		name   string
		stdout []byte
		stderr []byte
		err    error
	}{
		{name: "missing git", err: exec.ErrNotFound},
		{name: "permission failure", stderr: []byte("fatal: permission denied\n"), err: errors.New("exit status 128")},
		{name: "unrelated failure mentioning repository", stderr: []byte("fatal: not a git repository but permission denied\n"), err: errors.New("exit status 128")},
		{name: "malformed output", stdout: []byte("one\ntwo\n")},
		{name: "empty output", stdout: []byte{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repositoryRootAt(context.Background(), t.TempDir(), func(context.Context, string) ([]byte, []byte, error) {
				return tc.stdout, tc.stderr, tc.err
			})
			if err == nil {
				t.Fatal("repositoryRootAt() error = nil")
			}
		})
	}
}

func TestStableGitEnvironmentSetsOneLocale(t *testing.T) {
	environment := stableGitEnvironment([]string{"PATH=/bin", "LC_ALL=fr_FR", "OTHER=value", "LC_ALL=de_DE"})
	localeCount := 0
	for _, value := range environment {
		if strings.HasPrefix(value, "LC_ALL=") {
			localeCount++
			if value != "LC_ALL=C" {
				t.Fatalf("locale entry = %q, want LC_ALL=C", value)
			}
		}
	}
	if localeCount != 1 {
		t.Fatalf("LC_ALL entries = %d, want 1", localeCount)
	}
	for _, want := range []string{"PATH=/bin", "OTHER=value"} {
		found := false
		for _, value := range environment {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("environment missing %q: %v", want, environment)
		}
	}
}

func TestHelpIncludesOnlyImplementedCommandSurface(t *testing.T) {
	var stdout bytes.Buffer
	if code := execute(context.Background(), []string{"--help"}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "install") {
		t.Fatal("help output missing install")
	}
	for _, removed := range []string{"upgrade", "search", "logs", "catalog"} {
		if strings.Contains(stdout.String(), removed) {
			t.Fatalf("help output contains removed command %q", removed)
		}
	}
}

func TestRemovedCommandsDoNotReportSuccessfulPlaceholders(t *testing.T) {
	for _, args := range [][]string{{"upgrade"}, {"catalog", "list"}, {"search"}, {"logs"}} {
		var stdout bytes.Buffer
		if code := execute(context.Background(), args, &stdout, &bytes.Buffer{}); code == int(app.ExitSuccess) || strings.Contains(stdout.String(), "not implemented") {
			t.Fatalf("execute(%v) = %d, %q", args, code, stdout.String())
		}
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := execute(context.Background(), []string{"--output", "json", "search"}, &stdout, &stderr); code == int(app.ExitSuccess) {
		t.Fatalf("JSON exit code = %d, stderr=%q", code, stderr.String())
	}
}

func TestUnsupportedOutputModeReportsHumanError(t *testing.T) {
	var stderr bytes.Buffer
	if code := execute(context.Background(), []string{"--output", "xml", "install"}, &bytes.Buffer{}, &stderr); code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), `unsupported output mode "xml"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestSyncHumanNoOpAndAppliedOutput(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme", "--declare-only"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("declare-only exit code = %d, want 0", code)
		}
		var applied bytes.Buffer
		if code := execute(context.Background(), []string{"install"}, &applied, &bytes.Buffer{}); code != 0 {
			t.Fatalf("sync exit code = %d, want 0", code)
		}
		if !strings.Contains(applied.String(), "sync: applied") || !strings.Contains(applied.String(), "file_created") {
			t.Fatalf("applied output = %q", applied.String())
		}
		var noOp bytes.Buffer
		if code := execute(context.Background(), []string{"install"}, &noOp, &bytes.Buffer{}); code != 0 {
			t.Fatalf("repeat sync exit code = %d, want 0", code)
		}
		if got := strings.TrimSpace(noOp.String()); got != "sync: no changes (1 artifacts)" {
			t.Fatalf("no-op output = %q", got)
		}
	})
}

func TestSyncJSONOmitsChangesForNoOp(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("setup install exit code = %d, want 0", code)
		}
		var stdout bytes.Buffer
		if code := execute(context.Background(), []string{"--output", "json", "install"}, &stdout, &bytes.Buffer{}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		var envelope app.Result
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Details["outcome"] != "no_op" {
			t.Fatalf("outcome = %#v", envelope.Details["outcome"])
		}
		if _, ok := envelope.Details["changes"]; ok {
			t.Fatal("no-op JSON contains changes")
		}
	})
}

func TestSyncJSONIncludesTypedEffectiveChanges(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme", "--declare-only"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("setup declare-only exit code = %d, want 0", code)
		}
		var stdout bytes.Buffer
		if code := execute(context.Background(), []string{"--output", "json", "install"}, &stdout, &bytes.Buffer{}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		var envelope app.Result
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		changes, ok := envelope.Details["changes"].([]any)
		if !ok || len(changes) == 0 {
			t.Fatalf("changes = %#v", envelope.Details["changes"])
		}
		seen := map[string]bool{}
		for _, raw := range changes {
			change, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("change = %#v, want object", raw)
			}
			source, sourceOK := change["source"].(string)
			sourceVersion, versionOK := change["source_version"].(string)
			artifact, artifactOK := change["artifact"].(string)
			kind, kindOK := change["kind"].(string)
			if !sourceOK || !strings.HasPrefix(source, "file:") || !versionOK || sourceVersion == "" || !artifactOK || artifact != "base-readme" || !kindOK {
				t.Fatalf("change provenance = %#v", change)
			}
			seen[kind] = true
			switch kind {
			case "resolution_locked":
			case "file_created":
				if change["path"] != "README.md" || change["ownership_kind"] != "whole_file" {
					t.Fatalf("file change = %#v", change)
				}
			default:
				t.Fatalf("change kind = %#v", change["kind"])
			}
		}
		if !seen["resolution_locked"] || !seen["file_created"] {
			t.Fatalf("change kinds = %#v", seen)
		}
	})
}

func TestSyncJSONIncludesFileRemovedChange(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("setup install exit code = %d, want 0", code)
		}
		store := repositorystate.NewStore()
		manifest, err := store.LoadManifest(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		manifest.Declarations = nil
		if err := store.WriteManifest(context.Background(), root, manifest); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		if code := execute(context.Background(), []string{"--output", "json", "install", "--prune"}, &stdout, &stderr); code != int(app.ExitSuccess) {
			t.Fatalf("prune exit code = %d, stderr = %q", code, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty", stderr.String())
		}
		var envelope app.Result
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Details["outcome"] != "applied" {
			t.Fatalf("outcome = %#v, want applied", envelope.Details["outcome"])
		}
		changes, ok := envelope.Details["changes"].([]any)
		if !ok {
			t.Fatalf("changes = %#v, want array", envelope.Details["changes"])
		}
		for _, raw := range changes {
			change, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("change = %#v, want object", raw)
			}
			if change["kind"] == "file_removed" {
				if change["path"] != "README.md" || change["ownership_kind"] != "whole_file" {
					t.Fatalf("file removal change = %#v", change)
				}
				return
			}
		}
		t.Fatalf("changes = %#v, missing file_removed", changes)
	})
}

func TestSyncJSONIncludesUnsafeTopologyConflict(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeFixture(t, sourceRoot, "nested", "nested/a", "hello\n")
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("setup install exit code = %d, want 0", code)
		}
		if err := os.RemoveAll(filepath.Join(root, "nested")); err != nil {
			t.Fatal(err)
		}
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "nested")); err != nil {
			t.Skipf("symlink: %v", err)
		}

		var stdout, stderr bytes.Buffer
		if code := execute(context.Background(), []string{"--output", "json", "install"}, &stdout, &stderr); code != int(app.ExitUserActionConflict) {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
		var envelope app.Result
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		conflicts, ok := envelope.Details["conflicts"].([]any)
		if !ok {
			t.Fatalf("conflicts = %#v, want array", envelope.Details["conflicts"])
		}
		for _, raw := range conflicts {
			conflict, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("conflict = %#v, want object", raw)
			}
			if conflict["kind"] == "unsafe_topology" {
				paths, ok := conflict["paths"].([]any)
				if !ok || len(paths) != 1 || paths[0] != "nested/a" {
					t.Fatalf("unsafe topology paths = %#v, want [nested/a]", conflict["paths"])
				}
				return
			}
		}
		t.Fatalf("conflicts = %#v, missing unsafe_topology", conflicts)
	})
}

func TestChangeProvenanceInHumanAndJSONOutput(t *testing.T) {
	change := installsvc.Change{
		Kind:          installsvc.ChangeFileCreated,
		Source:        repositorystate.SourceIdentity{Type: "file", Locator: "./source"},
		SourceVersion: "sha256:" + strings.Repeat("a", 64),
		Artifact:      "a",
		Path:          "a.txt",
		OwnershipKind: installsvc.OwnershipWholeFile,
	}
	result := installsvc.Result{Operation: "sync", Outcome: installsvc.OutcomeApplied, ArtifactCount: 1, Changes: []installsvc.Change{change}}
	var human bytes.Buffer
	if err := writeResult(&human, result); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"source_version=sha256:" + strings.Repeat("a", 64), "ownership_kind=whole_file"} {
		if !strings.Contains(human.String(), want) {
			t.Fatalf("human output = %q, want %q", human.String(), want)
		}
	}
	data, err := json.Marshal(resultEnvelope("sync succeeded", result))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"source_version":"sha256:` + strings.Repeat("a", 64) + `"`, `"ownership_kind":"whole_file"`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("JSON = %s, want %s", data, want)
		}
	}
}

func TestInstallDryRunReportsPlannedWithoutWriting(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		var stdout bytes.Buffer
		if code := execute(context.Background(), []string{"install", "--dry-run", "file:" + sourceRoot}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), "install: planned") {
			t.Fatalf("stdout = %q, want planned label", stdout.String())
		}
		for _, name := range []string{
			repositorystate.ManifestFileName,
			repositorystate.LockfileFileName,
			repositorystate.MaterializationRecordFileName,
			".tbboot-operation.lock",
			"README.md",
		} {
			if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
				t.Fatalf("%s after dry run = %v, want absent", name, err)
			}
		}
	})
}

func TestSyncDryRunReportsPlannedWithoutWriting(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "--declare-only", "file:" + sourceRoot}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("declare-only exit code = %d, want 0", code)
		}
		manifestBefore, err := os.ReadFile(filepath.Join(root, repositorystate.ManifestFileName))
		if err != nil {
			t.Fatal(err)
		}

		var stdout bytes.Buffer
		if code := execute(context.Background(), []string{"install", "--dry-run"}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), "sync: planned") {
			t.Fatalf("stdout = %q, want planned label", stdout.String())
		}
		manifestAfter, err := os.ReadFile(filepath.Join(root, repositorystate.ManifestFileName))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(manifestAfter, manifestBefore) {
			t.Fatal("dry run changed manifest")
		}
		for _, name := range []string{repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName, "README.md"} {
			if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
				t.Fatalf("%s after dry run = %v, want absent", name, err)
			}
		}
	})
}

func TestInstallDryRunJSONIncludesDryRunForExplicitAndSync(t *testing.T) {
	for _, bare := range []bool{false, true} {
		name := "explicit"
		if bare {
			name = "bare"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := filepath.Join(root, "source")
			writeInstallFixture(t, sourceRoot)
			initGitRepo(t, root)

			withDir(t, root, func() {
				args := []string{"--output", "json", "install", "--dry-run"}
				if !bare {
					args = append(args, "file:"+sourceRoot)
				} else if code := execute(context.Background(), []string{"install", "--declare-only", "file:" + sourceRoot}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitSuccess) {
					t.Fatalf("declare-only exit code = %d, want 0", code)
				}
				var stdout bytes.Buffer
				if code := execute(context.Background(), args, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
					t.Fatalf("args %v exit code = %d, want 0", args, code)
				}
				var envelope app.Result
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if envelope.Details["dry_run"] != true || envelope.Details["outcome"] != "planned" {
					t.Fatalf("details = %#v, want dry_run=true and outcome=planned", envelope.Details)
				}
			})
		})
	}
}

func TestInstallDeclarationOnlyDryRunDoesNotCreateManifest(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		var stdout bytes.Buffer
		if code := execute(context.Background(), []string{"install", "--dry-run", "--declare-only", "--artifact", "base-readme", "file:" + sourceRoot}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), "install: planned") {
			t.Fatalf("stdout = %q, want planned label", stdout.String())
		}
		if _, err := os.Stat(filepath.Join(root, repositorystate.ManifestFileName)); !os.IsNotExist(err) {
			t.Fatalf("manifest after dry run = %v, want absent", err)
		}
	})
}

func TestInstallDryRunNoOpJSONIncludesDryRun(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("setup exit code = %d, want 0", code)
		}
		before := stateFiles(t, root)
		var stdout bytes.Buffer
		if code := execute(context.Background(), []string{"--output", "json", "install", "--dry-run"}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
			t.Fatalf("dry-run exit code = %d, want 0", code)
		}
		var envelope app.Result
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Details["dry_run"] != true || envelope.Details["outcome"] != "no_op" {
			t.Fatalf("details = %#v, want dry_run=true and outcome=no_op", envelope.Details)
		}
		if _, ok := envelope.Details["changes"]; ok {
			t.Fatal("no-op JSON contains changes")
		}
		if got := stateFiles(t, root); !reflect.DeepEqual(got, before) {
			t.Fatal("dry-run no-op changed repository state")
		}
	})
}

func TestSyncJSONIncludesAllTypedConflictsOnStderr(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("setup install exit code = %d, want 0", code)
		}
		writeTestFile(t, filepath.Join(root, "README.md"), "user edit\n")
		var stdout, stderr bytes.Buffer
		if code := execute(context.Background(), []string{"--output", "json", "install"}, &stdout, &stderr); code != int(app.ExitUserActionConflict) {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
		var envelope app.Result
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		conflicts, ok := envelope.Details["conflicts"].([]any)
		if !ok || len(conflicts) == 0 {
			t.Fatalf("conflicts = %#v", envelope.Details["conflicts"])
		}
		for _, raw := range conflicts {
			conflict, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("conflict = %#v, want object", raw)
			}
			source, sourceOK := conflict["source"].(string)
			paths, pathsOK := conflict["paths"].([]any)
			artifact, artifactOK := conflict["artifact"].(string)
			if conflict["kind"] != "drift" || !sourceOK || !strings.HasPrefix(source, "file:") || !artifactOK || artifact != "base-readme" || !pathsOK || len(paths) == 0 {
				t.Fatalf("conflict fields = %#v", conflict)
			}
			for _, path := range paths {
				value, ok := path.(string)
				if !ok || value == "" {
					t.Fatalf("conflict path = %#v", path)
				}
			}
		}
	})
}

func TestSyncExitCodesForValidationConflictAndTrust(t *testing.T) {
	if code := execute(context.Background(), []string{"install", "invalid"}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("validation exit code = %d, want 1", code)
	}

	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)
	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("setup install exit code = %d, want 0", code)
		}
		writeTestFile(t, filepath.Join(root, "README.md"), "user edit\n")
		if code := execute(context.Background(), []string{"install"}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitUserActionConflict) {
			t.Fatalf("conflict exit code = %d, want 2", code)
		}
	})

	trustedRoot := t.TempDir()
	initGitRepo(t, trustedRoot)
	external := t.TempDir()
	writeInstallFixture(t, external)
	withDir(t, trustedRoot, func() {
		if code := execute(context.Background(), []string{"install", "file:" + external, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitTrustOrPolicyDenial) {
			t.Fatalf("trust exit code = %d, want 3", code)
		}
	})
}

func TestSyncJSONSortsDeniedSources(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	store := repositorystate.NewStore()
	if err := store.WriteManifest(context.Background(), root, repositorystate.Manifest{Declarations: []repositorystate.Declaration{
		{Source: repositorystate.SourceIdentity{Type: "file", Locator: second}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}},
		{Source: repositorystate.SourceIdentity{Type: "file", Locator: first}, Target: repositorystate.DeclarationTarget{Scope: repositorystate.DeclarationScopeSource}},
	}}); err != nil {
		t.Fatal(err)
	}
	withDir(t, root, func() {
		var stderr bytes.Buffer
		if code := execute(context.Background(), []string{"--output", "json", "install"}, &bytes.Buffer{}, &stderr); code != int(app.ExitTrustOrPolicyDenial) {
			t.Fatalf("exit code = %d, want 3", code)
		}
		var result app.Result
		if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		denied, ok := result.Details["denied_sources"].([]any)
		if !ok || len(denied) != 2 {
			t.Fatalf("denied_sources = %#v", result.Details["denied_sources"])
		}
		firstSource, firstOK := denied[0].(string)
		secondSource, secondOK := denied[1].(string)
		if !firstOK || !secondOK || strings.TrimPrefix(firstSource, "file:") != first || strings.TrimPrefix(secondSource, "file:") != second {
			t.Fatalf("denied_sources = %#v, want sorted locators", denied)
		}
	})
}

func TestExplicitSourceScopeInstallsAllArtifacts(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeTwoArtifactFixture(t, sourceRoot)
	initGitRepo(t, root)
	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		for _, path := range []string{"README.md", "SECOND.md"} {
			if _, err := os.Stat(filepath.Join(root, path)); err != nil {
				t.Fatalf("%s was not installed: %v", path, err)
			}
		}
		store := repositorystate.NewStore()
		manifest, err := store.LoadManifest(context.Background(), root)
		if err != nil || len(manifest.Declarations) != 1 || manifest.Declarations[0].Target.Scope != repositorystate.DeclarationScopeSource {
			t.Fatalf("manifest = %#v, %v", manifest, err)
		}
		lock, err := store.LoadLockfile(context.Background(), root)
		if err != nil || len(lock.Resolutions) != 1 || len(lock.Resolutions[0].Artifacts) != 2 {
			t.Fatalf("lockfile = %#v, %v", lock, err)
		}
		record, err := store.LoadMaterializationRecord(context.Background(), root)
		if err != nil || len(record.Artifacts) != 2 {
			t.Fatalf("materialization record = %#v, %v", record, err)
		}
		var repeat, repeatStderr bytes.Buffer
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot}, &repeat, &repeatStderr); code != 0 {
			t.Fatalf("repeat exit code = %d, want 0", code)
		}
		if got := strings.TrimSpace(repeat.String()); got != "install: no changes (2 artifacts)" {
			t.Fatalf("repeat output = %q", got)
		}
		if got := repeatStderr.String(); got != "" {
			t.Fatalf("repeat stderr = %q, want empty", got)
		}
	})
}

func TestExplicitArtifactScopeInstallsOnlyNamedArtifact(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeTwoArtifactFixture(t, sourceRoot)
	initGitRepo(t, root)
	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if _, err := os.Stat(filepath.Join(root, "README.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, "SECOND.md")); !os.IsNotExist(err) {
			t.Fatalf("SECOND.md stat error = %v, want not exist", err)
		}
	})
}

func TestRepeatedExplicitInstallDoesNotUpgradeSnapshot(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)
	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("setup install exit code = %d, want 0", code)
		}
		writeTestFile(t, filepath.Join(sourceRoot, "artifacts", "base-readme", "README.md"), "changed\n")
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != int(app.ExitOperationalOrValidationError) {
			t.Fatalf("exit code = %d, want 1", code)
		}
	})
}

func TestMultipleDeclarationRealPathSync(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	writeInstallFixture(t, first)
	writeFixture(t, second, "second", "SECOND.md", "second\n")
	initGitRepo(t, root)
	withDir(t, root, func() {
		for _, args := range [][]string{{"install", "file:" + first, "--declare-only"}, {"install", "file:" + second, "--artifact", "second", "--declare-only"}} {
			if code := execute(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
				t.Fatalf("declare-only exit code = %d, want 0", code)
			}
		}
		var stdout bytes.Buffer
		if code := execute(context.Background(), []string{"install"}, &stdout, &bytes.Buffer{}); code != 0 {
			t.Fatalf("sync exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), "(2 artifacts)") {
			t.Fatalf("stdout = %q", stdout.String())
		}
		for _, path := range []string{"README.md", "SECOND.md"} {
			if _, err := os.Stat(filepath.Join(root, path)); err != nil {
				t.Fatal(err)
			}
		}
		beforeNoOp := stateFiles(t, root)
		var noOp bytes.Buffer
		if code := execute(context.Background(), []string{"install"}, &noOp, &bytes.Buffer{}); code != 0 {
			t.Fatalf("repeat sync exit code = %d, want 0", code)
		}
		if got := strings.TrimSpace(noOp.String()); got != "sync: no changes (2 artifacts)" {
			t.Fatalf("no-op output = %q", got)
		}
		if got := stateFiles(t, root); !reflect.DeepEqual(got, beforeNoOp) {
			t.Fatal("no-op sync changed state")
		}

		writeTestFile(t, filepath.Join(root, "README.md"), "user edit\n")
		store := repositorystate.NewStore()
		manifest, err := store.LoadManifest(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		manifest.Declarations = manifest.Declarations[:1]
		if err := store.WriteManifest(context.Background(), root, manifest); err != nil {
			t.Fatal(err)
		}
		beforeConflict := stateFiles(t, root)
		var stderr bytes.Buffer
		if code := execute(context.Background(), []string{"install"}, &bytes.Buffer{}, &stderr); code != int(app.ExitUserActionConflict) {
			t.Fatalf("conflict exit code = %d, want 2", code)
		}
		if !strings.Contains(stderr.String(), "drift") || !strings.Contains(stderr.String(), "removal_required") {
			t.Fatalf("conflict output = %q", stderr.String())
		}
		if got := stateFiles(t, root); !reflect.DeepEqual(got, beforeConflict) {
			t.Fatal("conflicted sync changed state")
		}
	})
}

func TestPruneRequiresTargetlessInstall(t *testing.T) {
	var stderr bytes.Buffer
	if code := execute(context.Background(), []string{"install", "--prune", "file:source"}, &bytes.Buffer{}, &stderr); code != int(app.ExitOperationalOrValidationError) {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "prune requires targetless install") {
		t.Fatalf("stderr = %q, want prune validation", stderr.String())
	}
}

func TestPruneRemovesManagedArtifactFromCompleteDesiredState(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	writeInstallFixture(t, sourceRoot)
	initGitRepo(t, root)

	withDir(t, root, func() {
		if code := execute(context.Background(), []string{"install", "file:" + sourceRoot, "--artifact", "base-readme"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Fatalf("setup install exit code = %d, want 0", code)
		}
		store := repositorystate.NewStore()
		manifest, err := store.LoadManifest(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		manifest.Declarations = nil
		if err := store.WriteManifest(context.Background(), root, manifest); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		if code := execute(context.Background(), []string{"install", "--prune"}, &stdout, &stderr); code != int(app.ExitSuccess) {
			t.Fatalf("prune exit code = %d, stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "sync: applied") || !strings.Contains(stdout.String(), "file_removed") {
			t.Fatalf("stdout = %q, want removal summary", stdout.String())
		}
		if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
			t.Fatalf("README.md stat = %v, want removed", err)
		}
		lock, err := store.LoadLockfile(context.Background(), root)
		if err != nil || len(lock.Resolutions) != 0 {
			t.Fatalf("lockfile = %#v, %v, want empty", lock, err)
		}
		record, err := store.LoadMaterializationRecord(context.Background(), root)
		if err != nil || len(record.Artifacts) != 0 {
			t.Fatalf("materialization record = %#v, %v, want empty", record, err)
		}
	})
}

func TestJSONOutputErrorsGoToStderrAsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := execute(context.Background(), []string{"--output", "json", "install", "invalid"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	var result app.Result
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Message != "source reference must be formatted as <type>:<locator>" {
		t.Fatalf("message = %q", result.Message)
	}
}

func withDir(t *testing.T, dir string, run func()) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}()
	run()
}

func writeInstallFixture(t *testing.T, root string) {
	t.Helper()
	writeFixture(t, root, "base-readme", "README.md", "hello\n")
}

func writeTwoArtifactFixture(t *testing.T, root string) {
	t.Helper()
	writeInstallFixture(t, root)
	writeFixture(t, root, "second", "SECOND.md", "second\n")
	writeTestFile(t, filepath.Join(root, "tbboot-source.yaml"), "schema_version: 1\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n  - name: second\n    path: artifacts/second\n")
}

func writeFixture(t *testing.T, root, artifact, target, contents string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "tbboot-source.yaml"), "schema_version: 1\nartifacts:\n  - name: "+artifact+"\n    path: artifacts/"+artifact+"\n")
	writeTestFile(t, filepath.Join(root, "artifacts", artifact, "tbboot-artifact.yaml"), "schema_version: 1\nartifact:\n  name: "+artifact+"\n  version: 1.0.0\nsteps:\n  - type: file\n    path: "+target+"\n    source: "+target+"\n")
	writeTestFile(t, filepath.Join(root, "artifacts", artifact, target), contents)
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stateFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, name := range []string{repositorystate.ManifestFileName, repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = data
	}
	return files
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	if output, err := exec.CommandContext(t.Context(), "git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
}
