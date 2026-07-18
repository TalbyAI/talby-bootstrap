package repositorystate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndFormatSourceReference(t *testing.T) {
	for _, test := range []struct {
		input string
		want  SourceIdentity
	}{
		{input: "file:./sources/tools", want: SourceIdentity{Type: SourceTypeFile, Locator: "./sources/tools"}},
		{input: "git:https://example.com/tools.git", want: SourceIdentity{Type: SourceTypeGit, Locator: "https://example.com/tools.git"}},
	} {
		got, err := ParseSourceReference(test.input)
		if err != nil {
			t.Fatalf("ParseSourceReference(%q): %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("ParseSourceReference(%q) = %#v, want %#v", test.input, got, test.want)
		}
		if formatted := FormatSourceReference(got); formatted != test.input {
			t.Fatalf("FormatSourceReference(%#v) = %q, want %q", got, formatted, test.input)
		}
	}
}

func TestParseSourceReferenceRejectsInvalidReferences(t *testing.T) {
	for _, input := range []string{"", "file:", "git:", "http:x", "FILE:x", "file:source with space"} {
		if _, err := ParseSourceReference(input); err == nil {
			t.Fatalf("ParseSourceReference(%q) unexpectedly succeeded", input)
		}
	}
}

func TestNormalizeSourceIdentityCanonicalizesInRootFilePath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := NormalizeSourceIdentity(root, SourceIdentity{Type: SourceTypeFile, Locator: "sources/../sources"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Locator != "./sources" {
		t.Fatalf("normalized locator = %q, want ./sources", got.Locator)
	}
}
