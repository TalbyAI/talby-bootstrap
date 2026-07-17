# PR 25 review corrections implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the PR 25 findings and remove contradictory active promises about removed v1 diagnostics.

**Architecture:** Keep the correction documentation-only. Make `CONTEXT.md` and ADR-0002 state the same Source-Type-specific upgrade contract, then make each canonical overview agree that v1 diagnostics are immediate and not replayable.

**Tech Stack:** Markdown, `rg`, markdownlint through `just check-md`.

## Global constraints

- V1 exposes only `tbboot upgrade`; `install --upgrade` is not supported.
- `git:` upgrade selects the latest stable published Source Version allowed by policy.
- `file:` upgrade re-reads the current local path and records its snapshot hash; an unchanged hash is a no-op.
- Source scope updates every declared Artifact in the Source snapshot; Artifact scope updates only the selected Artifact.
- Upgrade leaves the Manifest unchanged and updates only affected Lockfile resolutions.
- V1 has no `logs` command, persisted Operation Logs, replay, operation identifiers, retention policy, or verbosity levels.
- Do not modify Go code or normalize ADR-0001 through ADR-0004 headings.
- Before any commit, stop and obtain explicit user confirmation as required by `AGENTS.md`.

---

### Task 1: Align the v1 upgrade documentation

**Files:**

- Modify: `CONTEXT.md:168`
- Modify: `CONTEXT.md:482`
- Modify: `CONTEXT.md:543`
- Modify: `CONTEXT.md:567-568`
- Modify: `docs/adr/0002-source-resolution-versioning-and-locking.md:24`
- Modify: `docs/adr/0005-operation-output-logs-and-exit-codes.md:1`

**Interfaces:**

- Consumes: the approved decisions in `docs/superpowers/specs/2026-07-17-pr-25-review-corrections-design.md`.
- Produces: one consistent normative upgrade contract across `CONTEXT.md` and ADR-0002, plus a sentence-case ADR-0005 title.

- [ ] **Step 1: Update the canonical Upgrade Command definition**

In `CONTEXT.md`, retain the existing targeting, ordering, Dry Run, Manifest, and Lockfile sentences at line 168. Replace the generic latest-stable sentence with these exact Source-Type rules before the final canonical-command sentence:

```markdown
For `git:` targets, upgrade advances to the latest stable published Source Version allowed by the active trust policy. For `file:` targets, upgrade re-reads the current local path and records the resulting snapshot hash; an unchanged snapshot is a no-op. These rules apply to both bare and explicit upgrade: Source scope updates every declared Artifact in the Source snapshot, while Artifact scope updates only the selected Artifact and leaves sibling Artifacts on their existing snapshots.
```

- [ ] **Step 2: Update the resolved-decision summaries**

In the command-surface decisions, replace the shortcut claim at line 543 with:

```markdown
- `upgrade` is the only v1 version-advancement command; `install --upgrade` is not supported
```

In the resolution/versioning decisions, replace the generic line 567 and expand the scope line 568 with:

```markdown
- `upgrade` advances `git:` targets to the latest stable published Source Version allowed by policy
- `upgrade` re-reads each targeted `file:` Source from its current local path and records the resulting snapshot hash; an unchanged snapshot is a no-op
- Source targets upgrade every declared Artifact in the Source snapshot, while Artifact targets upgrade only the selected Artifact and leave sibling Artifacts on their existing snapshots
```

Preserve the existing indentation of the surrounding nested list.

Also align the ambiguity summary near line 482 with:

```markdown
- upgrade selection could be under-specified — resolved: `upgrade` advances `git:` targets to the latest stable published Source Version allowed by trust and age rules, while `file:` targets record a new snapshot hash from the current local path
```

- [ ] **Step 3: Align ADR-0002 and ADR-0005**

Replace ADR-0002 line 24 with:

```markdown
Later syncs keep the previously resolved **Source Version** until the user explicitly upgrades. For `git:` targets, `upgrade` advances already-declared Sources or Artifacts to the latest stable published **Source Version** allowed by policy. For `file:` targets, it re-reads the current local path and records the resulting snapshot hash; an unchanged snapshot is a no-op. Source scope updates every declared Artifact in the Source snapshot, while Artifact scope updates only the selected Artifact and leaves sibling Artifacts on their existing snapshots. Successful upgrade writes only the affected exact resolutions to the **Lockfile** and leaves the **Manifest** unchanged.
```

Replace the ADR-0005 heading with:

```markdown
# ADR-0005: Operation output and exit codes
```

