# Reconcile loop design

## Status

Proposed.

## Context

The repository already has the first two slices of the next implementation roadmap defined at design level:

- repository-state persistence for the **Manifest** and **Lockfile**;
- a real `install --declare-only` flow that writes declared state without writing resolved state or consumer files.

Those slices make repository intent durable, but they still stop before the main product loop. The CLI can declare artifacts, yet it still cannot safely apply them into a consumer repository, detect user drift, or re-run reconciliation from persisted state.

The next roadmap topics, numbered 3 through 6 in `docs/research/2026-07-07-next-implementation-roadmap.md`, are the minimum remaining work needed to turn that declared-state foundation into the first usable reconcile loop:

- enforce the first **Trust Policy** rule for local `file:` installs;
- materialize managed files for simple artifact steps;
- detect whole-file drift from persisted managed state;
- replace placeholder bare `install` behavior with real sync.

This design covers those topics in one document so later planning can split implementation into multiple PR-sized plans without re-deciding product behavior each time.

## Goal

Define the first complete reconcile loop for v1 after declaration state exists.

That loop should let the product:

- reject unsafe local acquisition before writes occur;
- persist exact resolved state when a non-declare install succeeds;
- write simple managed files into the consumer repository;
- remember enough managed state to detect later drift safely;
- re-run reconciliation from **Manifest**, **Lockfile**, and managed state when `install` runs without an explicit source.

## Non-goals

This design deliberately excludes:

- real `git` source implementation or richer `git` trust workflows;
- fragment insertion or fragment drift handling;
- `script` and `prompt` materialization support;
- interactive trust approvals or interactive drift resolution;
- strong transactional rollback guarantees;
- `upgrade` behavior;
- catalog, search, or logs work.

## Decision

Represent roadmap topics 3 through 6 as three product capabilities delivered in sequence:

1. **Trust enforcement** as the admission check before repository mutation.
2. **Materialization with managed-state protection** as the first visible repository-writing behavior.
3. **Sync from persisted state** as the first full reconcile path for bare `install`.

This keeps one coherent product story while preserving the implementation sequence from the roadmap:

- PR 3 establishes trust enforcement;
- PR 4 establishes materialization;
- PR 5 adds managed-state drift protection;
- PR 6 turns bare `install` into real sync by reusing the earlier capabilities.

## Product model

The reconcile loop operates on three distinct state layers that should remain separate in both behavior and user explanation:

- the **Manifest** stores desired state;
- the **Lockfile** stores exact resolved state;
- the **Materialization Record** stores what the CLI last wrote into the repository as managed output.

The product should not blur those layers.

- Declared state is user intent.
- Locked state is replayable resolution.
- Materialized state is filesystem ownership and drift evidence.

Keeping those layers distinct is the core constraint that lets later `sync` and `upgrade` behavior stay understandable.

For the first usable reconcile loop, a successful non-declare install should update all three layers together:

- the **Manifest** records user intent;
- the **Lockfile** records the exact resolved source and artifact state used for the install;
- the **Materialization Record** records the managed output written into the repository.

The product should not treat file writes as complete unless the matching replay and ownership state has been persisted too.

## Capability 1: Trust enforcement

### Why this capability comes first

The first real write path should not normalize unsafe acquisition. Trust enforcement must happen before manifest, lockfile, or filesystem writes that would make an untrusted source look routine.

### Product decision

PR 3 should implement the smallest real **Trust Policy** behavior from ADR-0003:

- direct `file:` installs are allowed by default only when the source path is inside the current **Operation Root**;
- direct `file:` installs outside the **Operation Root** are denied before any state mutation;
- trust approval remains modeled by **Source Identity** in the versioned **Manifest**, even though this slice only enforces the default local `file:` rule;
- the denial should be user-readable and automation-classifiable through the existing result and exit-code model.

This slice should not broaden into generic approval management. It exists to establish the first hard admission rule, not the full trust system.

