# Example E2E Runner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `testdata/examples/` executable through one CLI-side Go test runner while allowing unfinished examples to stay visible as `broken`, `skipped`, or `deprecated`.

**Architecture:** Keep example schema parsing in `internal/examples` and keep command execution in `cmd/tbboot` through the existing in-process `execute(...)` test path. The runner stages each example in a temp workspace, initializes a Git repo there with the same helper pattern already used by CLI tests, copies the example source under the exact alias referenced by the example, rewrites only `file:<alias>` argv tokens to staged source directories, runs active and broken examples, and applies metadata-driven verification for exit code, stdout, stderr, JSON, and consumer state.

**Tech Stack:** Go 1.26.4, Cobra, `gopkg.in/yaml.v3`, stdlib `encoding/json`, stdlib filesystem helpers, `go test`, `just`.

## Global Constraints

- Do not add process-level binary execution; use the existing in-process `execute(context.Context, args, stdout, stderr)` path.
- Do not add argv templating; only replace the leading `tbboot` token and `file:<alias>` tokens.
- Do not add verification surfaces beyond exit code, stdout text, stdout JSON, stderr text, stderr JSON, and consumer state.
- Reuse the existing `initGitRepo(...)` helper pattern so staged workspaces match current CLI test preconditions.
- Support only the verification modes used by the current example library in this slice: `exit_code: exact`, text `exact|contains|absent`, JSON `exact|absent`, and `consumer_state: exact|absent`.
- Do not delete existing ad hoc CLI tests in this slice.
- Do not create commits from an agent. `AGENTS.md` requires explicit approval at commit time.

---

## File Structure

- Modify: `internal/examples/examples.go` - add `status`, stderr verification fields, validation, expected-file requirements, and status helper methods.
- Modify: `internal/examples/examples_test.go` - cover required status, invalid status, stderr validation, stderr expected-file requirements, and helper behavior.
- Modify: every `testdata/examples/**/example.yaml` - add `status`, `stderr_text`, and `stderr_json`.
- Modify/Create: expected files under `testdata/examples/**/expected/` only where metadata says stderr verification is not `absent`.
- Create: `cmd/tbboot/examples_e2e_test.go` - discover examples, stage workspaces, normalize argv, execute commands, verify outputs, and invert verification result for `broken`.

No new package is needed. Keep runner-only helpers unexported in `cmd/tbboot/examples_e2e_test.go`; move them only if a second package needs them later.

## Tasks

### Task 1: Extend example metadata with status and stderr policy

**Files:**

- Modify: `internal/examples/examples.go`
- Modify: `internal/examples/examples_test.go`

**Interfaces:**

- Produces: `Metadata.Status string`
- Produces: `Verification.StderrText string`
- Produces: `Verification.StderrJSON string`
- Produces: `func (Example) ShouldRun() bool`
- Produces: `func (Example) ExpectsPass() bool`

- [ ] **Step 1: Write failing metadata tests**

Add these tests to `internal/examples/examples_test.go` before the helper functions:

```go
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
```

Add this helper near `writeFile`:

```go
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
```

- [ ] **Step 2: Run metadata tests to verify failure**

Run:

```bash
go test ./internal/examples -run 'TestDiscoverRejectsMissingStatus|TestDiscoverRejectsInvalidStatus|TestDiscoverRequiresStderrExpectedFiles|TestExampleStatusHelpers' -v
```

Expected: build failure or test failure because `Status`, `StderrText`, `StderrJSON`, `ShouldRun`, and `ExpectsPass` do not exist yet.

- [ ] **Step 3: Add fields and helpers**

Change `Metadata`, `Verification`, and add helpers in `internal/examples/examples.go`:

```go
type Metadata struct {
    SchemaVersion   int          `yaml:"schema_version"`
    ID              string       `yaml:"id"`
    Kind            string       `yaml:"kind"`
    Status          string       `yaml:"status"`
    Polarity        string       `yaml:"polarity"`
    Summary         string       `yaml:"summary"`
    Commands        []Command    `yaml:"commands"`
    Verification    Verification `yaml:"verification"`
    NormativeOutput []string     `yaml:"normative_outputs"`
    Tags            []string     `yaml:"tags"`
}

type Verification struct {
    ExitCode      string `yaml:"exit_code"`
    StdoutText    string `yaml:"stdout_text"`
    StdoutJSON    string `yaml:"stdout_json"`
    StderrText    string `yaml:"stderr_text"`
    StderrJSON    string `yaml:"stderr_json"`
    ConsumerState string `yaml:"consumer_state"`
}

func (e Example) ShouldRun() bool {
    return e.Metadata.Status == "active" || e.Metadata.Status == "broken"
}

func (e Example) ExpectsPass() bool {
    return e.Metadata.Status == "active"
}
```

In `validateMetadata`, add status validation after the kind check:

```go
if !isOneOf(meta.Status, "active", "broken", "skipped", "deprecated") {
    return fmt.Errorf("%s: status = %q, want active, broken, skipped, or deprecated", meta.ID, meta.Status)
}
```

