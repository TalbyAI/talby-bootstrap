# GitHub validation pipeline design

## Summary

Add a GitHub Actions workflow that validates repository changes on pull requests and on pushes to `main`.

The workflow should expose separate job status for Markdown and Go validation so failures are easier to diagnose from the pull request checks UI.

## Goals

- Run validation automatically before merge through `pull_request` checks.
- Re-run validation on direct updates to `main`.
- Cancel outdated runs for the same branch or pull request.
- Reuse the repository's existing `just` tasks instead of duplicating command logic.

## Non-goals

- Release automation
- Artifact publishing
- Multi-version Go compatibility testing

## Workflow shape

Create one workflow file at `.github/workflows/ci.yml`.

Triggers:

- `pull_request` for all pull requests
- `push` for the `main` branch only

Concurrency:

- Use a concurrency group derived from workflow name and git ref
- Enable `cancel-in-progress` so newer pushes replace older runs

Jobs:

- `markdown`
  - Runs on `ubuntu-latest`
  - Checks out the repository
  - Installs Node.js because `just check-md` uses `pnpx`
  - Installs `just`
  - Runs `just check-md`
- `go`
  - Runs on `ubuntu-latest`
  - Checks out the repository
  - Installs Go using the version declared in `go.mod`
  - Installs `just`
  - Runs `just check-go`

## Rationale

One workflow keeps CI entry points simple while separate jobs keep failures legible. Using `go.mod` as the Go version source avoids drift between local development and CI. Using existing `just` tasks preserves one validation contract across local and hosted execution.
