package examples

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverInitialExampleLibrary(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "examples")

	library, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if got, want := len(library.Examples), 6; got != want {
		t.Fatalf("len(library.Examples) = %d, want %d", got, want)
	}

	wantIDs := map[string]struct{}{
		"file-direct-install-multi-artifact":  {},
		"declare-only-flow":                   {},
		"declare-only-manifest-only":          {},
		"json-success-envelope-minimal":       {},
		"file-direct-install-single-artifact": {},
		"ownership-conflict-overlapping-file": {},
	}

	for _, example := range library.Examples {
		if _, ok := wantIDs[example.Metadata.ID]; !ok {
			t.Fatalf("unexpected example id %q", example.Metadata.ID)
		}

		delete(wantIDs, example.Metadata.ID)
	}

	if len(wantIDs) != 0 {
		t.Fatalf("missing example ids: %#v", wantIDs)
	}

	if _, err := os.Stat(filepath.Join(root, "README.md")); err != nil {
		t.Fatalf("root README missing: %v", err)
	}
}

func TestDiscoverRejectsVerificationFileMismatch(t *testing.T) {
	root := t.TempDir()
	exampleDir := filepath.Join(root, "atomic-cases", "bad-example")

	mkdirAll(t, filepath.Join(root, "scenarios"))
	mkdirAll(t, filepath.Join(exampleDir, "source"))
	mkdirAll(t, filepath.Join(exampleDir, "consumer"))
	mkdirAll(t, filepath.Join(exampleDir, "expected"))

	writeFile(t, filepath.Join(root, "README.md"), "# Examples\n")
	writeFile(t, filepath.Join(exampleDir, "README.md"), "# Bad Example\n")
	writeFile(t, filepath.Join(exampleDir, "example.yaml"), ""+
		"schema_version: 1\n"+
		"id: bad-example\n"+
		"kind: atomic-case\n"+
		"status: active\n"+
		"polarity: positive\n"+
		"summary: Broken verification contract.\n"+
		"commands:\n"+
		"  - argv:\n"+
		"      - tbboot\n"+
		"      - install\n"+
		"verification:\n"+
		"  exit_code: exact\n"+
		"  stdout_text: contains\n"+
		"  stdout_json: absent\n"+
		"  stderr_text: absent\n"+
		"  stderr_json: absent\n"+
		"  consumer_state: exact\n"+
		"normative_outputs:\n"+
		"  - expected/exit-code.txt\n"+
		"  - expected/consumer\n")
	writeFile(t, filepath.Join(exampleDir, "source", "talby-source.yaml"), ""+
		"schema_version: 1\n"+
		"source:\n"+
		"  name: example\n"+
		"artifacts: []\n")
	writeFile(t, filepath.Join(exampleDir, "expected", "exit-code.txt"), "0\n")

	_, err := Discover(root)
	if err == nil {
		t.Fatal("Discover() error = nil, want mismatch error")
	}

	if got := err.Error(); got == "" || !containsAll(got,
		"bad-example",
		"expected/stdout-contains.yaml",
	) {
		t.Fatalf("error = %q, want missing stdout-contains reference", got)
	}
}

func TestDiscoverRejectsInvalidVerificationValue(t *testing.T) {
	root := t.TempDir()
	exampleDir := filepath.Join(root, "atomic-cases", "bad-verification")

	mkdirAll(t, filepath.Join(root, "scenarios"))
	mkdirAll(t, filepath.Join(exampleDir, "source"))
	mkdirAll(t, filepath.Join(exampleDir, "consumer"))
	mkdirAll(t, filepath.Join(exampleDir, "expected"))

	writeFile(t, filepath.Join(root, "README.md"), "# Examples\n")
	writeFile(t, filepath.Join(exampleDir, "README.md"), "# Bad Verification\n")
	writeFile(t, filepath.Join(exampleDir, "example.yaml"), ""+
		"schema_version: 1\n"+
		"id: bad-verification\n"+
		"kind: atomic-case\n"+
		"status: active\n"+
		"polarity: positive\n"+
		"summary: Invalid verification value.\n"+
		"commands:\n"+
		"  - argv:\n"+
		"      - tbboot\n"+
		"      - install\n"+
		"verification:\n"+
		"  exit_code: exact\n"+
		"  stdout_text: contians\n"+
		"  stdout_json: absent\n"+
		"  stderr_text: absent\n"+
		"  stderr_json: absent\n"+
		"  consumer_state: absent\n"+
		"normative_outputs:\n"+
		"  - expected/exit-code.txt\n")
	writeFile(t, filepath.Join(exampleDir, "source", "talby-source.yaml"), ""+
		"schema_version: 1\n"+
		"source:\n"+
		"  name: example\n"+
		"artifacts: []\n")
	writeFile(t, filepath.Join(exampleDir, "expected", "exit-code.txt"), "0\n")

	_, err := Discover(root)
	if err == nil {
		t.Fatal("Discover() error = nil, want invalid verification error")
	}

	if got := err.Error(); got == "" || !containsAll(got,
		"bad-verification",
		"verification.stdout_text",
		"contians",
	) {
		t.Fatalf("error = %q, want invalid stdout_text rejection", got)
	}
}