In `validateVerification`, add stderr checks:

```go
if !isOneOf(verification.StderrText, "exact", "contains", "absent") {
    return fmt.Errorf("%s: verification.stderr_text = %q, want exact, contains, or absent", exampleID, verification.StderrText)
}
if !isOneOf(verification.StderrJSON, "exact", "contains", "absent") {
    return fmt.Errorf("%s: verification.stderr_json = %q, want exact, contains, or absent", exampleID, verification.StderrJSON)
}
```

In `expectedOutputsForVerification`, add stderr expected files:

```go
switch v.StderrText {
case "exact":
    outputs = append(outputs, "expected/stderr.txt")
case "contains":
    outputs = append(outputs, "expected/stderr-contains.yaml")
}

switch v.StderrJSON {
case "exact":
    outputs = append(outputs, "expected/stderr.json")
case "contains":
    outputs = append(outputs, "expected/stderr-json-contains.yaml")
}
```

- [ ] **Step 4: Run metadata tests**

Run:

```bash
go test ./internal/examples
```

Expected: failure because the real example YAML files do not have `status`, `stderr_text`, and `stderr_json` yet.

### Task 2: Update current example metadata to match current CLI behavior

**Files:**

- Modify: `testdata/examples/scenarios/file-direct-install-multi-artifact/example.yaml`
- Modify: `testdata/examples/scenarios/declare-only-flow/example.yaml`
- Modify: `testdata/examples/atomic-cases/ambiguous-install-target-rejected/example.yaml`
- Modify: `testdata/examples/atomic-cases/declare-only-manifest-only/example.yaml`
- Modify: `testdata/examples/atomic-cases/file-direct-install-single-artifact/example.yaml`
- Modify: `testdata/examples/atomic-cases/json-success-envelope-minimal/example.yaml`
- Modify: `testdata/examples/atomic-cases/non-interactive-prompt-required/example.yaml`
- Modify: `testdata/examples/atomic-cases/ownership-conflict-overlapping-file/example.yaml`
- Modify: `testdata/examples/atomic-cases/trust-policy-denied-git-source/example.yaml`

**Interfaces:**

- Consumes: `status`, `stderr_text`, and `stderr_json` validation from Task 1.
- Produces: a loadable example library with explicit execution policy.

- [ ] **Step 1: Add statuses and stderr policies**

Update every current `example.yaml`, but do not hard-code values in the plan without checking the real CLI behavior first.

Use these rules:

- set `status` from current implementation reality, verified by the existing CLI tests and one runner dry run:
  - `active` only when the example already satisfies its declared contract today;
  - `broken` only when the example is intentionally kept as executable red state;
  - `skipped` or `deprecated` only when that is the explicit product intent.
- set `stderr_text` and `stderr_json` from the stream the CLI actually uses today:
  - success-path examples will usually keep `stderr_text: absent` and `stderr_json: absent`;
  - negative text-mode examples that currently emit user-facing failures on `stderr` must move their fragment files from `expected/stdout-contains.yaml` to `expected/stderr-contains.yaml` and set `stdout_text: absent`, `stderr_text: contains`;
  - negative JSON-mode examples, if any are added later, must verify the JSON error envelope on `stderr_json`, not `stdout_json`.
- leave `stdout_*` verification in place only for examples that actually write their contract to `stdout`.

At minimum, inspect and update these current negative examples to use the correct stream:

- `testdata/examples/atomic-cases/ambiguous-install-target-rejected/`
- `testdata/examples/atomic-cases/non-interactive-prompt-required/`
- `testdata/examples/atomic-cases/ownership-conflict-overlapping-file/`
- `testdata/examples/atomic-cases/trust-policy-denied-git-source/`

- [ ] **Step 2: Update JSON success expected output only if the current fixture is stale**

Compare `testdata/examples/atomic-cases/json-success-envelope-minimal/expected/stdout.json` against the current successful install envelope from the existing CLI test path. Update it only if it is stale.

```json
{
  "code": 0,
  "message": "install succeeded",
  "details": {
    "source": {
      "type": "file",
      "name": "local-example-source",
      "version": "local-snapshot-001"
    },
    "artifact": {
      "name": "base-readme",
      "version": "1.0.0",
      "path": "artifacts/base-readme"
    },
    "change": "installed",
    "files": [
      {
        "path": "README.md",
        "action": "created",
        "digest": "d12c5e04f7f2a896bcfe7d90a93f31308793d238fbc6c02ee00e17dc29b38506"
      }
    ]
  },
  "warnings": null
}
```

- [ ] **Step 3: Run loader tests**

Run:

```bash
go test ./internal/examples
```

Expected:

```text
ok      github.com/talby/talby-bootstrap/internal/examples
```

### Task 3: Add the E2E runner staging and execution skeleton

**Files:**

- Create: `cmd/tbboot/examples_e2e_test.go`

**Interfaces:**

