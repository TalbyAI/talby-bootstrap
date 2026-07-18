package repositorystate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func ParseSourceReference(raw string) (SourceIdentity, error) {
	typeName, locator, ok := strings.Cut(raw, ":")
	if !ok || locator == "" {
		return SourceIdentity{}, fmt.Errorf("source reference must be formatted as <type>:<locator>")
	}
	if typeName != SourceTypeFile && typeName != SourceTypeGit {
		return SourceIdentity{}, fmt.Errorf("unsupported source type %q", typeName)
	}
	if strings.TrimSpace(locator) != locator || strings.ContainsFunc(locator, unicode.IsSpace) {
		return SourceIdentity{}, fmt.Errorf("source reference locator must not contain whitespace")
	}
	return SourceIdentity{Type: typeName, Locator: locator}, nil
}

func FormatSourceReference(source SourceIdentity) string {
	return source.Type + ":" + source.Locator
}

func NormalizeSourceIdentity(root string, source SourceIdentity) (SourceIdentity, error) {
	if source.Type != SourceTypeFile && source.Type != SourceTypeGit {
		return SourceIdentity{}, fmt.Errorf("unsupported source type %q", source.Type)
	}
	if source.Locator == "" {
		return SourceIdentity{}, fmt.Errorf("source locator is required")
	}
	if source.Type == SourceTypeGit {
		if strings.TrimSpace(source.Locator) != source.Locator || strings.ContainsFunc(source.Locator, unicode.IsSpace) {
			return SourceIdentity{}, fmt.Errorf("source locator must not contain whitespace")
		}
		return source, nil
	}

	base, err := filepath.Abs(root)
	if err != nil {
		return SourceIdentity{}, err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return SourceIdentity{}, err
	}
	path := filepath.FromSlash(source.Locator)
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return SourceIdentity{}, err
	}
	path = filepath.Clean(path)
	canonical, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = canonical
	} else if !os.IsNotExist(err) {
		return SourceIdentity{}, err
	}
	rel, err := filepath.Rel(base, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if rel == "." {
			return SourceIdentity{Type: SourceTypeFile, Locator: "./"}, nil
		}
		return SourceIdentity{Type: SourceTypeFile, Locator: "./" + filepath.ToSlash(rel)}, nil
	}
	return SourceIdentity{Type: SourceTypeFile, Locator: filepath.ToSlash(path)}, nil
}

func AcquisitionLocator(root string, source SourceIdentity) (string, error) {
	normalized, err := NormalizeSourceIdentity(root, source)
	if err != nil {
		return "", err
	}
	if normalized != source {
		return "", fmt.Errorf("source locator is not normalized")
	}
	if source.Type == SourceTypeGit {
		return source.Locator, nil
	}
	path := filepath.FromSlash(source.Locator)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return filepath.Abs(path)
}
