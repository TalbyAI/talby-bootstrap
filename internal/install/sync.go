package install

import (
	"context"
	"errors"
	"fmt"
	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type desiredArtifact struct {
	Key             repositorystate.ArtifactKey
	Resolution      repositorystate.ArtifactResolution
	ResolvedVersion string
	Descriptor      source.ArtifactDescriptor
	InputPaths      []string
}
type plannedFile struct {
	Artifact desiredArtifact
	Step     source.MaterializationStep
	Observed materialize.Observation
	Digest   string
	Change   ChangeKind
}
type preparedOperation struct {
	Desired   []desiredArtifact
	Files     []plannedFile
	Lockfile  repositorystate.Lockfile
	Record    repositorystate.MaterializationRecord
	Changes   []Change
	Conflicts []Conflict
}

func (service Service) Sync(ctx context.Context, request SyncRequest) (result Result, err error) {
	if request.Root == "" {
		return Result{}, fmt.Errorf("repository root is required for sync")
	}
	operation, release, err := openOperationRoot(request.Root, request.DryRun)
	if err != nil {
		return Result{}, err
	}
	request.Root = operation.path
	if release != nil {
		defer func() {
			if releaseErr := release(); releaseErr != nil {
				err = errors.Join(err, fmt.Errorf("release operation lock: %w", releaseErr))
			}
		}()
	}
	if err := operation.validate(); err != nil {
		return Result{}, err
	}
	manifest, err := service.store.LoadManifest(ctx, request.Root)
	if err != nil {
		return Result{}, err
	}
	lock, _, err := service.loadLockfile(ctx, request.Root)
	if err != nil {
		return Result{}, err
	}
	record, _, err := service.loadMaterializationRecord(ctx, request.Root)
	if err != nil {
		return Result{}, err
	}
	if err := repositorystate.ValidateCrossDocumentState(lock, record); err != nil {
		return Result{}, err
	}
	prepared, err := service.prepare(ctx, request.Root, manifest, lock, record, operation.info)
	if err != nil {
		return Result{}, err
	}
	if len(prepared.Conflicts) > 0 {
		result := resultForConflicts("sync", len(prepared.Desired), prepared.Conflicts, request.DryRun)
		return result, UserActionError{Result: result}
	}
	return service.persistPrepared(ctx, request.Root, "sync", prepared, nil, request.DryRun, operation)
}
func (service Service) prepare(ctx context.Context, root string, manifest repositorystate.Manifest, lock repositorystate.Lockfile, record repositorystate.MaterializationRecord, expectedRoot ...os.FileInfo) (preparedOperation, error) {
	declarations := append([]repositorystate.Declaration(nil), manifest.Declarations...)
	slices.SortFunc(declarations, func(a, b repositorystate.Declaration) int {
		return strings.Compare(repositorystate.DeclarationKey(a), repositorystate.DeclarationKey(b))
	})
	denied := []repositorystate.SourceIdentity{}
	for _, d := range declarations {
		if !approved(manifest.TrustPolicy, d.Source) && outsideRoot(d.Source) {
			denied = append(denied, d.Source)
		}
	}
	if len(denied) > 0 {
		return preparedOperation{}, TrustPolicyError{Denied: denied}
	}
	prepared := preparedOperation{Lockfile: lock, Record: record}
	for _, d := range declarations {
		desired, maybe, err := service.resolveDeclaration(ctx, root, d, prepared.Lockfile, record)
		if err != nil {
			return preparedOperation{}, err
		}
		prepared.Desired = append(prepared.Desired, desired...)
		if maybe != nil {
			next, change, err := prepared.Lockfile.UpsertResolution(*maybe)
			if err != nil {
				return preparedOperation{}, err
			}
			prepared.Lockfile = next
			if change != repositorystate.ChangeKindUnchanged {
				prepared.Changes = append(prepared.Changes, Change{Kind: ChangeResolutionLocked, Source: d.Source, SourceVersion: maybe.ResolvedVersion, Artifact: d.Target.Artifact})
			}
		}
	}
	slices.SortFunc(prepared.Desired, func(a, b desiredArtifact) int {
		return strings.Compare(repositorystate.SourceIdentityKey(a.Key.Source)+"\x00"+a.Key.Name, repositorystate.SourceIdentityKey(b.Key.Source)+"\x00"+b.Key.Name)
	})
	if len(expectedRoot) > 0 && expectedRoot[0] != nil {
		if err := validateRootIdentity(root, expectedRoot[0]); err != nil {
			return preparedOperation{}, err
		}
	}
	files, conflicts, err := preflightFiles(root, prepared.Desired, record, expectedRoot...)
	if err != nil {
		return preparedOperation{}, err
	}
	prepared.Files = files
	prepared.Conflicts = conflicts
	var changes []Change
	prepared.Lockfile, changes, conflicts, err = prepareSyncUndesired(root, prepared.Desired, prepared.Lockfile, record, expectedRoot...)
	if err != nil {
		return preparedOperation{}, err
	}
	prepared.Changes = append(prepared.Changes, changes...)
	prepared.Conflicts = append(prepared.Conflicts, conflicts...)
	sortConflicts(prepared.Conflicts)
	return prepared, nil
}
func (service Service) resolveDeclaration(ctx context.Context, root string, d repositorystate.Declaration, lock repositorystate.Lockfile, record repositorystate.MaterializationRecord) ([]desiredArtifact, *repositorystate.Resolution, error) {
	acquire, err := repositorystate.AcquisitionLocator(root, d.Source)
	if err != nil {
		return nil, nil, err
	}
	impl, err := service.registry.Lookup(d.Source.Type)
	if err != nil {
		return nil, nil, err
	}
	resolved, err := impl.Resolve(ctx, source.ResolveRequest{Ref: source.Ref{Type: d.Source.Type, Locator: acquire}})
	if err != nil {
		return nil, nil, err
	}
	var selected []source.ArtifactDescriptor
	var locked *repositorystate.Resolution
	if d.Target.Scope == repositorystate.DeclarationScopeArtifact {
		r, _, ok := lock.Artifact(repositorystate.ArtifactKey{Source: d.Source, Name: d.Target.Artifact})
		if ok {
			selected, err = verifyLocked(d, r, resolved)
			locked = &r
		} else {
			selected, err = selectedArtifacts(resolved, d.Target)
		}
	} else {
		for _, r := range lock.Resolutions {
			if r.Source == d.Source {
				if locked != nil {
					return nil, nil, fmt.Errorf("multiple locked snapshots for source")
				}
				x := r
				locked = &x
			}
		}
		if locked != nil {
			selected, err = verifyLocked(d, *locked, resolved)
		} else {
			selected, err = selectedArtifacts(resolved, d.Target)
		}
	}
	if err != nil {
		return nil, nil, err
	}
	out := make([]desiredArtifact, 0, len(selected))
	for _, a := range selected {
		out = append(out, desiredArtifact{Key: repositorystate.ArtifactKey{Source: d.Source, Name: a.Name}, Resolution: repositorystate.ArtifactResolution{Name: a.Name, Version: a.Version}, ResolvedVersion: resolved.Identity.Version, Descriptor: a, InputPaths: resolved.InputPaths})
	}
	if locked == nil {
		r := resolutionFor(d.Source, resolved, selected)
		return out, &r, nil
	}
	return out, nil, nil
}
func verifyLocked(d repositorystate.Declaration, locked repositorystate.Resolution, resolved source.ResolvedSource) ([]source.ArtifactDescriptor, error) {
	if locked.ResolvedVersion != resolved.Identity.Version {
		return nil, fmt.Errorf("locked source version %q no longer matches current source snapshot %q", locked.ResolvedVersion, resolved.Identity.Version)
	}
	selected, err := selectedArtifacts(resolved, d.Target)
	if err != nil {
		return nil, err
	}
	if d.Target.Scope == repositorystate.DeclarationScopeSource && len(selected) != len(locked.Artifacts) {
		return nil, fmt.Errorf("locked artifact set no longer matches current source")
	}
	versions := map[string]string{}
	for _, a := range locked.Artifacts {
		versions[a.Name] = a.Version
	}
	for _, a := range selected {
		if versions[a.Name] != a.Version {
			return nil, fmt.Errorf("locked artifact version no longer matches current source")
		}
	}
	return selected, nil
}

func observeTarget(root, target string, expectedRoot ...os.FileInfo) (materialize.Observation, error) {
	observed, err := materialize.Observe(root, target)
	if err != nil {
		return materialize.Observation{}, err
	}
	if len(expectedRoot) > 0 && expectedRoot[0] != nil && !materialize.SameRootIdentity(observed, expectedRoot[0]) {
		return materialize.Observation{}, materialize.ChangedSincePreflightError{Path: "."}
	}
	return observed, nil
}

func preflightFiles(root string, desired []desiredArtifact, record repositorystate.MaterializationRecord, expectedRoot ...os.FileInfo) ([]plannedFile, []Conflict, error) {
	activeInputs := map[string]struct{}{}
	for _, artifact := range desired {
		for _, input := range artifact.InputPaths {
			activeInputs[materialize.PathKey(input)] = struct{}{}
		}
	}
	owners := map[string]repositorystate.ArtifactKey{}
	for _, a := range record.Artifacts {
		for _, f := range a.Files {
			owners[materialize.PathKey(filepath.FromSlash(f.Path))] = repositorystate.ManagedArtifactKey(a)
		}
	}
	var files []plannedFile
	var conflicts []Conflict
	claimed := map[string]desiredArtifact{}
	for _, artifact := range desired {
		managed, isManaged := record.Artifact(artifact.Key)
		if isManaged {
			if managed.ResolvedVersion != artifact.ResolvedVersion || managed.ArtifactVersion != artifact.Resolution.Version {
				return nil, nil, fmt.Errorf("managed artifact version does not match desired resolution")
			}
		}
		observations := make([]materialize.Observation, len(artifact.Descriptor.Steps))
		expected := map[string]struct{}{}
		for i, step := range artifact.Descriptor.Steps {
			if step.Type != "file" {
				return nil, nil, fmt.Errorf("unsupported step type %q", step.Type)
			}
			observed, err := observeTarget(root, step.TargetPath, expectedRoot...)
			if err != nil {
				return nil, nil, err
			}
			path := materialize.PathKey(filepath.FromSlash(observed.Path))
			if _, ok := expected[path]; ok {
				return nil, nil, fmt.Errorf("duplicate desired target %q", step.TargetPath)
			}
			expected[path] = struct{}{}
			observations[i] = observed
		}
		if isManaged {
			if len(expected) != len(managed.Files) {
				return nil, nil, fmt.Errorf("managed artifact path set does not match desired artifact")
			}
			for _, file := range managed.Files {
				if _, ok := expected[materialize.PathKey(filepath.FromSlash(file.Path))]; !ok {
					return nil, nil, fmt.Errorf("managed artifact path set does not match desired artifact")
				}
			}
		}
	targetSteps:
		for i, step := range artifact.Descriptor.Steps {
			observed := observations[i]
			path := materialize.PathKey(observed.AbsolutePath)
			relativePath := materialize.PathKey(filepath.FromSlash(observed.Path))
			for _, name := range []string{repositorystate.ManifestFileName, repositorystate.LockfileFileName, repositorystate.MaterializationRecordFileName, repositorystate.RecoveryStateFileName} {
				reserved := materialize.PathKey(filepath.FromSlash(name))
				if relativePath == reserved || strings.HasPrefix(relativePath, reserved+string(filepath.Separator)) {
					return nil, nil, fmt.Errorf("target %q is reserved", step.TargetPath)
				}
			}
			lockPath := materialize.PathKey(filepath.FromSlash(operationLockName))
			if relativePath == lockPath || strings.HasPrefix(relativePath, lockPath+string(filepath.Separator)) {
				return nil, nil, fmt.Errorf("target %q is reserved", step.TargetPath)
			}
			if _, ok := activeInputs[path]; ok {
				return nil, nil, fmt.Errorf("target %q overlaps source input", step.TargetPath)
			}
			for _, input := range artifact.InputPaths {
				same, err := materialize.SamePathIdentity(observed, input)
				if err != nil {
					return nil, nil, err
				}
				if same {
					return nil, nil, fmt.Errorf("target %q overlaps source input", step.TargetPath)
				}
			}
			if other, ok := claimed[path]; ok && other.Key != artifact.Key {
				conflicts = append(conflicts, Conflict{Kind: ConflictOwnership, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
				continue targetSteps
			}
			// ponytail: O(n²) identity scan; index identities if large artifact sets make it measurable.
			for _, prior := range files {
				if prior.Observed.Path != observed.Path && materialize.SameEntryIdentity(prior.Observed, observed) {
					conflicts = append(conflicts, Conflict{Kind: ConflictOwnership, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
					continue targetSteps
				}
			}
			claimed[path] = artifact
			digest := materialize.Digest(step.SourceBytes)
			change := ChangeFileCreated
			ownerKey := relativePath
			if owner, ok := owners[ownerKey]; ok && owner != artifact.Key {
				conflicts = append(conflicts, Conflict{Kind: ConflictOwnership, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
				continue targetSteps
			}
			for _, managedArtifact := range record.Artifacts {
				for _, managedFile := range managedArtifact.Files {
					managedPath := materialize.PathKey(filepath.FromSlash(managedFile.Path))
					if managedPath == ownerKey {
						continue
					}
					same, err := materialize.SamePathIdentity(observed, filepath.Join(root, filepath.FromSlash(managedFile.Path)))
					if err != nil {
						if errors.Is(err, os.ErrNotExist) {
							continue
						}
						return nil, nil, err
					}
					if same {
						conflicts = append(conflicts, Conflict{Kind: ConflictOwnership, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
						continue targetSteps
					}
				}
			}
			if owner, ok := owners[ownerKey]; ok {
				managed, _ := record.Artifact(owner)
				recorded := ""
				for _, f := range managed.Files {
					if materialize.PathKey(filepath.FromSlash(f.Path)) == ownerKey {
						recorded = f.Digest
					}
				}
				if observed.Kind != materialize.EntryRegular || observed.Digest != recorded {
					conflicts = append(conflicts, Conflict{Kind: ConflictDrift, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
					continue targetSteps
				}
				if digest != observed.Digest {
					change = ChangeFileUpdated
				} else {
					change = ""
				}
			} else {
				if observed.Kind == materialize.EntryRegular {
					if observed.Digest == digest {
						change = ChangeOwnershipAdopted
					} else {
						conflicts = append(conflicts, Conflict{Kind: ConflictOwnership, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
						continue targetSteps
					}
				} else if observed.Kind != materialize.EntryAbsent {
					conflicts = append(conflicts, Conflict{Kind: ConflictOwnership, Source: artifact.Key.Source, Artifact: artifact.Key.Name, Paths: []string{observed.Path}})
					continue targetSteps
				}
			}
			files = append(files, plannedFile{Artifact: artifact, Step: step, Observed: observed, Digest: digest, Change: change})
		}
	}
	return files, conflicts, nil
}
func prepareSyncUndesired(root string, desired []desiredArtifact, lock repositorystate.Lockfile, record repositorystate.MaterializationRecord, expectedRoot ...os.FileInfo) (repositorystate.Lockfile, []Change, []Conflict, error) {
	keys := map[repositorystate.ArtifactKey]struct{}{}
	for _, d := range desired {
		keys[d.Key] = struct{}{}
	}
	var conflicts []Conflict
	for _, a := range record.Artifacts {
		if _, ok := keys[repositorystate.ManagedArtifactKey(a)]; !ok {
			conflicts = append(conflicts, Conflict{Kind: ConflictRemovalRequired, Source: a.Source, Artifact: a.Artifact})
			for _, file := range a.Files {
				observed, err := observeTarget(root, file.Path, expectedRoot...)
				if err != nil {
					return repositorystate.Lockfile{}, nil, nil, err
				}
				if observed.Kind != materialize.EntryRegular || observed.Digest != file.Digest {
					conflicts = append(conflicts, Conflict{Kind: ConflictDrift, Source: a.Source, Artifact: a.Artifact, Paths: []string{file.Path}})
				}
			}
		}
	}
	next, removed := lock.KeepArtifacts(keys)
	changes := make([]Change, 0, len(removed))
	for _, key := range removed {
		if _, managed := record.Artifact(key); !managed {
			resolution, _, _ := lock.Artifact(key)
			changes = append(changes, Change{Kind: ChangeLockPruned, Source: key.Source, SourceVersion: resolution.ResolvedVersion, Artifact: key.Name})
		}
	}
	return next, changes, conflicts, nil
}
func applyPrepared(root string, prepared preparedOperation, dryRun bool) (repositorystate.MaterializationRecord, []Change, []string, error) {
	record := prepared.Record
	changes := append([]Change(nil), prepared.Changes...)
	var created []string
	slices.SortFunc(prepared.Files, func(a, b plannedFile) int {
		return strings.Compare(repositorystate.SourceIdentityKey(a.Artifact.Key.Source)+"\x00"+a.Artifact.Key.Name+"\x00"+a.Observed.Path, repositorystate.SourceIdentityKey(b.Artifact.Key.Source)+"\x00"+b.Artifact.Key.Name+"\x00"+b.Observed.Path)
	})
	byArtifact := map[repositorystate.ArtifactKey][]repositorystate.ManagedFileRecord{}
	for _, file := range prepared.Files {
		if !dryRun && (file.Change == ChangeFileCreated || file.Change == ChangeFileUpdated) {
			if err := materialize.Write(file.Observed, file.Step.SourceBytes); err != nil {
				return record, nil, created, err
			}
			if file.Change == ChangeFileCreated {
				created = append(created, file.Observed.AbsolutePath)
			}
		}
		byArtifact[file.Artifact.Key] = append(byArtifact[file.Artifact.Key], repositorystate.ManagedFileRecord{Path: file.Observed.Path, Digest: file.Digest})
		if file.Change != "" {
			changes = append(changes, Change{Kind: file.Change, Source: file.Artifact.Key.Source, SourceVersion: file.Artifact.ResolvedVersion, Artifact: file.Artifact.Key.Name, Path: file.Observed.Path, OwnershipKind: OwnershipWholeFile})
		}
	}
	for _, d := range prepared.Desired {
		files, ok := byArtifact[d.Key]
		if !ok {
			continue
		}
		record = repositorystate.UpsertManagedArtifact(record, managedRecordFor(d.Key.Source, source.ResolvedSource{Identity: source.Identity{Version: d.ResolvedVersion}}, d.Descriptor, files))
	}
	return record, changes, created, nil
}
func revalidateAdoptions(files []plannedFile) error {
	for _, file := range files {
		if file.Change != ChangeOwnershipAdopted {
			continue
		}
		if err := materialize.Revalidate(file.Observed); err != nil {
			return err
		}
	}
	return nil
}
func (service Service) persistPrepared(ctx context.Context, root, operation string, prepared preparedOperation, manifest *repositorystate.Manifest, dryRun bool, expectedRoot ...operationRoot) (Result, error) {
	if !dryRun {
		if err := revalidateAdoptions(prepared.Files); err != nil {
			var changed materialize.ChangedSincePreflightError
			if errors.As(err, &changed) {
				conflict := Conflict{Kind: ConflictDrift, Paths: []string{changed.Path}}
				for _, file := range prepared.Files {
					if file.Observed.Path == changed.Path {
						conflict.Source = file.Artifact.Key.Source
						conflict.Artifact = file.Artifact.Key.Name
						break
					}
				}
				result := resultForConflicts(operation, len(prepared.Desired), []Conflict{conflict}, false)
				return result, UserActionError{Result: result}
			}
			return Result{}, err
		}
	}
	record, changes, created, err := applyPrepared(root, prepared, dryRun)
	if err != nil {
		cleanup(created)
		var changed materialize.ChangedSincePreflightError
		if errors.As(err, &changed) {
			conflict := Conflict{Kind: ConflictDrift, Paths: []string{changed.Path}}
			for _, file := range prepared.Files {
				if file.Observed.Path == changed.Path {
					conflict.Source = file.Artifact.Key.Source
					conflict.Artifact = file.Artifact.Key.Name
					break
				}
			}
			result := resultForConflicts(operation, len(prepared.Desired), []Conflict{conflict}, dryRun)
			return result, UserActionError{Result: result}
		}
		return Result{}, err
	}
	if dryRun {
		if len(changes) == 0 {
			return Result{Operation: operation, Outcome: OutcomeNoOp, DryRun: true, ArtifactCount: len(prepared.Desired)}, nil
		}
		return Result{Operation: operation, Outcome: OutcomePlanned, DryRun: true, ArtifactCount: len(prepared.Desired), Changes: changes}, nil
	}
	if len(expectedRoot) > 0 {
		if err := expectedRoot[0].validate(); err != nil {
			result := resultForConflicts(operation, len(prepared.Desired), []Conflict{{Kind: ConflictDrift, Paths: []string{"."}}}, false)
			return result, UserActionError{Result: result}
		}
	}
	if len(changes) == 0 {
		return Result{Operation: operation, Outcome: OutcomeNoOp, ArtifactCount: len(prepared.Desired)}, nil
	}
	lockChanged := false
	recordChanged := false
	for _, change := range changes {
		switch change.Kind {
		case ChangeResolutionLocked, ChangeLockPruned:
			lockChanged = true
		case ChangeFileCreated, ChangeFileUpdated, ChangeOwnershipAdopted:
			recordChanged = true
		}
	}
	if lockChanged {
		if err := service.store.WriteLockfile(ctx, root, prepared.Lockfile); err != nil {
			cleanup(created)
			return Result{}, err
		}
	}
	if recordChanged {
		if err := service.store.WriteMaterializationRecord(ctx, root, record); err != nil {
			cleanup(created)
			return Result{}, err
		}
	}
	if manifest != nil {
		if err := service.store.WriteManifest(ctx, root, *manifest); err != nil {
			cleanup(created)
			return Result{}, err
		}
	}
	return Result{Operation: operation, Outcome: OutcomeApplied, ArtifactCount: len(prepared.Desired), Changes: changes}, nil
}
func cleanup(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
func sortConflicts(values []Conflict) {
	slices.SortFunc(values, func(a, b Conflict) int {
		return strings.Compare(string(a.Kind)+"\x00"+repositorystate.SourceIdentityKey(a.Source)+"\x00"+a.Artifact+"\x00"+strings.Join(a.Paths, "\x00"), string(b.Kind)+"\x00"+repositorystate.SourceIdentityKey(b.Source)+"\x00"+b.Artifact+"\x00"+strings.Join(b.Paths, "\x00"))
	})
}
