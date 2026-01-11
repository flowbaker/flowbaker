package parsers

import (
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type FileFormat string

const (
	FileFormatAuto   FileFormat = "auto"
	FileFormatJSON   FileFormat = "json"
	FileFormatNDJSON FileFormat = "ndjson"
	FileFormatCSV    FileFormat = "csv"
	FileFormatTSV    FileFormat = "tsv"
	FileFormatXLSX   FileFormat = "xlsx"
	FileFormatXML    FileFormat = "xml"
	FileFormatYAML   FileFormat = "yaml"
)

var SupportedFormats = []FileFormat{
	FileFormatJSON,
	FileFormatNDJSON,
	FileFormatCSV,
	FileFormatTSV,
	FileFormatXLSX,
	FileFormatXML,
	FileFormatYAML,
}

type FileParser interface {
	FormatName() FileFormat
	CanParse(content []byte, contentType, fileName string) bool
	Parse(content []byte) ([]domain.Item, error)
}

type ParserRegistry struct {
	parsers map[FileFormat]FileParser
	order   []FileFormat
}

func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make(map[FileFormat]FileParser),
		order:   make([]FileFormat, 0),
	}
}

func (r *ParserRegistry) Register(parser FileParser) {
	format := parser.FormatName()
	r.parsers[format] = parser
	r.order = append(r.order, format)
}

func (r *ParserRegistry) GetParser(format FileFormat) (FileParser, error) {
	if parser, ok := r.parsers[format]; ok {
		return parser, nil
	}
	return nil, fmt.Errorf("unsupported file format: %s", format)
}

func (r *ParserRegistry) DetectAndGetParser(content []byte, contentType, fileName string) (FileParser, error) {
	for _, format := range r.order {
		parser := r.parsers[format]
		if parser.CanParse(content, contentType, fileName) {
			return parser, nil
		}
	}
	return nil, fmt.Errorf("unable to detect file format. Supported formats: %s", strings.Join(formatNames(), ", "))
}

func (r *ParserRegistry) Parse(content []byte, contentType, fileName string, format FileFormat) ([]domain.Item, error) {
	var parser FileParser
	var err error

	if format == FileFormatAuto || format == "" {
		parser, err = r.DetectAndGetParser(content, contentType, fileName)
		if err != nil {
			return nil, err
		}
	} else {
		parser, err = r.GetParser(format)
		if err != nil {
			return nil, err
		}
	}

	return parser.Parse(content)
}

func IsValidFormat(format FileFormat) bool {
	if format == FileFormatAuto {
		return true
	}
	for _, f := range SupportedFormats {
		if f == format {
			return true
		}
	}
	return false
}

func formatNames() []string {
	names := make([]string, len(SupportedFormats))
	for i, f := range SupportedFormats {
		names[i] = string(f)
	}
	return names
}

func hasExtension(fileName string, extensions ...string) bool {
	fileNameLower := strings.ToLower(fileName)
	for _, ext := range extensions {
		if strings.HasSuffix(fileNameLower, ext) {
			return true
		}
	}
	return false
}

func hasContentType(contentType string, types ...string) bool {
	contentTypeLower := strings.ToLower(contentType)
	for _, t := range types {
		if strings.Contains(contentTypeLower, t) {
			return true
		}
	}
	return false
}
