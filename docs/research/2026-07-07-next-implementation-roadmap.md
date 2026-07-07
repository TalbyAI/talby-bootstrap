# Next implementation roadmap

Date: 2026-07-07

## Question

What implementation tranche would add the most value to this repository next, and how should that work be split into clear pull-request-sized slices?

## Context reviewed

- The repository already has core domain language in `CONTEXT.md`, `UBIQUITOUS_LANGUAGE.md`, and accepted ADRs in `docs/adr/`.
- The current Go implementation provides:
  - a thin Cobra command surface;
  - a real `install` core for source lookup and artifact selection;
  - a real `file` source resolver;
  - JSON and human output envelopes;
  - passing tests across the current codebase.
- The current implementation stops before the product's main value loop:
  - `install` with a source resolves and selects an artifact, but does not write a manifest, lockfile, or files into a consumer repository;
  - `install` without arguments still returns a sync placeholder;
  - `upgrade`, `search`, `logs`, and catalog commands are still placeholders.

## Recommendation

The next tranche that adds the most value is a vertical slice of real installation behavior:

- declare the install target in a **Manifest**;
- persist exact resolved state in a **Lockfile**;
- materialize at least simple file-based artifact content into the target repository;
- enforce the first usable **Trust Policy** rules for local `file:` installs;
- support a real `sync` path when `install` runs without an explicit source.

This tranche is the highest-value next step because it converts the repository from "resolution proof" into the first end-to-end version of the actual product. Until this exists, the CLI demonstrates internal architecture, but it still does not perform the main user job defined by the ADRs: safely reconciling reusable artifacts into a repository.

## Why this should come before other roadmap items

This work should be prioritized ahead of `git` sources, `upgrade`, `catalog`, `search`, and `logs`.

- `git` sources are important, but they mainly broaden acquisition reach. They do not complete the product loop by themselves.
- `upgrade` depends on a real manifest and lockfile model. Without persisted declared state and resolved state, upgrade semantics remain premature.
- `catalog`, `search`, and catalog refresh improve discovery, but discovery without installation leaves the main workflow incomplete.
- `logs` become much more valuable once there are real reconcile and materialization operations to record.

In short: the repository does not most urgently need broader command coverage. It needs the first complete reconcile path.

## Recommended PR sequence

The slices below are ordered to maximize shipped value while keeping each PR reviewable and technically coherent.

### PR 1: Manifest and lockfile domain foundation

**Goal**

Create the first durable repository state model so install results can be declared and replayed.

**Why this matters**

This establishes the minimum persistence boundary required by ADR-0002. Without it, every install remains an ephemeral resolution result with no reproducible repository state.

**Expected scope**

- Add manifest domain types under `internal/` with explicit request/result-oriented APIs.
- Add lockfile domain types under `internal/` with the exact resolved source and artifact data needed for replay.
- Define YAML read/write behavior for:
  - `talby-artifacts.yaml`
  - `talby-artifacts.lock.yaml`
- Model one-source / one-artifact installs clearly without over-designing multi-target behavior.
- Preserve stable source identity and exact resolved source version in the lockfile.
- Make room for later trust policy data in the manifest shape, even if only a minimal version is active in this PR.

**Out of scope**

- Writing files from artifact steps.
- Drift detection.
- `git` source support.
- Upgrade flows.

**Validation**

- Focused read/write tests for manifest and lockfile round-tripping.
- Tests that prove a resolved `file:` source snapshot becomes reproducible persisted state.
- CLI-adjacent tests that confirm install can now produce persisted declaration state, if routed through the core.

### PR 2: Real declare-only install flow

**Goal**

Implement `install --declare-only` as a real behavior that updates only the manifest and leaves lockfile and consumer files untouched.

**Why this matters**

This is the smallest meaningful user-facing workflow that proves the manifest model is real and that command intent can be separated from materialization.

**Expected scope**

- Add `--declare-only` parsing on the install command.
- Extend the install service so it can update the manifest without performing reconcile writes.
- Define overwrite behavior for declaration-only changes when the same source or artifact is already declared.
- Return structured result data that clearly distinguishes:
  - declaration changes made;
  - no-op declaration cases;
  - conflicts or invalid requests.

**Out of scope**

- Writing lockfile state during declare-only mode.
- Materializing artifact steps.
- Interactive prompting.

**Validation**

- Tests that verify only the manifest changes.
- Tests that verify lockfile absence or unchanged lockfile content.
- CLI tests for both human and JSON output modes.

### PR 3: Minimal trust-policy enforcement for local file installs

**Goal**

Enforce the first real trust rule: local `file:` sources are allowed by default only when they live inside the current operation root.

**Why this matters**

This is the first point where the implementation starts honoring ADR-0003 in behavior rather than only in documentation. It prevents the early installation path from silently normalizing unsafe acquisition patterns.

**Expected scope**

- Introduce a narrow trust-policy service or validator in `internal/`.
- Add operation-root detection behind an explicit seam.
- Evaluate direct `file:` install requests against the operation root before manifest, lockfile, or materialization writes occur.
- Return trust denials through the existing result and exit-code model in a way that later `git` policy work can extend cleanly.

