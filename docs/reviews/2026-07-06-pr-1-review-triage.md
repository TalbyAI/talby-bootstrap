# PR review triage for PR #1

## PR metadata

- Repository: `TalbyAI/talby-bootstrap`
- PR: `#1`
- Title: `Build initial tbboot CLI architecture and Go skeleton`
- Branch: `feature/architecture`
- Base: `main`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1>
- Analysis date: `2026-07-06`
- Review scope: unresolved GitHub review threads only

## Review completion checklist

- [x] Implement the 12 `ready-to-apply` items below.
- [x] Run the narrowest relevant validation for touched Go, Markdown, HTML, shell, and YAML files.
- [ ] Reply to the GitHub threads with the applied change references.
- [ ] Resolve the GitHub threads after validation passes.

## Classification rubric

- `ready-to-apply`: the intended change is already constrained by the current repo, spec, or code pattern.
- `needs-user-input`: the comment implies a product, contract, or wording decision that is not already settled.
- `not-applicable`: the comment is outdated or should be rejected on technical or product grounds.

## Summary table

| File | Line | Theme | Classification | Notes |
| --- | ---: | --- | --- | --- |
| `cmd/tbboot/root.go` | 37 | CLI error reporting | `ready-to-apply` | `execute()` returns exit code `1` without printing the Cobra error. |
| `CONTEXT.md` | 156 | Upgrade selector wording | `ready-to-apply` | Current wording conflicts with the canonical `upgrade <source-ref> [--artifact ...]` shape elsewhere in the same file. |
| `docs/adr/0005-operation-output-logs-and-exit-codes.md` | 15 | JSON envelope contract | `ready-to-apply` | ADR omits `warnings` and recorded operation metadata present in the broader contract. |
| `learning-records/0001-learning-goal-established.md` | 1 | Heading style | `ready-to-apply` | Heading is still Title Case. |
| `learning-records/0001-learning-goal-established.md` | 3 | Accented Spanish terms | `ready-to-apply` | Terms are still unaccented in an English sentence. |
| `lessons/0001-superpowers-lifecycle-map.html` | 45 | HTML escaping | `ready-to-apply` | Raw `>` characters are still present in `->` arrows. |
| `reference/superpowers-lifecycle-cheatsheet.html` | 23 | HTML escaping | `ready-to-apply` | Raw `>` characters are still present in `->` arrows. |
| `RESOURCES.md` | 8 | Spelling / terminology | `ready-to-apply` | `conceptualizacion` is still present. |
| `UBIQUITOUS_LANGUAGE.md` | 3 | Glossary intro wording | `ready-to-apply` | `closed source` still reads as a typo for `source of truth`. |
| `UBIQUITOUS_LANGUAGE.md` | 52 | Sync definition | `ready-to-apply` | Glossary row is narrower than the repo's lockfile-backed model. |
| `.devcontainer/toolchain-audit.sh` | 134 | Temp-file cleanup | `ready-to-apply` | `exit 1` bypasses cleanup when `diff` reports mismatch. |
| `.markdownlint-cli2.yaml` | 1 | Line endings | `ready-to-apply` | File is still CRLF, confirmed from raw bytes. |

## Quick wins

- Print Cobra execution errors to `stderr` in `cmd/tbboot/root.go` and extend `TestInvalidOutputModeFailsAsValidationError` to assert emitted error text.
- Align the `Upgrade Command` paragraph in `CONTEXT.md` with the already-declared canonical form at `CONTEXT.md:566-567`.
- Expand the ADR JSON envelope wording to include `warnings` and recorded operation metadata so it matches the spec contract.
- Normalize obvious docs/lint issues:
  - sentence-case heading in `learning-records/0001-learning-goal-established.md`
  - accented Spanish terms in `learning-records/0001-learning-goal-established.md`
  - HTML-escaped arrows in `lessons/0001-superpowers-lifecycle-map.html`
  - HTML-escaped arrows in `reference/superpowers-lifecycle-cheatsheet.html`
  - spelling fix in `RESOURCES.md`
  - glossary intro wording in `UBIQUITOUS_LANGUAGE.md`
  - lockfile-aware `Sync` glossary row in `UBIQUITOUS_LANGUAGE.md`
  - LF-only newlines in `.markdownlint-cli2.yaml`
- Guarantee temp-file cleanup in `.devcontainer/toolchain-audit.sh` even when `compare()` exits on a mismatch.

## Detailed entries

### 1. `cmd/tbboot/root.go:37`

