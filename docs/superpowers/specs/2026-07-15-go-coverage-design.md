# Go coverage design

## Goal

Raise total Go statement coverage from 76.2% to at least 90%, while keeping 80% as the enforced minimum.

## Scope

- Add focused tests for meaningful public behavior and error paths, starting with the lowest-covered packages.
- Prefer `internal/repositorystate` and `internal/materialize`, then cover remaining high-value gaps only as needed.
- Do not change production behavior or add dependencies solely for coverage.
- Keep `just check-coverage` as the minimum-coverage gate.

## Verification

Run `just check-coverage` and `just coverage`. Success requires all Go tests to pass, the gate to pass, and reported total statement coverage to be at least 90%.