### User-visible behavior

When a user runs `install <file-source>`:

- if the path is inside the current operation root, the install flow may continue;
- if the path is outside the current operation root, the operation stops before writing manifest, lockfile, or managed files.

The denial should make the reason explicit: the source is outside the operation root and is not allowed by default.

### Boundaries

This capability should remain narrow:

- it evaluates direct local `file:` install requests;
- it does not yet consume persisted `approved_sources` to broaden direct-install behavior;
- it does not yet introduce `git:` approval workflows;
- it does not yet approve risky step types;
- it does not ask the user for confirmation.

That boundary matters because the product needs one enforceable rule first, not a half-finished policy matrix.

## Capability 2: Materialization with managed-state protection

### Why this capability is split across PR 4 and PR 5

The product becomes visibly useful in PR 4 when it writes files, but it becomes operationally credible only in PR 5 when it can detect that managed files have later changed. Materialization without managed-state protection would make repeated installs unsafe or ambiguous.

### Product decision for PR 4

PR 4 should materialize only simple `file` steps as whole-file writes into the consumer repository.

The first materialization slice should:

- persist the resolved install result into the **Lockfile** as part of the same successful non-declare install;
- write managed files using deterministic target paths;
- write a minimal **Materialization Record** for those managed files in the same successful operation;
- treat ownership as exclusive for whole files;
- reject collisions rather than silently sharing ownership;
- report actual file changes, not just selected artifact metadata.

This is intentionally the smallest visible materialization model that matches ADR-0004 without introducing fragment complexity.

### User-visible behavior in PR 4

The first successful non-declare install should produce visible repository changes:

- the exact resolved source and artifact state is persisted beside the **Manifest** before the install is considered complete;
- files from supported `file` steps appear in the consumer repository;
- repeated application of unchanged input in the same repository can compare against the just-recorded managed state rather than rewriting arbitrarily;
- whole-file ownership conflicts stop the operation clearly.

At this point the product is doing useful work and leaves behind the minimum persisted evidence needed for later replay. The remaining gap is not whether state exists, but how fully later operations enforce drift and recovery rules.

### Product decision for PR 5

PR 5 should harden the initial **Materialization Record** behavior from PR 4 and use it to detect whole-file drift before overwrite.

The first drift slice should:

- refine the PR 4 record shape so it clearly tracks which managed files belong to which installed artifact, source identity, and locked source version;
- compare current content against the prior recorded managed content or digest;
- block silent overwrite when current whole-file content no longer matches that prior managed state;
- stop on managed-artifact removal requests that would require a prompt, rather than deleting files automatically in this tranche;
- surface drift in a form that users and automation can both understand.

This is the minimum needed to make later reconcile behavior believable in edited repositories.

### User-visible behavior in PR 5

After PR 5:

- re-running install over unchanged managed files succeeds cleanly;
- re-running install after a user edits a managed file stops safely instead of overwriting silently;
- re-running install or sync when final desired state would remove a managed artifact stops with a user-action conflict until a later prompt-capable flow exists;
- managed-state conflicts become explainable in terms of owned files and detected drift.

### Boundaries

This capability should stay within whole-file ownership in this tranche.

It should not yet include:

- fragment insertion;
- fragment boundary markers;
- script execution;
- prompt-driven changes;
- sophisticated rollback or recovery beyond the minimum needed to record failure state safely.

## Capability 3: Sync from persisted state

### Why this is the final step in the tranche

Bare `install` is currently a placeholder for sync. Replacing it earlier would force the product to invent reconcile semantics before trust checks, file ownership, and drift protection exist. Real sync only becomes coherent once those earlier behaviors are already defined and reusable.

### Product decision

PR 6 should replace `install` without an explicit source with real reconciliation from persisted repository state:

