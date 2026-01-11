package parsers

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type JSONParser struct{}

func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

func (p *JSONParser) FormatName() FileFormat {
	return FileFormatJSON
}

func (p *JSONParser) CanParse(content []byte, contentType, fileName string) bool {
	if hasExtension(fileName, ".json") {
		return true
	}

	if hasContentType(contentType, "application/json") {
		return true
	}

	trimmed := bytes.TrimSpace(content)
	if len(trimmed) > 0 {
		firstChar := trimmed[0]
		if (firstChar == '{' || firstChar == '[') && !bytes.Contains(trimmed, []byte("\n{")) {
			return true
		}
	}

	return false
}

func (p *JSONParser) Parse(content []byte) ([]domain.Item, error) {
	var result interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if arr, ok := result.([]interface{}); ok {
		items := make([]domain.Item, len(arr))
		for i, item := range arr {
			items[i] = item
		}
		return items, nil
	}

	return []domain.Item{result}, nil
}
