package transform

import (
	"fmt"
	"strconv"
	"strings"
)

type Access interface {
	Apply(data any) (any, error)
}

type FieldAccess struct {
	Name string
}

func (f FieldAccess) Apply(data any) (any, error) {
	if m, ok := data.(map[string]any); ok {
		if val, exists := m[f.Name]; exists {
			return val, nil
		}
		return nil, fmt.Errorf("field '%s' not found", f.Name)
	}
	return nil, fmt.Errorf("cannot access field '%s' on %T", f.Name, data)
}

type IndexAccess struct {
	Index int
}

func (i IndexAccess) Apply(data any) (any, error) {
	switch arr := data.(type) {
	case []interface{}:
		if i.Index < 0 || i.Index >= len(arr) {
			return nil, fmt.Errorf("index %d out of bounds [0:%d]", i.Index, len(arr))
		}
		return arr[i.Index], nil
	case []string:
		if i.Index < 0 || i.Index >= len(arr) {
			return nil, fmt.Errorf("index %d out of bounds [0:%d]", i.Index, len(arr))
		}
		return arr[i.Index], nil
	case []int:
		if i.Index < 0 || i.Index >= len(arr) {
			return nil, fmt.Errorf("index %d out of bounds [0:%d]", i.Index, len(arr))
		}
		return arr[i.Index], nil
	case []float64:
		if i.Index < 0 || i.Index >= len(arr) {
			return nil, fmt.Errorf("index %d out of bounds [0:%d]", i.Index, len(arr))
		}
		return arr[i.Index], nil
	default:
		return nil, fmt.Errorf("cannot index %T", data)
	}
}

type Segment struct {
	Accesses []Access
}

func (s Segment) Apply(data any) (any, error) {
	current := data
	for _, access := range s.Accesses {
		var err error
		current, err = access.Apply(current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

type FieldPathParser struct{}

func NewFieldPathParser() *FieldPathParser {
	return &FieldPathParser{}
}

func (p *FieldPathParser) GetValue(data any, fieldPath string) (any, error) {
	if fieldPath == "" {
		return nil, fmt.Errorf("empty field path")
	}

	segments := p.parseSegments(fieldPath)
	current := data

	for _, segment := range segments {
		var err error
		current, err = segment.Apply(current)
		if err != nil {
			return nil, err
		}
	}

	return current, nil
}

func (p *FieldPathParser) SetValue(data any, fieldPath string, value any) error {
	if fieldPath == "" {
		return fmt.Errorf("empty field path")
	}

	m, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("root data must be a map, got %T", data)
	}

	segments := p.parseSegments(fieldPath)
	if len(segments) == 0 {
		return fmt.Errorf("invalid field path")
	}

	// Navigate to the parent of the target field
	current := m
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]
		if len(segment.Accesses) != 1 {
			return fmt.Errorf("complex segments not supported for setting values")
		}

		fieldAccess, ok := segment.Accesses[0].(FieldAccess)
		if !ok {
			return fmt.Errorf("only field access supported for setting values, not array indices")
		}

		// Get or create the next level
		next, exists := current[fieldAccess.Name]
		if !exists {
			// Create new map for nested field
			next = make(map[string]any)
			current[fieldAccess.Name] = next
		}

		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set nested field: '%s' is not a map", fieldAccess.Name)
		}

		current = nextMap
	}

	// Set the final field
	lastSegment := segments[len(segments)-1]
	if len(lastSegment.Accesses) != 1 {
		return fmt.Errorf("complex segments not supported for setting values")
	}

	fieldAccess, ok := lastSegment.Accesses[0].(FieldAccess)
	if !ok {
		return fmt.Errorf("only field access supported for setting values")
	}

	current[fieldAccess.Name] = value
	return nil
}

func (p *FieldPathParser) DeleteValue(data any, fieldPath string) error {
	if fieldPath == "" {
		return fmt.Errorf("empty field path")
	}

	m, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("root data must be a map, got %T", data)
	}

	segments := p.parseSegments(fieldPath)
	if len(segments) == 0 {
		return fmt.Errorf("invalid field path")
	}

	// Navigate to the parent of the target field
	current := m
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]
		if len(segment.Accesses) != 1 {
			return fmt.Errorf("complex segments not supported for deleting values")
		}

		fieldAccess, ok := segment.Accesses[0].(FieldAccess)
		if !ok {
			return fmt.Errorf("only field access supported for deleting values")
		}

		next, exists := current[fieldAccess.Name]
		if !exists {
			// Field doesn't exist, nothing to delete
			return nil
		}

		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot delete nested field: '%s' is not a map", fieldAccess.Name)
		}

		current = nextMap
	}

	// Delete the final field
	lastSegment := segments[len(segments)-1]
	if len(lastSegment.Accesses) != 1 {
		return fmt.Errorf("complex segments not supported for deleting values")
	}

	fieldAccess, ok := lastSegment.Accesses[0].(FieldAccess)
	if !ok {
		return fmt.Errorf("only field access supported for deleting values")
	}

	delete(current, fieldAccess.Name)
	return nil
}

func (p *FieldPathParser) parseSegments(path string) []Segment {
	parts := strings.Split(path, ".")
	segments := make([]Segment, len(parts))

	for i, part := range parts {
		segments[i] = p.parseSegment(part)
	}

	return segments
}

func (p *FieldPathParser) parseSegment(input string) Segment {
	var accesses []Access

	i := 0
	fieldStart := 0

	for i < len(input) {
		if input[i] == '[' {
			if i > fieldStart {
				fieldName := input[fieldStart:i]
				accesses = append(accesses, FieldAccess{Name: fieldName})
			}

			i++
			indexStart := i
			for i < len(input) && input[i] != ']' {
				i++
			}

			if i < len(input) {
				indexStr := input[indexStart:i]
				if index, err := strconv.Atoi(indexStr); err == nil {
					accesses = append(accesses, IndexAccess{Index: index})
				}
			}

			i++
			fieldStart = i
		} else {
			i++
		}
	}

	if fieldStart < len(input) {
		fieldName := input[fieldStart:]
		if fieldName != "" {
			accesses = append(accesses, FieldAccess{Name: fieldName})
		}
	}

	return Segment{Accesses: accesses}
}
