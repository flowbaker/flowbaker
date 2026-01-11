package parsers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type CSVParser struct {
	delimiter rune
}

func NewCSVParser() *CSVParser {
	return &CSVParser{
		delimiter: ',',
	}
}

func (p *CSVParser) FormatName() FileFormat {
	return FileFormatCSV
}

func (p *CSVParser) CanParse(content []byte, contentType, fileName string) bool {
	if hasExtension(fileName, ".csv") {
		return true
	}

	if hasContentType(contentType, "text/csv", "application/csv") {
		return true
	}

	return false
}

func (p *CSVParser) Parse(content []byte) ([]domain.Item, error) {
	return parseDelimited(content, ',')
}

func parseDelimited(content []byte, delimiter rune) ([]domain.Item, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("file is empty")
		}
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}

	var items []domain.Item
	lineNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse line %d: %w", lineNum+1, err)
		}

		item := make(map[string]interface{})
		for i, value := range record {
			if i < len(headers) {
				item[headers[i]] = strings.TrimSpace(value)
			}
		}
		items = append(items, item)
		lineNum++
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("file has no data rows")
	}

	return items, nil
}
