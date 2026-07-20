---
name: session-assumptions
description: Analyze the current conversation/session and list agent-made assumptions, inferred decisions, defaults, or interpretations that shaped the result but were not explicitly present in the user's instructions, supplied documents, AGENTS.md, repository context, codebase, or tool outputs. Use when the user invokes $session-assumptions, asks what assumptions were made, asks what decisions the agent introduced, or wants a flat reviewable list of unstated choices behind the current session output.
---

# Session Assumptions

Surface agent-added choices that merit discussion; provide the full inventory
when requested.

## Modes

- `$session-assumptions`: return only items with `discussion-value: high`.
- `$session-assumptions all`: return every meaningful item.

## Process

1. Review the current session result and the path taken to produce it.
2. Treat these as prior information:
   - user messages and explicit requirements
   - attached or quoted specs, designs, tickets, and docs
   - AGENTS.md and repository instructions
   - relevant codebase facts and tool outputs
3. Identify choices, defaults, interpretations, scope decisions, ordering choices, naming choices, omitted alternatives, and assumptions that were not directly stated in prior information.
4. Exclude facts copied from prior information.
5. Exclude routine execution details unless they changed the output.
6. Assign `confidence` from certainty that the agent introduced the item.
7. Assign `discussion-value` from usefulness of reviewing the item before
   continuing:
   - `high`: questionable, consequential, or needs user agreement
   - `medium`: useful context, but no decision is currently needed
   - `low`: harmless implementation discretion
8. In default mode, keep only items with `discussion-value: high`. In `all`
   mode, keep every meaningful item.

## Output

Return a flat Markdown list. Keep each item short.

Use this shape:

```markdown
- decision: <agent-added choice>
  basis: <why the agent likely chose it>
  confidence: high|medium|low
  discussion-value: high|medium|low
  impact: <what it affected>
```

If default mode has no high-value items, say:

```markdown
No high-value assumptions to discuss.
```

If `all` mode has no meaningful agent-added decisions, say:

```markdown
No meaningful unstated assumptions found.
```

If session context is insufficient to separate prior information from
agent-added choices, say that first. In default mode, list only uncertain items
whose consequences still give them `discussion-value: high`; in `all` mode,
list every likely item with `confidence: low`.
