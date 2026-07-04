# Superpowers Software Development Resources

## Knowledge

- Local skill: `superpowers:using-superpowers`
  Defines the rule for invoking relevant skills before acting. Use for: deciding whether a skill applies.
- Local skill: `superpowers:brainstorming`
  Turns an idea into an approved design/spec before implementation. Use for: conceptualizacion, product shaping, design approval.
- Local skill: `superpowers:writing-plans`
  Converts an approved spec into a detailed implementation plan. Use for: breaking work into testable, agent-executable tasks.
- Local skill: `superpowers:using-git-worktrees`
  Sets up isolated implementation workspaces. Use for: starting feature work safely.
- Local skill: `superpowers:test-driven-development`
  Enforces red-green-refactor. Use for: features, bug fixes, behavior changes.
- Local skill: `superpowers:subagent-driven-development`
  Executes a plan with fresh implementation and review subagents per task. Use for: multi-task implementation in this Codex environment.
- Local skill: `superpowers:executing-plans`
  Executes a written plan inline when subagents are not available or not desired. Use for: simpler or constrained sessions.
- Local skill: `superpowers:requesting-code-review`
  Requests focused review using exact context and git SHAs. Use for: task checkpoints and pre-merge review.
- Local skill: `superpowers:receiving-code-review`
  Evaluates review feedback technically before implementing. Use for: handling reviewer comments without blindly applying them.
- Local skill: `superpowers:verification-before-completion`
  Requires fresh verification before claiming completion. Use for: final status, commits, PRs, merges.
- Local skill: `superpowers:finishing-a-development-branch`
  Guides final branch handling after tests pass. Use for: merge, PR, keep branch, or discard decisions.

## Wisdom (Communities)

- Pairing with Codex in this workspace
  Use for: practicing the lifecycle on real tasks, not toy examples.

## Gaps

- Add project-specific examples after the first real feature is taken through the workflow.