- Consumes: `examples.Discover`, `Example.ShouldRun`, `Example.ExpectsPass`.
- Produces: `TestExamplesE2E`.
- Produces: `stageExample(t, example) string`.
- Produces: `normalizeExampleArgs(argv, workspace) ([]string, error)`.

- [ ] **Step 1: Add runner skeleton**

Create `cmd/tbboot/examples_e2e_test.go`:

```go
package tbboot

import (
    "bytes"
    "context"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/talby/talby-bootstrap/internal/examples"
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
    copyTree(t, filepath.Join(example.Path, "source"), filepath.Join(workspace, ".tbboot-example", "sources", sourceAlias))
    return workspace
}

func readExampleSourceAlias(t *testing.T, example examples.Example) string {
    t.Helper()

    // Read source/talby-source.yaml and return source.name.
    // This keeps staged aliases aligned with the example's real descriptor.
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
            args[i] = "file:" + filepath.Join(workspace, ".tbboot-example", "sources", alias)
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
```

- [ ] **Step 2: Add a temporary verification stub**

Add this stub at the end of `cmd/tbboot/examples_e2e_test.go` so the file builds until Task 4:

```go
func verifyExample(example examples.Example, workspace string, result commandResult) error {
    return nil
}
```

- [ ] **Step 3: Run the skeleton**

Run:

```bash
go test ./cmd/tbboot -run TestExamplesE2E -v
```

Expected: failure from a `broken example satisfied its contract` message because verification is still stubbed. This confirms status inversion is wired after staging and repo initialization are correct.

### Task 4: Implement metadata-driven verification

**Files:**

- Modify: `cmd/tbboot/examples_e2e_test.go`

**Interfaces:**

- Consumes: `commandResult`.
- Produces: `verifyExample(example, workspace, result) error`.

- [ ] **Step 1: Replace imports**

Add these imports to `cmd/tbboot/examples_e2e_test.go`:

```go
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

    "gopkg.in/yaml.v3"

    "github.com/talby/talby-bootstrap/internal/examples"
)
```

- [ ] **Step 2: Replace the verification stub**

Replace `verifyExample` with:

```go
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
```

- [ ] **Step 3: Add consumer-state comparison helpers**

Append:

```go
func verifyConsumerState(example examples.Example, workspace string) error {
    wantRoot := filepath.Join(example.Path, "expected", "consumer")
    want, err := snapshotTree(wantRoot, nil)
    if err != nil {
        return err
    }
    got, err := snapshotTree(workspace, map[string]struct{}{
        ".tbboot-example": {},
    })
    if err != nil {
        return err
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
```

- [ ] **Step 4: Run the E2E runner**

Run:

```bash
go test ./cmd/tbboot -run TestExamplesE2E -v
```

Expected:

```text
PASS
```

The expected pass includes active examples satisfying their contract and broken examples failing their declared contract. If a new example starts using `exit_code: class` or JSON `contains`, keep that out of this slice and add it in a follow-up change with dedicated coverage.

### Task 5: Final verification and cleanup

**Files:**

- Modify: `cmd/tbboot/examples_e2e_test.go`
- Modify: `internal/examples/examples.go`
- Modify: `internal/examples/examples_test.go`
- Modify: `testdata/examples/**/example.yaml`
- Modify: `testdata/examples/atomic-cases/json-success-envelope-minimal/expected/stdout.json`

**Interfaces:**

- Consumes: all previous tasks.
- Produces: repository checks passing.

- [ ] **Step 1: Format Go code**

Run:

```bash
gofmt -w internal/examples/examples.go internal/examples/examples_test.go cmd/tbboot/examples_e2e_test.go
```

Expected: command exits 0.

- [ ] **Step 2: Run Go checks**

Run:

```bash
go test ./...
```

Expected:

```text
ok      github.com/talby/talby-bootstrap/cmd/tbboot
ok      github.com/talby/talby-bootstrap/internal/app
ok      github.com/talby/talby-bootstrap/internal/examples
ok      github.com/talby/talby-bootstrap/internal/install
ok      github.com/talby/talby-bootstrap/internal/materialize
ok      github.com/talby/talby-bootstrap/internal/repositorystate
ok      github.com/talby/talby-bootstrap/internal/source
ok      github.com/talby/talby-bootstrap/internal/source/file
```

- [ ] **Step 3: Run repository checks**

Run:

```bash
just check
```

Expected: Markdown and Go checks pass.

- [ ] **Step 4: Review intentional skips**

Run:

```bash
go test ./cmd/tbboot -run TestExamplesE2E -v
```

Expected: no `skipped` or `deprecated` examples yet unless a later edit intentionally introduced one. Broken examples should appear as passing subtests, not skipped subtests.

## Self-Review

- Spec coverage: status schema, stderr schema, active/broken/skipped/deprecated behavior, staging, file alias normalization, in-process execution, and modeled verification surfaces are covered.
- Deferred by design: exact failure-mode assertions for `broken`, argv templating, shell-out binary execution, extra verification surfaces, and deletion of old CLI tests.
- Type consistency: Task 1 defines the metadata fields and helpers used by Tasks 3 and 4.
