---
name: gh-create-pr
description: Analyze the current branch against its base branch, propose a pull request title and description for user review, and publish the pull request with GitHub CLI only after explicit confirmation. Use when the user wants to open a GitHub pull request from the current worktree, asks for PR title/body help, or wants a safe review-then-publish PR workflow.
---

# GH create PR

Use a confirmation-gated workflow. First inspect branch state and diff scope, then draft the pull request metadata for review, and only create the PR after the user explicitly approves publication.

## Workflow

### 1. Establish branch and PR context

- Identify the current branch.
- Identify the default base branch from the remote. Prefer the branch named by the user; otherwise default to the remote HEAD branch, usually `main`.
- Check whether a PR already exists for the current branch with `gh pr status` or `gh pr view --head <branch>`.
- If a PR already exists, stop and report it instead of creating a duplicate.

### 2. Analyze the current branch against the base

- Resolve the base ref before summarizing changes.
- Inspect:
  - `git log --oneline <base>..HEAD`
  - `git diff --stat <base>...HEAD`
  - the touched files when needed for scope and risk
- Summarize the branch in terms of:
  - primary purpose
  - major change areas
  - validation already run or still missing
  - notable risks, non-goals, or deferred work worth mentioning in the PR body

Keep the analysis focused on what a reviewer needs to know, not on a file-by-file changelog.

### 3. Draft the PR metadata

Propose both:

- one concise PR title
- one PR description in Markdown

The draft should usually contain:

- `Summary`
- `What changed`
- `Validation`

Add a short note only when there is an important scope limitation, deferred follow-up, or reviewer caveat.

Prefer accurate scope over marketing language. If the branch is scaffold or groundwork rather than full feature delivery, say so clearly.

### 4. Review gate

Show the proposed title and description to the user before publishing.

- Do not create the PR in the same step as the draft unless the user explicitly asked for immediate publication and also confirms the final draft.
- If the user requests edits, revise the draft and show the updated version again.
- Only proceed when the user gives explicit approval to publish.

### 5. Publish the PR

Once approved:

- create the PR with `gh pr create --base <base> --head <branch> --title ... --body ...`
- return the PR URL

If GitHub rejects the creation because a PR already exists, surface the existing PR URL instead of retrying blindly.

## Operating rules

- Prefer the narrowest git inspection that still explains the branch well.
- Use three-dot diff (`<base>...HEAD`) for change summary against the merge base.
- Avoid claiming validation passed unless the relevant command was run in the current session or the user explicitly provided the result.
- Keep PR descriptions concise; save deeper rationale to repo docs when needed.
- If the branch includes unrelated changes or an unexpectedly dirty worktree, call that out before drafting the PR.
- Never publish without explicit user confirmation after showing the draft.

## Output shape

Before approval, return:

- base branch
- branch summary
- proposed title
- proposed description
- explicit question asking whether to publish

After approval, return:

- created PR URL
- final title