- [ ] **Step 4: Verify the disputed contract text**

Run:

```bash
rg -n 'install --upgrade|upgrade.*file:|file:.*upgrade|latest stable published' CONTEXT.md docs/adr/0002-source-resolution-versioning-and-locking.md docs/adr/0005-operation-output-logs-and-exit-codes.md
```

Expected: `install --upgrade` appears only in the explicit statement that it is unsupported; `git:` and `file:` upgrade behavior agrees between `CONTEXT.md` and ADR-0002.

- [ ] **Step 5: Validate Markdown**

Run:

```bash
just check-md
```

Expected: exit code `0` with no markdownlint findings.

Run:

```bash
git diff --check
```

Expected: exit code `0` with no whitespace errors.

- [ ] **Step 6: Request approval and commit**

Show the diff and validation results, state the intended commit, and wait for explicit user confirmation. After confirmation, run:

```bash
git add CONTEXT.md docs/adr/0002-source-resolution-versioning-and-locking.md docs/adr/0005-operation-output-logs-and-exit-codes.md docs/superpowers/plans/2026-07-17-pr-25-review-corrections.md
git commit -m "docs: address PR 25 review feedback"
```

### Task 2: Remove obsolete diagnostics promises

**Files:**

- Modify: `docs/adr/0001-cli-surfaces-and-command-model.md:17`
- Modify: `ARCHITECTURE.md:11-15,23,29`
- Modify: `UBIQUITOUS_LANGUAGE.md:66-68`
- Modify: `CONTEXT.md:578`

**Interfaces:**

- Consumes: the immediate-only diagnostics decision in `docs/superpowers/specs/2026-07-17-pr-25-review-corrections-design.md`.
- Produces: one v1 command and reporting surface with no remaining active promise of logs, replay, or verbosity.

- [ ] **Step 1: Remove `logs` from the v1 command surface**

In ADR-0001, replace the Artifact Management Surface item with:

```markdown
- **Artifact Management Surface** centered on `install`, `upgrade`, and `search`.
```

- [ ] **Step 2: Align the architecture overview**

In `ARCHITECTURE.md`, make these exact replacements:

```markdown
- **Command surface**: parses artifact, catalog, search, and upgrade commands into explicit user intent.
- **Operation reporting**: emits short human summaries, machine-readable JSON, and exit codes.
- [ADR-0005: Operation output and exit codes](docs/adr/0005-operation-output-logs-and-exit-codes.md)
- Auditability: managed changes report provenance in immediate operation output.
```

- [ ] **Step 3: Remove obsolete glossary entries**

Delete the `Logs Command` and `Operation Log` rows from `UBIQUITOUS_LANGUAGE.md`. Replace its Operation Summary row with:

```markdown
| **Operation Summary** | The default concise human-readable output after install, Sync, or upgrade, followed only by effective or planned changes. | Full diagnostic dump, success-only silence |
```

- [ ] **Step 4: Correct the residual upgrade summary**

In `CONTEXT.md`, replace line 578 with:

```markdown
- v1 adds no extra version-selection controls beyond direct-install `--source-version`; `git:` upgrade selects the latest stable published Source Version allowed by policy, while `file:` upgrade records a fresh local snapshot hash
```

- [ ] **Step 5: Verify canonical documentation**

Run:

```bash
rg -n 'Logs Command|Operation Log|replayable operation logs|upgrade-to-latest-stable-allowed' docs/adr/0001-cli-surfaces-and-command-model.md ARCHITECTURE.md UBIQUITOUS_LANGUAGE.md CONTEXT.md
```

Expected: exit code `1`; none of these obsolete promises remain.

Run:

```bash
rg -n 'V1 has no persisted operation logs|no verbosity controls' CONTEXT.md docs/adr/0005-operation-output-logs-and-exit-codes.md
```

Expected: exit code `0`; both canonical contracts state immediate-only diagnostics.

- [ ] **Step 6: Validate Markdown and request commit approval**

Run:

```bash
just check-md
git diff --check
```

Expected: both commands exit `0`.

Show the diff and validation results. Before any commit, obtain explicit user confirmation. After confirmation, run:

```bash
git add CONTEXT.md ARCHITECTURE.md UBIQUITOUS_LANGUAGE.md docs/adr/0001-cli-surfaces-and-command-model.md docs/superpowers/specs/2026-07-17-pr-25-review-corrections-design.md docs/superpowers/plans/2026-07-17-pr-25-review-corrections.md
git commit -m "docs: align v1 diagnostics contract"
```
