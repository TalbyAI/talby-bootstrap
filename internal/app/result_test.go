package app

import "testing"

func TestExitCodesMatchADR(t *testing.T) {
	if ExitSuccess != 0 {
		t.Fatalf("ExitSuccess = %d, want 0", ExitSuccess)
	}
	if ExitOperationalOrValidationError != 1 {
		t.Fatalf("ExitOperationalOrValidationError = %d, want 1", ExitOperationalOrValidationError)
	}
	if ExitUserActionConflict != 2 {
		t.Fatalf("ExitUserActionConflict = %d, want 2", ExitUserActionConflict)
	}
	if ExitTrustOrPolicyDenial != 3 {
		t.Fatalf("ExitTrustOrPolicyDenial = %d, want 3", ExitTrustOrPolicyDenial)
	}
}

func TestSuccessResult(t *testing.T) {
	got := Success("ok")
	if got.Code != ExitSuccess || got.Message != "ok" {
		t.Fatalf("Success() = %#v", got)
	}
}
