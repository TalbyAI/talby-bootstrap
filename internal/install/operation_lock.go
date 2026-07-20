package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/talby/talby-bootstrap/internal/materialize"
)

const operationLockName = ".tbboot-operation.lock"

type operationRoot struct {
	path string
	info os.FileInfo
}

func openOperationRoot(path string, dryRun bool) (operationRoot, func() error, error) {
	canonical, err := canonicalOperationRoot(path)
	if err != nil {
		return operationRoot{}, nil, err
	}
	if dryRun {
		info, err := os.Stat(canonical)
		if err != nil {
			return operationRoot{}, nil, err
		}
		return operationRoot{path: canonical, info: info}, nil, nil
	}
	before, err := os.Stat(canonical)
	if err != nil {
		return operationRoot{}, nil, err
	}
	release, err := acquireOperationLock(canonical)
	if err != nil {
		return operationRoot{}, nil, err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		_ = release()
		return operationRoot{}, nil, err
	}
	if !sameFileIdentity(before, info) {
		_ = release()
		return operationRoot{}, nil, materialize.ChangedSincePreflightError{Path: "."}
	}
	return operationRoot{path: canonical, info: info}, release, nil
}

func canonicalOperationRoot(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("operation root must be a directory")
	}
	return canonical, nil
}

func (root operationRoot) validate() error {
	return validateRootIdentity(root.path, root.info)
}

func validateRootIdentity(path string, expected os.FileInfo) error {
	info, err := os.Stat(path)
	if err != nil || !sameFileIdentity(expected, info) {
		return materialize.ChangedSincePreflightError{Path: "."}
	}
	return nil
}

func sameFileIdentity(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b)
}

func acquireOperationLock(root string) (func() error, error) {
	path := filepath.Join(root, operationLockName)
	if err := os.Mkdir(path, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("operation root is already locked")
		}
		return nil, err
	}
	return func() error { return os.Remove(path) }, nil
}