func TestDiscoverRejectsSourceDescriptorTypeField(t *testing.T) {
	root := t.TempDir()
	exampleDir := filepath.Join(root, "atomic-cases", "bad-source-type")

	mkdirAll(t, filepath.Join(root, "scenarios"))
	mkdirAll(t, filepath.Join(exampleDir, "source"))
	mkdirAll(t, filepath.Join(exampleDir, "consumer"))
	mkdirAll(t, filepath.Join(exampleDir, "expected"))

	writeFile(t, filepath.Join(root, "README.md"), "# Examples\n")
	writeFile(t, filepath.Join(exampleDir, "README.md"), "# Bad Source Type\n")
	writeFile(t, filepath.Join(exampleDir, "example.yaml"), ""+
		"schema_version: 1\n"+
		"id: bad-source-type\n"+
		"kind: atomic-case\n"+
		"status: active\n"+
		"polarity: positive\n"+
		"summary: Source descriptor incorrectly declares source.type.\n"+
		"commands:\n"+
		"  - argv:\n"+
		"      - tbboot\n"+
		"      - install\n"+
		"      - file:example\n"+
		"verification:\n"+
		"  exit_code: exact\n"+
		"  stdout_text: absent\n"+
		"  stdout_json: absent\n"+
		"  stderr_text: absent\n"+
		"  stderr_json: absent\n"+
		"  consumer_state: absent\n"+
		"normative_outputs:\n"+
		"  - expected/exit-code.txt\n")
	writeFile(t, filepath.Join(exampleDir, "source", "talby-source.yaml"), ""+
		"schema_version: 1\n"+
		"source:\n"+
		"  name: example\n"+
		"  type: file\n"+
		"artifacts: []\n")
	writeFile(t, filepath.Join(exampleDir, "expected", "exit-code.txt"), "0\n")

	_, err := Discover(root)
	if err == nil {
		t.Fatal("Discover() error = nil, want source descriptor type error")
	}

	if got := err.Error(); got == "" || !containsAll(got,
		"bad-source-type",
		"source.type",
		"talby-source.yaml",
	) {
		t.Fatalf("error = %q, want source.type rejection", got)
	}
}

func TestDiscoverRejectsMissingStatus(t *testing.T) {
	root := t.TempDir()
	exampleDir := filepath.Join(root, "atomic-cases", "missing-status")
	writeMinimalExample(t, root, exampleDir, ""+
		"schema_version: 1\n"+
		"id: missing-status\n"+
		"kind: atomic-case\n"+
		"polarity: positive\n"+
		"summary: Missing status.\n"+
		"commands:\n"+
		"  - argv:\n"+
		"      - tbboot\n"+
		"      - install\n"+
		"verification:\n"+
		"  exit_code: exact\n"+
		"  stdout_text: absent\n"+
		"  stdout_json: absent\n"+
		"  stderr_text: absent\n"+
		"  stderr_json: absent\n"+
		"  consumer_state: absent\n"+
		"normative_outputs:\n"+
		"  - expected/exit-code.txt\n")

	_, err := Discover(root)
	if err == nil {
		t.Fatal("Discover() error = nil, want missing status error")
	}
	if got := err.Error(); got == "" || !containsAll(got, "missing-status", "status") {
		t.Fatalf("error = %q, want status rejection", got)
	}
}

