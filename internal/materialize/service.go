package materialize

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	rootInfo     os.FileInfo
	targetInfo   os.FileInfo
}
type ChangedSincePreflightError struct{ Path string }
type targetParentError struct{}

func (err ChangedSincePreflightError) Error() string {
	return fmt.Sprintf("target %q changed after preflight", err.Path)
}
func (targetParentError) Error() string { return "target parent must be a real directory" }
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
	opened, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return Observation{}, err
	}
	defer func() { _ = opened.Close() }()
	return observeRooted(opened, canonicalRoot, clean)
}
func observeRooted(root *os.Root, rootPath, target string) (Observation, error) {
	if err := inspectParents(root, target); err != nil {
		return Observation{}, err
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		return Observation{}, err
	}
	ob := Observation{Root: rootPath, Path: filepath.ToSlash(target), AbsolutePath: filepath.Join(rootPath, target), Kind: EntryAbsent, rootInfo: rootInfo}
	info, err := root.Lstat(target)
	if os.IsNotExist(err) {
		return ob, nil
	}
	if err != nil {
		return Observation{}, err
	}
	ob.Mode = info.Mode()
	ob.targetInfo = info
	if info.Mode()&os.ModeSymlink != 0 {
		ob.Kind = EntrySymlink
		return ob, nil
	}
	if isWindowsReparsePoint(info) || !info.Mode().IsRegular() {
		ob.Kind = EntryOther
		return ob, nil
	}
	data, err := root.ReadFile(target)
	if err != nil {
		return Observation{}, err
	}
	ob.Kind = EntryRegular
	ob.Digest = Digest(data)
	return ob, nil
}
func inspectParents(root *os.Root, target string) error {
	parent := filepath.Dir(target)
	if parent == "." {
		return nil
	}
	path := ""
	for _, component := range strings.Split(parent, string(filepath.Separator)) {
		path = filepath.Join(path, component)
		info, err := root.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !isRealDirectory(info) {
			return targetParentError{}
		}
	}
	return nil
}
func isRealDirectory(info os.FileInfo) bool {
	return info.IsDir() && info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 && !isWindowsReparsePoint(info)
}
func PathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
func SameObservation(a, b Observation) bool {
	return sameFileIdentity(a.rootInfo, b.rootInfo) && sameFileIdentity(a.targetInfo, b.targetInfo) && a.Root == b.Root && a.Path == b.Path && a.AbsolutePath == b.AbsolutePath && a.Kind == b.Kind && a.Mode == b.Mode && a.Digest == b.Digest
}
func SameEntryIdentity(a, b Observation) bool {
	return a.targetInfo != nil && b.targetInfo != nil && os.SameFile(a.targetInfo, b.targetInfo)
}
func SamePathIdentity(observed Observation, path string) (bool, error) {
	if observed.targetInfo == nil {
		return false, nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return os.SameFile(observed.targetInfo, info), nil
}
func sameFileIdentity(a, b os.FileInfo) bool {
	return a == nil && b == nil || a != nil && b != nil && os.SameFile(a, b)
}
func Revalidate(observed Observation) error {
	current, err := Observe(observed.Root, observed.Path)
	if err != nil {
		var parent targetParentError
		if errors.As(err, &parent) {
			return ChangedSincePreflightError{Path: observed.Path}
		}
		return err
	}
	if !SameObservation(observed, current) {
		return ChangedSincePreflightError{Path: observed.Path}
	}
	return nil
}
func Write(observed Observation, content []byte) error {
	root, err := os.OpenRoot(observed.Root)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return writeRooted(root, observed, content)
}
func writeRooted(root *os.Root, observed Observation, content []byte) error {
	if err := revalidateRoot(root, observed); err != nil {
		return err
	}
	path := filepath.FromSlash(observed.Path)
	parent, base := filepath.Dir(path), filepath.Base(path)
	if err := root.MkdirAll(parent, 0755); err != nil {
		return err
	}
	dir, err := root.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	file, tmp, err := createTemp(dir, base)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Remove(tmp) }()
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
	if err := revalidateRoot(root, observed); err != nil {
		return err
	}
	currentParent, err := root.Lstat(parent)
	if err != nil {
		return ChangedSincePreflightError{Path: observed.Path}
	}
	openedParent, err := dir.Stat(".")
	if err != nil {
		return err
	}
	if !isRealDirectory(currentParent) || !os.SameFile(currentParent, openedParent) {
		return ChangedSincePreflightError{Path: observed.Path}
	}
	return dir.Rename(tmp, base)
}
func revalidateRoot(root *os.Root, observed Observation) error {
	current, err := observeRooted(root, observed.Root, filepath.FromSlash(observed.Path))
	if err != nil {
		var parent targetParentError
		if errors.As(err, &parent) {
			return ChangedSincePreflightError{Path: observed.Path}
		}
		return err
	}
	if !SameObservation(observed, current) {
		return ChangedSincePreflightError{Path: observed.Path}
	}
	return nil
}
func createTemp(root *os.Root, base string) (*os.File, string, error) {
	for range 100 {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := "." + base + "." + hex.EncodeToString(random[:]) + ".tmp"
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			return file, name, nil
		}
		if !os.IsExist(err) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("create temporary file for %q: too many collisions", base)
}
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}
