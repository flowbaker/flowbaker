package parsers

import (
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"gopkg.in/yaml.v3"
)

type YAMLParser struct{}

func NewYAMLParser() *YAMLParser {
	return &YAMLParser{}
}

func (p *YAMLParser) FormatName() FileFormat {
	return FileFormatYAML
}

func (p *YAMLParser) CanParse(content []byte, contentType, fileName string) bool {
	if hasExtension(fileName, ".yaml", ".yml") {
		return true
	}

	if hasContentType(contentType, "application/yaml", "text/yaml", "application/x-yaml") {
		return true
	}

	return false
}

func (p *YAMLParser) Parse(content []byte) ([]domain.Item, error) {
	var result interface{}
	if err := yaml.Unmarshal(content, &result); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	result = convertYAMLTypes(result)

	if arr, ok := result.([]interface{}); ok {
		items := make([]domain.Item, len(arr))
		for i, item := range arr {
			items[i] = item
		}
		return items, nil
	}

	return []domain.Item{result}, nil
}

func convertYAMLTypes(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = convertYAMLTypes(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = convertYAMLTypes(v)
		}
		return result
	default:
		return val
	}
}
