# PR 43 review comment fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the unsafe-topology regression plan with the implemented targetless `install` CLI operation.

**Architecture:** Documentation-only change. Keep `Sync` as the internal reconciliation operation and describe the public CLI invocation as targetless `install`.

**Tech Stack:** Markdown, `just check-md`, Git.

## Global Constraints

- Modify only `docs/superpowers/plans/2026-07-21-pr-review-remediation.md`.
- Keep `TestSyncJSONIncludesUnsafeTopologyConflict` unchanged.
- Add no CLI command, production code, test code, dependency, or abstraction.
- Leave the GitHub review thread unresolved until validation completes.
- Do not create a commit without explicit user approval at that moment.

---

### Task 1: Correct the remediation-plan terminology

**Files:**

- Modify: `docs/superpowers/plans/2026-07-21-pr-review-remediation.md:53`

**Interfaces:**

- Consumes: Existing targetless `tbboot install` CLI behavior and internal `Service.Sync` terminology.
- Produces: A plan sentence that names the public operation accurately while retaining the internal-operation context.

- [ ] **Step 1: Replace the mismatched operation wording**

Change the sentence fragment:

```text
Run JSON targetless Sync and assert exit 2, empty stdout, a JSON stderr envelope, and a conflict with kind unsafe_topology and path nested/a.
```

to:

```text
Run targetless `install` in JSON mode (the CLI's Sync reconciliation path) and assert exit 2, empty stdout, a JSON stderr envelope, and a conflict with kind unsafe_topology and path nested/a.
```

Leave the test name and executable command unchanged.

- [ ] **Step 2: Run the Markdown check**

Run:

```sh
just check-md
```

Expected: PASS.

- [ ] **Step 3: Check the diff for whitespace errors**

Run:

```sh
git diff --check HEAD
```

Expected: no output and exit code 0.

- [ ] **Step 4: Review the resulting change**

Confirm the diff contains only the terminology correction in the remediation plan. Do not change Go files or resolve the GitHub thread in this task.

- [ ] **Step 5: Commit after explicit user approval**

After the user explicitly authorizes the commit, run:

```sh
git add docs/superpowers/plans/2026-07-21-pr-review-remediation.md
git commit -m "docs: align PR 43 remediation terminology"
```
