# Decision rubric

Use this rubric to classify each review comment after checking the current repository state.

## `ready-to-apply`

Use when the next action is direct execution and does not need a fresh product decision.

Common cases:

- typo, grammar, accents, sentence-case, line endings, escaping, lint cleanup
- test or validation gap where expected behavior is already defined
- bug fix where current repo behavior clearly contradicts the intended contract
- cleanup of leaked temp files, missing stderr output, or similar local correctness issues

Do not use this label just because the fix is small. The key question is whether the intended outcome is already determined.

## `needs-user-input`

Use when the comment implies a decision you should not lock in silently.

Common cases:

- wording that defines canonical product vocabulary
- behavior that changes a public contract, JSON envelope, CLI semantics, or policy surface
- scope choices between two plausible interpretations
- ambiguous suggestions where the repo provides no clear tie-breaker

When using this label, state the exact decision needed in one sentence.

## `not-applicable`

Use when the comment should not be implemented.

Common cases:

- the file has changed and the comment is now outdated
- the issue is already fixed in the current branch
- the suggestion is technically wrong, incomplete, or harmful in this repo
- the reviewer inferred a problem that is not present after inspection

Split the reason explicitly:

- `outdated`: later changes invalidated the comment
- `rejected`: the suggestion should be declined on technical or product grounds

## Required note per comment

For every classified comment, record:

1. the reviewer claim
2. the current-state evidence
3. the classification
4. the reason
5. the recommended next action

## Escalation rule

If you are less than confident that a comment is `not-applicable`, do not reject it casually. Re-open the cited file, inspect surrounding code or text, and explain the uncertainty. When the uncertainty is about product intent rather than technical validity, classify as `needs-user-input`.
