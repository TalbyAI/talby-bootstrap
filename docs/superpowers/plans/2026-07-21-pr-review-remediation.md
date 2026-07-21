# PR review remediation implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Correct the three accepted review findings for issue #33: JSON coverage, inaccessible-path conflict aggregation, and adoption revalidation ordering.

**Architecture:** Keep CLI rendering in cmd/tbboot, conflict classification and persistence ordering in internal/install, and existing filesystem error types in internal/materialize. Reuse the current app.Result, UserActionError, ConflictTopology, revalidateAdoptions, and test fixtures; add no new package, interface, or dependency.

**Tech Stack:** Go 1.26.4, Cobra 1.10.2, Go standard library, just.

## Global constraints

- Keep Request.Prune and both targetless-prune validations; this was an intentional design decision.
- Keep the BACKLOG.md change; it is accepted pull-request scope.
- Classify permission-denied recorded paths as ConflictTopology and continue preflight so Sync returns exit class 2 with all detectable conflicts.
- Revalidate adopted files after applyPrepared and before any repository-state write.
- Preserve existing best-effort cleanup and drift mapping for write-time races.
- Add no production abstraction for one new behavior.
- Do not create a commit without asking the user for explicit approval at that moment.

---

### Task 1: Cover new JSON result shapes

**Files:**

- Modify: cmd/tbboot/root_test.go
- Do not modify: cmd/tbboot/install.go or JSON envelope code; existing serialization already carries file_removed and unsafe_topology.

**Interfaces:**

- Consumes: execute, app.Result, existing install fixtures, and resultEnvelope.
- Produces: regression coverage proving exit code, stream, and typed JSON behavior for both new result kinds.

- [ ] **Step 1: Add the JSON success regression for prune removal**

Add TestSyncJSONIncludesFileRemovedChange. Use existing writeInstallFixture, initGitRepo, withDir, and repository-state store helpers. Perform an explicit install, clear Manifest.Declarations, run JSON targetless install --prune, and assert exit 0, empty stderr, outcome applied, and a changes entry with kind file_removed, path README.md, and ownership_kind whole_file.

~~~go
func TestSyncJSONIncludesFileRemovedChange(t *testing.T) {
    // Reuse existing fixture/setup helpers.
    // After clearing Manifest.Declarations, execute:
    // execute(context.Background(), []string{"--output", "json", "install", "--prune"}, &stdout, &stderr)
    // Decode stdout as app.Result.
    // Assert code 0, stderr empty, details.outcome == "applied".
    // Find details.changes[].kind == "file_removed".
    // Assert that entry has path == "README.md" and ownership_kind == "whole_file".
}
~~~

- [ ] **Step 2: Add the JSON conflict regression for unsafe topology**

Add TestSyncJSONIncludesUnsafeTopologyConflict. Create a source fixture targeting nested/a, perform the initial explicit install, replace the consumer nested directory with a symlink to an outside temporary directory, and skip only when symlink creation is unavailable. Run JSON targetless Sync and assert exit 2, empty stdout, a JSON stderr envelope, and a conflict with kind unsafe_topology and path nested/a.

~~~go
func TestSyncJSONIncludesUnsafeTopologyConflict(t *testing.T) {
    // Reuse existing fixture/setup helpers and writeFixture with target nested/a.
    // Install once, remove root/nested, then create a symlink root/nested -> outside.
    // Execute: execute(context.Background(), []string{"--output", "json", "install"}, &stdout, &stderr)
    // Assert exit app.ExitUserActionConflict and empty stdout.
    // Decode stderr as app.Result.
    // Find details.conflicts[].kind == "unsafe_topology".
    // Assert its paths contains exactly "nested/a".
}
~~~

- [ ] **Step 3: Run the focused CLI tests**

Run:

~~~sh
go test ./cmd/tbboot -run 'TestSyncJSONIncludes(FileRemovedChange|UnsafeTopologyConflict)$' -count=1
~~~

Expected: PASS.

---

### Task 2: Aggregate inaccessible recorded paths as topology conflicts

**Files:**

- Modify: internal/install/sync.go at prepareSyncUndesired
- Modify: internal/install/sync_test.go

**Interfaces:**

- Consumes: observeTarget errors from prepareSyncUndesired.
- Produces: ConflictTopology for os.ErrPermission, while unrelated I/O errors still return as operational errors.

- [ ] **Step 1: Add the failing permission regression**

Add TestSyncAggregatesInaccessibleManagedPath. First sync one artifact, clear Manifest.Declarations, chmod its managed target to 000, and restore permissions with t.Cleanup. If the test process can still read the file after chmod, skip because the runner bypasses file permissions. Run normal targetless Sync and require UserActionError containing both ConflictRemovalRequired and ConflictTopology.

~~~go
func TestSyncAggregatesInaccessibleManagedPath(t *testing.T) {
    // Initial sync of artifact "a" targeting "a".
    // Clear Manifest.Declarations.
    // Chmod root/a to 000; restore 0644 in t.Cleanup.
    // Skip if os.ReadFile(root/a) still succeeds.
    // Run service.Sync with SyncRequest{Root: root}.
    // Assert errors.As(err, &UserActionError{}).
    // Assert hasConflict(result, ConflictRemovalRequired).
    // Assert hasConflict(result, ConflictTopology).
}
~~~

