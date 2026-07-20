package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const operationLockName = ".tbboot-operation.lock"

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
