# Superpowers and Ponytail compatibility check

Date: 2026-07-07

## Question

If both plugins are installed for Codex, do they affect how the agent responds to coding requests, and do they conflict with each other?

## Sources

- Superpowers repository: <https://github.com/obra/superpowers>
- Ponytail repository: <https://github.com/DietrichGebert/ponytail>
- Local installed skill files:
  - `superpowers/skills/using-superpowers/SKILL.md`
  - `superpowers/skills/brainstorming/SKILL.md`
  - `ponytail/skills/ponytail/SKILL.md`
- Local plugin manifests and hook configs:
  - `superpowers/.codex-plugin/plugin.json`
  - `ponytail/.codex-plugin/plugin.json`
  - `ponytail/hooks/claude-codex-hooks.json`
- Superpowers release notes

## Findings

- Yes, both plugins are behavior-shaping plugins. They do not just add tools; they change how the agent approaches coding work.

### Ponytail

- Ponytail is explicitly persistent:
  - `ACTIVE EVERY RESPONSE.`
- Its main effect is to force a minimalism policy:
  - YAGNI first
  - reuse existing code first
  - standard library before new code
  - native platform features before dependencies
  - shortest working diff
- It also changes response style:
  - code first
  - at most three short lines of explanation unless asked for more
- Its Codex/Claude hook config shows lifecycle hooks for startup, subagent start, and prompt submit.

### Superpowers

- Superpowers is primarily a workflow framework:
  - check skills before acting
  - brainstorm before implementation
  - write a design
  - write a plan
  - use TDD during implementation
  - review and verify before completion
- Its `using-superpowers` bootstrap says skills must be invoked before any response or action when relevant.
- Its `brainstorming` skill hard-gates implementation:
  - no code or implementation action until a design is presented and approved
- In the Codex plugin manifest tested here, `hooks` is explicitly `{}`. Superpowers release notes say this was done to stop Codex from auto-registering the Claude SessionStart hook.

## Conflict analysis

### No obvious technical/plugin-command conflict

- I found no sign of command-name collision as the primary issue.
- Superpowers release notes explicitly say plugin-provided commands are namespaced to avoid conflicts.
- Ponytail skills are also namespaced in Codex with `@ponytail...`.

### Real behavioral conflict

These plugins do conflict at the instruction level for many coding requests.

- Superpowers says:
  - invoke workflow skills before acting
  - brainstorm before implementation
  - do not write code until the design is approved
- Ponytail says:
  - active every response
  - code first
  - shortest diff wins
  - explanation should be extremely short
  - if the request is complex, ship the lazy version and question it in the same response

Those defaults pull in different directions on a typical "please change this code" request.

## Net assessment

- They are not naturally complementary as equal always-on plugins.
- Ponytail is a coding-style governor.
- Superpowers is a process governor.
- If both are active, the likely result is not a clean merge of philosophies but unstable prioritization depending on harness behavior and prompt ordering.

## Most likely behavior in Codex

- Ponytail is more obviously always-on in Codex because it ships lifecycle hooks for activation and mode tracking.
- Superpowers in Codex appears less hook-driven and more dependent on bootstrap/native skill discovery behavior.
- That means Ponytail may dominate day-to-day response style unless Superpowers is explicitly invoked or the harness injects its bootstrap separately.

## Public evidence search

- I looked for public reports, issues, or discussions about people running Ponytail and Superpowers together.
- I did not find an indexed public writeup or issue thread documenting combined use or a known compatibility pattern.
- I also found no obvious cross-reference between the two projects in their own docs or repo content.

## Practical recommendation

- If you want strict process discipline, install Superpowers and use Ponytail only as an explicit on-demand skill.
- If you want default minimalism and short diffs, install Ponytail and do not expect Superpowers' full design-first workflow to remain consistently in control.
- I would not rely on both as simultaneously always-on defaults without testing the exact harness behavior you care about.
