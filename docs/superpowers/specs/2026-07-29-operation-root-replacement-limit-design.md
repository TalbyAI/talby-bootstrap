# Operation Root replacement recovery limit design

## Status

Proposed follow-up documentation for issue #34.

## Context

An external process can rename the acquired Operation Root and create a different directory at its former path after `tbboot` has started mutating files. The 0.1 rollback implementation retains the acquired root identity and detects this replacement before restoration or Recovery State persistence.

The replacement is an unusual external topology race. Supporting recovery through the moved directory would require retaining and using an opened rooted handle across materialization, rollback, repository-state persistence, and verification. That expansion is outside the intended issue #34 scope.

## Decision

Product 0.1 keeps the current safe-stop behavior:

- `tbboot` never restores files, removes directories, or writes Recovery State through the replacement path.
- `tbboot` does not search for, restore, or write Recovery State into the moved original root.
- The operation returns sanitized operational exit class `1`. Wrapped causes remain available to internal `errors.Is` and `errors.As` inspection.
- The existing public-service regression test continues to prove that the replacement root receives no Recovery State.

This behavior is a safety boundary, not successful rollback. Recovery State cannot satisfy its canonical repository-root contract after the Operation Root identity is lost, so `tbboot` reports that recovery state could not be written instead of claiming protected recovery.

## Documentation ownership

- ADR-0004 will describe the topology-race scenario, current safe-stop behavior, and possible rooted-handle evolution.
- `ARCHITECTURE.md` will summarize that replacement of the acquired Operation Root stops rollback and Recovery State persistence without touching the replacement.
- No implementation comment is added; the ADR and regression test own the rationale and executable evidence.

## Deferred evolution

A separate follow-up issue will track recovery through an opened rooted handle. That work must address cross-platform handle semantics, materialization and repository-state writes, reverse rollback, created-directory cleanup, Recovery State placement, and verification after the original root has moved.

The follow-up must not weaken the invariant that an unrelated replacement directory remains untouched.

## Acceptance criteria

- Canonical documentation distinguishes safe stop from successful rollback when the Operation Root identity changes.
- ADR-0004 contains the detailed rationale and future upgrade path.
- `ARCHITECTURE.md` contains only the concise system-level boundary.
- A separate future issue tracks rooted-handle recovery.
- Product behavior and existing tests remain unchanged.
