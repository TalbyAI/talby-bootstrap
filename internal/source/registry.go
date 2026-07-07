package source

import "fmt"

type Registry interface {
	Lookup(sourceType string) (Source, error)
}

type StaticRegistry struct {
	sources map[string]Source
}

func NewStaticRegistry(sources map[string]Source) StaticRegistry {
	if sources == nil {
		return StaticRegistry{sources: map[string]Source{}}
	}

	staticSources := make(map[string]Source, len(sources))
	for sourceType, source := range sources {
		staticSources[sourceType] = source
	}

	return StaticRegistry{sources: staticSources}
}

func (r StaticRegistry) Lookup(sourceType string) (Source, error) {
	source, ok := r.sources[sourceType]
	if !ok {
		return nil, fmt.Errorf("unsupported source type %q", sourceType)
	}

	return source, nil
}
