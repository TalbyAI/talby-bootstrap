package repositorystate

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type yamlTestDocument struct {
	SchemaVersion int    `yaml:"schema_version"`
	Name          string `yaml:"name"`
}

func TestDecodeStrictYAMLRejectsUnsafeSyntax(t *testing.T) {
	tests := map[string]string{
		"duplicate key":                     "schema_version: 1\nname: one\nname: two\n",
		"explicit null":                     "schema_version: 1\nname: null\n",
		"alias":                             "defaults: &defaults\n  name: one\ncopy: *defaults\n",
		"anchor":                            "name: &name one\n",
		"merge key":                         "defaults: &defaults\n  name: one\n<<: *defaults\n",
		"custom tag":                        "name: !custom one\n",
		"verbatim tag URI":                  "schema_version: 1\nname: !<tag:example.com,2026:value> one\n",
		"verbatim HTTPS tag URI":            "schema_version: 1\nname: !<https://example.com/tag> one\n",
		"verbatim non-core YAML URI tag":    "schema_version: 1\nname: !<tag:yaml.org,2002:evil> one\n",
		"scheme-less verbatim tag":          "schema_version: 1\nname: !<foo> one\n",
		"verbatim tag with escaped control": "schema_version: 1\nname: !<foo:%7F> one\n",
		"non-scalar key":                    "? [one, two]\n: value\n",
		"multiple documents":                "schema_version: 1\nname: one\n---\nschema_version: 1\nname: two\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var got yamlTestDocument
			if err := decodeStrictYAML([]byte(input), &got); err == nil {
				t.Fatalf("decodeStrictYAML(%q) unexpectedly succeeded", input)
			}
		})
	}
}

func TestDecodeStrictYAMLAcceptsCoreTag(t *testing.T) {
	var got yamlTestDocument
	if err := decodeStrictYAML([]byte("schema_version: 1\nname: !!str one\n"), &got); err != nil {
		t.Fatalf("decodeStrictYAML() rejected core tag: %v", err)
	}
}

func TestValidateYAMLNodeAcceptsCoreTags(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		kind yaml.Kind
	}{
		{name: "bool", tag: "!!bool", kind: yaml.ScalarNode},
		{name: "binary", tag: "!!binary", kind: yaml.ScalarNode},
		{name: "float", tag: "!!float", kind: yaml.ScalarNode},
		{name: "int", tag: "!!int", kind: yaml.ScalarNode},
		{name: "map", tag: "!!map", kind: yaml.MappingNode},
		{name: "merge", tag: "!!merge", kind: yaml.ScalarNode},
		{name: "seq", tag: "!!seq", kind: yaml.SequenceNode},
		{name: "str", tag: "!!str", kind: yaml.ScalarNode},
		{name: "timestamp", tag: "!!timestamp", kind: yaml.ScalarNode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateYAMLNode(&yaml.Node{Kind: tt.kind, Tag: tt.tag}); err != nil {
				t.Fatalf("validateYAMLNode() rejected %s: %v", tt.tag, err)
			}
		})
	}
}

func TestDecodeStrictYAMLRejectsUnknownFields(t *testing.T) {
	var got yamlTestDocument
	if err := decodeStrictYAML([]byte("schema_version: 1\nunknown: value\n"), &got); err == nil {
		t.Fatal("decodeStrictYAML unexpectedly accepted unknown field")
	}
}

func TestEncodeYAMLIsCanonical(t *testing.T) {
	got, err := encodeYAML(yamlTestDocument{SchemaVersion: 1, Name: "one"})
	if err != nil {
		t.Fatal(err)
	}
	want := "schema_version: 1\nname: one\n"
	if string(got) != want {
		t.Fatalf("encodeYAML() = %q, want %q", got, want)
	}
	if strings.HasPrefix(string(got), "\ufeff") || strings.Contains(string(got), "\r") {
		t.Fatalf("encodeYAML() is not UTF-8 LF text: %q", got)
	}
}
