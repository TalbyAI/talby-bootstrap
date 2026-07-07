# Install core review

## Scope

- Review date: `2026-07-07`
- Branch under review: `feature/install`
- Base for comparison: `origin/main...HEAD`
- Reviewed against:
  - [docs/superpowers/plans/2026-07-07-install-core.md](/workspaces/talby-bootstrap/docs/superpowers/plans/2026-07-07-install-core.md)
  - [docs/superpowers/specs/2026-07-07-install-core-design.md](/workspaces/talby-bootstrap/docs/superpowers/specs/2026-07-07-install-core-design.md)

## Validation

- `just check-go`

## Standards findings

### 1. `install` no longer preserves the documented bare-command `Sync` behavior

- Severity: high
- Files:
  - [cmd/tbboot/install.go](/workspaces/talby-bootstrap/cmd/tbboot/install.go:43)
  - [docs/adr/0001-cli-surfaces-and-command-model.md](/workspaces/talby-bootstrap/docs/adr/0001-cli-surfaces-and-command-model.md:20)
  - [CONTEXT.md](/workspaces/talby-bootstrap/CONTEXT.md:160)
- Issue:
  - The command now uses `cobra.ExactArgs(1)`, which rejects bare `tbboot install`.
  - Repo language and ADRs define bare `install` as the user-facing entrypoint for **Sync**.
- Why it matters:
  - This is a behavioral regression against the documented CLI contract, not just a wording mismatch.
- Decision:
  - Applicable.
  - Merge-blocking.
- Required action:
  - Rework the `install` command shape so bare `tbboot install` is accepted again.
  - Route the zero-argument case to the documented **Sync** behavior instead of failing argument validation.
  - Update CLI tests to cover both bare `install` and explicit `install <source-ref>` so this contract cannot regress again.

### 2. JSON success output does not use the documented JSON envelope

- Severity: high
- Files:
  - [cmd/tbboot/install.go](/workspaces/talby-bootstrap/cmd/tbboot/install.go:57)
  - [docs/adr/0005-operation-output-logs-and-exit-codes.md](/workspaces/talby-bootstrap/docs/adr/0005-operation-output-logs-and-exit-codes.md:15)
  - [CONTEXT.md](/workspaces/talby-bootstrap/CONTEXT.md:135)
- Issue:
  - Success output encodes only `source` and `artifact`.
  - The repo contract defines one canonical **JSON Output Envelope** for machine-readable CLI output.
- Why it matters:
  - This breaks consistency for automation and diverges from the stated operation-result contract.
- Decision:
  - Applicable.
  - Merge-blocking.
- Required action:
  - Replace the ad hoc success JSON payload with the canonical **JSON Output Envelope**.
  - Put the install-specific payload under the envelope `details` field rather than emitting top-level `source` and `artifact` fields directly.
  - Update CLI JSON tests to assert the envelope shape on success, not only on error.

### 3. New design text uses non-canonical domain vocabulary

- Severity: medium
- Files:
  - [docs/superpowers/specs/2026-07-07-install-core-design.md](/workspaces/talby-bootstrap/docs/superpowers/specs/2026-07-07-install-core-design.md:123)
  - [docs/superpowers/specs/2026-07-07-install-core-design.md](/workspaces/talby-bootstrap/docs/superpowers/specs/2026-07-07-install-core-design.md:129)
  - [UBIQUITOUS_LANGUAGE.md](/workspaces/talby-bootstrap/UBIQUITOUS_LANGUAGE.md:20)
  - [CONTEXT.md](/workspaces/talby-bootstrap/CONTEXT.md:35)
- Issue:
  - The design says “source kinds”.
  - Repo terminology explicitly prefers **Source Type** and avoids “source kind”.
- Why it matters:
  - The repo is documentation-first; terminology drift weakens the design as a source of truth.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Required action:
  - Replace `source kind` / `source kinds` with **Source Type** in the install-core design doc.
  - Re-scan nearby wording for any other terminology that drifts from `CONTEXT.md` and `UBIQUITOUS_LANGUAGE.md`.

### 4. The new plan heading does not follow the repo heading style

- Severity: low
- Files:
  - [docs/superpowers/plans/2026-07-07-install-core.md](/workspaces/talby-bootstrap/docs/superpowers/plans/2026-07-07-install-core.md:1)
  - [AGENTS.md](/workspaces/talby-bootstrap/AGENTS.md:22)
- Issue:
  - The heading is title case: `# Install Core Implementation Plan`.
  - Repo guidance requires sentence-case Markdown headings.
- Why it matters:
  - Minor, but it is a direct style miss in a newly added primary document.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Required action:
  - Rename the top heading in the plan to sentence case.
  - Keep the rest of the plan content unchanged unless another style mismatch is found nearby.

