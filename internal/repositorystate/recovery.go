package repositorystate

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	RecoveryCodeRollbackIncomplete   = "rollback_incomplete"
	RecoveryResultRestoreFailed      = "restore_failed"
	RecoveryResultVerificationFailed = "verification_failed"
	RecoveryExpectedAbsent           = "absent"
	RecoveryExpectedFile             = "file"
)

func ValidateRecoveryState(root string, state RecoveryState) error {
	if state.Code != RecoveryCodeRollbackIncomplete {
		return fmt.Errorf("recovery code must be %q", RecoveryCodeRollbackIncomplete)
	}
	if state.Summary == "" || strings.ContainsAny(state.Summary, "\r\n") {
		return fmt.Errorf("recovery summary must be a single non-empty line")
	}
	for _, r := range state.Summary {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("recovery summary contains control characters")
		}
	}
	if len(state.Observations) == 0 {
		return fmt.Errorf("recovery state requires observations")
	}
	paths := map[string]struct{}{}
	for _, observation := range state.Observations {
		path := filepath.FromSlash(observation.Path)
		if observation.Path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != observation.Path || observation.Path == "." || observation.Path == ".." || strings.HasPrefix(observation.Path, "../") || strings.Contains(observation.Path, "\\") || (len(observation.Path) >= 2 && observation.Path[1] == ':' && ((observation.Path[0] >= 'A' && observation.Path[0] <= 'Z') || (observation.Path[0] >= 'a' && observation.Path[0] <= 'z'))) {
			return fmt.Errorf("recovery observation path must be canonical and root-relative")
		}
		key := managedPathKey(observation.Path)
		if _, ok := paths[key]; ok {
			return fmt.Errorf("recovery observation path must be unique")
		}
		paths[key] = struct{}{}
		if observation.Result != RecoveryResultRestoreFailed && observation.Result != RecoveryResultVerificationFailed {
			return fmt.Errorf("unsupported recovery observation result %q", observation.Result)
		}
		switch observation.ExpectedState {
		case RecoveryExpectedAbsent:
			if observation.Digest != "" || observation.Mode != 0 {
				return fmt.Errorf("absent recovery state must not contain file metadata")
			}
		case RecoveryExpectedFile:
			if !isSHA256Digest(observation.Digest) {
				return fmt.Errorf("file recovery state requires a sha256 digest")
			}
		default:
			return fmt.Errorf("unsupported recovery expected state %q", observation.ExpectedState)
		}
		if observation.Owner != nil {
			if err := validateRecoveryOwner(root, *observation.Owner); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRecoveryOwner(root string, owner RecoveryOwner) error {
	normalized, err := NormalizeSourceIdentity(root, owner.Source)
	if err != nil {
		return fmt.Errorf("recovery owner source: %w", err)
	}
	if normalized != owner.Source || owner.ResolvedVersion == "" || owner.Artifact == "" {
		return fmt.Errorf("recovery owner is incomplete or not canonical")
	}
	return nil
}

func isSHA256Digest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return false
	}
	digest := value[len(prefix):]
	decoded, err := hex.DecodeString(digest)
	return err == nil && hex.EncodeToString(decoded) == digest
}
