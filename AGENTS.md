# Repository Guidelines

## Project Structure & Module Organization

This repository contains both Talby Bootstrap product documentation and the Go implementation of the `tbboot` CLI. Top-level files such as `CONTEXT.md`, `ARCHITECTURE.md`, and `UBIQUITOUS_LANGUAGE.md` have distinct canonical ownership described below. Architecture decisions live in `docs/adr/`; temporary design work may use `docs/superpowers/specs/` and `docs/superpowers/plans/`, but those files are working material rather than sources of truth. Review and research artifacts live under `docs/reviews/` and `docs/research/`.

Go entrypoint code lives in `main.go`. CLI surface area lives in `cmd/tbboot/`. Reusable command-independent behavior belongs under `internal/`, grouped by domain such as `internal/install/`, `internal/source/`, and `internal/app/`.

## Documentation lifecycle

- Keep active product language in `CONTEXT.md`, the system overview in `ARCHITECTURE.md`, terminology in `UBIQUITOUS_LANGUAGE.md`, and durable architectural decisions in `docs/adr/`.
- Treat `docs/superpowers/plans/` and `docs/superpowers/specs/` as temporary working material. Before removing them, extract any still-valid rule into the canonical document that owns it.
- Do not use a plan or spec as the source of truth for implemented behavior. Update canonical documentation and search for obsolete claims when a contract changes.
- Keep implementation-specific task order, function names, test names, review-thread notes, and commit instructions out of durable documentation unless they describe a reusable repository rule.
- Keep fixture conventions beside their fixtures, such as in `testdata/examples/README.md`.

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

ADR files use numbered names in `docs/adr/`, for example `0005-operation-output-logs-and-exit-codes.md`. Temporary specs, when a session needs one, use dated, descriptive names in `docs/superpowers/specs/` and are removed after their durable decisions are extracted.

Markdown linting is configured in `.markdownlint-cli2.yaml`; line length is intentionally disabled, and duplicate headings are allowed only across different sibling scopes.

Go code should keep package boundaries shallow and explicit. Follow these conventions:

- Keep `main.go` minimal: it should start the CLI and return the process exit code.
- Keep `cmd/tbboot/` focused on Cobra wiring, argument parsing, flag handling, and rendering command output.
- Put command-independent behavior in `internal/<domain>/` packages with small services and explicit `Request` and `Result` types.
- Model extensibility behind interfaces and registries in `internal/` when behavior varies by type or backend. Prefer explicit lookup points over command-level branching.
- Validate inputs early and return direct, user-readable errors that work for both human output and JSON output.
- When adding machine-readable output, reuse the shared `internal/app.Result` envelope and keep success and error shapes consistent across commands.

## Testing Guidelines

Use `just check-md` or `just check-go` during iteration. Before submitting any change, run `just check` once as the final aggregate gate; do not repeat its child tasks in the same final checklist unless separate diagnostics are needed.

When validating a pull request branch, check both pending tracked changes and the committed PR range. Run `git diff --check HEAD` for pending tracked changes and `git diff --check <base>...HEAD` for committed PR changes, replacing `<base>` with the pull request's actual base branch.

### Validation guardrails

- Treat sentence-case headings as a formatting requirement. Markdown code blocks nested under list items must use spaces-only indentation or move to a top-level fence; never create lines with spaces before tabs.
- In plans and specs, document repository tasks such as `just check`, `just check-md`, `just check-go`, and `just fmt-go` instead of direct `npx markdownlint-cli2` or duplicated final commands.
- Before reporting completion, run `just check`, `git diff --check HEAD`, and `git diff --check <base>...HEAD`. All must exit successfully; the diff checks must print no output. Report any pre-existing range failure instead of marking validation as passed.
- For cross-compilation, do not run a foreign `go test` binary on the host. Use `GOOS=<target> go test -c -o <temporary-output> ...` and clean the temporary output with a shell trap.

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

When removing or replacing a documented product contract, search every canonical document in scope for specific obsolete claims. Avoid broad single-term searches when valid negations use the same terms.

### Grilling interviews

In this repository, when `/grilling` or `$superpowers:brainstorming` runs
directly or through another skill, these repository-local rules override their
one-question-at-a-time instructions.

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
