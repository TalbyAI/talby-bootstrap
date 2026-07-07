package source

import (
	"context"
	"testing"
)

type stubSource struct{}

func (stubSource) Capabilities() Capabilities { return Capabilities{} }

func (stubSource) Resolve(context.Context, ResolveRequest) (ResolvedSource, error) {
	return ResolvedSource{}, nil
}

func TestStaticRegistryLookupReturnsRegisteredSource(t *testing.T) {
	want := stubSource{}
	sources := map[string]Source{
		"file": want,
	}
	registry := NewStaticRegistry(sources)
	sources["file"] = nil

	got, err := registry.Lookup("file")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if got != want {
		t.Fatalf("Lookup() source = %#v, want %#v", got, want)
	}
}

func TestStaticRegistryLookupRejectsUnknownType(t *testing.T) {
	registry := NewStaticRegistry(nil)

	_, err := registry.Lookup("git")
	if err == nil {
		t.Fatal("Lookup() error = nil, want unknown source type error")
	}
	if got, want := err.Error(), `unsupported source type "git"`; got != want {
		t.Fatalf("Lookup() error = %q, want %q", got, want)
	}
}
