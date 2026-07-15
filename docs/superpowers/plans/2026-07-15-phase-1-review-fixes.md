# Phase 1 review fixes implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix five verified phase 1 review findings and record incomplete rollback recovery as phase 3 debt.

**Architecture:** Keep existing package boundaries and concrete helpers. Add one focused regression per bug, then make the smallest local change that satisfies the approved phase 1 design and repository vocabulary.

**Tech Stack:** Go 1.26.4, Cobra 1.10.2, Go standard library, Markdown, `just`.

## Global constraints

- Do not add a planner, reconciliation package, dependency, or shared resolution abstraction.
- Keep verified rollback and Recovery State out of phase 1 implementation.
- Compare filesystem identity using canonical paths.
- Preserve shared human and JSON result shapes through `install.Change`.
- Do not create a commit without fresh explicit user approval.

---

### Task 1: Record incomplete recovery debt

**Files:**

- Modify: `BACKLOG.md`

**Interfaces:**

- Consumes: phase 3 scope in `docs/superpowers/plans/2026-07-08-v1-roadmap.md`.
- Produces: one unprioritized backlog entry for verified rollback and Recovery State.

- [ ] **Step 1: Append the debt entry once**

```markdown
## Record incomplete materialization recovery

Priority: unprioritized

Harden rollback beyond removing newly created files. Preserve pre-write state where possible, verify restoration after partial failures, and record explicit **Recovery State** whenever rollback cannot be completed and verified. This is phase 3 debt; phase 1 intentionally retains best-effort semantics.
```

- [ ] **Step 2: Validate Markdown**

Run: `just check-md`

Expected: PASS.

### Task 2: Canonicalize file Source identity before trust admission

**Files:**

- Modify: `internal/repositorystate/manifest.go`
- Test: `internal/repositorystate/manifest_test.go`

**Interfaces:**

- Consumes: `NormalizeSourceIdentity(root string, source SourceIdentity) (SourceIdentity, error)`.
- Produces: relative identity only when canonical Source path is inside canonical Operation Root; otherwise canonical absolute identity.

- [ ] **Step 1: Write failing symlink regression**

Create an external directory, symlink it below the Operation Root, normalize the symlink locator, and assert returned locator equals the external canonical absolute path.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/repositorystate -run TestNormalizeSourceIdentityCanonicalizesSymlinkContainment -count=1`

Expected: FAIL because current result remains relative.

- [ ] **Step 3: Implement canonical normalization**

Resolve both Operation Root and Source path with `filepath.EvalSymlinks` before `filepath.Rel`. Return slash-normalized relative path only when canonical containment succeeds.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/repositorystate -count=1`

Expected: PASS.

### Task 3: Replay artifact declarations from merged snapshots

**Files:**

- Modify: `internal/install/sync.go`
- Test: `internal/install/sync_test.go`

**Interfaces:**

- Consumes: `verifyLocked(declaration, locked, resolved)`.
- Produces: artifact scope validates its single named entry; source scope validates the complete locked set.

- [ ] **Step 1: Write failing merged-snapshot replay regression**

Sync two artifact-level declarations from one Source snapshot, then Sync again and require `OutcomeNoOp`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/install -run TestSyncReplaysArtifactDeclarationsFromMergedSnapshot -count=1`

Expected: FAIL with `locked artifact set no longer matches current source`.

- [ ] **Step 3: Implement scope-aware length validation**

Apply the complete-set length check only when `d.Target.Scope == repositorystate.DeclarationScopeSource`; retain exact name/version checks for both scopes.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/install -run 'TestSync(ReplaysArtifactDeclarationsFromMergedSnapshot|ReplaysExact)' -count=1`

Expected: PASS.

### Task 4: Compare canonical managed target sets

**Files:**

- Modify: `internal/install/sync.go`
- Test: `internal/install/sync_test.go`

**Interfaces:**

