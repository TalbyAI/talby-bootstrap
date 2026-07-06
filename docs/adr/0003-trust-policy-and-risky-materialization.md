# ADR-0003: Trust Policy and Risky Materialization

## Status

Accepted for v1.

## Context

Talby Bootstrap writes into user repositories. Remote content, executable steps, and prompt-driven changes need explicit trust decisions before materialization.

## Decision

The base **Trust Policy** lives in the versioned **Manifest**. Local machine settings may harden this policy but must not silently weaken it.

V1 supports only `file` and `git` as approvable **Source Types**.

- `file:` sources are allowed by default only when they point inside the current **Operation Root**.
- `git:` sources always require explicit approval in the **Manifest**.

Source approval does not approve every behavior delivered by that source. Risky **Materialization Step Types** require explicit allowlisting and first-install confirmation. In v1, `script` and `prompt` are risky step types.

When a source type can prove publication time, a **Minimum Age Rule** may reject resolved targets that are too new.

## Consequences

- Trust decisions are reviewable in repository history.
- Approving a source does not silently approve executable or prompt-driven changes.
- The first version avoids source-type sprawl while the trust model is still small.
- Policy denial uses a distinct exit code so automation can classify it.