- [ ] **Step 2: Run the regression and verify it fails**

Run:

~~~sh
go test ./internal/install -run TestSyncAggregatesInaccessibleManagedPath -count=1
~~~

Expected: FAIL because prepareSyncUndesired returns the permission error instead of continuing with a typed user-action conflict.

- [ ] **Step 3: Classify only permission errors and continue**

In prepareSyncUndesired, extend the existing observeTarget error branch. Keep the existing UnsafeTopologyError behavior. Add errors.Is(err, os.ErrPermission) to the same conflict path, set safeToRemove to false, continue scanning the remaining files, and leave unrelated errors on the existing return path.

~~~go
if err != nil {
    var topology materialize.UnsafeTopologyError
    if errors.As(err, &topology) || errors.Is(err, os.ErrPermission) {
        conflicts = append(conflicts, Conflict{
            Kind: ConflictTopology,
            Source: a.Source,
            Artifact: a.Artifact,
            Paths: []string{file.Path},
        })
        safeToRemove = false
        continue
    }
    return repositorystate.Lockfile{}, nil, nil, nil, err
}
~~~

- [ ] **Step 4: Run focused and aggregate install tests**

Run:

~~~sh
go test ./internal/install -run 'TestSync(AggregatesInaccessibleManagedPath|AggregatesOwnershipDriftAndRemovalWithoutWrites|AggregatesUnsafeTopologyAsConflict)$' -count=1
~~~

Expected: PASS.

---

### Task 3: Revalidate adoptions after file application

**Files:**

- Modify: internal/install/sync.go at persistPrepared
- Modify: internal/install/sync_test.go

**Interfaces:**

- Consumes: applyPrepared results and revalidateAdoptions.
- Produces: no repository-state write when an adopted file changes after other prepared operations; the existing UserActionError drift mapping remains in force.

- [ ] **Step 1: Add a regression that observes the ordering**

Add TestPersistPreparedRevalidatesAdoptionAfterApply. Prepare one ChangeOwnershipAdopted file and one created file. Arrange for the adopted file to change during application of the created operation, then assert persistPrepared returns one ConflictDrift and does not write the Materialization Record. Use the existing countingStore and UserActionError assertions. The test must fail against the current order because adoption is checked before applyPrepared.

~~~go
func TestPersistPreparedRevalidatesAdoptionAfterApply(t *testing.T) {
    // Create an existing identical adopted target and observe it.
    // Prepare a second created target whose application changes the adopted target.
    // Use a test store that records materialization-record writes.
    // Call persistPrepared.
    // Assert errors.As returned error to UserActionError with ConflictDrift.
    // Assert the managed-state file is absent or byte-for-byte unchanged.
}
~~~

- [ ] **Step 2: Run the regression and verify it fails**

Run:

~~~sh
go test ./internal/install -run TestPersistPreparedRevalidatesAdoptionAfterApply -count=1
~~~

Expected: FAIL because the current implementation calls revalidateAdoptions before applyPrepared.

- [ ] **Step 3: Move the existing check to the state-write boundary**

Delete the pre-apply adoption block. After applyPrepared succeeds and before the dry-run return or any WriteLockfile, WriteMaterializationRecord, or WriteManifest call, run the same check. Preserve the existing ChangedSincePreflightError-to-ConflictDrift mapping and call cleanup(created) before returning from this post-apply failure path.

~~~go
if !dryRun {
    if err := revalidateAdoptions(prepared.Files); err != nil {
        cleanup(created)
        var changed materialize.ChangedSincePreflightError
        if errors.As(err, &changed) {
            conflict := Conflict{Kind: ConflictDrift, Paths: []string{changed.Path}}
            for _, file := range slices.Concat(prepared.Files, prepared.Removals) {
                if file.Observed.Path == changed.Path {
                    conflict.Source = file.Artifact.Key.Source
                    conflict.Artifact = file.Artifact.Key.Name
                    break
                }
            }
            result := resultForConflicts(operation, len(prepared.Desired), []Conflict{conflict}, false)
            return result, UserActionError{Result: result}
        }
        return Result{}, err
    }
}
~~~

Do not add a new generic error-mapper abstraction for this one call site.

- [ ] **Step 4: Run focused install tests**

Run:

~~~sh
go test ./internal/install -run 'Test(PersistPreparedRevalidatesAdoptionAfterApply|PersistPreparedClassifiesAdoptionParentRaceAsDrift|SyncRevalidatesAdoptedFileBeforePersistence)$' -count=1
~~~

Expected: PASS.

---

### Task 4: Final verification

**Files:**

- Verify only; no additional files should change.

- [ ] **Step 1: Run aggregate repository checks**

Run:

~~~sh
just check
go test -race ./... -count=1
git diff --check HEAD
git diff --check main...HEAD
~~~

Expected: all commands exit successfully; both diff checks print no output.

- [ ] **Step 2: Inspect the final diff**

Confirm the diff contains only the accepted remediations and their tests. Confirm Request.Prune, both validation sites, and the accepted BACKLOG.md change remain unchanged.

- [ ] **Step 3: Stop before committing**

Report the verified result and ask for explicit approval before any commit. Do not run git commit without that approval.
