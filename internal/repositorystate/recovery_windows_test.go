//go:build windows

package repositorystate

import "testing"

func TestValidateRecoveryStateRejectsCaseAliasPaths(t *testing.T) {
	state := RecoveryState{
		Code:    RecoveryCodeRollbackIncomplete,
		Summary: "rollback incomplete",
		Observations: []RecoveryObservation{
			{Path: "Folder/File", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent},
			{Path: "folder/file", Result: RecoveryResultVerificationFailed, ExpectedState: RecoveryExpectedAbsent},
		},
	}
	if err := ValidateRecoveryState(t.TempDir(), state); err == nil {
		t.Fatal("expected case-alias rejection")
	}
}
