package tbboot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/talby/talby-bootstrap/internal/examples"
	"gopkg.in/yaml.v3"
)

func TestExamplesE2E(t *testing.T) {
	library, err := examples.Discover(filepath.Join("..", "..", "testdata", "examples"))
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	for _, example := range library.Examples {
		example := example
		t.Run(example.Metadata.ID, func(t *testing.T) {
			switch example.Metadata.Status {
			case "skipped":
				t.Skip("example status is skipped")
			case "deprecated":
				t.Skip("example status is deprecated")
			}

			if !example.ShouldRun() {
				t.Fatalf("status %q should not run and was not handled", example.Metadata.Status)
			}

			err := runExample(t, example)
			if example.ExpectsPass() {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatalf("broken example satisfied its contract; promote %s to active", example.Metadata.ID)
			}
		})
	}
}

type commandResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func runExample(t *testing.T, example examples.Example) error {
	t.Helper()

	workspace := stageExample(t, example)
	initGitRepo(t, workspace)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Getwd() error = %w", err)
	}
	if err := os.Chdir(workspace); err != nil {
		return fmt.Errorf("Chdir(%q) error = %w", workspace, err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore Chdir(%q) error = %v", cwd, err)
		}
	}()

	var last commandResult
	for _, command := range example.Metadata.Commands {
		args, err := normalizeExampleArgs(command.Argv, workspace)
		if err != nil {
			return err
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		last = commandResult{
			exitCode: execute(context.Background(), args, &stdout, &stderr),
			stdout:   stdout.String(),
			stderr:   stderr.String(),
		}
	}

	return verifyExample(example, workspace, last)
}

func stageExample(t *testing.T, example examples.Example) string {
	t.Helper()

	workspace := t.TempDir()
	copyTree(t, filepath.Join(example.Path, "consumer"), workspace)
	sourceAlias := readExampleSourceAlias(t, example)
	copyTree(t, filepath.Join(example.Path, "source"), filepath.Join(workspace, ".tbboot-example", "sources", filepath.FromSlash(sourceAlias)))
	return workspace
}

func readExampleSourceAlias(t *testing.T, example examples.Example) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(example.Path, "source", "talby-source.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(source/talby-source.yaml) error = %v", err)
	}
	var descriptor struct {
		Source struct {
			Name string `yaml:"name"`
		} `yaml:"source"`
	}
	if err := yaml.Unmarshal(data, &descriptor); err != nil {
		t.Fatalf("Unmarshal(source/talby-source.yaml) error = %v", err)
	}
	if descriptor.Source.Name == "" {
		t.Fatal("source/talby-source.yaml missing source.name")
	}
	return descriptor.Source.Name
}

func normalizeExampleArgs(argv []string, workspace string) ([]string, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("command argv must not be empty")
	}
	args := append([]string(nil), argv...)
	if args[0] != "tbboot" {
		return nil, fmt.Errorf("command must start with tbboot, got %q", args[0])
	}
	args = args[1:]
	for i, arg := range args {
		if strings.HasPrefix(arg, "file:") {
			alias := strings.TrimPrefix(arg, "file:")
			if alias == "" {
				return nil, fmt.Errorf("empty file source alias")
			}
			args[i] = "file:" + filepath.Join(workspace, ".tbboot-example", "sources", filepath.FromSlash(alias))
		}
	}
	return args, nil
}

func copyTree(t *testing.T, src string, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
}

func verifyExample(example examples.Example, workspace string, result commandResult) error {
	if err := verifyExitCode(example, result.exitCode); err != nil {
		return err
	}
	if err := verifyTextOutput(example, "stdout_text", example.Metadata.Verification.StdoutText, result.stdout); err != nil {
		return err
	}
	if err := verifyTextOutput(example, "stderr_text", example.Metadata.Verification.StderrText, result.stderr); err != nil {
		return err
	}
	if err := verifyJSONOutput(example, "stdout_json", example.Metadata.Verification.StdoutJSON, result.stdout); err != nil {
		return err
	}
	if err := verifyJSONOutput(example, "stderr_json", example.Metadata.Verification.StderrJSON, result.stderr); err != nil {
		return err
	}
	if example.Metadata.Verification.ConsumerState == "exact" {
		if err := verifyConsumerState(example, workspace); err != nil {
			return err
		}
	}
	return nil
}

