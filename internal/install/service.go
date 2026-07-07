package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
)

type Request struct {
	Root        string
	Source      source.Ref
	Artifact    string
	DeclareOnly bool
}

type SyncRequest struct {
	Root string
}

type ChangeKind string

const (
	ChangeDeclared  ChangeKind = "declared"
	ChangeInstalled ChangeKind = "installed"
	ChangeNoOp      ChangeKind = "noop"
)

type Result struct {
	Source   source.Identity
	Artifact source.ArtifactDescriptor
	Change   ChangeKind
	Files    []materialize.FileChange
}

type FileChange = materialize.FileChange

type ConflictError struct {
	SourceName string
	Artifact   string
}

func (e ConflictError) Error() string {
	return fmt.Sprintf(
		`artifact %q from source %q is already declared with different input; use upgrade`,
		e.Artifact,
		e.SourceName,
	)
}

type TrustPolicyError struct {
	SourceType    string
	Locator       string
	OperationRoot string
}

func (e TrustPolicyError) Error() string {
	return fmt.Sprintf(
		`source %q is outside the operation root %q and is not allowed by default`,
		e.Locator,
		e.OperationRoot,
	)
}

type ManagedArtifactRemovalError struct {
	Artifact repositorystate.ManagedArtifactKey
}

func (e ManagedArtifactRemovalError) Error() string {
	return fmt.Sprintf(
		`sync would remove managed artifact %q from source %q; removal requires user action`,
		e.Artifact.Artifact,
		e.Artifact.Source.Name,
	)
}

type Service struct {
	registry source.Registry
	store    repositorystate.Store
}

func NewService(registry source.Registry, store repositorystate.Store) Service {
	return Service{
		registry: registry,
		store:    store,
	}
}

func (s Service) Install(ctx context.Context, req Request) (Result, error) {
	if req.Source.Type == "" {
		return Result{}, fmt.Errorf("source type is required")
	}
	if req.Source.Locator == "" {
		return Result{}, fmt.Errorf("source locator is required")
	}
	if req.DeclareOnly && req.Root == "" {
		return Result{}, fmt.Errorf("repository root is required for declare-only install")
	}
	if err := enforceDirectSourceTrust(req.Root, req.Source); err != nil {
		return Result{}, err
	}

	sourceImpl, err := s.registry.Lookup(req.Source.Type)
	if err != nil {
		return Result{}, err
	}

	resolved, err := sourceImpl.Resolve(ctx, source.ResolveRequest{Ref: req.Source})
	if err != nil {
		return Result{}, err
	}

	artifact, err := selectArtifact(resolved.Artifacts, req.Artifact)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Source:   resolved.Identity,
		Artifact: artifact,
	}
	if req.Root == "" {
		return result, nil
	}

	manifest, err := s.loadManifestOrEmpty(ctx, req.Root)
	if err != nil {
		return Result{}, err
	}

	decl := ManifestDeclaration(req, result)
	next, change := manifest.UpsertDeclaration(decl)
	switch change {
	case repositorystate.ChangeKindInserted:
	case repositorystate.ChangeKindUnchanged:
	default:
		return Result{}, ConflictError{
			SourceName: decl.Source.Name,
			Artifact:   decl.Target.Artifact,
		}
	}
	if change == repositorystate.ChangeKindInserted {
		if err := s.store.WriteManifest(ctx, req.Root, next); err != nil {
			return Result{}, err
		}
	}
	if req.DeclareOnly {
		if change == repositorystate.ChangeKindUnchanged {
			result.Change = ChangeNoOp
		} else {
			result.Change = ChangeDeclared
		}
		return result, nil
	}

	lockfile, err := s.loadLockfileOrEmpty(ctx, req.Root)
	if err != nil {
		return Result{}, err
	}
	nextLockfile, _ := lockfile.UpsertResolution(LockfileResolution(result))
	if err := s.store.WriteLockfile(ctx, req.Root, nextLockfile); err != nil {
		return Result{}, err
	}

	record, err := s.loadMaterializationRecordOrEmpty(ctx, req.Root)
	if err != nil {
		return Result{}, err
	}
	matResult, err := materialize.Apply(ctx, materialize.Request{
		Root:     req.Root,
		Key:      ManagedArtifactKeyFor(result),
		Record:   record,
		Artifact: result.Artifact,
	})
	if err != nil {
		return Result{}, err
	}

	nextRecord := repositorystate.UpsertManagedArtifact(record, ManagedArtifactRecordFor(result, matResult))
	if err := s.store.WriteMaterializationRecord(ctx, req.Root, nextRecord); err != nil {
		rollbackCreatedFiles(matResult.CreatedPaths)
		return Result{}, err
	}

	result.Files = append(result.Files, matResult.Changes...)
	if allFileChangesUnchanged(matResult.Changes) {
		result.Change = ChangeNoOp
	} else {
		result.Change = ChangeInstalled
	}
	return result, nil
}

