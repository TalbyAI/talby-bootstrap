# Example E2E Runner Design

## Status

Approved for planning.

## Context

Talby Bootstrap already has a shared example library under `testdata/examples/`. That library documents intended CLI behavior and gives the repository a growing set of concrete examples for installs, failures, and repository state transitions.

What is still missing is one shared end-to-end runner that executes those examples as tests instead of maintaining parallel ad hoc CLI fixtures in Go tests. The repository also needs a way to keep examples for not-yet-implemented behavior without forcing the test suite to fail on every run.

The design goal is to make the example library the primary executable contract for CLI behavior while preserving room for red-green development as features are implemented incrementally.

## Decision

Add a single end-to-end example runner test near the CLI package and extend `example.yaml` with an explicit execution `status`.

The runner will discover every example in `testdata/examples/`, stage each one into a temporary workspace, execute its declared CLI commands, and verify its declared outputs. The new `status` field controls whether the example runs and what outcome the suite expects from it.

This keeps the examples as the one source of truth and avoids writing a second fixture system beside them.

## Example metadata change

Add a required `status` field to every `example.yaml`.

Recommended shape:

```yaml
schema_version: 1
id: file-direct-install-multi-artifact
kind: scenario
status: active
polarity: positive
summary: Install two artifacts from a file source and record manifest, lockfile, and materialized files.
commands:
  - argv:
      - tbboot
      - install
      - file:local-example-source
verification:
  exit_code: exact
  stdout_text: exact
  stdout_json: absent
  stderr_text: absent
  stderr_json: absent
  consumer_state: exact
normative_outputs:
  - expected/exit-code.txt
  - expected/stdout.txt
  - expected/consumer
tags:
  - install
  - file-source
  - multi-artifact
```

Allowed values:

- `active`
- `broken`
- `skipped`
- `deprecated`

Field intent:

- `active`: the runner executes the example and the example must satisfy all declared verification rules.
- `broken`: the runner executes the example and expects the example to fail today. If the example unexpectedly satisfies all verification rules, the test fails so the example can be promoted to `active`.
- `skipped`: the runner does not execute the example. This is for examples expected to return later once implementation work reaches them.
- `deprecated`: the runner does not execute the example. This is for examples kept temporarily as documentation of behavior that is no longer meant to be reactivated and will be removed later.

`status` is intentionally about execution policy, not business semantics. `polarity` continues to describe whether the represented product behavior is positive or negative.

## Runner placement

The end-to-end runner should live near the CLI tests, not under `internal/`, because the behavior under test is the public `tbboot` command surface.

Recommended location:

- `cmd/tbboot/examples_e2e_test.go`

The example metadata loader remains in `internal/examples/` and stays the single place that validates example structure and schema rules.

## Loader changes

`internal/examples` should be extended to:

- parse the new `status` field;
- validate that `status` is one of `active`, `broken`, `skipped`, or `deprecated`;
- parse `stderr_text` and `stderr_json` verification fields;
- validate `stderr_text` with the same `exact`, `contains`, and `absent` values as `stdout_text`;
- validate `stderr_json` with the same `exact`, `contains`, and `absent` values as `stdout_json`;
- require the matching `expected/stderr*` files when stderr verification is not `absent`;
- expose simple helpers derived from status, such as whether the example should run and whether it is expected to pass or fail.

The loader should stay declarative. It should not execute commands or know CLI details. It should only enforce example library integrity and expose metadata in a shape the CLI-side runner can consume.

## Runner staging model

Each example runs as its own subtest with the example `id` as the subtest name.

The runner stages each example into a temporary workspace with this shape:

```text
<temp-dir>/
  <example consumer files copied here at workspace root>
  .tbboot-example/
    sources/
      <source-alias>/
        <example source files copied here when present>
```

The runner executes commands from the staged consumer root so repository-root behavior matches real CLI use. The staged source root gives `file:` examples a local filesystem target without network access. Consumer-state comparison ignores `.tbboot-example/` because it is runner scaffolding, not product state.

The runner should use the same in-process command execution path already used by existing CLI tests when possible. That keeps the first implementation smaller than building and shelling out to a separate binary for every example.

## Command execution contract

The runner reads `commands[].argv` exactly as declared in `example.yaml`, with only the minimal normalization needed to bind example-local aliases to staged filesystem paths.

The first implementation should support the command shapes already present in the example library:

- `tbboot install file:local-example-source`
- `tbboot install file:local-example-source --artifact <name>`
- `tbboot install file:local-example-source --declare-only`
- `tbboot install file:<example-source-alias>`
- `tbboot install git:<locator>`
- JSON-output variants of the same install surface

Minimal argv normalization rules:

- replace the leading `tbboot` token with the existing test harness invocation path used by CLI tests;
- replace any `file:<alias>` token with a `file:` locator pointing to the staged `.tbboot-example/sources/<alias>` directory;
- leave non-file source locators, such as `git:github.com/example/library`, unchanged.

