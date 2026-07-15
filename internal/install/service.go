package install

import (
	"context"
	"errors"
	"fmt"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
	"path/filepath"
	"slices"
	"strings"
)

type Request struct {
	Root        string
	Source      source.Ref
	Artifact    string
	DeclareOnly bool
}
type SyncRequest struct{ Root string }
type Outcome string

const (
	OutcomeNoOp     Outcome = "no_op"
	OutcomeApplied  Outcome = "applied"
	OutcomeConflict Outcome = "conflict"
)

type ChangeKind string

const (
	ChangeDeclarationAdded ChangeKind = "declaration_added"
	ChangeFileCreated      ChangeKind = "file_created"
	ChangeFileUpdated      ChangeKind = "file_updated"
	ChangeOwnershipAdopted ChangeKind = "ownership_adopted"
	ChangeResolutionLocked ChangeKind = "resolution_locked"
	ChangeLockPruned       ChangeKind = "lock_pruned"
)

type OwnershipKind string

const OwnershipWholeFile OwnershipKind = "whole_file"

type Change struct {
	Kind          ChangeKind                     `json:"kind"`
	Source        repositorystate.SourceIdentity `json:"source"`
	SourceVersion string                         `json:"source_version,omitempty"`
	Artifact      string                         `json:"artifact,omitempty"`
	Path          string                         `json:"path,omitempty"`
	OwnershipKind OwnershipKind                  `json:"ownership_kind,omitempty"`
}
type ConflictKind string

const (
	ConflictIntent          ConflictKind = "intent"
	ConflictOwnership       ConflictKind = "ownership"
	ConflictDrift           ConflictKind = "drift"
	ConflictRemovalRequired ConflictKind = "removal_required"
)

type Conflict struct {
	Kind     ConflictKind                   `json:"kind"`
	Source   repositorystate.SourceIdentity `json:"source"`
	Artifact string                         `json:"artifact,omitempty"`
	Paths    []string                       `json:"paths,omitempty"`
}
type Result struct {
	Operation     string     `json:"operation"`
	Outcome       Outcome    `json:"outcome"`
	ArtifactCount int        `json:"artifact_count"`
	Changes       []Change   `json:"changes,omitempty"`
	Conflicts     []Conflict `json:"conflicts,omitempty"`
}
type UserActionError struct{ Result Result }

func (e UserActionError) Error() string {
	return fmt.Sprintf("operation has %d conflict(s)", len(e.Result.Conflicts))
}

type TrustPolicyError struct {
	Denied []repositorystate.SourceIdentity
}

func (e TrustPolicyError) Error() string {
	values := append([]repositorystate.SourceIdentity(nil), e.Denied...)
	slices.SortFunc(values, func(a, b repositorystate.SourceIdentity) int {
		return strings.Compare(repositorystate.SourceIdentityKey(a), repositorystate.SourceIdentityKey(b))
	})
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = v.Type + ":" + v.Locator
	}
	return "unapproved sources: " + strings.Join(parts, ", ")
}

type Service struct {
	registry source.Registry
	store    repositorystate.Store
}

