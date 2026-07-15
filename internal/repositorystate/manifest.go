package repositorystate

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

func NormalizeSourceIdentity(root string, source SourceIdentity) (SourceIdentity, error) {
	if source.Type == "" || source.Locator == "" {
		return SourceIdentity{}, fmt.Errorf("source type and locator are required")
	}
	if source.Type != SourceTypeFile {
		return SourceIdentity{}, fmt.Errorf("unsupported source type %q", source.Type)
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return SourceIdentity{}, err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return SourceIdentity{}, err
	}
	path := source.Locator
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
		path = rel
	}
	return SourceIdentity{Type: source.Type, Locator: filepath.ToSlash(path)}, nil
}
func AcquisitionLocator(root string, source SourceIdentity) (string, error) {
	normalized, err := NormalizeSourceIdentity(root, source)
	if err != nil {
		return "", err
	}
	if normalized != source {
		return "", fmt.Errorf("source locator is not normalized")
	}
	path := source.Locator
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return filepath.Abs(path)
}
func ValidateManifest(root string, manifest Manifest) error {
	scopes, targets := map[string]DeclarationScope{}, map[string]struct{}{}
	for _, source := range manifest.TrustPolicy.ApprovedSources {
		n, err := NormalizeSourceIdentity(root, source)
		if err != nil {
			return fmt.Errorf("trust policy approved source: %w", err)
		}
		if n != source {
			return fmt.Errorf("trust policy approved source locator is not normalized")
		}
	}
	for _, declaration := range manifest.Declarations {
		n, err := NormalizeSourceIdentity(root, declaration.Source)
		if err != nil {
			return fmt.Errorf("declaration source: %w", err)
		}
		if n != declaration.Source {
			return fmt.Errorf("declaration source locator is not normalized")
		}
		if err := validateDeclarationTarget(declaration.Target); err != nil {
			return err
		}
		key := DeclarationKey(declaration)
		if _, ok := targets[key]; ok {
			return fmt.Errorf("duplicate declaration")
		}
		targets[key] = struct{}{}
		sourceKey := SourceIdentityKey(declaration.Source)
		if old, ok := scopes[sourceKey]; ok && old != declaration.Target.Scope {
			return fmt.Errorf("source %q mixes source and artifact scopes", declaration.Source.Locator)
		}
		scopes[sourceKey] = declaration.Target.Scope
		if declaration.Input != nil && declaration.Input.Locator != "" {
			input, err := NormalizeSourceIdentity(root, SourceIdentity{Type: declaration.Source.Type, Locator: declaration.Input.Locator})
			if err != nil {
				return fmt.Errorf("declaration input: %w", err)
			}
			if input != declaration.Source {
				return fmt.Errorf("declaration input locator does not match source locator")
			}
		}
	}
	return nil
}
func (manifest Manifest) AddDeclaration(root string, declaration Declaration) (Manifest, ChangeKind, error) {
	normalized, err := NormalizeSourceIdentity(root, declaration.Source)
	if err != nil {
		return Manifest{}, "", err
	}
	declaration.Source = normalized
	next := Manifest{TrustPolicy: TrustPolicy{ApprovedSources: append([]SourceIdentity(nil), manifest.TrustPolicy.ApprovedSources...)}, Declarations: append([]Declaration(nil), manifest.Declarations...)}
	key := DeclarationKey(declaration)
	for _, old := range next.Declarations {
		if DeclarationKey(old) == key {
			if reflect.DeepEqual(old, declaration) {
				return next, ChangeKindUnchanged, nil
			}
			return Manifest{}, "", fmt.Errorf("declaration already exists with different input")
		}
	}
	next.Declarations = append(next.Declarations, declaration)
	if err := ValidateManifest(root, next); err != nil {
		return Manifest{}, "", err
	}
	return next, ChangeKindInserted, nil
}
func SourceIdentityKey(source SourceIdentity) string {
	return source.Type + "\x00" + filepath.ToSlash(source.Locator)
}
func DeclarationKey(declaration Declaration) string {
	return SourceIdentityKey(declaration.Source) + "\x00" + string(declaration.Target.Scope) + "\x00" + declaration.Target.Artifact
}
func validateDeclarationTarget(target DeclarationTarget) error {
	switch target.Scope {
	case DeclarationScopeArtifact:
		if target.Artifact == "" {
			return fmt.Errorf("artifact target requires artifact name")
		}
	case DeclarationScopeSource:
		if target.Artifact != "" {
			return fmt.Errorf("source target must not include artifact name")
		}
	default:
		return fmt.Errorf("declaration scope is required")
	}
	return nil
}