func (s Service) Sync(ctx context.Context, req SyncRequest) (Result, error) {
	if req.Root == "" {
		return Result{}, fmt.Errorf("repository root is required for sync")
	}

	manifest, err := s.store.LoadManifest(ctx, req.Root)
	if err != nil {
		return Result{}, err
	}
	if len(manifest.Declarations) != 1 {
		return Result{}, fmt.Errorf("sync requires exactly one declaration")
	}
	decl := manifest.Declarations[0]
	if decl.Input == nil || decl.Input.Locator == "" {
		return Result{}, fmt.Errorf("sync requires a persisted source locator")
	}

	lockfile, err := s.store.LoadLockfile(ctx, req.Root)
	if err != nil {
		return Result{}, err
	}
	resolution, err := findResolution(lockfile, decl.Source, decl.Target.Artifact)
	if err != nil {
		return Result{}, err
	}

	record, err := s.loadMaterializationRecordForSync(ctx, req.Root)
	if err != nil {
		return Result{}, err
	}

	ref := source.Ref{
		Type:    decl.Source.Type,
		Locator: decl.Input.Locator,
		Version: resolution.ResolvedVersion,
	}
	if err := enforcePersistedTrust(req.Root, manifest.TrustPolicy, ref, decl.Source); err != nil {
		return Result{}, err
	}

	sourceImpl, err := s.registry.Lookup(ref.Type)
	if err != nil {
		return Result{}, err
	}
	resolved, err := sourceImpl.Resolve(ctx, source.ResolveRequest{Ref: ref})
	if err != nil {
		return Result{}, err
	}
	if resolved.Identity.Version != resolution.ResolvedVersion {
		return Result{}, fmt.Errorf(
			"locked source version %q no longer matches current source snapshot %q",
			resolution.ResolvedVersion,
			resolved.Identity.Version,
		)
	}
	artifact, err := selectArtifact(resolved.Artifacts, decl.Target.Artifact)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Source:   resolved.Identity,
		Artifact: artifact,
	}
	if err := ensureNoManagedArtifactRemoval(record, ManagedArtifactKeyFor(result)); err != nil {
		return Result{}, err
	}
	matResult, err := materialize.Apply(ctx, materialize.Request{
		Root:     req.Root,
		Key:      ManagedArtifactKeyFor(result),
		Record:   record,
		Artifact: artifact,
	})
	if err != nil {
		return Result{}, err
	}

	nextRecord := repositorystate.UpsertManagedArtifact(record, ManagedArtifactRecordFor(result, matResult))
	if err := s.store.WriteMaterializationRecord(ctx, req.Root, nextRecord); err != nil {
		rollbackCreatedFiles(matResult.CreatedPaths)
		return Result{}, err
	}
	result.Files = append(result.Files, matResult.Changes...)
	if allFileChangesUnchanged(matResult.Changes) {
		result.Change = ChangeNoOp
	} else {
		result.Change = ChangeInstalled
	}
	return result, nil
}

func enforcePersistedTrust(root string, policy repositorystate.TrustPolicy, ref source.Ref, identity repositorystate.SourceIdentity) error {
	if isApprovedSource(policy, identity) {
		return nil
	}

	return enforceDirectSourceTrust(root, ref)
}

func enforceDirectSourceTrust(root string, ref source.Ref) error {
	if ref.Type != repositorystate.SourceTypeFile || root == "" {
		return nil
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absLocator, err := filepath.Abs(ref.Locator)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absLocator)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return TrustPolicyError{
			SourceType:    ref.Type,
			Locator:       absLocator,
			OperationRoot: absRoot,
		}
	}

	return nil
}