func NewService(registry source.Registry, store repositorystate.Store) Service {
	return Service{registry: registry, store: store}
}
func (service Service) Install(ctx context.Context, request Request) (Result, error) {
	if request.Root == "" {
		return Result{}, fmt.Errorf("repository root is required")
	}
	if request.Source.Type == "" {
		return Result{}, fmt.Errorf("source type is required")
	}
	if request.Source.Locator == "" {
		return Result{}, fmt.Errorf("source locator is required")
	}
	if request.Source.Version != "" {
		return Result{}, fmt.Errorf("requested source versions are not supported")
	}
	identity, err := repositorystate.NormalizeSourceIdentity(request.Root, repositorystate.SourceIdentity{Type: request.Source.Type, Locator: request.Source.Locator})
	if err != nil {
		return Result{}, err
	}
	manifest, err := service.loadManifestOrEmpty(ctx, request.Root)
	if err != nil {
		return Result{}, err
	}
	declaration := declarationFor(request, identity)
	next, kind, err := manifest.AddDeclaration(request.Root, declaration)
	if err != nil {
		result := resultForConflicts("install", 0, []Conflict{{Kind: ConflictIntent, Source: identity, Artifact: request.Artifact}})
		return result, UserActionError{Result: result}
	}
	if request.DeclareOnly && kind == repositorystate.ChangeKindUnchanged {
		return Result{Operation: "install", Outcome: OutcomeNoOp}, nil
	}
	if !approved(manifest.TrustPolicy, identity) && outsideRoot(identity) {
		return Result{}, TrustPolicyError{Denied: []repositorystate.SourceIdentity{identity}}
	}
	acquire, err := repositorystate.AcquisitionLocator(request.Root, identity)
	if err != nil {
		return Result{}, err
	}
	impl, err := service.registry.Lookup(identity.Type)
	if err != nil {
		return Result{}, err
	}
	resolved, err := impl.Resolve(ctx, source.ResolveRequest{Ref: source.Ref{Type: identity.Type, Locator: acquire}})
	if err != nil {
		return Result{}, err
	}
	selected, err := selectedArtifacts(resolved, declaration.Target)
	if err != nil {
		return Result{}, err
	}
	if request.DeclareOnly {
		if kind == repositorystate.ChangeKindInserted {
			if err := service.store.WriteManifest(ctx, request.Root, next); err != nil {
				return Result{}, err
			}
			return Result{Operation: "install", Outcome: OutcomeApplied, ArtifactCount: len(selected), Changes: []Change{{Kind: ChangeDeclarationAdded, Source: identity, Artifact: request.Artifact}}}, nil
		}
		return Result{Operation: "install", Outcome: OutcomeNoOp, ArtifactCount: len(selected)}, nil
	}
	lock, err := service.loadLockfileOrEmpty(ctx, request.Root)
	if err != nil {
		return Result{}, err
	}
	record, err := service.loadMaterializationRecordOrEmpty(ctx, request.Root)
	if err != nil {
		return Result{}, err
	}
	var locked *repositorystate.Resolution
	if declaration.Target.Scope == repositorystate.DeclarationScopeArtifact {
		if resolution, _, ok := lock.Artifact(repositorystate.ArtifactKey{Source: identity, Name: request.Artifact}); ok {
			locked = &resolution
		}
	} else {
		for _, resolution := range lock.Resolutions {
			if resolution.Source == identity {
				if locked != nil {
					return Result{}, fmt.Errorf("multiple locked snapshots for source")
				}
				copy := resolution
				locked = &copy
			}
		}
	}
	if locked != nil {
		selected, err = verifyLocked(declaration, *locked, resolved)
		if err != nil {
			return Result{}, err
		}
	}
	desired := make([]desiredArtifact, 0, len(selected))
	for _, artifact := range selected {
		desired = append(desired, desiredArtifact{Key: repositorystate.ArtifactKey{Source: identity, Name: artifact.Name}, Resolution: repositorystate.ArtifactResolution{Name: artifact.Name, Version: artifact.Version}, ResolvedVersion: resolved.Identity.Version, Descriptor: artifact, InputPaths: resolved.InputPaths})
	}
	prepared := preparedOperation{Desired: desired, Lockfile: lock, Record: record}
	if locked == nil {
		resolution := resolutionFor(identity, resolved, selected)
		var change repositorystate.ChangeKind
		prepared.Lockfile, change, err = lock.UpsertResolution(resolution)
		if err != nil {
			return Result{}, err
		}
		if change != repositorystate.ChangeKindUnchanged {
			prepared.Changes = append(prepared.Changes, Change{Kind: ChangeResolutionLocked, Source: identity, SourceVersion: resolved.Identity.Version, Artifact: request.Artifact})
		}
	}
	if kind == repositorystate.ChangeKindInserted {
		prepared.Changes = append(prepared.Changes, Change{Kind: ChangeDeclarationAdded, Source: identity, Artifact: request.Artifact})
	}
	prepared.Files, prepared.Conflicts, err = preflightFiles(request.Root, desired, record)
	if err != nil {
		return Result{}, err
	}
	if len(prepared.Conflicts) > 0 {
		result := resultForConflicts("install", len(desired), prepared.Conflicts)
		return result, UserActionError{Result: result}
	}
	return service.persistPrepared(ctx, request.Root, "install", prepared, &next)
}
func selectedArtifacts(resolved source.ResolvedSource, target repositorystate.DeclarationTarget) ([]source.ArtifactDescriptor, error) {
	if target.Scope == repositorystate.DeclarationScopeSource {
		if len(resolved.Artifacts) == 0 {
			return nil, fmt.Errorf("source must contain at least one artifact")
		}
		return append([]source.ArtifactDescriptor(nil), resolved.Artifacts...), nil
	}
	for _, a := range resolved.Artifacts {
		if a.Name == target.Artifact {
			return []source.ArtifactDescriptor{a}, nil
		}
	}
	return nil, fmt.Errorf("artifact %q was not found", target.Artifact)
}
func (service Service) loadManifestOrEmpty(ctx context.Context, root string) (repositorystate.Manifest, error) {
	m, err := service.store.LoadManifest(ctx, root)
	if stateNotFound(err, repositorystate.StateFileManifest) {
		return repositorystate.Manifest{}, nil
	}
	return m, err
}
func (service Service) loadLockfileOrEmpty(ctx context.Context, root string) (repositorystate.Lockfile, error) {
	v, err := service.store.LoadLockfile(ctx, root)
	if stateNotFound(err, repositorystate.StateFileLockfile) {
		return repositorystate.Lockfile{}, nil
	}
	return v, err
}
func (service Service) loadMaterializationRecordOrEmpty(ctx context.Context, root string) (repositorystate.MaterializationRecord, error) {
	v, err := service.store.LoadMaterializationRecord(ctx, root)
	if stateNotFound(err, repositorystate.StateFileMaterializationRecord) {
		return repositorystate.MaterializationRecord{}, nil
	}
	return v, err
}
func stateNotFound(err error, file repositorystate.StateFile) bool {
	var e repositorystate.StateFileError
	return errors.As(err, &e) && e.File == file && e.Kind == repositorystate.StateFileErrorNotFound
}
func approved(policy repositorystate.TrustPolicy, source repositorystate.SourceIdentity) bool {
	for _, a := range policy.ApprovedSources {
		if a == source {
			return true
		}
	}
	return false
}
func outsideRoot(identity repositorystate.SourceIdentity) bool {
	return filepath.IsAbs(filepath.FromSlash(identity.Locator))
}
func resultForConflicts(operation string, count int, conflicts []Conflict) Result {
	return Result{Operation: operation, Outcome: OutcomeConflict, ArtifactCount: count, Conflicts: conflicts}
}