- Consumes: `materialize.Observe` canonical `Observation.Path`.
- Produces: managed path-set validation using canonical consumer-relative paths.

- [ ] **Step 1: Write failing equivalent-target regression**

Sync an Artifact targeting `./foo`, then Sync again and require `OutcomeNoOp`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/install -run TestSyncReplaysCanonicallyEquivalentTargetPath -count=1`

Expected: FAIL with `managed artifact path set does not match desired artifact`.

- [ ] **Step 3: Implement canonical expected set**

Observe every desired target before managed-record comparison and build the expected set from `Observation.Path`. Reuse those observations during the remaining preflight loop so each target is inspected once.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/install -run TestSyncReplaysCanonicallyEquivalentTargetPath -count=1`

Expected: PASS.

### Task 5: Classify write races as drift

**Files:**

- Modify: `internal/install/sync.go`
- Test: `internal/install/sync_test.go`

**Interfaces:**

- Consumes: `materialize.ChangedSincePreflightError` returned by `applyPrepared`.
- Produces: `UserActionError` with one `ConflictDrift` and exit class `2`.

- [ ] **Step 1: Write failing classification regression**

Prepare a file observation, mutate the target before `persistPrepared`, and assert the returned error is `UserActionError` containing the canonical path.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/install -run TestPersistPreparedClassifiesWriteRaceAsDrift -count=1`

Expected: FAIL because current error remains operational.

- [ ] **Step 3: Implement one error mapper**

In `persistPrepared`, map `ChangedSincePreflightError` from both `applyPrepared` and adoption revalidation to the existing drift result shape.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/install -run 'Test(PersistPreparedClassifiesWriteRaceAsDrift|SyncRevalidatesAdoptedFileBeforePersistence)' -count=1`

Expected: PASS.

### Task 6: Complete effective-change provenance

**Files:**

- Modify: `internal/install/service.go`
- Modify: `internal/install/sync.go`
- Modify: `cmd/tbboot/install.go`
- Test: `internal/install/sync_test.go`
- Test: `cmd/tbboot/root_test.go`

**Interfaces:**

- Consumes: `desiredArtifact.ResolvedVersion` and locked resolution versions.
- Produces: optional `source_version` and `ownership_kind` fields on `install.Change`; file changes use ownership kind `whole_file`.

- [ ] **Step 1: Write failing service and CLI provenance regressions**

Assert file changes contain `SourceVersion == "snapshot"` and `OwnershipKind == "whole_file"`; assert JSON and human rendering expose both values.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/install ./cmd/tbboot -run 'Provenance' -count=1`

Expected: FAIL because fields do not exist.

- [ ] **Step 3: Add minimal provenance fields**

```go
type Change struct {
    Kind          ChangeKind                     `json:"kind"`
    Source        repositorystate.SourceIdentity `json:"source"`
    SourceVersion string                         `json:"source_version,omitempty"`
    Artifact      string                         `json:"artifact,omitempty"`
    Path          string                         `json:"path,omitempty"`
    OwnershipKind string                         `json:"ownership_kind,omitempty"`
}
```

Populate Source Version for resolution, file, adoption, and pruning changes. Populate `whole_file` only for file ownership changes. Render optional values from the same `Change` model.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/install ./cmd/tbboot -count=1`

Expected: PASS.

### Task 7: Final verification and review

**Files:**

- Verify all modified Go and Markdown files.

**Interfaces:**

- Consumes: Tasks 1-6.
- Produces: formatted, tested diff ready for user review.

- [ ] **Step 1: Format Go files**

Run: `just fmt-go`

- [ ] **Step 2: Run full checks**

Run: `just check`

Expected: PASS.

- [ ] **Step 3: Run race detector**

Run: `go test -race ./... -count=1`

Expected: PASS.

- [ ] **Step 4: Review diff**

Run: `git diff --check && git diff --stat && git status --short`

Expected: no whitespace errors; only intended files changed. Do not commit without fresh explicit approval.