func (s Service) loadManifestOrEmpty(ctx context.Context, root string) (repositorystate.Manifest, error) {
	manifest, err := s.store.LoadManifest(ctx, root)
	if err == nil {
		return manifest, nil
	}

	var stateErr repositorystate.StateFileError
	if errors.As(err, &stateErr) &&
		stateErr.File == repositorystate.StateFileManifest &&
		stateErr.Kind == repositorystate.StateFileErrorNotFound {
		return repositorystate.Manifest{}, nil
	}

	return repositorystate.Manifest{}, err
}

func (s Service) loadLockfileOrEmpty(ctx context.Context, root string) (repositorystate.Lockfile, error) {
	lockfile, err := s.store.LoadLockfile(ctx, root)
	if err == nil {
		return lockfile, nil
	}

	var stateErr repositorystate.StateFileError
	if errors.As(err, &stateErr) &&
		stateErr.File == repositorystate.StateFileLockfile &&
		stateErr.Kind == repositorystate.StateFileErrorNotFound {
		return repositorystate.Lockfile{}, nil
	}

	return repositorystate.Lockfile{}, err
}

func (s Service) loadMaterializationRecordOrEmpty(ctx context.Context, root string) (repositorystate.MaterializationRecord, error) {
	record, err := s.store.LoadMaterializationRecord(ctx, root)
	if err == nil {
		return record, nil
	}

	var stateErr repositorystate.StateFileError
	if errors.As(err, &stateErr) &&
		stateErr.File == repositorystate.StateFileMaterializationRecord &&
		stateErr.Kind == repositorystate.StateFileErrorNotFound {
		return repositorystate.MaterializationRecord{}, nil
	}

	return repositorystate.MaterializationRecord{}, err
}

func (s Service) loadMaterializationRecordForSync(ctx context.Context, root string) (repositorystate.MaterializationRecord, error) {
	record, err := s.store.LoadMaterializationRecord(ctx, root)
	if err == nil {
		return record, nil
	}

	var stateErr repositorystate.StateFileError
	if errors.As(err, &stateErr) &&
		stateErr.File == repositorystate.StateFileMaterializationRecord &&
		stateErr.Kind == repositorystate.StateFileErrorNotFound {
		return repositorystate.MaterializationRecord{}, fmt.Errorf("sync requires existing materialization record")
	}

	return repositorystate.MaterializationRecord{}, err
}

func isApprovedSource(policy repositorystate.TrustPolicy, identity repositorystate.SourceIdentity) bool {
	for _, approved := range policy.ApprovedSources {
		if approved == identity {
			return true
		}
	}

	return false
}

func ensureNoManagedArtifactRemoval(record repositorystate.MaterializationRecord, desired repositorystate.ManagedArtifactKey) error {
	for _, artifact := range record.Artifacts {
		if artifact.Key != desired {
			return ManagedArtifactRemovalError{Artifact: artifact.Key}
		}
	}

	return nil
}

func selectArtifact(artifacts []source.ArtifactDescriptor, wanted string) (source.ArtifactDescriptor, error) {
	if wanted != "" {
		for _, artifact := range artifacts {
			if artifact.Name == wanted {
				return artifact, nil
			}
		}

		return source.ArtifactDescriptor{}, fmt.Errorf("artifact %q was not found", wanted)
	}

	if len(artifacts) != 1 {
		return source.ArtifactDescriptor{}, fmt.Errorf("install target must resolve to exactly one artifact")
	}

	return artifacts[0], nil
}

func findResolution(lockfile repositorystate.Lockfile, sourceID repositorystate.SourceIdentity, artifact string) (repositorystate.Resolution, error) {
	for _, resolution := range lockfile.Resolutions {
		if resolution.Source == sourceID && resolution.Artifact.Name == artifact {
			return resolution, nil
		}
	}
	return repositorystate.Resolution{}, fmt.Errorf("sync requires a matching persisted resolution")
}

func allFileChangesUnchanged(changes []materialize.FileChange) bool {
	if len(changes) == 0 {
		return true
	}
	for _, change := range changes {
		if change.Action != "unchanged" {
			return false
		}
	}
	return true
}

func rollbackCreatedFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
