---
name: manage-backlog
description: Manage unrefined ideas in a repository backlog. Use when the user wants to add ideas, list or prioritize backlog ideas, or remove ideas after they become features or are rejected.
---

# Manage backlog

Keep `BACKLOG.md` as the single source of truth for ideas awaiting a product decision.

## Workflow

1. Locate `BACKLOG.md` at the repository root. If no backlog exists and the user is adding an idea, create it there using the format below. Complete when exactly one backlog is selected.
2. Match the request to one operation below. Perform only that operation. Complete when every idea named by the user is accounted for.
3. Preserve unrelated entries and unresolved wording. Run the repository's Markdown check after edits. Complete when the check passes or its exact blocker is reported.
4. Report ideas added, reprioritized, removed, or listed. For removals, include whether each idea became a feature or was rejected.

## Add ideas

- Compare the idea with existing entries before editing.
- Append a new section when no equivalent idea exists.
- Enrich the existing entry when the idea is already present; keep unresolved questions unresolved.
- Use `unprioritized` unless the user supplies a priority.

Complete when each submitted idea appears once and its substantive notes are preserved.

## List ideas

Read every idea and group results in this order: `high`, `medium`, `low`, `unprioritized`. Do not modify the file.

Complete when every backlog heading appears once in the response with its priority.

## Prioritize ideas

Apply only priorities or ordering explicitly chosen by the user. If the requested ranking lacks enough criteria to determine an order, ask for that decision instead of inventing product intent.

Complete when every named idea has the requested priority and all other priorities remain unchanged.

## Remove ideas

Remove an entire idea section only when the user states that it became a feature or was rejected. If the outcome is missing, ask for it before editing. Do not keep an archive entry in the backlog.

Complete when each retired idea is absent and every unrelated section is unchanged.

## Backlog format

```markdown
# Backlog

Ideas awaiting discussion. Inclusion does not mean acceptance as a feature.

## Idea title

Priority: unprioritized

Notes preserving intent and open questions.
```
