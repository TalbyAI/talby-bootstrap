# Phase 1 final review fixes implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the four accepted Phase 1 correctness findings and remove the deferred requested-version contract while rejecting unknown Manifest fields.

**Architecture:** Keep fixes at existing ownership seams: Sync owns complete-operation preflight, the `file:` Source owns snapshot identity, repository state owns persisted-path validation, and materialization owns write-time race classification. Reuse existing types and helpers; add no packages, interfaces, or dependencies.

**Tech Stack:** Go 1.26.4, Cobra 1.10.2, `gopkg.in/yaml.v3`, Go standard library, Markdown, `just`.

## Global constraints

- Preserve slash-normalized persisted consumer paths and platform-aware comparison keys.
- Preserve exact replay and trust-before-resolution behavior.
- Keep `file:` as the only Phase 1 Source and do not add requested-version support.
- Reject unknown YAML fields instead of silently discarding them.
- Use one focused regression per finding and run it red before production changes.
- Do not add a planner, reconciliation package, dependency, or migration.
- Do not commit without fresh explicit user approval.

---

### Task 1: Protect every active Source input

**Files:**

- Modify: `internal/install/sync.go`
- Test: `internal/install/sync_test.go`

**Interfaces:**

- Consumes: `desiredArtifact.InputPaths []string` containing canonical absolute Source inputs.
- Produces: `preflightFiles` rejects any desired consumer target matching any active input path.

- [ ] **Step 1: Write the failing two-Source regression**

Create two `desiredArtifact` values whose Sources differ. Give Source A a target equal to one canonical `InputPaths` entry from Source B. Call `preflightFiles` and require an error containing `overlaps source input`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/install -run TestPreflightRejectsTargetOverlappingAnotherSourceInput -count=1`

Expected: FAIL because `preflightFiles` checks only `artifact.InputPaths`.

- [ ] **Step 3: Implement one complete-operation input set**

At the start of `preflightFiles`, build `activeInputs := map[string]struct{}{}` from every desired Artifact input using `materialize.PathKey(input)`. Replace the per-Artifact input loop with one lookup by the observed target key.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/install -run 'TestPreflightRejectsTargetOverlappingAnotherSourceInput|TestSync' -count=1`

Expected: PASS.

### Task 2: Frame local snapshot hashing

**Files:**

- Modify: `internal/source/file/source.go`
- Test: `internal/source/file/source_test.go`

**Interfaces:**

- Consumes: every descriptor field and payload currently written to the snapshot hash.
- Produces: an unambiguous length-prefixed byte stream using a small local `writeSnapshotField(hash.Hash, []byte)` helper.

- [ ] **Step 1: Write the failing collision regression**

Resolve the same two-step Source twice. Use payloads `x` and `b\x00z` first, then `xb\x00` and `z`, with the second target path `b`. Require different resolved Source versions.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/source/file -run TestResolveFramesSnapshotPayloads -count=1`

Expected: FAIL because both states currently hash the same concatenated byte stream.

- [ ] **Step 3: Add minimal framing**

Write each logical hash field as an unsigned 64-bit big-endian length followed by its bytes. Route `sourceBytes`, Artifact name/path/descriptor bytes, file-step target path, and file payload through the helper.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/source/file -count=1`

Expected: PASS.

### Task 3: Canonicalize persisted ownership paths on every platform

**Files:**

- Modify: `internal/repositorystate/materialization_record.go`
- Modify: `internal/install/sync.go`
- Test: `internal/repositorystate/materialization_record_test.go`
- Test: `internal/install/service_windows_test.go`

**Interfaces:**

- Consumes: slash-normalized `ManagedFileRecord.Path` values.
- Produces: validation accepts only `filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))) == path`; uniqueness and ownership use `materialize.PathKey(filepath.FromSlash(path))` where materialization context is available.

- [ ] **Step 1: Write failing nested-path and alias regressions**

Add a platform-independent validation test for slash-normalized nested paths. Add Windows-only tests requiring `dir/file` validation and case-insensitive ownership conflict detection.

- [ ] **Step 2: Verify RED**