The first version should not add a generic templating language for argv rewriting. The file-source alias rule is enough for current examples. Add more rewriting rules only when the library produces a concrete need.

## Verification contract

The runner verifies only surfaces already modeled in the example metadata:

- exit code
- stdout text
- stdout JSON
- stderr text
- stderr JSON
- consumer state

Verification rules:

- `exit_code: exact` compares against `expected/exit-code.txt`.
- `exit_code: class` compares against the expected exit-code class contract already defined by the repository.
- `stdout_text: exact` compares normalized stdout against `expected/stdout.txt`.
- `stdout_text: contains` checks required fragments from `expected/stdout-contains.yaml`.
- `stdout_text: absent` performs no stdout text assertion.
- `stdout_json: exact` compares JSON semantically against `expected/stdout.json`, not byte-for-byte.
- `stdout_json: contains` checks only required JSON fragments or fields from the expected file format already used by the example library.
- `stdout_json: absent` performs no JSON assertion.
- `stderr_text: exact` compares normalized stderr against `expected/stderr.txt`.
- `stderr_text: contains` checks required fragments from `expected/stderr-contains.yaml`.
- `stderr_text: absent` performs no stderr text assertion.
- `stderr_json: exact` compares JSON semantically against `expected/stderr.json`, not byte-for-byte.
- `stderr_json: contains` checks only required JSON fragments or fields from `expected/stderr-json-contains.yaml`.
- `stderr_json: absent` performs no JSON stderr assertion.
- `consumer_state: exact` compares the staged consumer repository contents against `expected/consumer/`.
- `consumer_state: absent` performs no consumer-state assertion.

The first version should not introduce verification surfaces beyond command streams and consumer state, such as logs or timing. Those can be added when examples need them.

## Status-driven test behavior

The runner's behavior is determined by `status`:

### `active`

The runner executes the example and applies all declared verifications. The subtest passes only if the example satisfies its full declared contract.

### `broken`

The runner executes the example and still applies the same declared verifications, but the expected subtest result is inverted:

- if execution and verification fully succeed, the subtest fails because the example no longer belongs in `broken`;
- if command execution completes but declared verification fails, the subtest passes because the repository still reproduces the known not-yet-implemented behavior;
- if staging, argv normalization, loader validation, or the runner harness itself fails, the subtest fails even when the example is `broken`.

The first version should not require examples in `broken` to fail in one specific way. It is enough that the example does not yet satisfy its declared contract. This keeps the initial runner small and still useful for red-green development.

### `skipped`

The runner does not execute the example and marks the subtest skipped. This is for examples that should remain in the suite as future executable intent without affecting the current test result.

### `deprecated`

The runner does not execute the example and marks the subtest skipped with a reason indicating deprecation. This keeps the example visible in test output while making it clear that reactivation is not expected.

## Relationship to existing tests

The repository already has CLI tests in `cmd/tbboot/root_test.go` that use hand-built fixtures. The new runner should not force immediate deletion of those tests.

The intended rollout is:

1. add the example runner;
2. mark example statuses according to current implementation reality;
3. keep existing targeted tests that still cover behavior not yet represented well enough in the example library;
4. remove redundant ad hoc tests later when an equivalent example covers the same contract clearly.

This keeps the first change small and avoids turning the runner introduction into a broad test refactor.

## Initial rollout

The first implementation batch should do only the following:

1. add `status` to the example metadata schema and validation;
2. add `stderr_text` and `stderr_json` to the example metadata schema and validation;
3. update every current `example.yaml` to declare one of the four allowed statuses and the stderr verification policy;
4. add one CLI-side end-to-end runner that discovers the whole example library;
5. execute `active` and `broken` examples;
6. skip `skipped` and `deprecated` examples;
7. support only the install-oriented command shapes already present in the current example set, including `file:<alias>` source normalization.

This is enough to make the whole example library visible to `go test` immediately while preserving incremental development.

## Maintenance rules

Once the runner exists, example-library maintenance should follow these rules:

- every example must declare `status`;
- every example must explicitly declare stdout and stderr verification policy;
- new examples should default to the narrowest truthful status rather than being left implied;
- when a `broken` example starts passing, the test failure is the signal to promote it to `active`;
- `skipped` means the team still expects future activation;
- `deprecated` means the example is temporary documentation on the path to removal, not a future executable target.

These rules make the example library function as both documentation and execution backlog.

## Consequences

- `testdata/examples/` becomes the executable contract for more of the CLI surface.
- The suite gains visibility into all examples immediately, even when some features are not implemented yet.
- The `broken` status supports red-green development without hiding unfinished work.
- The first runner stays small by reusing existing loader code and existing in-process CLI execution patterns.

## Deferred work

- Add richer failure classification for `broken` examples only if the team later needs exact failure-mode assertions.
- Add more argv aliasing only when new example families require it.
- Add more verification surfaces only after the example library creates concrete pressure for them.
- Revisit whether some current ad hoc CLI tests should be deleted after runner coverage is proven sufficient.
