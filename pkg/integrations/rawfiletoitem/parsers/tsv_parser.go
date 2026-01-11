package parsers

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

type TSVParser struct{}

func NewTSVParser() *TSVParser {
	return &TSVParser{}
}

func (p *TSVParser) FormatName() FileFormat {
	return FileFormatTSV
}

func (p *TSVParser) CanParse(content []byte, contentType, fileName string) bool {
	if hasExtension(fileName, ".tsv") {
		return true
	}

	if hasContentType(contentType, "text/tab-separated-values", "text/tsv") {
		return true
	}

	return false
}

func (p *TSVParser) Parse(content []byte) ([]domain.Item, error) {
	return parseDelimited(content, '\t')
}