- Author: `coderabbitai`
- Severity/category: `Functional Correctness`, `Major`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261623>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: `SilenceErrors: true` suppresses Cobra's automatic error printing, and `execute()` maps the error to exit code `1` without printing any message.
- Current-state summary: the claim is still correct.
- Current-state verification:
  - `cmd/tbboot/root.go:33-35` returns exit code `1` from `root.Execute()` errors without writing to `stderr`.
  - `cmd/tbboot/root.go:43-44` still sets `SilenceUsage: true` and `SilenceErrors: true`.
  - `cmd/tbboot/root_test.go:48-53` still checks only the exit code for invalid `--output`.
- Classification: `ready-to-apply`
- Reasoning: the repo already chose Cobra with explicit exit-code mapping, so printing the error and extending the existing test is a direct correctness fix, not a product decision.
- Proposed action: print the returned error to `stderr` before returning exit code `1`, then assert the emitted error text in `TestInvalidOutputModeFailsAsValidationError`.

### 2. `CONTEXT.md:156`

- Author: `coderabbitai`
- Severity/category: `Functional Correctness`, `Minor`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261635>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: the `Upgrade Command` paragraph uses selector wording that conflicts with the canonical `upgrade` shape elsewhere in the file.
- Current-state summary: the claim is still correct.
- Current-state verification:
  - `CONTEXT.md:156` says upgrade reuses install target forms as `typed source or catalog-qualified source`.
  - `CONTEXT.md:566-567` separately defines the canonical explicit upgrade shape as `tbboot upgrade <source-ref> [--artifact <artifact-name>]`.
- Classification: `ready-to-apply`
- Reasoning: the file already declares the canonical selector model, so this is a wording-alignment fix rather than a new design decision.
- Proposed action: rewrite the `Upgrade Command` paragraph to use the same `source-ref` plus optional `--artifact` vocabulary already defined later in `CONTEXT.md`.

### 3. `docs/adr/0005-operation-output-logs-and-exit-codes.md:15`

- Author: `coderabbitai`
- Severity/category: `Data Integrity & Integration`, `Major`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261645>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: the ADR's JSON envelope description omits `warnings` and recorded metadata even though the broader operation-result contract includes them.
- Current-state summary: the claim is still correct.
- Current-state verification:
  - `docs/adr/0005-operation-output-logs-and-exit-codes.md:15` limits the envelope to `code`, `message`, and `details`.
  - `docs/superpowers/specs/2026-07-04-language-framework-decision-design.md:58` says the core returns `code, message, details, warnings, and any recorded operation metadata`.
  - `CONTEXT.md:651` still uses the narrower phrasing, so the ADR and broader design are currently inconsistent.
- Classification: `ready-to-apply`
- Reasoning: the broader contract is already documented in the active design spec, so aligning the ADR is a direct consistency repair.
- Proposed action: extend the ADR's envelope wording to include `warnings` and recorded operation metadata, then align any nearby text that implies a smaller shape.

### 4. `learning-records/0001-learning-goal-established.md:1`

- Author: `coderabbitai`
- Severity/category: `Maintainability & Code Quality`, `Major`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261648>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: the heading is Title Case instead of sentence case.
- Current-state summary: the claim is still correct.
- Current-state verification: `learning-records/0001-learning-goal-established.md:1` is still `# Learning Goal Established`.
- Classification: `ready-to-apply`
- Reasoning: the repo guidelines explicitly require sentence-case Markdown headings.
- Proposed action: rename the heading to sentence case without changing meaning.

### 5. `learning-records/0001-learning-goal-established.md:3`

- Author: `coderabbitai`
- Severity/category: `Maintainability & Code Quality`, `Minor`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261653>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: Spanish lifecycle terms in the sentence are missing accents.
- Current-state summary: the claim is still correct.
- Current-state verification: `learning-records/0001-learning-goal-established.md:3` still contains `conceptualizacion`, `diseno`, `implementacion`, `revision`, and `publicacion`.
- Classification: `ready-to-apply`
- Reasoning: this is a localized wording fix with no product-surface ambiguity.
- Proposed action: update the listed Spanish lifecycle terms to their accented forms while preserving the rest of the sentence.

### 6. `lessons/0001-superpowers-lifecycle-map.html:45`

- Author: `coderabbitai`
- Severity/category: `Functional Correctness`, `Minor`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261661>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: raw `>` characters inside `->` should be escaped in HTML text.
- Current-state summary: the claim is still correct.
- Current-state verification: `lessons/0001-superpowers-lifecycle-map.html:45` still contains `conversacion -> spec -> plan -> tests -> codigo -> review -> PR`.
- Classification: `ready-to-apply`
- Reasoning: HTML validity is already the controlling standard here.
- Proposed action: replace each raw `->` arrow in that paragraph with `-&gt;`.

### 7. `reference/superpowers-lifecycle-cheatsheet.html:23`

