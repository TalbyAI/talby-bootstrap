# 0.1 Contract Reset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the undeployed pre-0.1 persistence and descriptor shapes with strict schema-version-1 YAML contracts, canonical `tbboot-` filenames, scalar Source References, and a working minimal in-root `file:` install.

**Architecture:** Keep `SourceIdentity{Type, Locator}` as the internal dispatch and normalization type, but parse and format scalar `file:`/`git:` references only at YAML and CLI boundaries. Centralize strict YAML syntax checks in `internal/repositorystate`, keep semantic validation beside each domain model, and keep the existing install/materialization flow with the smallest adapter changes. Add Recovery State persistence without implementing rollback lifecycle behavior.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, standard-library path/crypto/encoding helpers, existing Cobra CLI and Go tests.

## Global Constraints

- Every document uses integer `schema_version: 1`.
- Readers reject unknown fields, duplicate keys, explicit nulls, multiple documents, aliases, anchors, merge keys, custom tags, and non-scalar keys.
- Writers emit UTF-8 without BOM, LF endings, two-space indentation, final newline, omitted empty optional collections, and deterministic ordering.
- Canonical filenames use the `tbboot-` prefix.
- Persisted Source References are scalar `file:<locator>` or `git:<locator>` values; no migration from old `{type, locator}` values exists.
- Product 0.1 retains only whole-file `file` steps in this slice.
- Git references are stored and validated but not acquired until ticket #35.
- Do not add dependencies or create commits without explicit user approval.

## File Map

- Modify `internal/repositorystate/model.go`: add Source Reference, Git commit, and Recovery State domain fields while preserving internal structured identities.
- Create `internal/repositorystate/yaml.go`: shared strict YAML syntax validation and deterministic encoding helpers.
- Create `internal/repositorystate/source_reference.go`: scalar Source Reference parse/format and canonical normalization helpers.
- Modify `internal/repositorystate/manifest.go`, `lockfile.go`, and `materialization_record.go`: validate scalar-reference-compatible domain contracts, source-specific versions, and deterministic sort keys.
- Modify `internal/repositorystate/store.go`: use canonical filenames, strict decoder, scalar DTOs, and six document read/write seams.
- Create `internal/repositorystate/recovery.go`: Recovery State semantic validation and lookup helpers.
- Modify `internal/source/file/source.go`: read `tbboot-*` descriptors, remove published source name, and enforce strict descriptor/path/version rules.
- Modify `internal/install/service.go`, `repository_state.go`, `sync.go`, `cmd/tbboot/install.go`, and `cmd/tbboot/root.go`: adapt declaration shape and canonical references, remove successful placeholders, and preserve minimal file install.
- Modify `internal/repositorystate/*_test.go`, `internal/source/file/source_test.go`, `internal/install/*_test.go`, and `cmd/tbboot/*_test.go`: test public seams and final contract.
- Modify active product docs and test fixtures under `CONTEXT.md`, `ARCHITECTURE.md`, `UBIQUITOUS_LANGUAGE.md`, `docs/adr/`, and `testdata/examples/`; leave historical research/spec/plan artifacts unchanged.

### Task 1: Add strict YAML and scalar Source Reference seams

**Files:**

- Create: `internal/repositorystate/yaml.go`
- Create: `internal/repositorystate/source_reference.go`
- Test: `internal/repositorystate/yaml_test.go`
- Test: `internal/repositorystate/source_reference_test.go`

**Interfaces:**

- `decodeStrictYAML(data []byte, value any) error` validates YAML syntax before decoding with `KnownFields(true)` and rejects unsupported YAML constructs.
- `encodeYAML(value any) ([]byte, error)` returns deterministic UTF-8 YAML with a final LF.
- `ParseSourceReference(raw string) (SourceIdentity, error)` parses exactly `file:` or `git:` scalar references.
- `FormatSourceReference(source SourceIdentity) string` returns the canonical scalar form.
- `NormalizeSourceIdentity(root string, source SourceIdentity) (SourceIdentity, error)` keeps file path normalization and validates both supported source types.

- [ ] Write tests for duplicate keys, explicit null, aliases, anchors, merge keys, custom tags, non-scalar keys, multiple documents, unknown fields, and canonical encoded output.
- [ ] Run `go test ./internal/repositorystate -run 'TestStrictYAML|TestSourceReference'`; confirm new tests fail for missing helpers or old decoder behavior.
- [x] Implement node-level syntax inspection, one-document EOF checking, `KnownFields(true)` decoding, scalar source parsing, and file/git normalization.
- [x] Run the focused tests again; confirm pass.

### Task 2: Reset domain models and persistence for all six documents

**Files:**

