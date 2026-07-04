# ADR-0005: Operation Output, Logs, and Exit Codes

## Status

Accepted for v1.

## Context

The CLI needs useful default output for humans and stable classification for automation. Verbose diagnostics should be available without making every successful command noisy.

## Decision

Successful install and sync operations show a short **Operation Summary** by default. Extra detail is selected by **Verbosity Level**: `summary`, `normal`, or `verbose`.

Machine-readable output uses one **JSON Output Envelope** with `code`, `message`, and `details`.

V1 uses four **Exit Codes**:

- `0`: success, including success with warnings;
- `1`: operational or validation error;
- `2`: user-action conflict, including required prompts in non-interactive or JSON output;
- `3`: trust or policy denial.

Operations are recorded under the **User Configuration Directory** and scoped by **Operation Root**. `logs` without arguments replays the most recent operation for the current root. `logs list` lists recorded operations. `logs <operation-id>` replays a chosen operation, defaulting to the original verbosity level unless overridden.

The default **Operation Retention Policy** keeps at most 100 operations and no more than one week of history.

## Consequences

- Human output stays short by default.
- CI can rely on exit code classes rather than parsing messages.
- Diagnostics can be inspected after the fact without re-running materialization.
- Operation history is intentionally bounded in v1.
