package repositorystate

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

func decodeStrictYAML(data []byte, value any) error {
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("file is empty")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if err := validateYAMLNode(&document); err != nil {
		return err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple YAML documents are not supported")
		}
		return err
	}

	decoder = yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(value); err != nil {
		return err
	}
	return nil
}

func DecodeStrictYAML(data []byte, value any) error { return decodeStrictYAML(data, value) }

func validateYAMLNode(node *yaml.Node) error {
	if node == nil {
		return fmt.Errorf("YAML document is empty")
	}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return fmt.Errorf("YAML document must contain one value")
		}
		return validateYAMLNode(node.Content[0])
	}
	if node.Anchor != "" {
		return fmt.Errorf("YAML anchors are not supported")
	}
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are not supported")
	}
	if strings.HasPrefix(node.Tag, "!") && !strings.HasPrefix(node.Tag, "!!") {
		return fmt.Errorf("custom YAML tags are not supported")
	}
	if node.Tag == "!!null" {
		return fmt.Errorf("explicit YAML null is not supported")
	}

	switch node.Kind {
	case yaml.MappingNode:
		keys := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode {
				return fmt.Errorf("YAML mapping keys must be scalars")
			}
			if key.Value == "<<" {
				return fmt.Errorf("YAML merge keys are not supported")
			}
			if _, ok := keys[key.Value]; ok {
				return fmt.Errorf("duplicate YAML key %q", key.Value)
			}
			keys[key.Value] = struct{}{}
			if err := validateYAMLNode(key); err != nil {
				return err
			}
			if err := validateYAMLNode(node.Content[i+1]); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := validateYAMLNode(child); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		return nil
	default:
		return fmt.Errorf("unsupported YAML node")
	}
	return nil
}

func encodeYAML(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(buffer.Bytes(), []byte("\r\n"), []byte("\n")), nil
}
