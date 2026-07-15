# Superpowers issue draft

Template chosen: `feature_request.md`

Why this template:

- This is better framed as a prompt or workflow improvement than a platform/runtime failure.
- The current skill says "one question at a time", but it does not explicitly define how to handle context-only replies.
- The requested change is an additional guardrail in `skills/brainstorming/SKILL.md`, not a harness bug fix.

Suggested title:

`brainstorming: keep the current question open when the user asks for context instead of answering`

Issue body, formatted to match `feature_request.md`:

```md
- [ ] I searched existing issues and this has not been proposed before

## What problem does this solve?

The `brainstorming` skill says to ask one question at a time, but it does not explicitly say what to do when the user cannot answer yet and instead asks for more context about the current question.

In practice, this can produce a confusing flow:

1. The agent asks question A.
2. The user asks for more context, implications, or trade-offs about question A.
3. The agent provides that context.
4. The agent then immediately asks question B.

From the user's point of view, question A is still open. The user did not answer it yet; they only asked for help understanding it. Advancing anyway breaks the one-question-at-a-time intent and makes the brainstorming flow feel like a checklist instead of a guided decision process.

This matters most during design work, where the current question often needs clarification before the user can make a choice. If the skill moves on too early, it weakens the validation loop and creates avoidable confusion.

## Proposed solution

Add an explicit rule to `skills/brainstorming/SKILL.md` saying that a request for more context does not count as an answer to the current question.

Proposed wording:

> **Do not advance past an unanswered question.** If the user asks for more context, implications, examples, or trade-offs about the current question, treat that as clarification on the same question rather than progress to the next one. Answer only the clarification request, then restate and re-ask the same decision in clearer words. Do not introduce a new design question until the user has explicitly answered the current one.

Optional stronger wording:

> A request for more context is not an answer. It does not satisfy the one-question-at-a-time rule. The current question remains open until the user chooses an option or gives an explicit decision.

Expected behavior after this change:

- The agent asks question A.
- The user asks for context.
- The agent gives context only.
- The agent reformulates and re-asks question A.
- The agent waits.

## What alternatives did you consider?

1. Leave the current prompt as-is.

   This keeps the skill shorter, but it leaves too much ambiguity in a common interaction pattern. "One question at a time" is not enough by itself, because it does not define whether a clarification request closes the current question.

2. Treat this as a bug in a specific harness or model instead of a skill issue.

   I do not think that is the best framing. The problematic behavior follows naturally from a missing instruction in the skill itself, so the most direct fix is to tighten the prompt.

3. Add the guardrail to platform-adaptation docs instead of `brainstorming/SKILL.md`.

   That may help for some harness-specific question tools, but the core issue is flow control inside brainstorming. The primary instruction belongs in the brainstorming skill.

## Is this appropriate for core Superpowers?

Yes.

This is not tied to a specific project domain, repo layout, or third-party tool. It improves the default quality of brainstorming conversations in any context where the user may need clarification before deciding. The rule strengthens the existing one-question-at-a-time behavior rather than introducing a new workflow specific to one platform.

## Environment (required)

| Field | Value |
|-------|-------|
| Superpowers version | 6.1.1 cached curated copy observed in Codex session |
| Harness (Claude Code, Cursor, etc.) | Codex |
| Harness version | Not captured in the session |
| Your model + version | GPT-5 Codex |
| All plugins installed | Not fully enumerated in the session |

## Context

This came up in a real brainstorming session while designing phase 1 of a roadmap in a live repository. The flow was:

- agent asked a design decision question;
- user asked for more context before answering;
- agent gave the context;
- agent then moved on to a new question.

That made the conversation harder to follow because the original decision was still unresolved.

Related issues reviewed before filing:

- #849 `Brainstorming skill asks generic questions despite rich project context`
  - Related because it is also about weak question selection in brainstorming.
  - Not a duplicate because it focuses on asking questions whose answers already exist in context, not on advancing after a context-only reply.

- #1773 `brainstorming: v6.0.0 platform-neutral bootstrap reframe makes Claude Code use AskUserQuestion`
  - Related because it discusses how brainstorming questions are delivered and preserves one-at-a-time conversational flow.
  - Not a duplicate because it is about tool routing and presentation, not about whether the current question remains open.

- #1642 `Multi-agent parallel code review + adaptive batched questioning in brainstorming`
  - Related because it discusses questioning strategy in brainstorming.
  - Not a duplicate because it is about batching questions for simple tasks, not about clarification on the current question.

- #388 `Brainstorm skill: Use the AskUserQuestion tool`
  - Related only at the level of brainstorming question UX.
  - Not a duplicate because it is about using a specific question tool, not question-state handling.

- #114 `Use the native AskUserQuestion tool to ask questions in the UI`
  - Related only at the level of question delivery.
  - Not a duplicate because it does not address clarification versus answer state.

- #530 `Feature Request: brainstorming skill should proactively challenge product assumptions`
  - Related because it proposes a refinement to brainstorming behavior.
  - Not a duplicate because it is about adding assumption-challenging behavior, not preventing premature advance.
```

Related issue links:

- [#849](https://github.com/obra/superpowers/issues/849)
- [#1773](https://github.com/obra/superpowers/issues/1773)
- [#1642](https://github.com/obra/superpowers/issues/1642)
- [#388](https://github.com/obra/superpowers/issues/388)
- [#114](https://github.com/obra/superpowers/issues/114)
- [#530](https://github.com/obra/superpowers/issues/530)

Template sources used:

- [feature_request.md](https://github.com/obra/superpowers/blob/main/.github/ISSUE_TEMPLATE/feature_request.md)
- [bug_report.md](https://github.com/obra/superpowers/blob/main/.github/ISSUE_TEMPLATE/bug_report.md)
- [platform_support.md](https://github.com/obra/superpowers/blob/main/.github/ISSUE_TEMPLATE/platform_support.md)
