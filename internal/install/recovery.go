package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/talby/talby-bootstrap/internal/materialize"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
)

type RecoveryConflictError struct {
	Observations []repositorystate.RecoveryObservation
}

func (RecoveryConflictError) Error() string {
	return repositorystate.RecoveryCodeRollbackIncomplete + ": repository recovery requires user action"
}

type recoveryClearError struct {
	cause error
}

func (recoveryClearError) Error() string {
	return repositorystate.RecoveryCodeRollbackIncomplete + ": recovery state could not be cleared"
}

func (err recoveryClearError) Unwrap() error {
	return err.cause
}

func (service Service) inspectRecovery(ctx context.Context, root operationRoot, dryRun bool) error {
	state, err := service.store.LoadRecoveryState(ctx, root.path)
	if stateNotFound(err, repositorystate.StateFileRecovery) {
		return nil
	}
	if err != nil {
		return err
	}
	observations := slices.Clone(state.Observations)
	slices.SortFunc(observations, func(a, b repositorystate.RecoveryObservation) int {
		return strings.Compare(a.Path, b.Path)
	})
	matched := make([]materialize.Observation, 0, len(observations))
	for _, expected := range observations {
		observed, observeErr := materialize.Observe(root.path, expected.Path)
		if observeErr != nil || !matchesRecovery(expected, observed) {
			return RecoveryConflictError{Observations: slices.Clone(observations)}
		}
		matched = append(matched, observed)
	}
	if dryRun {
		return nil
	}
	clear := func() error {
		for _, observed := range matched {
			if err := materialize.Revalidate(observed); err != nil {
				return RecoveryConflictError{Observations: slices.Clone(observations)}
			}
		}
		if err := service.store.RemoveRecoveryState(ctx, root.path); err != nil {
			return err
		}
		_, err := service.store.LoadRecoveryState(ctx, root.path)
		if stateNotFound(err, repositorystate.StateFileRecovery) {
			return nil
		}
		if err == nil {
			return fmt.Errorf("recovery state remains after removal")
		}
		return err
	}
	if err := runMutation(service.mutationHook, mutationRecoveryClear, repositorystate.RecoveryStateFileName, clear); err != nil {
		var conflict RecoveryConflictError
		if errors.As(err, &conflict) {
			return err
		}
		return recoveryClearError{cause: err}
	}
	return nil
}

func matchesRecovery(expected repositorystate.RecoveryObservation, observed materialize.Observation) bool {
	switch expected.ExpectedState {
	case repositorystate.RecoveryExpectedAbsent:
		return observed.Kind == materialize.EntryAbsent
	case repositorystate.RecoveryExpectedFile:
		return observed.Kind == materialize.EntryRegular &&
			observed.Digest == expected.Digest &&
			observed.Mode.Perm() == os.FileMode(expected.Mode).Perm()
	default:
		return false
	}
}
