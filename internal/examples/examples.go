package examples

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type Library struct {
	Root     string
	Examples []Example
}

type Example struct {
	Path     string
	Metadata Metadata
}

type Metadata struct {
	SchemaVersion   int          `yaml:"schema_version"`
	ID              string       `yaml:"id"`
	Kind            string       `yaml:"kind"`
	Polarity        string       `yaml:"polarity"`
	Summary         string       `yaml:"summary"`
	Commands        []Command    `yaml:"commands"`
	Verification    Verification `yaml:"verification"`
	NormativeOutput []string     `yaml:"normative_outputs"`
	Tags            []string     `yaml:"tags"`
}

type Command struct {
	Argv []string `yaml:"argv"`
}

type Verification struct {
	ExitCode      string `yaml:"exit_code"`
	StdoutText    string `yaml:"stdout_text"`
	StdoutJSON    string `yaml:"stdout_json"`
	ConsumerState string `yaml:"consumer_state"`
}

func Discover(root string) (Library, error) {
	library := Library{Root: root}

	if err := requireFile(root, "README.md"); err != nil {
		return Library{}, err
	}

	for _, group := range []string{"scenarios", "atomic-cases"} {
		groupDir := filepath.Join(root, group)
		entries, err := os.ReadDir(groupDir)
		if err != nil {
			return Library{}, fmt.Errorf("read %s: %w", groupDir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			examplePath := filepath.Join(groupDir, entry.Name())
			example, err := loadExample(examplePath, group)
			if err != nil {
				return Library{}, err
			}
			library.Examples = append(library.Examples, example)
		}
	}

	sort.Slice(library.Examples, func(i, j int) bool {
		return library.Examples[i].Metadata.ID < library.Examples[j].Metadata.ID
	})

	return library, nil
}

func loadExample(path string, parentGroup string) (Example, error) {
	if err := requireFile(path, "README.md"); err != nil {
		return Example{}, err
	}
	if err := requireFile(path, "example.yaml"); err != nil {
		return Example{}, err
	}
	if err := requireFile(filepath.Join(path, "source"), "talby-source.yaml"); err != nil {
		return Example{}, err
	}
	for _, dir := range []string{"source", "consumer", "expected"} {
		if err := requireDir(path, dir); err != nil {
			return Example{}, err
		}
	}

	data, err := os.ReadFile(filepath.Join(path, "example.yaml"))
	if err != nil {
		return Example{}, fmt.Errorf("read %s: %w", filepath.Join(path, "example.yaml"), err)
	}

	var meta Metadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return Example{}, fmt.Errorf("parse %s: %w", filepath.Join(path, "example.yaml"), err)
	}

	if err := validateMetadata(path, parentGroup, meta); err != nil {
		return Example{}, err
	}
	if err := validateSourceDescriptor(filepath.Join(path, "source", "talby-source.yaml"), meta.ID); err != nil {
		return Example{}, err
	}

	return Example{
		Path:     path,
		Metadata: meta,
	}, nil
}

func validateMetadata(path string, parentGroup string, meta Metadata) error {
	if meta.SchemaVersion != 1 {
		return fmt.Errorf("%s: schema_version = %d, want 1", meta.ID, meta.SchemaVersion)
	}
	if meta.ID == "" {
		return fmt.Errorf("%s: id is required", path)
	}
	if want := filepath.Base(path); meta.ID != want {
		return fmt.Errorf("%s: id %q must match directory name %q", meta.ID, meta.ID, want)
	}
	if meta.Kind != singularKind(parentGroup) {
		return fmt.Errorf("%s: kind = %q, want %q", meta.ID, meta.Kind, singularKind(parentGroup))
	}
	switch meta.Polarity {
	case "positive", "negative":
	default:
		return fmt.Errorf("%s: polarity = %q, want positive or negative", meta.ID, meta.Polarity)
	}
	if meta.Summary == "" {
		return fmt.Errorf("%s: summary is required", meta.ID)
	}
	if len(meta.Commands) == 0 {
		return fmt.Errorf("%s: at least one command is required", meta.ID)
	}
	for _, command := range meta.Commands {
		if len(command.Argv) == 0 {
			return fmt.Errorf("%s: command argv must not be empty", meta.ID)
		}
	}
	if err := validateVerification(meta.ID, meta.Verification); err != nil {
		return err
	}

	requiredOutputs := []string{"expected/exit-code.txt"}
	requiredOutputs = append(requiredOutputs, expectedOutputsForVerification(meta.Verification)...)

	for _, rel := range requiredOutputs {
		if err := requirePath(path, rel); err != nil {
			return fmt.Errorf("%s: %w", meta.ID, err)
		}
	}

	for _, rel := range meta.NormativeOutput {
		if err := requirePath(path, rel); err != nil {
			return fmt.Errorf("%s: normative output %q: %w", meta.ID, rel, err)
		}
	}

	return nil
}

