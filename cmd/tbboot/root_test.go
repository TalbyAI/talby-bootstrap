package tbboot

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestHelpIncludesV1CommandSurfaces(t *testing.T) {
	var stdout bytes.Buffer
	if code := execute(context.Background(), []string{"--help"}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"install", "upgrade", "search", "logs", "catalog"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q", want)
		}
	}
}

func TestPlaceholderCommandsRenderHumanAndJSON(t *testing.T) {
	for _, args := range [][]string{{"upgrade"}, {"catalog", "list"}} {
		var stdout bytes.Buffer
		if code := execute(context.Background(), args, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) || strings.TrimSpace(stdout.String()) != "not implemented" {
			t.Fatalf("execute(%v) = %d, %q", args, code, stdout.String())
		}
	}
	var stdout bytes.Buffer
	if code := execute(context.Background(), []string{"--output", "json", "search"}, &stdout, &bytes.Buffer{}); code != int(app.ExitSuccess) {
		t.Fatalf("JSON exit code = %d", code)
	}
	var result app.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Message != "not implemented" {
		t.Fatalf("JSON result = %#v, %v", result, err)
	}
}

func TestUnsupportedOutputModeReportsHumanError(t *testing.T) {
	var stderr bytes.Buffer
	if code := execute(context.Background(), []string{"--output", "xml", "search"}, &bytes.Buffer{}, &stderr); code != int(app.ExitOperationalOrValidationError) {
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
			source, ok := change["source"].(map[string]any)
			locator, locatorOK := source["locator"].(string)
			sourceVersion, versionOK := change["source_version"].(string)
			artifact, artifactOK := change["artifact"].(string)
			kind, kindOK := change["kind"].(string)
			if !ok || source["type"] != "file" || !locatorOK || locator == "" || !versionOK || sourceVersion == "" || !artifactOK || artifact != "base-readme" || !kindOK {
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

func TestChangeProvenanceInHumanAndJSONOutput(t *testing.T) {
	change := installsvc.Change{
		Kind:          installsvc.ChangeFileCreated,
		Source:        repositorystate.SourceIdentity{Type: "file", Locator: "source"},
		SourceVersion: "snapshot",
		Artifact:      "a",
		Path:          "a.txt",
		OwnershipKind: installsvc.OwnershipWholeFile,
	}
	result := installsvc.Result{Operation: "sync", Outcome: installsvc.OutcomeApplied, ArtifactCount: 1, Changes: []installsvc.Change{change}}
	var human bytes.Buffer
	if err := writeResult(&human, result); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"source_version=snapshot", "ownership_kind=whole_file"} {
		if !strings.Contains(human.String(), want) {
			t.Fatalf("human output = %q, want %q", human.String(), want)
		}
	}
	data, err := json.Marshal(resultEnvelope("sync succeeded", result))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"source_version":"snapshot"`, `"ownership_kind":"whole_file"`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("JSON = %s, want %s", data, want)
		}
	}
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
			source, ok := conflict["source"].(map[string]any)
			paths, pathsOK := conflict["paths"].([]any)
			locator, locatorOK := source["locator"].(string)
			artifact, artifactOK := conflict["artifact"].(string)
			if conflict["kind"] != "drift" || !ok || source["type"] != "file" || !locatorOK || locator == "" || !artifactOK || artifact != "base-readme" || !pathsOK || len(paths) == 0 {
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
		firstSource := denied[0].(map[string]any)
		secondSource := denied[1].(map[string]any)
		if firstSource["locator"] != first || secondSource["locator"] != second {
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
	if result.Message != "source must be formatted as <type>:<locator>" {
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
	writeTestFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: base-readme\n    path: artifacts/base-readme\n  - name: second\n    path: artifacts/second\n")
}

func writeFixture(t *testing.T, root, artifact, target, contents string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "talby-source.yaml"), "schema_version: 1\nsource:\n  name: local-example-source\nartifacts:\n  - name: "+artifact+"\n    path: artifacts/"+artifact+"\n")
	writeTestFile(t, filepath.Join(root, "artifacts", artifact, "talby-artifact.yaml"), "schema_version: 1\nartifact:\n  name: "+artifact+"\n  version: 1.0.0\nsteps:\n  - type: file\n    path: "+target+"\n    source: "+target+"\n")
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