- Modify: `internal/repositorystate/model.go`
- Modify: `internal/repositorystate/manifest.go`
- Modify: `internal/repositorystate/lockfile.go`
- Modify: `internal/repositorystate/materialization_record.go`
- Create: `internal/repositorystate/recovery.go`
- Modify: `internal/repositorystate/store.go`
- Test: `internal/repositorystate/store_test.go`
- Test: `internal/repositorystate/manifest_test.go`
- Test: `internal/repositorystate/lockfile_test.go`
- Test: `internal/repositorystate/materialization_record_test.go`
- Create: `internal/repositorystate/recovery_test.go`

**Interfaces:**

- `Store` gains `LoadRecoveryState` and `WriteRecoveryState`.
- State DTOs use scalar `source`, `scope`, `artifact`, `source_version`, and optional `commit` fields matching the approved contract.
- `RecoveryState` validates fixed code `rollback_incomplete`, sanitized summary, canonical observations, expected state, result, digest, mode, and optional owner.

- [ ] Replace old manifest DTO tests with literal final-contract YAML using scalar references and flat declarations.
- [ ] Add tests for canonical filenames, deterministic ordering, omitted empty optional fields, all six schema versions, and strict malformed-document rejection at each public load seam.
- [ ] Add Recovery State round-trip and sanitization tests.
- [ ] Run `go test ./internal/repositorystate`; confirm failures identify the old DTO/model behavior.
- [x] Implement final DTOs, scalar conversions, strict reads/writes, Recovery State persistence, source-specific validation, and canonical sorting.
- [x] Run `gofmt -w internal/repositorystate` and `go test ./internal/repositorystate`; confirm pass.

### Task 3: Reset published file Source descriptors

**Files:**

- Modify: `internal/source/file/source.go`
- Test: `internal/source/file/source_test.go`

**Interfaces:**

- `Source.Resolve` reads `tbboot-source.yaml` and `tbboot-artifact.yaml`.
- The Source Descriptor no longer carries a published source name.
- Artifact versions require canonical `MAJOR.MINOR.PATCH` SemVer and only `file` steps are accepted.

- [ ] Update fixture helpers to write final filenames and descriptor shapes.
- [ ] Add tests for missing source name removal, lowercase hyphenated names, clean relative paths, descriptor name matching, canonical SemVer, unknown fields, duplicate YAML keys, and unsupported step types.
- [ ] Run `go test ./internal/source/file`; confirm failures against old names/shapes.
- [x] Implement strict descriptor decoding through the shared YAML boundary and minimum semantic/path validation.
- [x] Run `gofmt -w internal/source/file/source.go internal/source/file/source_test.go` and the focused tests; confirm pass.

### Task 4: Adapt minimal install and CLI surface

**Files:**

- Modify: `internal/install/service.go`
- Modify: `internal/install/repository_state.go`
- Modify: `internal/install/sync.go`
- Modify: `cmd/tbboot/install.go`
- Modify: `cmd/tbboot/root.go`
- Test: `internal/install/service_test.go`
- Test: `internal/install/sync_test.go`
- Test: `cmd/tbboot/root_test.go`

**Interfaces:**

- Explicit install accepts scalar source references at the CLI and retains structured `source.Ref` internally.
- Existing in-root file install and pinned Sync write only canonical state files.
- Removed catalog/search/logs/upgrade placeholders are absent from help and no longer report success.

- [ ] Add one end-to-end test using final source descriptors and assert `tbboot-artifacts.yaml`, lockfile, and managed record contents.
- [ ] Add a CLI test asserting removed placeholders do not return a successful `not implemented` result.
- [ ] Run `go test ./internal/install ./cmd/tbboot`; confirm failures from old declaration and filename assumptions.
- [x] Adapt declaration construction, source parsing/formatting, reserved paths, trust display, and result rendering without changing later-ticket behavior.
- [x] Run the focused tests and `go test ./...`; confirm pass.

### Task 5: Update active documentation and executable fixtures

**Files:**

- Modify: `CONTEXT.md`
- Modify: `ARCHITECTURE.md`
- Modify: `UBIQUITOUS_LANGUAGE.md`
- Modify: applicable `docs/adr/*.md`
- Modify: `testdata/examples/**`

- [x] Replace active `talby-*` filenames and old structured Source Reference examples with the canonical 0.1 forms.
- [x] Remove active promises for catalog/search/logs/placeholders and unsupported materialization types where current docs still present them as 0.1 behavior.
- [x] Rename executable fixtures and expected outputs to `tbboot-*` and update YAML contents.
- [x] Run `just check-md` and the example E2E tests; confirm all active documentation and fixtures match implementation.

### Task 6: Final verification and review

**Files:**

- Verify all changed files from Tasks 1–5.

- [x] Run `git diff --check HEAD`.
- [x] Run `just check-go` and `just check-md`.
- [x] Run `just check` and inspect the complete output.
- [x] Re-read the design and this plan, then verify every acceptance criterion for issue #30 against code, tests, and fixtures.
- [x] Run the repository code review workflow against the branch diff and fix all Critical/Important findings.
- [x] Report the uncommitted branch state and the exact verification commands; request separate approval before any commit.
