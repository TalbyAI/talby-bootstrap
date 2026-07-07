# Example Library Design

## Status

Approved for planning.

## Context

Talby Bootstrap needs a shared library of examples before broad implementation work begins. The examples should serve two purposes at once:

- document the intended shape of **Sources**, **Artifacts**, consumer **Manifest** and **Lockfile** state, commands, and outputs;
- provide stable fixtures that future Go tests can execute and verify without inventing separate ad hoc test data later.

The repository already defines the core v1 domain language in `CONTEXT.md`, the compact glossary in `UBIQUITOUS_LANGUAGE.md`, and key command, materialization, and output behavior in the accepted ADRs. The example library should sharpen those decisions at a more operational level without prematurely locking the implementation into a large test harness.

## Decision

Create a versioned example library under `testdata/examples/`.

The library is organized by example type, not by audience:

```text
testdata/examples/
  README.md
  scenarios/
    <example-id>/
      README.md
      example.yaml
      source/
      consumer/
      expected/
  atomic-cases/
    <example-id>/
      README.md
      example.yaml
      source/
      consumer/
      expected/
```

Each example is a self-contained directory that can be read by a human and consumed by tests. `scenarios/` are broad, positive, narrative flows that show end-to-end usage. `atomic-cases/` are small, focused examples that each fix one primary rule or contract. Atomic cases may be positive or negative.

Every example includes:

- `README.md` with a short human explanation;
- `example.yaml` with structured metadata for test consumption;
- `consumer/` with the initial consumer repository state before the represented command runs;
- `expected/` with the normative expected results for that example.

Examples include `source/` when the represented command needs staged published source content. The published `talby-source.yaml` describes content only; acquisition semantics such as `file` or `git` belong in the represented command and consumer-side expected state, not in the source descriptor itself. Negative examples that fail before acquisition may omit `source/`.

## Example metadata schema

Each example has an `example.yaml` file. The first version of this schema should stay intentionally small and explicit rather than trying to describe a full generic test runner.

Recommended shape:

```yaml
schema_version: 1
id: file-direct-install-single-artifact
kind: atomic-case
polarity: positive
summary: Install one artifact from a file source with a minimal successful result.
commands:
  - argv:
      - tbboot
      - install
      - file:.tbboot-example/sources/local-example-source
      - --artifact
      - base-readme
verification:
  exit_code: exact
  stdout_text: contains
  stdout_json: absent
  consumer_state: exact
normative_outputs:
  - expected/exit-code.txt
  - expected/stdout-contains.yaml
  - expected/consumer
tags:
  - install
  - file-source
  - single-artifact
```

Field intent:

- `schema_version`: version of the example metadata format.
- `id`: stable identifier for documentation and future test names.
- `kind`: `scenario` or `atomic-case`.
- `polarity`: `positive` or `negative`.
- `summary`: one-line statement of what the example demonstrates.
- `commands`: one or more represented CLI invocations as explicit argv arrays.
- `verification`: declares whether a result is checked exactly, partially, or omitted.
- `normative_outputs`: files or directories inside the example that define the contract.
- `tags`: optional discovery labels for grouping related examples later.

This metadata is intentionally declarative. It tells a test harness what to inspect without embedding procedural logic into the example itself.

## Runner staging contract

The first shared runner should use one deterministic staging layout for every example:

```text
<temp-workspace>/
  <consumer contents copied here>
  .tbboot-example/
    sources/
      <contents of example source/ copied here when present>
```

The runner executes represented commands from the staged consumer root, not from inside the example fixture directory. This keeps the **Manifest**, **Lockfile**, and **Operation Root** semantics aligned with the accepted ADRs while still giving `file:` examples a stable in-repository locator.

For v1, the runner should execute with stdin closed and without an interactive TTY. Examples that cover **JSON Output Envelope** behavior must request JSON explicitly in `commands[].argv`; examples that cover prompt-required failures should rely on the documented non-interactive contract rather than on an implicit interactive mode.

Because `file:` sources are allowed by default only when they are inside the current **Operation Root**, examples that expect default `file:` approval should use locators under `.tbboot-example/sources/`. Outside-root denial fixtures are deferred from v1 until the runner has an explicit contract for staging state outside the consumer root. Until then, example authors should not rely on an implied outside-root path shape in this library.

## Verification mode contract

The `verification` object should use a small fixed vocabulary rather than free-form meanings. The first version should allow:

- `exit_code`: `exact` or `class`
- `stdout_text`: `exact`, `contains`, or `absent`
- `stdout_json`: `exact`, `contains`, or `absent`
- `consumer_state`: `exact` or `absent`
- `logs`: `exact`, `contains`, or `absent` when the example covers recorded operations

`exact` means the referenced file or directory is the full contract for that surface. `contains` means the expected file declares required fragments only. `absent` means the surface is intentionally out of scope for that example. `class` for exit codes is reserved for cases that care about the accepted v1 exit-code class and not the exact integer.

## Expected result conventions

Each example must include either `expected/exit-code.txt` or `expected/exit-code-class.txt`, depending on whether the contract fixes the exact exit code or only the accepted v1 exit-code class. Other files under `expected/` are selected based on the contract the example is intended to fix.

Recommended conventions:

