package parsers

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/xuri/excelize/v2"
)

type XLSXParser struct{}

func NewXLSXParser() *XLSXParser {
	return &XLSXParser{}
}

func (p *XLSXParser) FormatName() FileFormat {
	return FileFormatXLSX
}

func (p *XLSXParser) CanParse(content []byte, contentType, fileName string) bool {
	if hasExtension(fileName, ".xlsx", ".xls") {
		return true
	}

	if hasContentType(contentType,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-excel") {
		return true
	}

	if len(content) >= 4 && content[0] == 0x50 && content[1] == 0x4B {
		return true
	}

	return false
}

func (p *XLSXParser) Parse(content []byte) ([]domain.Item, error) {
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel file has no sheets")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("Excel sheet is empty")
	}

	headers := rows[0]
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}

	var items []domain.Item
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		item := make(map[string]interface{})

		for j, value := range row {
			if j < len(headers) && headers[j] != "" {
				item[headers[j]] = strings.TrimSpace(value)
			}
		}

		if len(item) > 0 {
			items = append(items, item)
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("Excel file has no data rows")
	}

	return items, nil
}