Run on the current platform: `go test ./internal/repositorystate -run TestValidateMaterializationRecordAcceptsSlashNormalizedNestedPath -count=1`

Compile Windows tests: `GOOS=windows go test ./internal/install ./internal/repositorystate -run '^$' -exec=true`

Expected: the validation regression exposes the current platform-normalization mismatch when executed on Windows; the ownership regression fails on Windows before the fix.

- [ ] **Step 3: Implement platform-aware persisted-path handling**

Validate persisted paths after `filepath.FromSlash`, retaining slash-normalized storage. In `preflightFiles`, key the owner map and recorded-file lookup with `materialize.PathKey(filepath.FromSlash(path))`; continue emitting `Observation.Path` in persisted results.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/repositorystate ./internal/install -count=1`

Compile Windows tests: `GOOS=windows go test ./internal/install ./internal/repositorystate -run '^$' -exec=true`

Expected: PASS.

### Task 4: Classify parent-topology write races as drift

**Files:**

- Modify: `internal/materialize/service.go`
- Test: `internal/materialize/service_test.go`
- Test: `internal/install/sync_test.go`

**Interfaces:**

- Consumes: target-parent symlink or non-directory detected during `Write` re-observation.
- Produces: `ChangedSincePreflightError`, already mapped by `persistPrepared` to drift exit class `2`.

- [ ] **Step 1: Write the failing parent-race regression**

Observe an absent nested target, replace its parent directory with a symlink after preflight, call `Write`, and require `errors.As(err, *ChangedSincePreflightError)`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/materialize -run TestWriteClassifiesParentTopologyRaceAsChanged -count=1`

Expected: FAIL with generic `target parent must be a real directory`.

- [ ] **Step 3: Map only revalidation topology failures**

Introduce a private typed parent-topology error from `Observe` and one exported `Revalidate(Observation) error` seam that maps topology or target changes to `ChangedSincePreflightError`. Reuse it from `Write` and adoption revalidation; leave initial preflight and unrelated IO errors unchanged.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/materialize ./internal/install -count=1`

Expected: PASS.

### Task 5: Remove requested-version reservation and reject unknown YAML

**Files:**

- Modify: `docs/superpowers/specs/2026-07-14-phase-1-install-sync-design.md`
- Modify: `internal/repositorystate/store.go`
- Test: `internal/repositorystate/store_test.go`

**Interfaces:**

- Consumes: Manifest, Lockfile, and Materialization Record YAML.
- Produces: decoding with `yaml.Decoder.KnownFields(true)`; unknown fields, including `declarations[].input.version`, return invalid-format state errors.

- [ ] **Step 1: Write failing unknown-field regressions**

Add focused load tests for an unknown top-level Manifest field and `input.version`. Require `StateFileError{File: StateFileManifest, Kind: StateFileErrorInvalidFormat}`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/repositorystate -run 'TestLoadManifestRejectsUnknownField|TestLoadManifestRejectsInputVersion' -count=1`

Expected: FAIL because `yaml.Unmarshal` ignores unknown fields.

- [ ] **Step 3: Decode repository state strictly**

Replace repository-state `yaml.Unmarshal` calls with a small generic decode helper using `yaml.NewDecoder(bytes.NewReader(data))` and `KnownFields(true)`. Remove requested-version reservation, validation, Sync, and test requirements from the approved design; state that Phase 1 Manifest input contains only optional locator metadata.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/repositorystate -count=1 && just check-md`

Expected: PASS.

### Task 6: Final verification

**Files:**

- Verify all modified Go and Markdown files.

**Interfaces:**

- Consumes: Tasks 1-5.
- Produces: formatted, race-tested branch with no whitespace errors.

- [ ] **Step 1: Format**

Run: `just fmt-go`

- [ ] **Step 2: Run repository checks**

Run: `just check`

Expected: PASS.

- [ ] **Step 3: Run race detector**

Run: `go test -race ./... -count=1`

Expected: PASS.

- [ ] **Step 4: Inspect final diff**

Run: `git diff --check && git status --short && git diff --stat`

Expected: no whitespace errors and only intended files changed. Do not commit without fresh explicit approval.