func verifyExitCode(example examples.Example, got int) error {
	if example.Metadata.Verification.ExitCode != "exact" {
		return fmt.Errorf("%s: exit_code mode %q is not implemented in this first slice", example.Metadata.ID, example.Metadata.Verification.ExitCode)
	}
	wantRaw, err := os.ReadFile(filepath.Join(example.Path, "expected", "exit-code.txt"))
	if err != nil {
		return err
	}
	want, err := strconv.Atoi(strings.TrimSpace(string(wantRaw)))
	if err != nil {
		return fmt.Errorf("%s: parse expected exit code: %w", example.Metadata.ID, err)
	}
	if got != want {
		return fmt.Errorf("%s: exit code = %d, want %d", example.Metadata.ID, got, want)
	}
	return nil
}

func verifyTextOutput(example examples.Example, field string, mode string, got string) error {
	switch mode {
	case "absent":
		return nil
	case "exact":
		want, err := os.ReadFile(filepath.Join(example.Path, "expected", expectedTextFile(field)))
		if err != nil {
			return err
		}
		if normalizeText(got) != normalizeText(string(want)) {
			return fmt.Errorf("%s: %s mismatch\nwant:\n%s\ngot:\n%s", example.Metadata.ID, field, want, got)
		}
	case "contains":
		var expected struct {
			Fragments []string `yaml:"fragments"`
		}
		data, err := os.ReadFile(filepath.Join(example.Path, "expected", expectedContainsFile(field)))
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(data, &expected); err != nil {
			return fmt.Errorf("%s: parse %s contains file: %w", example.Metadata.ID, field, err)
		}
		for _, fragment := range expected.Fragments {
			if !strings.Contains(got, fragment) {
				return fmt.Errorf("%s: %s missing fragment %q in %q", example.Metadata.ID, field, fragment, got)
			}
		}
	}
	return nil
}

func verifyJSONOutput(example examples.Example, field string, mode string, got string) error {
	switch mode {
	case "absent":
		return nil
	case "exact":
		wantData, err := os.ReadFile(filepath.Join(example.Path, "expected", expectedJSONFile(field)))
		if err != nil {
			return err
		}
		var want any
		if err := json.Unmarshal(wantData, &want); err != nil {
			return fmt.Errorf("%s: parse expected %s: %w", example.Metadata.ID, field, err)
		}
		var gotValue any
		if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
			return fmt.Errorf("%s: parse actual %s: %w\n%s", example.Metadata.ID, field, err, got)
		}
		if !reflect.DeepEqual(gotValue, want) {
			return fmt.Errorf("%s: %s JSON mismatch\nwant:\n%s\ngot:\n%s", example.Metadata.ID, field, wantData, got)
		}
	case "contains":
		return fmt.Errorf("%s: %s contains verification is not implemented in this first slice", example.Metadata.ID, field)
	}
	return nil
}

func expectedTextFile(field string) string {
	if field == "stderr_text" {
		return "stderr.txt"
	}
	return "stdout.txt"
}

func expectedContainsFile(field string) string {
	if field == "stderr_text" {
		return "stderr-contains.yaml"
	}
	return "stdout-contains.yaml"
}

func expectedJSONFile(field string) string {
	if field == "stderr_json" {
		return "stderr.json"
	}
	return "stdout.json"
}

func normalizeText(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func normalizeWorkspacePath(s string, workspace string) string {
	return strings.ReplaceAll(normalizeText(s), filepath.ToSlash(workspace), "$WORKSPACE")
}

func verifyConsumerState(example examples.Example, workspace string) error {
	wantRoot := filepath.Join(example.Path, "expected", "consumer")
	want, err := snapshotTree(wantRoot, nil)
	if err != nil {
		return err
	}
	got, err := snapshotTree(workspace, map[string]struct{}{
		".tbboot-example": {},
		".git":            {},
	})
	if err != nil {
		return err
	}
	for path, content := range want {
		want[path] = normalizeWorkspacePath(content, workspace)
	}
	for path, content := range got {
		got[path] = normalizeWorkspacePath(content, workspace)
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("%s: consumer state mismatch\nwant:\n%s\ngot:\n%s", example.Metadata.ID, formatSnapshot(want), formatSnapshot(got))
	}
	return nil
}

func snapshotTree(root string, ignored map[string]struct{}) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		first := rel
		if i := strings.IndexRune(rel, filepath.Separator); i >= 0 {
			first = rel[:i]
		}
		if _, ok := ignored[first]; ok {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = normalizeText(string(data))
		return nil
	})
	return files, err
}

func formatSnapshot(files map[string]string) string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out bytes.Buffer
	for _, key := range keys {
		fmt.Fprintf(&out, "--- %s\n%s\n", key, files[key])
	}
	return out.String()
}
