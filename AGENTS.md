# Repository Guidelines

## Project Structure & Module Organization

This repository is currently documentation-first for the Talby Bootstrap CLI design. Top-level files such as `CONTEXT.md`, `ARCHITECTURE.md`, `MISSION.md`, `NOTES.md`, `RESOURCES.md`, and `UBIQUITOUS_LANGUAGE.md` hold the core product language and decisions. Architecture decisions live in `docs/adr/` and design specs live in `docs/superpowers/specs/`. Learning artifacts are under `learning-records/`, rendered references under `reference/`, and lessons under `lessons/`.

Go implementation code lives in `main.go`, `cmd/tbboot/`, and `internal/`. Keep Cobra-specific parsing in `cmd/tbboot/`; command-independent behavior belongs under `internal/`.

## Build, Test, and Development Commands

Use `just` for repository tasks:

```sh
just
just check-md
just fix-md
just check-go
just fmt-go
just check
```

`just` prints available tasks. `just check-md` runs `pnpx markdownlint-cli2 .` across Markdown files. `just fix-md` applies markdownlint auto-fixes where possible. `just check-go` runs Go tests, `just fmt-go` formats Go files, and `just check` runs Markdown and Go checks. These commands require `just`, PowerShell, Go, and `pnpx`/Node tooling on the machine.

## Coding Style & Naming Conventions

Markdown is the primary format. Use sentence-case headings, short sections, and repository terms from `CONTEXT.md` exactly, such as **Artifact**, **Source**, **Manifest**, **Lockfile**, and **Materialization Record**. Prefer concise prose over speculative implementation detail.

ADR files use numbered names in `docs/adr/`, for example `0005-operation-output-logs-and-exit-codes.md`. New specs should use dated, descriptive names in `docs/superpowers/specs/`, matching the existing pattern.

Markdown linting is configured in `.markdownlint-cli2.yaml`; line length is intentionally disabled, and duplicate headings are allowed only across different sibling scopes.

## Testing Guidelines

Run `just check` before submitting changes. For documentation-only changes, `just check-md` is enough. For Go changes, run `just check-go` at minimum.

## Commit & Pull Request Guidelines

Recent history uses conventional-style subjects such as `chore: Add architecture documentation and ADRs`. Keep commit messages short, imperative, and scoped to the actual change.

Pull requests should include a brief summary, the reason for the change, and validation performed, usually `just check-md`. Link related issues or decision records when applicable. Include screenshots only for rendered HTML artifacts in `reference/` or `lessons/`.

## Security & Configuration Tips

Do not commit local secrets, keys, `.env` files, or generated local state. `.gitignore` already excludes `.local-*` and `.tmp-*`; keep machine-specific files in those patterns.

## Agent-Specific Instructions

Agents must never commit changes in this repository. They may suggest concise commit messages, but the user must review the diff and perform any commit manually.
