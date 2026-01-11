package parsers

import (
	"bytes"
	"fmt"

	"github.com/clbanning/mxj/v2"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

type XMLParser struct{}

func NewXMLParser() *XMLParser {
	return &XMLParser{}
}

func (p *XMLParser) FormatName() FileFormat {
	return FileFormatXML
}

func (p *XMLParser) CanParse(content []byte, contentType, fileName string) bool {
	if hasExtension(fileName, ".xml") {
		return true
	}

	if hasContentType(contentType, "application/xml", "text/xml") {
		return true
	}

	trimmed := bytes.TrimSpace(content)
	if len(trimmed) > 0 && trimmed[0] == '<' {
		return true
	}

	return false
}

func (p *XMLParser) Parse(content []byte) ([]domain.Item, error) {
	mv, err := mxj.NewMapXml(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	data := map[string]interface{}(mv)

	for _, val := range data {
		if arr, ok := val.([]interface{}); ok {
			items := make([]domain.Item, len(arr))
			for i, v := range arr {
				items[i] = v
			}
			return items, nil
		}

		if nested, ok := val.(map[string]interface{}); ok {
			for _, nestedVal := range nested {
				if arr, ok := nestedVal.([]interface{}); ok {
					items := make([]domain.Item, len(arr))
					for i, v := range arr {
						items[i] = v
					}
					return items, nil
				}
			}
		}
	}

	return []domain.Item{data}, nil
}