func TestDiscoverRejectsInvalidStatus(t *testing.T) {
	root := t.TempDir()
	exampleDir := filepath.Join(root, "atomic-cases", "bad-status")
	writeMinimalExample(t, root, exampleDir, ""+
		"schema_version: 1\n"+
		"id: bad-status\n"+
		"kind: atomic-case\n"+
		"status: flaky\n"+
		"polarity: positive\n"+
		"summary: Invalid status.\n"+
		"commands:\n"+
		"  - argv:\n"+
		"      - tbboot\n"+
		"      - install\n"+
		"verification:\n"+
		"  exit_code: exact\n"+
		"  stdout_text: absent\n"+
		"  stdout_json: absent\n"+
		"  stderr_text: absent\n"+
		"  stderr_json: absent\n"+
		"  consumer_state: absent\n"+
		"normative_outputs:\n"+
		"  - expected/exit-code.txt\n")

	_, err := Discover(root)
	if err == nil {
		t.Fatal("Discover() error = nil, want invalid status error")
	}
	if got := err.Error(); got == "" || !containsAll(got, "bad-status", "flaky", "active, broken, skipped, or deprecated") {
		t.Fatalf("error = %q, want invalid status rejection", got)
	}
}

func TestDiscoverRequiresStderrExpectedFiles(t *testing.T) {
	root := t.TempDir()
	exampleDir := filepath.Join(root, "atomic-cases", "stderr-contract")
	writeMinimalExample(t, root, exampleDir, ""+
		"schema_version: 1\n"+
		"id: stderr-contract\n"+
		"kind: atomic-case\n"+
		"status: active\n"+
		"polarity: negative\n"+
		"summary: Stderr contract.\n"+
		"commands:\n"+
		"  - argv:\n"+
		"      - tbboot\n"+
		"      - install\n"+
		"verification:\n"+
		"  exit_code: exact\n"+
		"  stdout_text: absent\n"+
		"  stdout_json: absent\n"+
		"  stderr_text: contains\n"+
		"  stderr_json: exact\n"+
		"  consumer_state: absent\n"+
		"normative_outputs:\n"+
		"  - expected/exit-code.txt\n")

	_, err := Discover(root)
	if err == nil {
		t.Fatal("Discover() error = nil, want missing stderr files")
	}
	if got := err.Error(); got == "" || !containsAll(got, "stderr-contract", "expected/stderr-contains.yaml") {
		t.Fatalf("error = %q, want missing stderr contains reference", got)
	}

	writeFile(t, filepath.Join(exampleDir, "expected", "stderr-contains.yaml"), "fragments:\n  - failure\n")
	_, err = Discover(root)
	if err == nil {
		t.Fatal("Discover() error = nil, want missing stderr json file")
	}
	if got := err.Error(); got == "" || !containsAll(got, "stderr-contract", "expected/stderr.json") {
		t.Fatalf("error = %q, want missing stderr json reference", got)
	}
}

func TestExampleStatusHelpers(t *testing.T) {
	cases := []struct {
		status      string
		shouldRun   bool
		expectsPass bool
	}{
		{status: "active", shouldRun: true, expectsPass: true},
		{status: "broken", shouldRun: true, expectsPass: false},
		{status: "skipped", shouldRun: false, expectsPass: false},
		{status: "deprecated", shouldRun: false, expectsPass: false},
	}
	for _, tc := range cases {
		example := Example{Metadata: Metadata{Status: tc.status}}
		if got := example.ShouldRun(); got != tc.shouldRun {
			t.Fatalf("%s ShouldRun() = %v, want %v", tc.status, got, tc.shouldRun)
		}
		if got := example.ExpectsPass(); got != tc.expectsPass {
			t.Fatalf("%s ExpectsPass() = %v, want %v", tc.status, got, tc.expectsPass)
		}
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writeMinimalExample(t *testing.T, root string, exampleDir string, metadata string) {
	t.Helper()
	mkdirAll(t, filepath.Join(root, "scenarios"))
	mkdirAll(t, filepath.Join(root, "atomic-cases"))
	mkdirAll(t, filepath.Join(exampleDir, "source"))
	mkdirAll(t, filepath.Join(exampleDir, "consumer"))
	mkdirAll(t, filepath.Join(exampleDir, "expected"))
	writeFile(t, filepath.Join(root, "README.md"), "# Examples\n")
	writeFile(t, filepath.Join(exampleDir, "README.md"), "# Example\n")
	writeFile(t, filepath.Join(exampleDir, "example.yaml"), metadata)
	writeFile(t, filepath.Join(exampleDir, "source", "talby-source.yaml"), ""+
		"schema_version: 1\n"+
		"source:\n"+
		"  name: example\n"+
		"artifacts: []\n")
	writeFile(t, filepath.Join(exampleDir, "expected", "exit-code.txt"), "0\n")
}

func containsAll(got string, wants ...string) bool {
	for _, want := range wants {
		if !strings.Contains(got, want) {
			return false
		}
	}
	return true
}
