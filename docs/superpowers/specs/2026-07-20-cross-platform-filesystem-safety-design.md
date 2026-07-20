# Cross-platform filesystem safety design

## Goal

Implement issue #31 on top of the completed contract reset: every mutating
install operation uses one canonical Operation Root, serializes mutations for
that root, validates source and target topology before writes, and materializes
files through deterministic atomic replacement. Dry Run remains fully
non-mutating.

## Current context

The CLI already discovers a Git repository root for `install` and `Sync`, and
the install service already has preflight ownership checks. The materialization
package already uses `os.Root`, same-directory temporary files, mode handling,
and target revalidation. Repository-state writers already use atomic
same-directory replacement. This work tightens those existing seams instead of
introducing a new filesystem abstraction.

## Chosen approach

Use the standard library and existing package boundaries:

- `cmd/tbboot` owns root discovery and the `--dry-run` flag.
- `internal/install` owns operation locking, request/result propagation, and
  source/target preflight coordination.
- `internal/materialize` owns confined observation, identity checks, race
  revalidation, and atomic target replacement.
- `internal/repositorystate` keeps its existing atomic state writers.

An exclusive `.tbboot-operation.lock` directory in the Operation Root is the
cross-platform lock. Creating it with `os.Mkdir` is atomic; an existing path
fails the operation. The lock is removed after the operation completes. A
stale lock is not taken over automatically; this keeps lock ownership
deterministic and avoids adding platform-specific locking code in this slice.

## Operation flow

For mutating `install` and bare `install`/`Sync`:

1. Resolve and canonicalize the Operation Root.
2. Acquire the root lock before reading repository state or resolving a source.
3. Load and validate manifest, lockfile, and materialization state.
4. Resolve sources and build the complete desired operation.
5. Validate source inputs, target paths, topology, special-file status, and
   conflicting path identities before any write.
6. Revalidate targets immediately before each atomic replacement.
7. Write materialized files and repository state through existing atomic
   writers.
8. Release the lock.

For Dry Run, the same read, resolve, and preflight flow runs without acquiring
the lock and without writing files, manifest, lockfile, or materialization
state. Changes are reported with `planned`; no changes remain `no_op`.

## Root discovery

`repositoryRoot` runs `git rev-parse --show-toplevel` from the current working
directory and canonicalizes the result. It falls back to the canonical current
directory only when Git explicitly reports that the directory is not a
repository. Missing Git, permission errors, malformed output, and other Git
failures are returned instead of silently changing the scope of the operation.

## Safety rules

- Relative source and target paths are normalized against the canonical root.
- Every consumed source path and every target parent component must be a real
  directory or regular file as appropriate; symlinks, Windows reparse points,
  and special files are rejected.
- Escapes outside the Operation Root or source root are rejected before
  mutation. Existing trust-policy behavior for approved external `file:`
  sources remains unchanged.
- Target collisions are checked using canonical path keys and filesystem
  identity, including platform case behavior and hard-link aliases where the
  operating system exposes identity.
- Existing target modes are preserved; new files use `0644`.
- Replacement uses a unique temporary file in the target's directory followed
  by confined same-directory rename, so readers never observe a partially
  written file.
- Root, parent, target, and opened-directory identity are rechecked immediately
  before replacement. A race becomes the existing typed drift/conflict path.

## API and output changes

- Add `DryRun` to install and sync requests.
- Add `OutcomePlanned` and `DryRun` to operation results.
- Include `dry_run` in the JSON operation details for every result.
- Add `--dry-run` to `install`, including explicit installs and declaration-only
  installs.
- Preserve existing human output shape while labeling planned changes as
  planned rather than applied.

## Testing

Add focused tests at public seams:

- root discovery fallback and propagation of non-repository Git errors;
- lock acquisition, rejection of concurrent operations, and cleanup;
- Dry Run proving no lock, state, or target writes;
- symlink/reparse/special-file, escape, hard-link, and case-normalization
  rejection;
- mode preservation, same-directory atomic replacement, and race
  revalidation;
- human and JSON output, including `dry_run` and `planned`.

Run focused package tests while implementing, then `just check`, plus a Windows
cross-build/test compilation for platform-specific code.

## Scope boundaries

This slice does not add Git source acquisition, crash-recoverable advisory
locks, rollback/recovery lifecycle, new materialization step types, or a broad
filesystem interface. Those require separate contracts or platform-specific
design work.
