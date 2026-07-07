# Install core design

## Status

Approved for planning.

## Context

Talby Bootstrap already defines the command model, trust rules, output semantics, and example-library conventions for `install`. The repository now needs a concrete internal design for the first implementation slice of the `install` command.

This first slice should optimize for:

- command-independent functional tests driven by TDD;
- a core that can validate and orchestrate installation behavior without being tightly coupled to Cobra or direct process execution;
- explicit seams for environment access so validation-heavy tests can run without interacting with the real filesystem or other ambient state;
- a `Source` abstraction that works for `file` now and can later support `git` and additional acquisition channels without redesigning the core.

The current example library under `testdata/examples/` is intentionally install-centered and should guide the implementation, but not yet drive full end-to-end execution through the CLI binary. That broader runner is a later step once the install core is functionally stable.

## Decision

Implement `install` around a command-independent internal service with injected dependencies.

The CLI layer in `cmd/tbboot` should remain thin:

- parse flags and positional arguments;
- normalize user input into typed request values;
- call the install core;
- render the returned result as human or JSON output.

The install core should own semantic validation, source resolution, and orchestration of later reconciliation work through explicit interfaces.

## Architecture

The first implementation should separate responsibilities into four main areas.

### CLI boundary

`cmd/tbboot` owns Cobra-specific concerns only:

- command names and aliases;
- parsing and validation of raw CLI syntax;
- translation from CLI syntax into typed internal request values;
- exit code and output mapping.

The CLI should not contain install-specific domain logic beyond syntax-level validation that naturally belongs to command parsing.

### Install core

The install core is the main seam for TDD. It should expose one primary operation, conceptually equivalent to:

```go
Install(ctx context.Context, req InstallRequest) (InstallResult, error)
```

The exact names may vary, but the public shape should remain:

- one request type carrying normalized install intent;
- one result type carrying operation outcome data;
- dependency injection through constructor parameters or a service struct, not through globals.

This operation is the primary seam for functional tests. Tests should exercise install behavior through this boundary instead of through Cobra or process-level execution.

### Environment abstractions

The install core should depend on interfaces for environment interaction rather than reaching directly into the operating system.

Examples include:

- filesystem access;
- operation-root inspection;
- prompt capability or non-interactive mode checks;
- clock access when trust policies later require time-based checks.

The initial design should prefer small interfaces over one large environment object when possible. The goal is to allow:

- pure validation tests with no real environment access;
- integration-style internal tests with an in-memory or temporary implementation;
- CLI wiring that supplies concrete real implementations.

### Source subsystem

Source acquisition should be modeled behind a registry and a typed `Source` abstraction.

The install core should not switch on source type directly. Instead, it should:

1. receive a normalized typed source reference;
2. ask a `SourceRegistry` for the matching `Source` implementation;
3. resolve the source through the returned implementation;
4. continue install validation and orchestration based on the resolved source and its capabilities.

## Source model

The source model should be broad enough to avoid trapping the design in `file`-only assumptions, while still staying small enough for the first iteration.

### Source reference

The install core should use a typed source reference value rather than raw CLI strings. That value should be able to carry:

- source type, such as `file` or `git`;
- source locator;
- optional requested source version or ref;
- any later normalized metadata needed for lockfile identity.

This keeps CLI parsing concerns separate from domain behavior.

### Source registry

The source registry chooses the implementation for a normalized source type.

Conceptually:

```go
type SourceRegistry interface {
    Lookup(sourceType string) (Source, error)
}
```

The exact method shape may vary, but the behavior should support:

- a real registry used by the CLI and internal integration tests;
- a test registry that can replace `file` and `git` implementations with doubles or fakes;
- future registration of new Source Types without changing the install core.

The first iteration should include a real `file` registration path. `git` should be anticipated in the model but does not need a real behavioral implementation yet.

### Source capabilities

`Source` should explicitly report what it can do. The core should reason about capabilities instead of hard-coding assumptions about Source Types.

Capabilities that matter already for forward compatibility include:

- whether the source supports named refs or version selectors;
- whether the source can provide a stable resolved identity suitable for lockfile persistence;
- whether the source can provide a timestamp for the resolved reference;
- whether the source can enumerate or inspect available versions or references.

This should be represented as a structured `SourceCapabilities` value returned by the source implementation.

The first iteration does not need to exercise every capability, but the model should exist now so later trust and upgrade work can extend behavior without redesigning the source boundary.

### Source resolution

The core interaction with a source should be centered on resolution rather than ad hoc file reads.

Conceptually:

```go
type Source interface {
    Capabilities() SourceCapabilities
    Resolve(ctx context.Context, req ResolveRequest) (ResolvedSource, error)
}
```

The resolved source should carry the normative data the rest of the install pipeline needs, such as:

- stable resolved source identity;
- resolved version, revision, or snapshot marker;
- reference timestamp when available;
- access to the source descriptor and artifact descriptors;
- any additional metadata required for operation summaries or lockfile writing.

This allows the install core to validate future rules like minimum-age trust checks or source-version constraints without depending on source-specific conditionals.

## TDD seams and testing strategy

The main TDD seam for the first implementation is the install core operation itself, not the CLI binary.

Tests should be organized in layers.

### Core functional tests

These are the primary tests for the first iteration. They should:

- call the install core through its internal API;
- provide fake or in-memory implementations of the environment interfaces;
- provide a test `SourceRegistry` and test `Source` implementations;
- use the example library as a guide for scenarios and assertions.

These tests should verify semantic behavior, not Cobra wiring.

### Small CLI smoke tests

CLI tests should remain narrow and focused on:

- command shape;
- alias support;
- flag parsing;
- output mode wiring;
- exit-code mapping.

The CLI should not become the main place where install behavior is verified.

### Deferred end-to-end example runner

A full runner that executes the CLI against `testdata/examples/` remains a second step. That later work should reuse the same example contracts, but it should arrive after the install core has already proven functionally stable through internal tests.

## Use of examples in the first iteration

The examples should act as executable design references, not yet as the test harness itself.

In practice, this means:

- extracting install scenarios and edge cases from the current examples;
- deriving internal test names and assertions from the example IDs and summaries where useful;
- allowing the first internal tests to stage only the pieces of state they actually need;
- deferring process-level execution and fixture staging conventions from the example-library spec until the dedicated example runner exists.

The current mismatch between the example-library staging convention and some fixture command paths should not block the core design. That mismatch should be resolved before the dedicated end-to-end runner is introduced.

## Initial implementation slice

The first implementation slice should aim for:

- a typed `InstallRequest`;
- a command-independent install service boundary;
- environment interfaces sufficient to avoid direct ambient access in validation-heavy paths;
- a `SourceRegistry`;
- a real `file` source implementation;
- test doubles for the source registry and source implementations;
- internal tests that cover the first functional install rules without going through the CLI binary.

The first slice does not need to complete every later concern for lockfile, trust, prompting, or git resolution, but it should leave the boundaries ready for them.

## Deferred work

- Add a real `git` source implementation.
- Tighten the exact shape of lockfile-facing resolved source identity once lockfile writing is implemented.
- Introduce time-aware trust policy validation that consumes reference timestamps from resolved sources.
- Add a dedicated end-to-end runner that executes `testdata/examples/` through the CLI process contract.
- Expand the environment abstractions only when a concrete behavior needs them, rather than prebuilding a single all-purpose runtime interface.
