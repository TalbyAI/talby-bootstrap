---
name: gh-review-triage
description: Analyze GitHub pull request review comments, classify them against the current repo state, document them for follow-up, and optionally resolve valid comments with minimal verified changes. Use when a user asks to read PR comments, find unresolved reviews, triage review findings, decide which comments still apply, prepare a review summary document, or apply fixes and close out review feedback efficiently.
---

# GH review triage

Use a workflow-first approach. Start by establishing the PR context, then inspect review threads, then classify each comment against the current repository state before proposing or applying any fix.

When saving an analysis, optimize for a later agent or reviewer who wants to finish the PR review with minimal re-reading.

## Workflow

### 1. Establish PR context

- Identify the current branch.
- Find the associated GitHub PR. Prefer the PR for the current branch; if the user names a PR number, use that directly.
- Confirm repository owner/name and PR URL before summarizing review content.
- Prefer `gh` CLI and GitHub GraphQL for review threads so you can distinguish unresolved, resolved, and outdated comments.

Read [references/gh-queries.md](references/gh-queries.md) when you need example `gh` commands or GraphQL queries.

### 2. Gather review material

- Fetch unresolved review threads first.
- Capture enough detail to work later without reopening every thread:
  - author
  - severity/category if present
  - file path and line
  - timestamp
  - URL
  - `isResolved`
  - `isOutdated`
  - full substantive body
  - suggested diff or prompt block if present
- Preserve the initial comment body in full when it contains the core claim, a suggested diff, or an AI-agent prompt block.
- If a thread has replies that materially change the interpretation, summarize that thread context in the saved artifact.
- If the user asks for "all comments", gather every review thread, not only unresolved ones.

### 3. Compare each comment with current state

- Open the referenced file and inspect the cited area.
- Verify whether the comment still matches the current code or docs.
- Treat every review comment as a claim that must be checked, not blindly accepted.
- Record one of three classifications:
  - `ready-to-apply`
  - `needs-user-input`
  - `not-applicable`

Read [references/decision-rubric.md](references/decision-rubric.md) before classifying.

### 4. Explain the classification

For every comment, include:

- what the reviewer is claiming
- what the current file actually says or does
- whether the comment is still valid
- why it falls into the chosen classification
- the next concrete action

Use these meanings:

- `ready-to-apply`: direct execution item; behavior or wording is already constrained enough by the repo, spec, or codebase patterns.
- `needs-user-input`: the change implies a product, contract, scope, or wording decision that should be confirmed with the user.
- `not-applicable`: do not implement the comment. Use this when the thread is outdated, the issue was already fixed, or the suggestion is unsound for this repo.

For `not-applicable`, always state one explicit reason:

- `outdated`: later changes invalidated the comment
- `rejected`: the suggestion should not be applied on technical or product grounds

### 5. Produce a reusable artifact

When the user asks to save the analysis, write a Markdown document in the repo with:

- PR metadata
- review-completion checklist
- classification rubric used
- summary table
- quick wins list
- detailed entry per comment
- explicit decision queue for the user

The review-completion checklist should be an initially unchecked list that makes it obvious what remains before the PR review can be considered closed. Include only items that are actually pending in the current PR state.

Typical checklist items:

- confirm each `needs-user-input`
- implement remaining `ready-to-apply` items
- validate touched files or commands
- reply to or resolve GitHub threads
- document rejections for `not-applicable`

For each detailed entry, include:

- comment metadata
- current-state summary in plain language
- current-state verification
- classification
- reasoning
- proposed action

Comment metadata should include:

- author
- severity/category when present
- file path and cited line
- timestamp
- URL
- `isResolved`
- `isOutdated`

Current-state verification should be reproducible. Prefer referencing the current file and line inspected, not only prose like "still present".

If the review comment includes a suggested diff or concrete replacement text, preserve it in the artifact when it helps the next implementation step.

Keep the document operational. It should help a later agent apply fixes or justify rejecting a comment.

### 6. Resolve comments when asked

Only after analysis, apply changes for comments that are still valid and do not require user input.

- Keep edits minimal and local.
- Prefer existing repo patterns over new abstractions.
- Add or update tests when behavior changes.
- For comments classified `needs-user-input`, stop and ask only for the concrete missing decision.
- For comments classified `not-applicable`, document the rejection reason instead of forcing a change.

### 7. Validate before claiming completion

- Run the narrowest relevant checks first.
- For documentation-only updates, run Markdown checks if available.
- For code changes, run the relevant language tests and formatting checks that cover the touched surface.
- Report what was validated and what was not.

## Operating rules

- Prefer GitHub primary data (`gh pr view`, `gh api graphql`) over scraped web output when the task is about PR review state.
- Distinguish unresolved from outdated comments; do not present them as equivalent.
- Preserve the user's requested language in the output document unless the repo conventions clearly require otherwise.
- If a review comment includes a suggested patch, evaluate it against the current code rather than assuming it is correct.
- If the review bot or reviewer cites a test gap, check whether the test already exists before treating it as an open action.
- If a previous review-analysis document already exists, treat it as a secondary artifact for comparison, not as the primary source of PR state.
- Prefer normalized classification labels exactly as written in this skill over custom narrative labels.

## Output shapes

For quick status requests, return:

- PR identified
- number of unresolved comments
- short list of files/themes

For saved analysis, prefer:

1. PR metadata
2. review-completion checklist
3. summary table
4. quick wins list
5. detailed entries
6. explicit decision queue for the user

### Saved-analysis expectations

Use this structure by default:

1. `PR metadata`
2. `Review completion checklist`
3. `Classification rubric`
4. `Summary table`
5. `Quick wins`
6. `Detailed entries`
7. `Decision queue`

`Quick wins` should list the comments classified as `ready-to-apply`, preferably grouped or ordered so a follow-up agent can execute them efficiently.

`Decision queue` should include only the comments classified as `needs-user-input`, each phrased as the exact decision the user still needs to make.

## References

- [references/decision-rubric.md](references/decision-rubric.md): classification rules and edge cases
- [references/gh-queries.md](references/gh-queries.md): reusable `gh` and GraphQL query patterns
