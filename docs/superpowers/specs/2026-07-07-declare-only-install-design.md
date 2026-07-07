# Declare-only install design

## Status

Approved for planning.

## Context

The repository now has the repository-state foundation from PR 1:

- `internal/repositorystate` owns the manifest and lockfile models;
- `talby-artifacts.yaml` and `talby-artifacts.lock.yaml` have real read/write behavior;
- `install` still stops at source resolution and artifact selection.

The roadmap's PR 2 is the smallest user-visible step that proves declared repository state is real: `install --declare-only` should persist manifest intent without writing a lockfile or materializing files.

This slice should stay narrow. It should not start sync behavior, trust enforcement, or upgrade semantics. It only needs to make declaration intent durable and enforce that declaration-only installs do not silently mutate existing declarations.

## Goal

Implement `install --declare-only` as a real manifest-writing workflow that:

- requires an explicit source argument;
- resolves the source and selects one artifact using the existing install path;
- creates `talby-artifacts.yaml` if it does not exist;
- adds one artifact-scoped declaration to the manifest;
- leaves the lockfile untouched;
- rejects attempts to change an existing declaration for the same source and artifact.

## Non-goals

This design deliberately excludes:

- `install --declare-only` without an explicit source;
- writing `talby-artifacts.lock.yaml`;
- file materialization or materialization records;
- trust-policy enforcement;
- interactive conflict resolution;
- any real `upgrade` implementation.

## Decision

Extend the existing `internal/install` service rather than introducing a separate declaration module.

The install flow already owns source resolution and artifact selection. PR 2 should reuse that path and add one optional branch for declaration-only behavior. The only new dependency needed is the existing repository-state store.

This keeps the slice short:

- `cmd/tbboot` parses a new `--declare-only` flag;
- `internal/install` decides whether the request is resolution-only or declaration-only;
- `internal/repositorystate` remains the owner of manifest IO and validation.

## Architecture

### CLI boundary

`cmd/tbboot/install.go` should stay thin:

- parse `--declare-only`;
- keep requiring a positional `<source>` when declaration-only mode is used;
- pass a typed request into `internal/install`;
- render success, no-op, and conflict results in human and JSON output.

The CLI should not inspect manifest files or compare declarations directly.

### Install service

`internal/install.Request` should gain a `DeclareOnly bool`.

The existing install service should gain access to a `repositorystate.Store`. In declaration-only mode, the service should:

1. validate that the request includes an explicit source;
2. resolve the source and select one artifact using the current logic;
3. load the manifest, treating a missing manifest as an empty manifest;
4. compare the requested declaration against existing declarations for the same source and artifact;
5. append and write only when the declaration is new;
6. return a typed result that distinguishes created, no-op, and conflict outcomes.

Outside declaration-only mode, the current behavior should remain unchanged in this PR.

### Repository-state boundary

`internal/repositorystate` should not gain install-specific orchestration logic.

It may gain a small pure helper if needed for declaration lookup or append behavior, but PR 2 should avoid a larger abstraction. The install service can own the policy decision for:

- what counts as the same declared target;
- what counts as a no-op;
- what counts as a conflict that must wait for `upgrade`.

That policy belongs with install semantics, not with generic manifest storage.

## Declaration semantics

PR 2 should support only artifact-scoped declarations.

The declaration written by `install --declare-only` should continue using the existing `ManifestDeclaration(req, result)` mapping:

- source identity comes from the resolved source identity;
- target scope is `artifact`;
- target artifact is the selected artifact name;
- input locator and optional requested version come from the user request.

This PR should not attempt to create source-scoped declarations.

## Conflict model

Declaration-only behavior needs three outcomes.

### Created

If no declaration exists for the same source identity and artifact name, the service should add a new declaration and write the manifest.

### No-op

If a declaration already exists for the same source identity and artifact name and the input metadata is equivalent, the service should succeed without rewriting the manifest.

Equivalent means:

- same source type;
- same source name;
- same target artifact;
- same input locator;
- same input version.

### Conflict

If a declaration already exists for the same source identity and artifact name, but the input locator or input version differs, declaration-only mode should reject the request.

This is the key product rule for PR 2: declaration-only installs may add intent, but they may not silently replace existing declared intent. That mutation belongs to a later `upgrade` flow.

The error should say plainly that the declaration already exists with different input and that changing it requires `upgrade`.

## Manifest bootstrap behavior

If `talby-artifacts.yaml` does not exist, declaration-only install should create it.

The bootstrap behavior should be:

- load manifest;
- if the store returns `manifest not_found`, continue with an empty manifest value;
- add the new declaration;
- write the manifest through the existing store.

This PR should not create a lockfile during bootstrap.

## API shape

The internal API should stay close to the current service shape.

Conceptually:

```go
type Request struct {
    Source      source.Ref
    Artifact    string
    DeclareOnly bool
}

type Result struct {
    Source   source.Identity
    Artifact source.ArtifactDescriptor
    Change   ChangeKind
}

type ChangeKind string

const (
    ChangeDeclared ChangeKind = "declared"
    ChangeNoOp     ChangeKind = "noop"
)
```

A conflict should still return an error rather than a successful result with a special change kind.

The exact names may vary, but the result should be explicit enough for the CLI to render:

- declaration created;
- declaration already present;
- conflict requiring `upgrade`.

No new result envelope is needed. `internal/app.Result` already covers CLI output.

## Output behavior

### Human output

The text output should distinguish the success cases:

- created: `declared artifact <artifact> from <source>`
- no-op: `artifact <artifact> from <source> is already declared`

Conflict should surface as an error, with wording equivalent to:

`artifact "<artifact>" from source "<source>" is already declared with different input; use upgrade`

### JSON output

JSON output should continue using `internal/app.Result`.

For declaration-only success, the result should include:

- `message`, such as `declare-only succeeded`;
- `details.source`;
- `details.artifact`;
- `details.change`, with `declared` or `noop`.

Conflict should continue to flow through the existing error and exit-code handling.

## Validation and testing

The tests should stay focused on the new branch.

### Install service tests

Add tests for:

- manifest bootstrap when the file does not exist;
- declaration created for a new source and artifact;
- no-op when the same declaration already exists unchanged;
- conflict when locator differs;
- conflict when requested version differs;
- no lockfile creation or mutation during declaration-only install.

These tests should exercise the command-independent install service, not Cobra.

### CLI tests

Add narrow CLI tests for:

- `install <source> --artifact <name> --declare-only` success in human mode;
- the same command in JSON mode;
- `install --declare-only` without a source failing clearly;
- a conflict surfacing as an error through the existing CLI path.

## Deferred work

- Add a real `upgrade` flow that can replace an existing declaration intentionally.
- Extend install to write lockfile state once materialization behavior exists.
- Support source-scoped declaration workflows if later slices need them.
- Add trust enforcement before manifest writes.
- Replace the bare `install` sync placeholder in the later sync slice.
