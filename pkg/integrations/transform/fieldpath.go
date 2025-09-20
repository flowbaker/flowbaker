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
