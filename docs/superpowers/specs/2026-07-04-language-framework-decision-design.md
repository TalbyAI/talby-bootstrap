# Language and Framework Decision Design

## Status

Approved for planning.

## Context

Talby Bootstrap is a CLI named `tbboot` for reconciling reusable repository artifacts into a consumer repository or folder. V1 is non-interactive and must support predictable command execution, machine-readable output, stable exit codes, and safe materialization.

The primary distribution goal is installation through developer ecosystems rather than optimizing only for direct binary download. The expected audience is mixed across technology stacks, so the implementation language should not assume a .NET, Node, or frontend-heavy user base.

V2 is expected to add an interactive terminal mode for catalog browsing, source search, and log inspection. The v1 decision should avoid blocking that future mode without pulling a TUI framework into the first implementation.

## Decision

Implement `tbboot` in Go using Cobra for the v1 command surface.

Keep command parsing separate from the application core. Cobra commands should translate flags and arguments into explicit option structs, then call core functions such as install, upgrade, search, logs, and catalog operations.

Do not add an interactive UI framework in v1. If v2 needs a terminal UI, use Bubble Tea and reuse the same core operations instead of shelling out to Cobra commands.

## Alternatives Considered

### Go + Cobra

Go gives `go install ...@latest` distribution, simple cross-platform binaries, and no runtime requirement for users who consume releases directly. Cobra covers subcommands, flags, aliases, help text, and command validation with little custom code.

Bubble Tea is a natural later fit for an interactive terminal mode in the same language.

### .NET + Spectre.Console.Cli

.NET has a strong CLI ecosystem, good command modeling, and polished terminal output through Spectre.Console. It is the best option if Talby Bootstrap becomes primarily a .NET or Aspire-adjacent tool.

It is not the default because the audience is mixed and `dotnet tool` or runtime requirements are less neutral than Go for cross-stack repository tooling.

### TypeScript

TypeScript is strong when npm is the natural distribution channel and the user base already has Node installed. It also has good prompt and terminal UI libraries.

It is not the default because Node as a runtime adds more friction for a stack-neutral repository tool.

## Architecture

V1 should use a small boundary between CLI parsing and behavior:

- `cmd` package: Cobra command tree, flag parsing, aliases, validation, help text.
- Core packages: resolution, trust policy, materialization, operation reporting, catalog management, and logs.
- Reporting boundary: human summaries and JSON output envelope share the same operation result data.

The core should not depend on Cobra. This keeps tests direct and gives v2 interactive screens a clean API to call.

## Data and Control Flow

1. Cobra parses command name, flags, and positional arguments.
2. The command builds a typed options value.
3. The command calls the relevant core operation with context, options, filesystem access, and reporter/output dependencies.
4. The core returns an operation result with code, message, details, warnings, and any recorded operation metadata.
5. The command maps the result to human output or the JSON output envelope and exits with the documented exit code.

## Error Handling

CLI parse errors fail before core execution and return operational or validation error output.

Core errors are classified into the existing v1 exit code model:

- `0`: success, including success with warnings.
- `1`: operational or validation error.
- `2`: user-action conflict, including required prompts in non-interactive or JSON output.
- `3`: trust or policy denial.

Interactive prompts are out of scope for v1. Any action that would require a prompt must return exit code `2` unless an explicit non-interactive override exists.

## Testing

Use focused Go tests:

- Core unit tests for command-independent behavior.
- Small CLI smoke tests for command parsing, aliases, output mode selection, and exit code mapping.
- Golden output only where stable text shape is part of the contract.

Avoid broad end-to-end suites until real materialization behavior exists.

## Deferred Work

V2 may add Bubble Tea for interactive search, catalog browsing, and log inspection.

Configuration helpers such as Viper are deferred until configuration needs exceed straightforward file parsing and environment handling.

Additional distribution channels are deferred until the first release path proves insufficient.
