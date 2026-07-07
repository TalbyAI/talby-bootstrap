package repositorystate

import (
	"fmt"
	"reflect"
)

func ValidateManifest(m Manifest) error {
	seen := map[string]struct{}{}

	for _, approved := range m.TrustPolicy.ApprovedSources {
		if err := validateSourceIdentity(approved); err != nil {
			return fmt.Errorf("trust policy approved source: %w", err)
		}
	}

	for _, decl := range m.Declarations {
		if err := validateSourceIdentity(decl.Source); err != nil {
			return fmt.Errorf("declaration source: %w", err)
		}
		if err := validateDeclarationTarget(decl.Target); err != nil {
			return err
		}

		key := declarationKey(decl)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate declaration for source %s/%s target %s/%s", decl.Source.Type, decl.Source.Name, decl.Target.Scope, decl.Target.Artifact)
		}
		seen[key] = struct{}{}
	}

	return nil
}

func (m Manifest) UpsertDeclaration(decl Declaration) (Manifest, ChangeKind) {
	next := Manifest{
		TrustPolicy: TrustPolicy{
			ApprovedSources: append([]SourceIdentity(nil), m.TrustPolicy.ApprovedSources...),
		},
		Declarations: append([]Declaration(nil), m.Declarations...),
	}

	key := declarationKey(decl)
	for i, existing := range next.Declarations {
		if declarationKey(existing) != key {
			continue
		}
		if reflect.DeepEqual(existing, decl) {
			return next, ChangeKindUnchanged
		}
		next.Declarations[i] = decl
		return next, ChangeKindReplaced
	}

	next.Declarations = append(next.Declarations, decl)
	return next, ChangeKindInserted
}

func validateSourceIdentity(source SourceIdentity) error {
	if source.Type == "" {
		return fmt.Errorf("source type is required")
	}
	switch source.Type {
	case SourceTypeFile, SourceTypeGit:
	default:
		return fmt.Errorf("unsupported source type %q", source.Type)
	}
	if source.Name == "" {
		return fmt.Errorf("source name is required")
	}
	return nil
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

func declarationKey(decl Declaration) string {
	return decl.Source.Type + "\x00" + decl.Source.Name + "\x00" + string(decl.Target.Scope) + "\x00" + decl.Target.Artifact
}
