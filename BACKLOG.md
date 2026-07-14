# Backlog

Ideas awaiting discussion. Inclusion does not mean acceptance as a feature.

## Initialize empty repository configuration

Priority: unprioritized

Add an `init` command that initializes Talby Bootstrap configuration for a repository. The resulting repository configuration is empty.

## Diagnose configured Artifact state

Priority: unprioritized

Add a `doctor` command that visits every configured **Artifact** and verifies the state of each **Materialization Step**.

Some checks may be declarative. For example, a `file` step may be valid when its file exists and its content has not changed from the managed state.

Other checks describe external prerequisites rather than materialized files. An **Artifact** may require a globally installed tool, such as Node.js at a compatible version. Talby Bootstrap reports whether that requirement is satisfied; it does not install the external tool.

This idea needs further discussion before acceptance, including whether the **Artifact Descriptor** or its **Materialization Steps** need additional verification declarations and how each step type proves valid state.