- load the **Manifest** as desired state;
- load the **Lockfile** as exact resolved state;
- load managed-state records as the baseline for ownership and drift checks;
- evaluate trust from persisted policy plus default rules:
  - approved **Source Identity** values in the **Manifest** may authorize persisted or explicit sources that are not default-allowed;
  - unapproved local `file:` sources remain default-allowed only when they are inside the current **Operation Root**;
- reconcile desired state against the current filesystem using the earlier trust and drift rules.

This should be the first complete v1 command shape for "make the repository match its declared and locked state."

### User-visible behavior

When the user runs bare `install` in a repository with persisted state:

- if nothing changed and no drift exists, the command reports a no-op sync;
- if managed files need to be re-applied and remain safe to overwrite, the command applies those changes;
- if trust or drift checks fail, the command stops with a clear result and leaves the repository in a safe, explainable state.

The product should describe this as reconciliation, not as a new resolution workflow. Sync replays persisted state; it does not search for newer versions or re-decide user intent.

For `file:` sources specifically, replay should continue to use the exact locked snapshot semantics from ADR-0002. A later bare `install` may re-read the source location to materialize content again, but it should treat the locked snapshot hash as the reproducibility baseline and stop when persisted trust or managed-state rules do not allow rewrite.

### Boundaries

This capability should not absorb adjacent roadmap items.

It should not:

- perform `upgrade`;
- change declared targets automatically;
- broaden discovery behavior;
- invent new source-version selection rules beyond the lockfile semantics already defined by ADR-0002.

## Cross-capability rules

The following rules should hold across PRs 3 through 6.

### No writes before admission checks

Trust and ownership safety checks happen before filesystem mutation whenever the operation already has enough information to reject safely.

For successful non-declare installs, persisted state writes are part of the same protected operation. The CLI should not leave behind new managed files without also persisting the matching lock and managed-state evidence required to explain them later.

### Reconcile from persisted state, not from remembered CLI intent

Once state files exist, replay behavior should come from the **Manifest**, **Lockfile**, and **Materialization Record**, not from transient command-line context.

### Safe stop is better than clever recovery

In ambiguous situations, such as drift or ownership conflict, the product should stop clearly rather than guessing how to merge or overwrite.

### Visible work should map to visible state

If the CLI writes files, it should also write the managed-state evidence needed to explain those writes later. Materialization and managed-state recording should stay conceptually linked even if they land in adjacent PRs.

Likewise, if the CLI resolves a source version for a successful non-declare install, it should persist that exact resolved state in the **Lockfile** before the operation is considered complete.

## Recommended implementation sequence

The roadmap order remains the right sequence:

### PR 3: Minimal trust-policy enforcement for local `file:` installs

Establish the first admission rule before any broader write path becomes normal behavior.

### PR 4: Minimal materialization engine for `file` steps only

Deliver the first visible repository-writing behavior with whole-file ownership and collision failure.

### PR 5: Materialization record and basic whole-file drift detection

Add the managed-state evidence needed to make repeated application safe and explainable.

### PR 6: Real sync behavior for bare `install`

Reuse the earlier capabilities to complete the first end-to-end reconcile loop.

## Validation at product level

This tranche is successful when the repository can demonstrate the following user story end to end:

1. A user declares or installs an artifact from a trusted local `file:` source.
2. The CLI persists the exact resolved install state in the **Lockfile**.
3. The CLI writes managed files into the repository and records what it wrote.
4. A later bare `install` can re-run reconciliation from persisted state.
5. If the user edited a managed file or the source is not trusted, the CLI stops safely instead of overwriting silently.

If any one of those points is missing, the product still has a gap in the first real reconcile loop.

## Deferred work

The following work remains intentionally outside this design:

- richer source trust rules, especially for `git:`;
- risky step-type allowlisting and first-install confirmations;
- fragment ownership and fragment drift;
- user-guided conflict resolution;
- `upgrade` semantics on top of manifest and lockfile state;
- catalog-driven discovery and search;
- durable operation logs.
