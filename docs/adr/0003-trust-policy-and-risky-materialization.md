# ADR-0003: Trust Policy and Risky Materialization

## Status

Accepted for 0.1.

## Context

Talby Bootstrap writes into user repositories. The 0.1 implementation acquires local `file:` Sources and materializes whole-file `file` steps. In-root Sources are allowed by default; external absolute Sources require explicit Manifest approval of their **Source Identity**. Remote content, executable steps, and prompt-driven changes are deferred.

## Decision

The base **Trust Policy** lives in the versioned **Manifest**. Local machine settings may harden this policy but must not silently weaken it.

The persistence contract recognizes `file:` and `git:` Source References. 0.1 acquires `file:` Sources: in-root Sources are allowed by default, while external absolute Sources require explicit Manifest approval of their **Source Identity**. `git:` identities are stored but not acquired.

Trust approval is evaluated against **Source Identity**, not the published **Source Descriptor** alone. Broader trust policy, risky step allowlists, first-install confirmation, and **Minimum Age Rule** behavior are deferred with their source and step implementations.

## Consequences

- Trust decisions are reviewable in repository history.
- The 0.1 boundary prevents remote acquisition and executable or prompt-driven changes from reaching materialization.
- Future acquisition channels will require their own explicit trust decisions.
