# Install core review

## Scope

- Review date: `2026-07-07`
- Branch under review: `feature/install`
- Base for comparison: `origin/main...HEAD`
- Reviewed against:
  - [docs/superpowers/plans/2026-07-07-install-core.md](/workspaces/talby-bootstrap/docs/superpowers/plans/2026-07-07-install-core.md)
  - [docs/superpowers/specs/2026-07-07-install-core-design.md](/workspaces/talby-bootstrap/docs/superpowers/specs/2026-07-07-install-core-design.md)

## Validation

- `go test ./internal/source/file -run 'TestResolve(LoadsSourceIdentityAndArtifacts|ChangesSnapshotVersionWhenResolvedContentChanges)' -v`
- `go test ./...`

## Standards findings

### 1. Human success output is still ad hoc follow-up work

- Severity: low
- Files:
  - [cmd/tbboot/install.go](/workspaces/talby-bootstrap/cmd/tbboot/install.go:76)
  - [CONTEXT.md](/workspaces/talby-bootstrap/CONTEXT.md:167)
  - [docs/adr/0005-operation-output-logs-and-exit-codes.md](/workspaces/talby-bootstrap/docs/adr/0005-operation-output-logs-and-exit-codes.md:13)
- Issue:
  - The current human success path prints `selected artifact %s from %s`.
  - The repo’s long-term contract expects a stable **Operation Summary** shape for install and sync operations.
- Why it matters:
  - This is output-contract drift, but under the current branch policy it is not evident current-slice breakage.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Follow-up note:
  - Revisit the human summary once install starts reporting real reconciliation outcomes instead of only artifact selection.

### 2. Dead JSON mapping code remains in `cmd/tbboot/install.go`

- Severity: low
- Files:
  - [cmd/tbboot/install.go](/workspaces/talby-bootstrap/cmd/tbboot/install.go:17)
  - [cmd/tbboot/install.go](/workspaces/talby-bootstrap/cmd/tbboot/install.go:85)
- Issue:
  - `installResultJSON` and `mapInstallResult` are no longer used after the command switched to the canonical envelope shape under `details`.
- Why it matters:
  - This is cleanup debt only. It does not change current behavior.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Follow-up note:
  - Remove the dead mapping layer when the next output-focused slice touches `cmd/tbboot/install.go`.

### 3. Test fixture file helpers are duplicated across packages

- Severity: low
- Files:
  - [cmd/tbboot/root_test.go](/workspaces/talby-bootstrap/cmd/tbboot/root_test.go:156)
  - [internal/install/service_test.go](/workspaces/talby-bootstrap/internal/install/service_test.go:264)
  - [internal/source/file/source_test.go](/workspaces/talby-bootstrap/internal/source/file/source_test.go:216)
- Issue:
  - The same `MkdirAll` + `WriteFile` helper pattern appears in three test files.
- Why it matters:
  - This is a judgment-call maintainability smell. It does not change current install-core behavior.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Follow-up note:
  - Consolidate the helper only if install-fixture tests continue to expand.

## Spec findings

### 1. `file` source version was a constant placeholder instead of a local snapshot hash

- Severity: high
- Files:
  - [internal/source/file/source.go](/workspaces/talby-bootstrap/internal/source/file/source.go:110)
  - [internal/source/file/source_test.go](/workspaces/talby-bootstrap/internal/source/file/source_test.go:47)
  - [docs/adr/0002-source-resolution-versioning-and-locking.md](/workspaces/talby-bootstrap/docs/adr/0002-source-resolution-versioning-and-locking.md:22)
- Issue:
  - The branch originally returned `local-snapshot-001` for every resolved `file` source.
  - ADR-0002 says `file` should record a local snapshot hash as the resolved **Source Version**.
- Why it matters:
  - A constant placeholder is incorrect current-slice behavior, not just deferred future work.
- Decision:
  - Fixed in this branch.
  - Verified with a regression test that different resolved content produces different `local-snapshot-*` versions.

### 2. Install result still drops source-resolution metadata needed by later slices

- Severity: medium
- Files:
  - [internal/install/service.go](/workspaces/talby-bootstrap/internal/install/service.go:15)
  - [internal/source/model.go](/workspaces/talby-bootstrap/internal/source/model.go:34)
  - [docs/superpowers/plans/2026-07-07-install-core.md](/workspaces/talby-bootstrap/docs/superpowers/plans/2026-07-07-install-core.md:32)
- Issue:
  - `InstallResult` currently returns `source.Identity` plus the selected artifact, but drops extra resolved-source metadata such as `SourcePath`.
- Why it matters:
  - This leaves later manifest/lockfile/materialization slices with less data than the plan expected, but it does not break the behavior implemented in this branch.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Follow-up note:
  - Expand `install.Result` only when the next slice is ready to consume the additional source-resolution fields.

## Summary

- Standards findings: `3`
- Spec findings: `2`
- Fixed in this branch: `1`
  - `file` source now returns a content-derived `local-snapshot-*` hash instead of a constant placeholder version
- Documented follow-ups left intentionally for later: `4`
  - ad hoc human success summary
  - dead JSON mapping code in `cmd/tbboot/install.go`
  - duplicated fixture helpers across tests
  - `install.Result` dropping extra source-resolution metadata
