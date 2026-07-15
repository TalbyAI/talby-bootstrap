# Go coverage implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Raise total Go statement coverage to at least 90% while preserving the existing 80% minimum gate.

**Architecture:** Exercise existing behavior through package-local tests. Start with public mutations and persistence round trips in `internal/repositorystate`, then cover filesystem behavior in `internal/materialize`; use the coverage report to select only enough remaining error paths to cross 90%.

**Tech Stack:** Go standard `testing` package, `go test`, `go tool cover`, and existing `just` tasks.

## Global constraints

- Do not change production behavior.
- Do not add dependencies.
- Keep `just check-coverage` at 80%.
- Do not create commits without fresh explicit user approval.

---

### Task 1: Repository-state domain behavior

**Files:**

- Modify: `internal/repositorystate/lockfile_test.go`
- Modify: `internal/repositorystate/manifest_test.go`
- Modify: `internal/repositorystate/materialization_record_test.go`

**Interfaces:**

- Consumes: `Lockfile.Snapshot`, `Lockfile.KeepArtifacts`, `Manifest.AddDeclaration`, `AcquisitionLocator`, `UpsertManagedArtifact`.
- Produces: Tests for successful lookups, insertion/replacement/no-change results, stable sorting, filtering, and validation errors.

- [ ] **Step 1: Add focused tests**

```go
func TestLockfileSnapshotAndKeepArtifacts(t *testing.T) {
	source := SourceIdentity{Type: SourceTypeFile, Locator: "source"}
	lock := Lockfile{Resolutions: []Resolution{{Source: source, ResolvedVersion: "v1", Artifacts: []ArtifactResolution{{Name: "b", Version: "1"}, {Name: "a", Version: "1"}}}}}
	if _, ok := lock.Snapshot(SnapshotKey{Source: source, ResolvedVersion: "v1"}); !ok { t.Fatal("snapshot not found") }
	kept, removed := lock.KeepArtifacts(map[ArtifactKey]struct{}{{Source: source, Name: "a"}: {}})
	if len(kept.Resolutions) != 1 || len(kept.Resolutions[0].Artifacts) != 1 || len(removed) != 1 { t.Fatalf("kept=%#v removed=%#v", kept, removed) }
}
```

Add equivalent compact tests for declaration insertion/conflict and managed-artifact insertion/replacement, asserting returned `ChangeKind` and sorted values.

- [ ] **Step 2: Run package tests**

Run: `go test ./internal/repositorystate`

Expected: PASS.

---

### Task 2: Repository-state persistence

**Files:**

- Modify: `internal/repositorystate/store_test.go`

**Interfaces:**

- Consumes: `NewStore`, all three `Write*` and `Load*` method pairs, `StateFileError`.
- Produces: Round-trip coverage for Manifest, Lockfile, and Materialization Record plus missing, empty, malformed, and unsupported-schema errors.

- [ ] **Step 1: Add round-trip tests**

```go
func TestStoreRoundTripsRepositoryState(t *testing.T) {
	ctx, root, store := context.Background(), t.TempDir(), NewStore()
	// Build one valid value of each state type, write it, load it, and compare with reflect.DeepEqual.
	// Use a relative file Source locator rooted under root and a 64-character lowercase digest.
}
```

Use separate subtests for each state file so a failure identifies the broken format. Add malformed YAML and `schema_version: 2` cases and assert `errors.As(err, &StateFileError{})`.

- [ ] **Step 2: Run package tests with coverage**

Run: `go test -cover ./internal/repositorystate`

Expected: PASS and package coverage materially above 62.8%.

---

### Task 3: Materialization filesystem behavior

**Files:**

- Modify: `internal/materialize/service_test.go`

**Interfaces:**

- Consumes: `PathKey`, `Revalidate`, `Write`, `Digest`, and error `Error()` methods.
- Produces: Tests for canonical keys, successful and failed revalidation, digest output, invalid observations, and write rejection for non-regular targets.

- [ ] **Step 1: Add behavior tests**

```go
func TestDigestAndRevalidate(t *testing.T) {
	root := t.TempDir()
	observed, err := Observe(root, "file")
	if err != nil { t.Fatal(err) }
	if Digest([]byte("content")) == "" { t.Fatal("empty digest") }
	if err := Revalidate(observed); err != nil { t.Fatal(err) }
}
```

Add one test where the file changes after observation and one where `Write` receives a symlink or directory observation; assert concrete error types with `errors.As`.

- [ ] **Step 2: Run package tests with coverage**

Run: `go test -cover ./internal/materialize`

Expected: PASS and package coverage materially above 71.4%.

---

### Task 4: Close remaining meaningful gaps

**Files:**

- Modify only the existing test file corresponding to the largest uncovered function reported by `go tool cover -func=coverage.out`.

**Interfaces:**

- Consumes: Existing package APIs shown as uncovered by the fresh report.
- Produces: Tests for reachable user-visible success or error behavior; no tests for impossible OS failures or trivial entrypoint wrappers.

- [ ] **Step 1: Measure total coverage**

Run: `just coverage`

Expected: Current total and per-function gaps printed.

- [ ] **Step 2: Add the smallest focused test for the largest reachable gap**

Use existing test helpers and package-local fakes. Assert outputs, errors, filesystem state, or exit behavior; never assert only that a line executed.

- [ ] **Step 3: Repeat measurement and one focused test until total reaches 90%**

Run after each test file: `just coverage`

Expected: `total: ... 90.0%` or greater.

- [ ] **Step 4: Run final gates**

Run: `just check-coverage && just check-go`

Expected: Both commands exit 0; total coverage is at least 90%.