### 5. Test fixture file helpers are duplicated across packages

- Severity: low
- Files:
  - [cmd/tbboot/root_test.go](/workspaces/talby-bootstrap/cmd/tbboot/root_test.go:147)
  - [internal/install/service_test.go](/workspaces/talby-bootstrap/internal/install/service_test.go:264)
  - [internal/source/file/source_test.go](/workspaces/talby-bootstrap/internal/source/file/source_test.go:188)
- Issue:
  - The same `MkdirAll` + `WriteFile` helper pattern appears in three test files.
- Why it matters:
  - This is a judgment-call maintainability smell, not a contract violation. It is worth watching if more install-fixture tests are added.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Required action:
  - No immediate refactor is required for this branch.
  - If install-related fixture tests continue to grow, consolidate the helper into a small shared test utility rather than copying a fourth variant.

## Spec findings

### 1. Source capabilities are modeled but not enforced by the install core

- Severity: high
- Files:
  - [internal/install/service.go](/workspaces/talby-bootstrap/internal/install/service.go:36)
  - [internal/source/file/source.go](/workspaces/talby-bootstrap/internal/source/file/source.go:46)
  - [internal/install/service_test.go](/workspaces/talby-bootstrap/internal/install/service_test.go:49)
  - [docs/superpowers/specs/2026-07-07-install-core-design.md](/workspaces/talby-bootstrap/docs/superpowers/specs/2026-07-07-install-core-design.md:129)
- Issue:
  - The design says the core should reason about source capabilities rather than source-specific assumptions.
  - The implementation defines `Capabilities()` but `Service.Install` never reads it.
  - The tests currently accept `source.Ref{Version: "v1.2.3"}` for a `file` source even though that source reports `SupportsVersions: false`.
- Why it matters:
  - The current boundary looks future-ready but does not yet enforce one of the forward-compatibility rules it was introduced to support.
- Decision:
  - Not applicable for this merge gate.
  - No action required in the current branch.
- Rationale:
  - The design requires the capability model to exist, but the same design also says the first iteration does not need to exercise every capability.
  - Enforcing `SupportsVersions` for `source.Ref.Version` is a plausible next step, but this branch is not incorrect at its current boundary solely because that enforcement is deferred.
- Follow-up note:
  - If a later slice introduces `--source-version` end-to-end behavior, that slice should decide whether unsupported source types are rejected by the install core, by CLI normalization, or by both.

### 2. The branch contains scope creep outside the install-core plan

- Severity: medium
- Files:
  - [docs/research/2026-07-07-ponytail-codex-install.md](/workspaces/talby-bootstrap/docs/research/2026-07-07-ponytail-codex-install.md:1)
  - [docs/research/2026-07-07-superpowers-codex-install.md](/workspaces/talby-bootstrap/docs/research/2026-07-07-superpowers-codex-install.md:1)
  - [docs/research/2026-07-07-superpowers-ponytail-compatibility.md](/workspaces/talby-bootstrap/docs/research/2026-07-07-superpowers-ponytail-compatibility.md:1)
  - [docs/superpowers/plans/2026-07-07-install-core.md](/workspaces/talby-bootstrap/docs/superpowers/plans/2026-07-07-install-core.md:19)
- Issue:
  - The plan scopes this slice to install-core implementation files plus the install-core plan/spec updates.
  - The branch also adds Codex plugin installation and compatibility research unrelated to the install-core implementation slice.
- Why it matters:
  - The extra docs are not inherently wrong, but they make the branch less focused as an implementation of the cited plan and design.
- Decision:
  - Applicable.
  - Not merge-blocking for the current branch.
- Required action:
  - No code reimplementation is needed.
  - Decide during branch cleanup whether to keep the research docs here, move them to a follow-up branch, or explicitly widen the documented branch scope.
  - If the goal is a narrowly scoped merge, removing or splitting those docs would make the branch easier to review, but correctness of the install core does not depend on that split.

## Summary

- Standards findings: `5`
- Spec findings: `2`
- Merge-blocking items to fix now: `2`
  - bare `install` must remain the user-facing entrypoint for **Sync**
  - success JSON output must use the canonical **JSON Output Envelope**
- Applicable but not merge-blocking items: `4`
  - terminology drift in the install-core design doc
  - sentence-case heading in the install-core plan
  - duplicated fixture helpers across tests
  - extra research docs that widen the branch scope
- Not applicable for this merge gate: `1`
  - capability enforcement for version-related source behavior
- Highest-impact standards issue:
  - bare `install` no longer maps to documented **Sync** behavior
- Highest-impact spec issue:
  - the install core does not yet use source capabilities to validate version-related requests
