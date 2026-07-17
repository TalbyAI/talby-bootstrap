# ADR-0005: Operation Output and Exit Codes

## Status

Accepted for v1.

## Context

The CLI needs useful immediate diagnostics for humans and stable classification for automation without persisted logs, replay, or verbosity levels.

## Decision

Install, Sync, and upgrade show a short **Operation Summary** immediately. Human success is written to standard output, failure to standard error, and warnings use a `warning:` prefix on standard error. JSON mode writes exactly one envelope to the corresponding success or failure stream and leaves the other stream empty.

Success summaries use these stable shapes:

```text
install: applied 3 changes (2 artifacts)
sync: no changes (2 artifacts)
upgrade: would apply 3 changes (2 artifacts)
```

The first form is followed only by effective changes. Dry Run reports planned changes using the same detail shape but never describes them as applied. Each change is deterministic and includes its kind plus **Provenance Summary**: canonical Source Reference, Source Version, Artifact Name when applicable, canonical root-relative path when applicable, and ownership kind when applicable. Unchanged files are omitted.

Human change lines use this stable form:

```text
- <kind> source=<ref> [source_version=<version>] [artifact=<name>] [path=<path>] [ownership=<kind>]
```

Values are raw unless empty or containing whitespace, control characters, `"`, or `\`; those values use JSON-style string escaping. Human failures use `<operation>: <concise cause>` followed by `-` detail lines for conflicts, denied Sources, or recovery observations. Failures before an operation starts show only the concise cause. Cobra usage is not printed automatically.

Machine-readable output uses one **JSON Output Envelope** that always contains `code`, `message`, `details`, and `warnings`. Empty details and warnings are `{}` and `[]`, never omitted or null. When an operation has started, details always contain `operation`, `outcome`, and `dry_run`. Counts are included only when known; `changes`, `conflicts`, `denied_sources`, and recovery fields are included only when relevant. A failure before an operation starts may use empty details. Optional fields are omitted rather than set to null.

Outcomes are exactly `applied`, `no_op`, `planned`, `conflict`, `denied`, and `failed`. Dry Run with changes uses `planned`, Dry Run without changes uses `no_op`, and Dry Run with conflicts uses `conflict`. `artifact_count` is the number of Artifacts considered in the complete result. `change_count` equals the number of effective or planned change entries. Human output uses normal singular and plural forms.

V1 change kinds are `declaration_added`, `declaration_updated`, `resolution_locked`, `resolution_updated`, `file_created`, `file_updated`, `file_removed`, `ownership_adopted`, and `lock_pruned`. New change kinds require retained behavior that cannot use one of these.

JSON change objects contain `kind`, canonical scalar `source`, and optional `source_version`, `artifact`, `path`, and `ownership_kind`. Optional fields are omitted and never null. Changes are sorted lexicographically by kind, source, optional artifact, and optional path.

Conflict objects contain `kind`, canonical scalar `source`, optional `artifact`, and sorted canonical root-relative `paths`. Conflict kinds are `intent`, `ownership`, `drift`, and `removal_required`. The `artifact` field is omitted when absent and is never null. Each `paths` array is sorted first; conflict objects are then sorted lexicographically by kind, source, optional artifact, and the sorted paths.

`denied_sources` is a sorted array of canonical scalar Source References. Recovery detail uses a typed `recovery` object with stable recovery code, sanitized summary, and sorted canonical root-relative observations. `recovery_state_persisted` is a sibling boolean included only when rollback was incomplete.

Warnings are actionable, deterministic, deduplicated strings and never change the Exit Code. They are sorted before rendering and use no warning object schema or warning codes in v1. They are reserved for successful operations requiring attention or secondary recovery failures; routine skipped-version detail is omitted.

JSON output is one compact UTF-8 object followed by a newline and uses snake_case field names. Array order is contractual; object member order is not.

Output includes the full canonical Source Reference, including an approved absolute out-of-root `file:` locator. It suppresses credentials, temporary checkout and cache paths, stack traces, incidental raw OS paths, and raw internal errors.

Human success writes the summary and change lines to standard output and warnings to standard error. Human failure writes the cause, typed detail lines, then warnings to standard error. No ordering between standard output and standard error is guaranteed. JSON keeps the complete result in its single envelope.

V1 uses four **Exit Codes**:

- `0`: applied, no-op, Dry Run, or success with warnings;
- `1`: usage, validation, acquisition, unavailable Resolution, I/O, cancellation, rollback, or other operational failure;
- `2`: user-action conflict or unresolved Recovery State;
- `3`: trust or policy denial.

OS-forced termination may still produce a platform-standard code outside CLI control.

Validation and acquisition failures stop at the first error. Trust Policy denials and user-action conflicts are aggregated and sorted deterministically. Human failures use one concise cause line followed only by actionable typed details. JSON uses Exit Code plus typed `conflicts`, `denied_sources`, and recovery fields; v1 adds no universal error-code taxonomy. Messages are user-readable, not a stable automation API. Output never exposes credentials, raw internal errors, stack traces, or unsafe absolute paths.

V1 has no persisted operation logs, replay command, operation identifiers, retention policy, or verbosity levels.

## Consequences

- Human output stays short by default.
- CI can rely on exit code classes rather than parsing messages.
- Human and JSON output derive from the same deterministic result details.
- Diagnostics exist only for the current invocation in v1.
