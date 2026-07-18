package repositorystate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryStateRoundTripsSortedSanitizedObservations(t *testing.T) {
	root := t.TempDir()
	source := SourceIdentity{Type: SourceTypeFile, Locator: "./source"}
	state := RecoveryState{
		Code:    RecoveryCodeRollbackIncomplete,
		Summary: "rollback could not restore every path",
		Observations: []RecoveryObservation{
			{Path: "z", Result: RecoveryResultVerificationFailed, ExpectedState: RecoveryExpectedAbsent},
			{Path: "a", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedFile, Digest: "sha256:" + strings.Repeat("a", 64), Mode: 0o644, Owner: &RecoveryOwner{Source: source, ResolvedVersion: "sha256:" + strings.Repeat("b", 64), Artifact: "tool"}},
		},
	}
	store := NewStore()
	if err := store.WriteRecoveryState(context.Background(), root, state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, RecoveryStateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "source: file:./source") || strings.Index(string(data), "path: a") > strings.Index(string(data), "path: z") {
		t.Fatalf("recovery YAML = %s", data)
	}
	got, err := store.LoadRecoveryState(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != state.Code || len(got.Observations) != 2 || got.Observations[0].Path != "a" {
		t.Fatalf("LoadRecoveryState() = %#v", got)
	}
}

func TestValidateRecoveryStateRejectsRawErrorAndUnsafeObservation(t *testing.T) {
	root := t.TempDir()
	valid := RecoveryState{Code: RecoveryCodeRollbackIncomplete, Summary: "safe", Observations: []RecoveryObservation{{Path: "file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}}
	for _, invalid := range []RecoveryState{
		{Code: "raw error", Summary: "safe", Observations: valid.Observations},
		{Code: valid.Code, Summary: "line\nbreak", Observations: valid.Observations},
		{Code: valid.Code, Summary: valid.Summary, Observations: []RecoveryObservation{{Path: "../file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}},
	} {
		if err := ValidateRecoveryState(root, invalid); err == nil {
			t.Fatalf("ValidateRecoveryState(%#v) unexpectedly succeeded", invalid)
		}
	}
}
