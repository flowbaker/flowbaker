package parsers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type NDJSONParser struct{}

func NewNDJSONParser() *NDJSONParser {
	return &NDJSONParser{}
}

func (p *NDJSONParser) FormatName() FileFormat {
	return FileFormatNDJSON
}

func (p *NDJSONParser) CanParse(content []byte, contentType, fileName string) bool {
	if hasExtension(fileName, ".ndjson", ".jsonl") {
		return true
	}

	if hasContentType(contentType, "ndjson", "x-ndjson", "jsonlines") {
		return true
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "\n{") {
		return true
	}

	return false
}

func (p *NDJSONParser) Parse(content []byte) ([]domain.Item, error) {
	var items []domain.Item
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var item interface{}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("failed to parse NDJSON at line %d: %w", lineNum+1, err)
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("NDJSON file is empty")
	}

	return items, nil
}
