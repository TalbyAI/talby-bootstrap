package materialize

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type EntryKind string

const (
	EntryAbsent  EntryKind = "absent"
	EntryRegular EntryKind = "regular"
	EntrySymlink EntryKind = "symlink"
	EntryOther   EntryKind = "other"
)

type Observation struct {
	Root         string
	Path         string
	AbsolutePath string
	Kind         EntryKind
	Mode         os.FileMode
	Digest       string
}
type ChangedSincePreflightError struct{ Path string }

func (err ChangedSincePreflightError) Error() string {
	return fmt.Sprintf("target %q changed after preflight", err.Path)
}
func Observe(root, target string) (Observation, error) {
	if filepath.IsAbs(target) {
		return Observation{}, fmt.Errorf("file target path must stay within operation root")
	}
	clean := filepath.Clean(target)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return Observation{}, fmt.Errorf("file target path must stay within operation root")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Observation{}, err
	}
	absolute := filepath.Join(canonicalRoot, clean)
	for dir := filepath.Dir(absolute); dir != canonicalRoot; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Observation{}, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return Observation{}, fmt.Errorf("target parent must be a real directory")
		}
	}
	ob := Observation{Root: canonicalRoot, Path: filepath.ToSlash(clean), AbsolutePath: absolute, Kind: EntryAbsent}
	info, err := os.Lstat(absolute)
	if os.IsNotExist(err) {
		return ob, nil
	}
	if err != nil {
		return Observation{}, err
	}
	ob.Mode = info.Mode()
	if info.Mode()&os.ModeSymlink != 0 {
		ob.Kind = EntrySymlink
		return ob, nil
	}
	if !info.Mode().IsRegular() {
		ob.Kind = EntryOther
		return ob, nil
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return Observation{}, err
	}
	ob.Kind = EntryRegular
	ob.Digest = Digest(data)
	return ob, nil
}
func PathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
func SameObservation(a, b Observation) bool {
	return a.Root == b.Root && a.Path == b.Path && a.AbsolutePath == b.AbsolutePath && a.Kind == b.Kind && a.Mode.Perm() == b.Mode.Perm() && a.Digest == b.Digest
}
func Write(observed Observation, content []byte) error {
	current, err := Observe(observed.Root, observed.Path)
	if err != nil {
		return err
	}
	if !SameObservation(observed, current) {
		return ChangedSincePreflightError{Path: observed.Path}
	}
	if err := os.MkdirAll(filepath.Dir(observed.AbsolutePath), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(observed.AbsolutePath), "."+filepath.Base(observed.AbsolutePath)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if n, err := file.Write(content); err != nil || n != len(content) {
		_ = file.Close()
		if err != nil {
			return err
		}
		return io.ErrShortWrite
	}
	mode := os.FileMode(0644)
	if observed.Kind == EntryRegular {
		mode = observed.Mode.Perm()
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, observed.AbsolutePath)
}
func Digest(content []byte) string { sum := sha256.Sum256(content); return hex.EncodeToString(sum[:]) }
