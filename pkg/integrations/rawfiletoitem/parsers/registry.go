package parsers

func NewDefaultRegistry() *ParserRegistry {
	registry := NewParserRegistry()

	registry.Register(NewNDJSONParser())
	registry.Register(NewXLSXParser())
	registry.Register(NewTSVParser())
	registry.Register(NewCSVParser())
	registry.Register(NewYAMLParser())
	registry.Register(NewXMLParser())
	registry.Register(NewJSONParser())

	return registry
}
