# Repository Guidelines

## Project Structure & Module Organization

This repository contains both Talby Bootstrap product documentation and the Go implementation of the `tbboot` CLI. Top-level files such as `CONTEXT.md`, `ARCHITECTURE.md`, and `UBIQUITOUS_LANGUAGE.md` hold core product language and decisions. Architecture decisions live in `docs/adr/`, design specs and plans live in `docs/superpowers/specs/` and `docs/superpowers/plans/`, and review and research artifacts live under `docs/reviews/` and `docs/research/`.

Go entrypoint code lives in `main.go`. CLI surface area lives in `cmd/tbboot/`. Reusable command-independent behavior belongs under `internal/`, grouped by domain such as `internal/install/`, `internal/source/`, and `internal/app/`.

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

`just` prints available tasks. `just check-md` runs `npx -y markdownlint-cli2 .` across Markdown files. `just fix-md` applies markdownlint auto-fixes where possible. `just check-go` runs Go tests, `just fmt-go` formats Go files, and `just check` runs Markdown and Go checks. These commands require `just`, PowerShell, Go, and Node/npm tooling on the machine.

## Coding Style & Naming Conventions

Markdown is the primary format. Use sentence-case headings, short sections, and repository terms from `CONTEXT.md` exactly, such as **Artifact**, **Source**, **Manifest**, **Lockfile**, and **Materialization Record**. Prefer concise prose over speculative implementation detail.

ADR files use numbered names in `docs/adr/`, for example `0005-operation-output-logs-and-exit-codes.md`. New specs should use dated, descriptive names in `docs/superpowers/specs/`, matching the existing pattern.

Markdown linting is configured in `.markdownlint-cli2.yaml`; line length is intentionally disabled, and duplicate headings are allowed only across different sibling scopes.

Go code should keep package boundaries shallow and explicit. Follow these conventions:

- Keep `main.go` minimal: it should start the CLI and return the process exit code.
- Keep `cmd/tbboot/` focused on Cobra wiring, argument parsing, flag handling, and rendering command output.
- Put command-independent behavior in `internal/<domain>/` packages with small services and explicit `Request` and `Result` types.
- Model extensibility behind interfaces and registries in `internal/` when behavior varies by type or backend. Prefer explicit lookup points over command-level branching.
- Validate inputs early and return direct, user-readable errors that work for both human output and JSON output.
- When adding machine-readable output, reuse the shared `internal/app.Result` envelope and keep success and error shapes consistent across commands.

## Testing Guidelines

Run `just check` before submitting changes. For documentation-only changes, `just check-md` is enough. For Go changes, run `just check-go` at minimum.

For CLI changes, test exit codes and stdout/stderr behavior, including JSON mode when applicable. For `internal/` packages, prefer table-free, focused tests that cover both unit seams with fakes and at least one real-path integration-style case when local file resolution or descriptor parsing is central to the behavior.

## Commit & Pull Request Guidelines

Recent history uses conventional-style subjects such as `chore: Add architecture documentation and ADRs`. Keep commit messages short, imperative, and scoped to the actual change.

Pull requests should include a brief summary, the reason for the change, and validation performed, usually `just check-md`. Link related issues or decision records when applicable. Include screenshots only for rendered HTML artifacts in `reference/` or `lessons/`.

## Security & Configuration Tips

Do not commit local secrets, keys, `.env` files, or generated local state. `.gitignore` excludes `.local-*`, `.tmp-*`, `tmp/`, and `.superpowers/`. The `tmp/` and `.superpowers/` directories are used by repository tooling; keep their generated contents untracked.

## Agent-Specific Instructions

Agents should use the `caveman` skill in `lite` mode by default for responses unless the user explicitly asks for normal verbosity or a task requires fuller explanation.

During brainstorming, a request for context, implications, examples, or trade-offs does not answer the current question. Answer the clarification, then restate the same question and wait for an explicit decision. Do not advance to another design question while the current one remains unanswered.

Agents must not create commits without asking the user for confirmation at that moment and receiving an explicit approval. Prior general permission is not enough; the agent must pause, state that it intends to create a commit, and wait for a clear user confirmation before running the commit. Agents may still suggest concise commit messages when helpful.

## Agent skills

### Issue tracker

Issues are tracked in GitHub; external pull requests are not a triage surface. See `docs/agents/issue-tracker.md`.

### Triage labels

Use `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repository with `CONTEXT.md` and `docs/adr/`. See `docs/agents/domain.md`.

### Grilling interviews

In this repository, when `/grilling` runs directly or through another skill,
these repository-local rules override its one-question-at-a-time instruction.

- Ask questions in numbered batches of 5 to 10. Default to 7.
- Include the recommended answer or default beside every question.
- Only group questions that can be answered from the currently known context.
Do not ask downstream questions whose meaning depends on an unresolved answer.
- Find repository facts yourself. Ask the user for decisions, preferences, and
information that cannot be discovered locally.
- The user may answer all, some, or none of the questions.
- After each response:

1. Record the decisions the user resolved.
2. Answer any requests for explanation or additional detail.
3. Keep unanswered decisions open; never accept a recommendation silently.
4. Reword questions that were unclear.
5. Add newly unblocked questions until the next batch contains 5 to 10
    questions.

- When fewer than 5 meaningful questions remain, ask only those remaining.
Never invent filler questions to reach the batch minimum.
- If the user explicitly accepts a recommendation, treat that question as
resolved.
- Continue until no material decisions remain, then present the resulting
shared understanding for confirmation.
- Do not plan or implement the result until the user confirms that shared
understanding.
