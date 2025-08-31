package executor

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

// ConvertToArray converts any input data into an array of Items
func ConvertToArray(data interface{}) ([]domain.Item, error) {
	if data == nil {
		return []domain.Item{}, nil
	}

	// If input is []byte, try to unmarshal it first
	if bytes, ok := data.([]byte); ok {
		if len(bytes) == 0 {
			return []domain.Item{}, nil
		}

		var unmarshaled interface{}
		if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bytes: %w", err)
		}
		data = unmarshaled
	}

	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return handleSlice(value)
	case reflect.Map:
		// Convert to Item if it's not already in the right format
		if item, ok := data.(domain.Item); ok {
			return []domain.Item{item}, nil
		}
		if item, ok := data.(map[string]interface{}); ok {
			return []domain.Item{item}, nil
		}
		// Convert other map types to Item
		return convertMapToItem(value)
	case reflect.String:
		return handleString(value.String())
	case reflect.Float64, reflect.Int, reflect.Bool:
		return handlePrimitive(value.Interface())
	case reflect.Struct:
		return handleStruct(value)
	case reflect.Ptr:
		if value.IsNil() {
			return []domain.Item{}, nil
		}
		return ConvertToArray(value.Elem().Interface())
	default:
		return nil, fmt.Errorf("unsupported type: %T", data)
	}
}

// handleSlice processes slice/array inputs
func handleSlice(value reflect.Value) ([]domain.Item, error) {
	length := value.Len()
	items := make([]domain.Item, 0, length)

	for i := 0; i < length; i++ {
		elem := value.Index(i)

		// Handle nil elements in the slice
		if elem.Kind() == reflect.Ptr && elem.IsNil() {
			continue
		}

		// If element is already a map[string]interface{}, add it directly
		if item, ok := elem.Interface().(map[string]interface{}); ok {
			items = append(items, item)
			continue
		}

		// Convert the element to Item
		converted, err := ConvertToArray(elem.Interface())
		if err != nil {
			return nil, fmt.Errorf("error converting element at index %d: %w", i, err)
		}
		items = append(items, converted...)
	}

	return items, nil
}

// handleString processes string inputs
func handleString(s string) ([]domain.Item, error) {
	// Try to parse as JSON first
	var jsonData interface{}
	if err := json.Unmarshal([]byte(s), &jsonData); err == nil {
		return ConvertToArray(jsonData)
	}

	// If not JSON, wrap in an object
	return []domain.Item{map[string]any{"value": s}}, nil
}

// handlePrimitive processes primitive type inputs
func handlePrimitive(value interface{}) ([]domain.Item, error) {
	return []domain.Item{map[string]any{"value": value}}, nil
}

// handleStruct converts a struct to Item
func handleStruct(value reflect.Value) ([]domain.Item, error) {
	// Marshal the struct to JSON first
	jsonData, err := json.Marshal(value.Interface())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	// Unmarshal back to map[string]interface{}
	var item map[string]interface{}
	if err := json.Unmarshal(jsonData, &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal struct: %w", err)
	}

	return []domain.Item{item}, nil
}

// convertMapToItem converts any map type to Item
func convertMapToItem(value reflect.Value) ([]domain.Item, error) {
	// Marshal the map to JSON first
	jsonData, err := json.Marshal(value.Interface())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal map: %w", err)
	}

	// Unmarshal back to map[string]interface{}
	var item map[string]interface{}
	if err := json.Unmarshal(jsonData, &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal map: %w", err)
	}

	return []domain.Item{item}, nil
}