func validateVerification(exampleID string, verification Verification) error {
	if !isOneOf(verification.ExitCode, "exact", "class") {
		return fmt.Errorf("%s: verification.exit_code = %q, want exact or class", exampleID, verification.ExitCode)
	}
	if !isOneOf(verification.StdoutText, "exact", "contains", "absent") {
		return fmt.Errorf("%s: verification.stdout_text = %q, want exact, contains, or absent", exampleID, verification.StdoutText)
	}
	if !isOneOf(verification.StdoutJSON, "exact", "contains", "absent") {
		return fmt.Errorf("%s: verification.stdout_json = %q, want exact, contains, or absent", exampleID, verification.StdoutJSON)
	}
	if !isOneOf(verification.ConsumerState, "exact", "absent") {
		return fmt.Errorf("%s: verification.consumer_state = %q, want exact or absent", exampleID, verification.ConsumerState)
	}

	return nil
}

func isOneOf(got string, wants ...string) bool {
	for _, want := range wants {
		if got == want {
			return true
		}
	}

	return false
}

func singularKind(parentGroup string) string {
	switch parentGroup {
	case "scenarios":
		return "scenario"
	case "atomic-cases":
		return "atomic-case"
	default:
		return parentGroup
	}
}

func expectedOutputsForVerification(v Verification) []string {
	var outputs []string

	switch v.StdoutText {
	case "exact":
		outputs = append(outputs, "expected/stdout.txt")
	case "contains":
		outputs = append(outputs, "expected/stdout-contains.yaml")
	}

	switch v.StdoutJSON {
	case "exact":
		outputs = append(outputs, "expected/stdout.json")
	case "contains":
		outputs = append(outputs, "expected/stdout-json-contains.yaml")
	}

	if v.ConsumerState == "exact" {
		outputs = append(outputs, "expected/consumer")
	}

	return outputs
}

func requireFile(root string, rel string) error {
	path := filepath.Join(root, rel)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing %s", filepath.ToSlash(path))
	}
	if info.IsDir() {
		return fmt.Errorf("expected file at %s", filepath.ToSlash(path))
	}
	return nil
}

func requireDir(root string, rel string) error {
	path := filepath.Join(root, rel)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing %s", filepath.ToSlash(path))
	}
	if !info.IsDir() {
		return fmt.Errorf("expected directory at %s", filepath.ToSlash(path))
	}
	return nil
}

func requirePath(root string, rel string) error {
	path := filepath.Join(root, rel)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("missing %s", filepath.ToSlash(path))
	}
	return nil
}

func validateSourceDescriptor(path string, exampleID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: read %s: %w", exampleID, filepath.ToSlash(path), err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("%s: parse %s: %w", exampleID, filepath.ToSlash(path), err)
	}

	if hasNestedMapKey(&node, "source", "type") {
		return fmt.Errorf("%s: %s must not declare source.type; acquisition semantics belong in consumer manifest and lockfile", exampleID, filepath.ToSlash(path))
	}

	return nil
}

func hasNestedMapKey(node *yaml.Node, outerKey string, innerKey string) bool {
	if node == nil {
		return false
	}

	doc := node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i+1 < len(doc.Content); i += 2 {
		keyNode := doc.Content[i]
		valueNode := doc.Content[i+1]
		if keyNode.Value != outerKey || valueNode.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(valueNode.Content); j += 2 {
			if valueNode.Content[j].Value == innerKey {
				return true
			}
		}
	}

	return false
}
