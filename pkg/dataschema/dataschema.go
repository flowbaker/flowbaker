package dataschema

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

type DataSchema struct {
	schema *jsonschema.Schema
}

type Options struct {
	MaxDepth int
}

func FromStruct(v any, opts ...Options) (*DataSchema, error) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	s, err := jsonschema.ForType(t, nil)
	if err != nil {
		return nil, fmt.Errorf("dataschema: failed to reflect struct: %w", err)
	}

	if len(opts) > 0 && opts[0].MaxDepth > 0 {
		return applyMaxDepth(s, opts[0].MaxDepth)
	}

	return &DataSchema{schema: s}, nil
}

func applyMaxDepth(s *jsonschema.Schema, maxDepth int) (*DataSchema, error) {
	raw, err := s.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("dataschema: failed to marshal schema: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("dataschema: failed to unmarshal schema: %w", err)
	}

	defs, _ := m["$defs"].(map[string]any)

	pruneSchema(m, defs, 0, maxDepth)

	delete(m, "$defs")

	pruned, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("dataschema: failed to marshal pruned schema: %w", err)
	}

	return FromBytes(pruned)
}

func pruneSchema(schema map[string]any, defs map[string]any, depth, maxDepth int) {
	if ref, ok := schema["$ref"].(string); ok {
		resolved := resolveRef(ref, defs)
		if resolved != nil {
			delete(schema, "$ref")
			for k, v := range resolved {
				schema[k] = v
			}
		}
	}

	if depth >= maxDepth {
		delete(schema, "properties")
		delete(schema, "required")
		delete(schema, "additionalProperties")
		delete(schema, "items")
		delete(schema, "prefixItems")
		delete(schema, "allOf")
		delete(schema, "anyOf")
		delete(schema, "oneOf")
		return
	}

	if props, ok := schema["properties"].(map[string]any); ok {
		for key, prop := range props {
			propMap, ok := prop.(map[string]any)
			if !ok {
				continue
			}
			pruneSchema(propMap, defs, depth+1, maxDepth)
			props[key] = propMap
		}
	}

	if items, ok := schema["items"].(map[string]any); ok {
		pruneSchema(items, defs, depth, maxDepth)
	}

	if allOf, ok := schema["allOf"].([]any); ok {
		for _, item := range allOf {
			if itemMap, ok := item.(map[string]any); ok {
				pruneSchema(itemMap, defs, depth, maxDepth)
			}
		}
	}

	if anyOf, ok := schema["anyOf"].([]any); ok {
		for _, item := range anyOf {
			if itemMap, ok := item.(map[string]any); ok {
				pruneSchema(itemMap, defs, depth, maxDepth)
			}
		}
	}

	if oneOf, ok := schema["oneOf"].([]any); ok {
		for _, item := range oneOf {
			if itemMap, ok := item.(map[string]any); ok {
				pruneSchema(itemMap, defs, depth, maxDepth)
			}
		}
	}
}

func resolveRef(ref string, defs map[string]any) map[string]any {
	if defs == nil {
		return nil
	}

	const prefix = "#/$defs/"
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return nil
	}

	name := ref[len(prefix):]
	def, ok := defs[name].(map[string]any)
	if !ok {
		return nil
	}

	resolved := make(map[string]any, len(def))
	for k, v := range def {
		resolved[k] = v
	}
	return resolved
}

func FromBytes(data []byte) (*DataSchema, error) {
	var s jsonschema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("dataschema: failed to parse JSON schema: %w", err)
	}

	return &DataSchema{schema: &s}, nil
}

func FromString(s string) (*DataSchema, error) {
	return FromBytes([]byte(s))
}

func FromFile(path string) (*DataSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dataschema: failed to read file: %w", err)
	}

	return FromBytes(data)
}

func MustFromStruct(v any, opts ...Options) *DataSchema {
	d, err := FromStruct(v, opts...)
	if err != nil {
		panic(err)
	}
	return d
}

func MustFromBytes(data []byte) *DataSchema {
	d, err := FromBytes(data)
	if err != nil {
		panic(err)
	}
	return d
}

func MustFromString(s string) *DataSchema {
	d, err := FromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func MustFromFile(path string) *DataSchema {
	d, err := FromFile(path)
	if err != nil {
		panic(err)
	}
	return d
}

func (d *DataSchema) MarshalJSON() ([]byte, error) {
	if d == nil || d.schema == nil {
		return []byte("null"), nil
	}
	return d.schema.MarshalJSON()
}

func (d *DataSchema) UnmarshalJSON(data []byte) error {
	var s jsonschema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	d.schema = &s
	return nil
}

func (d *DataSchema) String() string {
	if d == nil || d.schema == nil {
		return ""
	}
	b, err := d.schema.MarshalJSON()
	if err != nil {
		return ""
	}
	return string(b)
}

func (d *DataSchema) IsEmpty() bool {
	return d == nil || d.schema == nil
}

func (d *DataSchema) Schema() *jsonschema.Schema {
	if d == nil {
		return nil
	}
	return d.schema
}
