package install

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
)

type mutationKind string

const (
	mutationWrite    mutationKind = "write"
	mutationRemove   mutationKind = "remove"
	mutationRestore  mutationKind = "restore"
	mutationRecovery mutationKind = "recovery_write"
)

const recoverySummary = "rollback could not restore every path"

type mutationHook func(kind mutationKind, path string, apply func() error) error

type journalEntry struct {
	prior          materialize.Observation
	bytes          []byte
	missingParents []string
	owner          *repositorystate.RecoveryOwner
}

type transaction struct {
	root  string
	store repositorystate.Store
	hook  mutationHook
	items []journalEntry
}

func runMutation(hook mutationHook, kind mutationKind, path string, apply func() error) error {
	if hook != nil {
		return hook(kind, path, apply)
	}
	return apply()
}

func (tx *transaction) run(kind mutationKind, path string, apply func() error) error {
	return runMutation(tx.hook, kind, path, apply)
}

func (tx *transaction) apply(kind mutationKind, path string, owner *repositorystate.RecoveryOwner, apply func() error) error {
	prior, err := materialize.Observe(tx.root, path)
	if err != nil {
		return err
	}
	priorBytes, err := materialize.ReadPrior(prior)
	if err != nil {
		return err
	}
	entry := journalEntry{
		prior:          prior,
		bytes:          slices.Clone(priorBytes),
		missingParents: materialize.MissingParents(prior),
	}
	if owner != nil {
		copy := *owner
		entry.owner = &copy
	}
	tx.items = append(tx.items, entry)
	return tx.run(kind, path, apply)
}

func (tx *transaction) fail(original error) (error, bool) {
	observations := tx.rollback()
	if len(observations) == 0 {
		return original, false
	}
	state := repositorystate.RecoveryState{
		Code:         repositorystate.RecoveryCodeRollbackIncomplete,
		Summary:      recoverySummary,
		Observations: observations,
	}
	write := func() error {
		return tx.store.WriteRecoveryState(context.Background(), tx.root, state)
	}
	if err := tx.run(mutationRecovery, repositorystate.RecoveryStateFileName, write); err != nil {
		return errors.Join(original, fmt.Errorf("write recovery state: %w", err)), false
	}
	observed, err := materialize.Observe(tx.root, repositorystate.RecoveryStateFileName)
	if err == nil && (observed.Kind != materialize.EntryRegular || observed.Mode.Perm() != 0o600) {
		err = fmt.Errorf("recovery state must be a regular file with mode 0600")
	}
	if err == nil {
		var loaded repositorystate.RecoveryState
		loaded, err = tx.store.LoadRecoveryState(context.Background(), tx.root)
		if err == nil && !reflect.DeepEqual(loaded, state) {
			err = fmt.Errorf("recovery state does not match written value")
		}
	}
	if err == nil {
		err = materialize.Revalidate(observed)
	}
	if err != nil {
		return errors.Join(original, fmt.Errorf("write recovery state: %w", err)), false
	}
	return original, true
}

func (tx *transaction) rollback() []repositorystate.RecoveryObservation {
	failed := map[string]repositorystate.RecoveryObservation{}
	missingParents := map[string]struct{}{}
	for i := len(tx.items) - 1; i >= 0; i-- {
		entry := tx.items[i]
		for _, path := range entry.missingParents {
			missingParents[path] = struct{}{}
		}
		current, observeErr := materialize.Observe(tx.root, entry.prior.Path)
		var restoreErr error
		if observeErr == nil {
			if entry.prior.Kind == materialize.EntryAbsent {
				if current.Kind != materialize.EntryAbsent {
					restoreErr = tx.run(mutationRestore, entry.prior.Path, func() error { return materialize.Remove(current) })
				}
			} else {
				restoreErr = tx.run(mutationRestore, entry.prior.Path, func() error {
					return materialize.Restore(current, entry.bytes, entry.prior.Mode.Perm())
				})
			}
		}
		restored, reobserveErr := materialize.Observe(tx.root, entry.prior.Path)
		if reobserveErr == nil && materialize.MatchesPrior(entry.prior, restored) {
			delete(failed, entry.prior.Path)
			continue
		}
		result := repositorystate.RecoveryResultVerificationFailed
		if observeErr != nil || restoreErr != nil || reobserveErr != nil {
			result = repositorystate.RecoveryResultRestoreFailed
		}
		failed[entry.prior.Path] = recoveryObservation(entry, result)
	}

	parents := make([]string, 0, len(missingParents))
	for path := range missingParents {
		parents = append(parents, path)
	}
	slices.SortFunc(parents, func(a, b string) int {
		if aDepth, bDepth := strings.Count(a, "/"), strings.Count(b, "/"); aDepth != bDepth {
			return bDepth - aDepth
		}
		return strings.Compare(b, a)
	})
	for _, path := range parents {
		current, observeErr := materialize.Observe(tx.root, path)
		var removeErr error
		if observeErr == nil && current.Kind != materialize.EntryAbsent {
			removeErr = materialize.Remove(current)
		}
		restored, reobserveErr := materialize.Observe(tx.root, path)
		if reobserveErr == nil && restored.Kind == materialize.EntryAbsent {
			continue
		}
		result := repositorystate.RecoveryResultVerificationFailed
		if observeErr != nil || removeErr != nil || reobserveErr != nil {
			result = repositorystate.RecoveryResultRestoreFailed
		}
		failed[path] = repositorystate.RecoveryObservation{Path: path, Result: result, ExpectedState: repositorystate.RecoveryExpectedAbsent}
	}

	observations := make([]repositorystate.RecoveryObservation, 0, len(failed))
	for _, observation := range failed {
		observations = append(observations, observation)
	}
	slices.SortFunc(observations, func(a, b repositorystate.RecoveryObservation) int {
		return strings.Compare(a.Path, b.Path)
	})
	return observations
}

func recoveryObservation(entry journalEntry, result string) repositorystate.RecoveryObservation {
	observation := repositorystate.RecoveryObservation{
		Path:   entry.prior.Path,
		Result: result,
		Owner:  entry.owner,
	}
	if entry.prior.Kind == materialize.EntryAbsent {
		observation.ExpectedState = repositorystate.RecoveryExpectedAbsent
		return observation
	}
	observation.ExpectedState = repositorystate.RecoveryExpectedFile
	observation.Digest = entry.prior.Digest
	observation.Mode = uint32(entry.prior.Mode.Perm())
	return observation
}
