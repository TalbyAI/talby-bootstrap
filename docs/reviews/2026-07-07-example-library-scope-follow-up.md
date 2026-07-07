# Example library scope follow-up

## Purpose

Capture the current limitation of the example library introduced on `feature/implement`, so future work on `upgrade`, `logs`, `catalog`, `search`, and other command surfaces does not assume the current schema is already command-neutral.

## Short PR note

The current example library is intentionally centered on `install` and install-adjacent contracts. The schema is reusable in part, but it will likely need targeted extension before it can model `upgrade`, `logs`, `catalog`, and other non-materialization flows cleanly. That expansion is deferred to future iterations so the current PR can stay focused on seeding visually reviewable install examples and the first validation layer around them.

## Current assessment

The present structure is a good fit for the current PR goal:

- seed a visually reviewable library under `testdata/examples/`;
- validate example structure and metadata in Go tests;
- prepare fixtures that future `install` execution tests can consume.

The current structure is not yet proven as a general example system for the whole CLI.

The reasons are concrete:

- The example set is almost entirely `install`-oriented.
- The expected-state model strongly favors consumer repository mutations through `consumer/` and `expected/consumer/`.
- The verification model is currently focused on exit code, stdout, JSON stdout, and consumer state.
- The runner contract assumes a command executed from a staged consumer root with optional staged source content, which maps naturally to `install` but does not yet cover all other command surfaces.

## Risk

If future iterations assume the current schema is already sufficient for every command, they will either:

- force non-`install` commands into awkward `install`-shaped fixtures; or
- add ad hoc exceptions in tests and runners instead of extending the example model deliberately.

Either outcome would make the library harder to read, less stable as documentation, and more expensive to evolve.

## Why this should not expand in the current PR

This PR should stay focused on the first real consumer of the library: `install`.

Expanding now would be premature because:

- there is not yet a real `install` runner exercising the examples end-to-end;
- the repository does not yet have implementation pressure from `logs`, `catalog`, `search`, or `upgrade`;
- several likely needs for those commands depend on details that are still not encoded in code or tests.

The correct sequence is to validate the install-focused model against the real install implementation first, then generalize only where another command proves the gap.

## Likely extension points for future commands

The following areas are the most likely places where the example model will need to grow.

### Additional staged state

Non-install commands will likely need fixture state outside the consumer repository, for example:

- user-scoped configuration under the **User Configuration Directory**;
- configured catalogs and catalog cache state;
- recorded operations for `logs`;
- staged outside-root sources for trust-policy or operation-root cases;
- command-specific environment variables or clock-dependent state.

The current `source/`, `consumer/`, and `expected/consumer/` layout is probably too narrow for that.

### Additional verification surfaces

The current verification contract is likely incomplete for future command families. Likely additions include:

- `stderr_text` for commands whose human contract is partly error-stream based;
- user-config state verification, not just consumer repository state;
- recorded operation or logs verification as a first-class surface;
- catalog cache verification;
- exact vs partial verification for command metadata beyond stdout.

### Multiple command phases

Some commands may require a setup command plus an inspection command in the same example. For example:

- materialize with `install`, then inspect with `logs`;
- register catalog state, then query it with `search`;
- declare with one command, then upgrade with another.

The current `commands[]` field can represent more than one argv, but the runner semantics for multi-command examples are not yet defined tightly enough for these cases.

### Command-family-specific fixture conventions

Future commands may benefit from additional conventional directories, such as:

- `user-config/`
- `catalog-cache/`
- `recorded-operations/`
- `env/` or explicit environment metadata

These should only be added once a concrete command needs them.

## Recommended next steps

1. Keep the current PR scoped to the install-oriented example library and its structural validation.
2. Implement the first real example runner for `install` using the current fixture model.
3. Validate that the current install examples remain readable to humans and stable enough for machine checks.
4. After the install runner exists, add one tracer example for the next command family that creates new pressure on the schema.
5. Extend the schema minimally from that pressure rather than designing a fully generic command model in advance.

## First candidates after install

The strongest next candidates for schema pressure are:

- `logs`, because it introduces recorded operation state outside the consumer repository;
- `catalog`, because it introduces user-scoped configuration and cache state;
- `upgrade`, because it reuses some install semantics but adds different resolution and lockfile expectations.

`logs` is probably the best first expansion candidate because it tests whether the library can model command behavior that does not primarily revolve around materializing files into the consumer repository.