- `expected/stdout.txt`: full expected human-readable output when exact matching is appropriate.
- `expected/stdout-contains.yaml`: structured required fragments when partial human-output matching is more stable.
- `expected/stdout.json`: full expected **JSON Output Envelope** when exact JSON output is normative.
- `expected/stdout-json-contains.yaml`: required JSON fields or fragments when the example fixes only a subset of the JSON contract.
- `expected/consumer/`: final consumer repository state when the example is supposed to mutate consumer files.
- `expected/logs/`: expected operation log artifacts only for examples that explicitly cover the **Logs Command** or recorded operation persistence.
- `expected/exit-code-class.txt`: expected exit-code class when the example verifies the v1 class only instead of the exact integer.

The verification mode in `example.yaml` must match the files present under `expected/`.

When JSON output is normative, the expected envelope should follow ADR-0005 and include `code`, `message`, `details`, `warnings`, and any required operation metadata for that example. Examples that only fix one JSON fragment should prefer `stdout_json: contains` to avoid freezing unrelated fields too early.

For `contains` verification, expected fragments should be stored as explicit YAML lists rather than plain text files with ad hoc separators. Recommended shape:

```yaml
fragments:
  - source file:.tbboot-example/sources/local-example-source is not approved
  - add the source to the Manifest trust policy to continue
```

This keeps partial assertions machine-readable, avoids delimiter parsing rules, and leaves room for future metadata if some fragments later need ordering or matching options.

## Verification policy

`scenarios/` should generally prefer fuller verification because they are the main narrative documentation of successful flows. In most scenario cases, the human output, final consumer state, and any manifest or lockfile mutations should be compared exactly unless there is a strong reason not to.

`atomic-cases/` should prefer the narrowest assertion that still fixes the intended contract. For many atomic cases, exact output is unnecessary and would create noise. Partial checks are preferred when the rule being tested is:

- the **Exit Code** class;
- presence of a normative message fragment;
- presence of a specific JSON `code`, `message`, warning, or detail fragment;
- confirmation that a consumer file did or did not change.

This keeps atomic examples focused and reduces fragility when unrelated wording evolves.

## Editorial rules

Each example `README.md` must declare:

- the purpose of the example;
- the represented command or commands;
- whether it is a `scenario` or `atomic-case`;
- whether it is positive or negative;
- which expected outputs are normative.

Examples should include only the context needed to explain and verify the rule they cover. If an atomic case needs substantial unrelated setup, it should be simplified or split. If a scenario only demonstrates one isolated rule, it should likely be demoted to an atomic case.

Example IDs should be stable and descriptive, such as:

- `file-direct-install-multi-artifact`
- `declare-only-manifest-only`
- `ownership-conflict-overlapping-file`

## Initial example set

The first implementation batch should seed the library with a small but representative set.

Initial scenarios:

- `file-direct-install-multi-artifact`: a positive direct install from `file:` that resolves multiple **Artifacts** and records resulting **Manifest**, **Lockfile**, materialized files, and default **Operation Summary**.
- `declare-only-flow`: a positive `--declare-only` flow that updates only the **Manifest** and leaves materialization state untouched.

Initial atomic positive cases:

- `declare-only-manifest-only`: minimal proof that `--declare-only` mutates only the **Manifest**.
- `json-success-envelope-minimal`: minimal successful JSON output contract.
- `file-direct-install-single-artifact`: minimal successful install of one **Artifact** from a `file:` source.

Initial atomic negative cases:

- `ownership-conflict-overlapping-file`: materialization fails with **Ownership Conflict**.
- `ambiguous-install-target-rejected`: ambiguous or invalid explicit target form is rejected.
- `trust-policy-denied-git-source`: a **Trust Policy** denial returns the expected class of failure.
- `non-interactive-prompt-required`: a path that would require prompting fails with the documented non-interactive contract.

This initial set is intentionally incomplete. Its purpose is to establish structure, naming, metadata, and verification conventions early.

## Data and test flow

The intended future flow is:

1. A shared Go test runner enumerates example directories under `testdata/examples/`.
2. The runner executes each example as its own subtest, using the example `id` as the subtest name.
3. The runner reads `example.yaml`.
4. The runner stages the `consumer/` tree and the example `source/` tree when present in a temporary workspace.
5. The runner executes the represented `tbboot` argv when executable behavior exists.
6. The runner verifies outputs according to the declared verification modes and normative expected files.

This allows the same example library to support human review and machine verification without duplicating fixtures elsewhere. It also means `go test -v` can report each example as its own line item while still keeping the evaluation logic centralized in one reusable runner.

## Consequences

- The repository gains a concrete operational reference for v1 behavior before broad implementation.
- Future tests can consume examples directly from `testdata/` using standard Go conventions.
- Examples can express both exact and partial expectations, which is especially important for focused atomic cases.
- The schema stays small enough to evolve without committing too early to a complex test framework.

## Deferred work

- Add a root `testdata/examples/README.md` that explains the library once the first examples exist.
- Decide later whether `example.yaml` needs fields for environment setup, explicit runner-staged outside-root fixtures, trust configuration overlays, or multi-command sequences beyond simple argv lists.
- Defer `logs`-focused examples until operation persistence behavior exists in code.
- Treat the current library as intentionally install-centered until the next command family creates concrete pressure to extend the schema; see `docs/reviews/2026-07-07-example-library-scope-follow-up.md`.