**Out of scope**

- `git:` trust approvals.
- Risky step-type allowlisting.
- Minimum-age rules.
- Interactive confirmations.

**Validation**

- Tests for inside-root allowed cases.
- Tests for outside-root denied cases.
- CLI tests that verify trust denials map to the correct exit-code class once that wiring exists.

### PR 4: Minimal materialization engine for `file` steps only

**Goal**

Write actual managed files into the consumer repository for simple file-based artifact steps.

**Why this matters**

This is the moment the product starts doing visible work for the user. It converts install from "declaration only" into the first real artifact application flow.

**Expected scope**

- Parse and apply `file` materialization steps from the artifact descriptor.
- Copy or render file content into the target repository using deterministic paths and clear error handling.
- Record enough ownership metadata to support later drift detection and safe overwrite decisions.
- Keep the implementation limited to whole-file ownership only in this slice.
- Ensure the install result reports effective changes rather than only selected artifacts.

**Out of scope**

- Fragment insertion.
- Script and prompt step types.
- Rollback sophistication beyond minimal best-effort cleanup.
- Cross-artifact conflict handling beyond whole-file collisions.

**Validation**

- Integration-style tests over temporary directories.
- Tests for first install, repeat install with no changes, and overwrite rejection or failure when ownership rules are violated.
- CLI tests that verify the default summary reflects actual file changes.

### PR 5: Materialization record and basic drift detection

**Goal**

Persist enough managed state to detect whether previously written files still match what the CLI last installed.

**Why this matters**

Without this, repeated installs and syncs will be unsafe or under-specified. This PR is the minimum needed to make reconcile behavior believable for edited repositories.

**Expected scope**

- Add a concrete **Materialization Record** format.
- Track managed files per installed artifact.
- Detect whole-file drift by comparing current content with the prior recorded content or digest.
- Prevent silent overwrite when drift is detected.
- Report drift in a user-readable, automation-usable way.

**Out of scope**

- Fragment drift.
- Recovery-state richness beyond what is necessary to surface a failed write safely.
- Interactive prompts for resolving drift.

**Validation**

- Tests covering:
  - unchanged managed files;
  - user-edited managed files;
  - repeated installs after drift;
  - removal or replacement scenarios that should stop safely.

### PR 6: Real sync behavior for bare `install`

**Goal**

Replace the current placeholder `install`-without-arguments path with real reconciliation from manifest plus lockfile plus managed state.

**Why this matters**

This completes the primary v1 command shape. At that point, the CLI can both declare/install new artifacts and re-run reconciliation against existing repository state.

**Expected scope**

- Load manifest and lockfile from the repository root.
- Resolve declared artifacts using persisted source identity and exact source version semantics.
- Reconcile desired state against current materialized state.
- Reuse trust checks and drift handling from earlier slices.
- Return a real operation summary for no-op and changed sync runs.

**Out of scope**

- Upgrade semantics.
- Logs persistence.
- Catalog-assisted resolution.

**Validation**

- Tests for successful no-op sync.
- Tests for sync after declaration plus materialization.
- Tests for sync blocked by drift or trust failures.

## Cross-PR guidance

The following rules should hold across the roadmap so the slices stay coherent.

### Keep command parsing thin

Continue keeping Cobra-specific parsing in `cmd/tbboot/` and move behavior into command-independent `internal/` services. The current repository is already aligned with that shape and should not regress into command-level branching.

### Prefer vertical slices over subsystem batching

Do not build all manifest code first, then all trust code, then all materialization code in isolation unless the slice still ships visible product value. Each PR should unlock a behavior the user can meaningfully exercise.

### Add only the minimum interfaces needed

The codebase already anticipates seams for source resolution and environment-sensitive logic. Continue that pattern, but avoid introducing large runtime abstractions before a concrete behavior needs them.

### Hold the line on v1 scope

The slices above are deliberately narrow. Avoid pulling in these concerns early:

- real `git` resolution;
- version search or rich upgrade policies;
- fragment insertion;
- script or prompt execution;
- catalog cache or search infrastructure;
- operation log persistence.

## PR checklist template

Use this checklist when preparing or reviewing a PR derived from this roadmap.

- Is the PR advancing the installation vertical slice rather than expanding into unrelated command surfaces?
- Does the PR leave `cmd/tbboot/` thin and move behavior into `internal/`?
- Are the request and result types explicit and testable without Cobra?
- Does the PR add only the minimum persistence or policy machinery required for its slice?
- Are trust and safety checks evaluated before repository writes when applicable?
- Are human output, JSON output, and exit-code behavior covered where the slice changes command outcomes?
- Are integration-style tests included when filesystem behavior is central to the slice?
- Does the PR keep deferred concerns explicitly out of scope instead of partially sketching them?

## Suggested next decision after this roadmap

If the team wants one concrete follow-up plan to execute next, the best choice is:

1. specify the manifest and lockfile data model in implementation detail;
2. plan a PR that ships real `--declare-only` behavior on top of that foundation;
3. then add materialization and sync incrementally.

That sequence keeps the repository moving toward a complete install workflow without jumping ahead into source breadth or secondary commands.
