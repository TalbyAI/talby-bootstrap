# 0.1 contract reset design

## Goal

Reset the published and persisted contracts for the first public `tbboot` release so all six YAML document types use schema version 1, canonical `tbboot-` filenames, deterministic serialization, and scalar `file:`/`git:` Source References. Keep the current minimal in-root `file:` install working while removing undeployed compatibility shapes and successful command placeholders.

## Scope

This slice includes:

- strict schema-versioned readers and deterministic writers for Source Descriptor, Artifact Descriptor, Manifest, Lockfile, Materialization Record, and Recovery State;
- scalar Source Reference parsing and formatting at consumer-facing and persisted boundaries while retaining structured source type and locator fields internally;
- canonical `tbboot-` filenames and current-product documentation and fixtures updated to those names;
- strict descriptor validation for the retained whole-file `file` Materialization Step;
- the existing minimal in-root `file:` install path adapted to the new persisted shapes;
- removal of successful `search`, `logs`, `catalog`, and `upgrade` placeholders.

This slice does not implement the later tickets' complete filesystem safety, operation lock, prune, rollback lifecycle, HTTPS Git acquisition, file upgrade, Git upgrade, acceptance CI, or release packaging. Recovery State has a strict model and persistence format here; runtime creation and clearing belong to the rollback ticket.

Historical research, design, and plan documents remain evidence. Current product language, examples, and executable fixtures are updated where they describe the active contract.

## Contract

All readers accept exactly one YAML document with integer `schema_version: 1`. Unknown fields, duplicate mapping keys, explicit nulls, multiple documents, aliases, anchors, merge keys, custom tags, and non-scalar mapping keys are rejected before acquisition or mutation. Validation reports the first deterministic document error. Comments and key ordering are accepted.

Writers emit UTF-8 without a BOM, LF line endings, two-space indentation, a final newline, omitted empty optional collections, and sorted collections. Writers validate the domain model before serializing and use atomic replacement already provided by repository-state persistence.

The canonical filenames are:

| Document | Filename |
| --- | --- |
| Source Descriptor | `tbboot-source.yaml` |
| Artifact Descriptor | `tbboot-artifact.yaml` |
| Manifest | `tbboot-artifacts.yaml` |
| Lockfile | `tbboot-artifacts.lock.yaml` |
| Materialization Record | `tbboot-artifacts.managed.yaml` |
| Recovery State | `tbboot-artifacts.recovery.yaml` |

Source References are scalar strings in the form `file:<locator>` or `git:<locator>`. The internal `SourceIdentity` remains structured so dispatch and normalization do not duplicate parsing. This slice stores and validates `git:` identities but does not acquire them.

### Published descriptors

The Source Descriptor contains `schema_version` and a non-empty `artifacts` list. Each artifact reference has a unique lowercase ASCII hyphenated `name` and clean relative `path`; there is no published source name or transport field.

The Artifact Descriptor contains `schema_version`, `artifact.name`, canonical `MAJOR.MINOR.PATCH` SemVer `artifact.version`, optional non-empty `description`, and one or more steps. Product 0.1 accepts only `type: file`; each step has a clean Artifact-relative regular-file `source` and clean Operation-Root-relative `path`. Descriptor names must match the Source Descriptor reference. Existing containment checks remain in this slice; the complete cross-platform topology and race contract is ticket #31.

### Persisted state

The Manifest contains `schema_version`, optional `trust_policy.approved_sources`, and optional `declarations`. Each declaration stores scalar `source`, `scope` (`source` or `artifact`), a required `artifact` only for artifact scope, and optional `source_version` only for an explicitly pinned Git version. Duplicate or overlapping intent is invalid. Approved Sources and declarations are written in canonical lexical order.

The Lockfile contains `schema_version` and `resolutions`. Each resolution stores scalar `source`, `source_version`, an optional Git `commit`, and a non-empty list of unique `{name, version}` artifacts. File Source Versions are `sha256:<lowercase-hex>` snapshot hashes. Git versions and commit validation are modeled now; Git acquisition is deferred. Resolutions sort by Source Reference and Source Version, then artifacts by name.

The Materialization Record contains `schema_version` and one entry per Managed Artifact. Entries store scalar `source`, `source_version`, optional Git `commit`, `artifact`, `artifact_version`, and non-empty files containing canonical root-relative `path` and `sha256:<lowercase-hex>` digest. Entries sort by Source Reference, Source Version, and Artifact; files sort by path.

Recovery State contains `schema_version`, fixed code `rollback_incomplete`, a sanitized non-empty `summary`, and at least one unique observation. Each observation stores canonical root-relative `path`, `result` (`restore_failed` or `verification_failed`), expected state (`absent` or `file`), an optional digest and supported mode for files, and an optional owner containing Source, Source Version, and Artifact. Raw errors, prior contents, absolute paths, credentials, and stack traces are never persisted.

Cross-document validation remains limited to rules needed by the current install path: a Lockfile or Materialization Record without a Manifest is invalid state when loaded for reconciliation, and materialized entries must correspond to lockfile resolutions. Full recovery lifecycle validation belongs to ticket #34.

## Data flow

1. CLI parses one direct Source Reference and converts it to the existing structured source request.
2. Repository-state readers decode strict YAML, parse scalar Source References, validate the complete domain object, and return the first error without rewriting the input.
3. The file Source resolves `tbboot-source.yaml` and each artifact's `tbboot-artifact.yaml`, validates the retained descriptor contract, and returns the same materialization input used by the existing local install path.
4. Install and Sync persist only canonical `tbboot-` documents. Existing declared intent and lock semantics remain pinned; no migration from `talby-*` or structured `{type, locator}` documents is attempted.
5. The CLI exposes only the retained implemented `install` surface in this slice. Removed placeholders fail as unknown commands rather than reporting successful work.

## Error handling

Missing, unsupported, malformed, ambiguous, or non-canonical YAML is a validation error before source acquisition or mutation. No reader migrates or rewrites an invalid document. Missing Manifest remains the current empty-state behavior required for a first explicit install; other missing state files retain current optional-state semantics until later cross-document tickets tighten them.

The strict YAML decoder is shared by all six document readers. Domain validators own semantic errors such as invalid Source References, paths, names, versions, duplicate intent, or incompatible source-specific fields. Error ordering follows document order and stable collection order where the format permits it.

## Testing

Tests exercise public reader/writer and Source `Resolve` seams. They cover canonical round trips and deterministic output, every strict-YAML rejection class, scalar Source Reference round trips and normalization, schema and semantic validation, canonical filenames, descriptor path and version rules, Recovery State sanitization, and a minimal in-root file install using the final fixtures. The full Go suite and Markdown checks run before handoff.
