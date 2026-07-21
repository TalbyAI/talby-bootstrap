# Backlog

Ideas awaiting discussion. Inclusion does not mean acceptance as a feature.

## Diagnose configured Artifact state

Priority: unprioritized

Add a `doctor` command that visits every configured **Artifact** and verifies the state of each **Materialization Step**.

Some checks may be declarative. For example, a `file` step may be valid when its file exists and its content has not changed from the managed state.

Other checks describe external prerequisites rather than materialized files. An **Artifact** may require a globally installed tool, such as Node.js at a compatible version. Talby Bootstrap reports whether that requirement is satisfied; it does not install the external tool.

This idea needs further discussion before acceptance, including whether the **Artifact Descriptor** or its **Materialization Steps** need additional verification declarations and how each step type proves valid state.

## Declare inline Artifacts and export them as Sources

Priority: unprioritized

Allow a consumer repository's **Manifest** to declare an **Artifact** inline, including its **Materialization Steps**, without acquiring it from a repository or external **Source**. The local Artifact should be testable and approvable in the consumer repository, then exportable as a publishable external Source once it is proven and approved.

This idea needs further discussion before acceptance, including the inline Manifest syntax, how inline Artifacts participate in locking and identity, what approval state is required, and how export preserves provenance and produces the external Source layout.

## Define phase-specific Materialization Step operations

Priority: unprioritized

Allow each **Materialization Step** to define separate commands for applying its change to the destination, detecting whether it is correctly installed, and uninstalling it. By default, the step type supplies the installation command and generally determines the uninstall and detection commands, but a step may define each operation independently.

This idea needs further discussion before acceptance, including the operation names, command contracts, and how step-type defaults can be overridden.

## Parameterize Source operations

Priority: unprioritized

Implement parameter definitions at the **Source** level. Each **Artifact** may use those parameters when executing its installation, uninstallation, or verification operations.

This idea needs further discussion before acceptance, including parameter declaration syntax, value resolution, validation, and how parameters participate in locking and reproducibility.

## Add interactive CLI mode

Priority: unprioritized

Add an interactive mode to the CLI. When invoked, the mode may receive additional parameters that condition its behavior; once entered, it can execute commands and present selectable options interactively.

This idea needs further discussion before acceptance, including the interactive command surface, parameter handling, and behavior in non-interactive environments.

## Add additional Materialization Step types

Priority: unprioritized

Support step types including `whole file`, `fragment`, `prompts`, and `bash commands`, along with other step types identified later.

This idea needs further discussion before acceptance, including each type's input format, installation behavior, verification behavior, and uninstall behavior.

## Support multiple shells for Shell command steps

Priority: unprioritized

Provide Shell command compatibility for different shells, including POSIX-compatible shells and PowerShell.

This idea needs further discussion before acceptance, including shell selection, command representation, platform detection, and portability requirements.

## Declare dependencies between Sources and Artifacts

Priority: unprioritized

Implement dependencies between **Sources** and **Artifacts** so an **Artifact** can declare that another Artifact must be installed first.

This idea needs further discussion before acceptance, including dependency identity, ordering, cycle detection, and failure handling.
