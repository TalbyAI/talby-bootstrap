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
	text := string(data)
	aIndex := strings.Index(text, "path: a")
	zIndex := strings.Index(text, "path: z")
	if !strings.Contains(text, "source: file:./source") || aIndex < 0 || zIndex < 0 || aIndex > zIndex {
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
		{Code: valid.Code, Summary: valid.Summary, Observations: []RecoveryObservation{{Path: "..", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}},
		{Code: valid.Code, Summary: valid.Summary, Observations: []RecoveryObservation{{Path: "dir\\file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}},
		{Code: valid.Code, Summary: valid.Summary, Observations: []RecoveryObservation{{Path: "C:/file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}},
		{Code: valid.Code, Summary: valid.Summary, Observations: []RecoveryObservation{{Path: "C:file", Result: RecoveryResultRestoreFailed, ExpectedState: RecoveryExpectedAbsent}}},
	} {
		if err := ValidateRecoveryState(root, invalid); err == nil {
			t.Fatalf("ValidateRecoveryState(%#v) unexpectedly succeeded", invalid)
		}
	}
}