- Author: `coderabbitai`
- Severity/category: `Functional Correctness`, `Minor`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261665>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: raw `>` characters inside `->` should be escaped in HTML text.
- Current-state summary: the claim is still correct.
- Current-state verification: `reference/superpowers-lifecycle-cheatsheet.html:23` still contains `idea -> spec aprobada -> plan -> worktree -> TDD -> reviews -> verificacion -> merge/PR`.
- Classification: `ready-to-apply`
- Reasoning: HTML validity is already the controlling standard here.
- Proposed action: replace each raw `->` arrow in that rule line with `-&gt;`.

### 8. `RESOURCES.md:8`

- Author: `coderabbitai`
- Severity/category: `Maintainability & Code Quality`, `Minor`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261670>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: `conceptualizacion` is misspelled or should use the accented Spanish form.
- Current-state summary: the claim is still correct.
- Current-state verification: `RESOURCES.md:8` still says `Use for: conceptualizacion, product shaping, design approval.`
- Classification: `ready-to-apply`
- Reasoning: this is a wording correction local to one sentence, with no unresolved contract question.
- Proposed action: replace `conceptualizacion` with the chosen corrected form and keep the rest of the sentence unchanged.

### 9. `UBIQUITOUS_LANGUAGE.md:3`

- Author: `coderabbitai`
- Severity/category: `Maintainability & Code Quality`, `Minor`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261676>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: `closed source` is a typo and should be `source of truth`.
- Current-state summary: the claim is still correct.
- Current-state verification: `UBIQUITOUS_LANGUAGE.md:3` still says `` `CONTEXT.md` is the closed source for Talby Bootstrap v1 domain language. ``
- Classification: `ready-to-apply`
- Reasoning: this is a direct wording correction with an obvious intended meaning.
- Proposed action: change `closed source` to `source of truth` while keeping the rest of the intro intact.

### 10. `UBIQUITOUS_LANGUAGE.md:52`

- Author: `coderabbitai`
- Severity/category: `Functional Correctness`, `Minor`
- Timestamp: `2026-07-06T10:51:38Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3528261685>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: the glossary row for `Sync` should include the lockfile-backed model used elsewhere in the repo.
- Current-state summary: the claim is still correct.
- Current-state verification:
  - `UBIQUITOUS_LANGUAGE.md:52` defines `Sync` only against the `Manifest`.
  - `UBIQUITOUS_LANGUAGE.md:68` already says `Sync` reconciles the target repository against the `Manifest` and `Lockfile`.
- Classification: `ready-to-apply`
- Reasoning: the glossary itself already contains contradictory statements, so aligning the table row is a straightforward consistency fix.
- Proposed action: update the `Sync` table row to mention the recorded `Lockfile` and derived `Resolution`.

### 11. `.devcontainer/toolchain-audit.sh:134`

- Author: `coderabbitai`
- Severity/category: `Stability & Availability`, `Minor`
- Timestamp: `2026-07-06T20:54:02Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3531989207>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: the temp file created in `compare()` leaks when `diff` fails because cleanup runs after `exit 1`.
- Current-state summary: the claim is still correct.
- Current-state verification:
  - `.devcontainer/toolchain-audit.sh:117-120` still creates a temp file when `current_file` is omitted.
  - `.devcontainer/toolchain-audit.sh:132-134` exits immediately on snapshot mismatch.
  - `.devcontainer/toolchain-audit.sh:137-139` performs cleanup only after the branch that already exited.
- Classification: `ready-to-apply`
- Reasoning: the current control flow clearly leaks temp files on one real path, and the fix is local.
- Proposed action: add deferred cleanup for `temp_file` so mismatch exits still remove the generated file.

### 12. `.markdownlint-cli2.yaml:1`

- Author: `coderabbitai`
- Severity/category: `Maintainability & Code Quality`, `Minor`
- Timestamp: `2026-07-06T20:54:02Z`
- URL: <https://github.com/TalbyAI/talby-bootstrap/pull/1#discussion_r3531989210>
- `isResolved`: `false`
- `isOutdated`: `false`
- Reviewer claim: the file uses CRLF line endings and should be normalized to LF.
- Current-state summary: the claim is still correct.
- Current-state verification:
  - `nl -ba .markdownlint-cli2.yaml` shows content unchanged.
  - Raw bytes from `od -An -t x1 -N 80 .markdownlint-cli2.yaml` include repeated `0d 0a` sequences, confirming CRLF line endings.
- Classification: `ready-to-apply`
- Reasoning: line-ending normalization is direct repo hygiene with no decision ambiguity.
- Proposed action: rewrite `.markdownlint-cli2.yaml` with LF-only line endings and no content changes.

## Decision queue

No `needs-user-input` items. The current unresolved review state is an implementation queue.
