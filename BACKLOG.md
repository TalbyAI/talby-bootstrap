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
