package repositorystate

import (
	"strings"
	"testing"
)

type yamlTestDocument struct {
	SchemaVersion int    `yaml:"schema_version"`
	Name          string `yaml:"name"`
}

func TestDecodeStrictYAMLRejectsUnsafeSyntax(t *testing.T) {
	tests := map[string]string{
		"duplicate key":      "schema_version: 1\nname: one\nname: two\n",
		"explicit null":      "schema_version: 1\nname: null\n",
		"alias":              "defaults: &defaults\n  name: one\ncopy: *defaults\n",
		"anchor":             "name: &name one\n",
		"merge key":          "defaults: &defaults\n  name: one\n<<: *defaults\n",
		"custom tag":         "name: !custom one\n",
		"non-scalar key":     "? [one, two]\n: value\n",
		"multiple documents": "schema_version: 1\nname: one\n---\nschema_version: 1\nname: two\n",
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
