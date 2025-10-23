package manipulation

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/flowbaker/flowbaker/pkg/integrations/transform"
	"github.com/rs/zerolog/log"
)

type ManipulationIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewManipulationIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &ManipulationIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *ManipulationIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewManipulationIntegration(ManipulationIntegrationDependencies{
		ParameterBinder: c.binder,
	})
}

type ManipulationIntegration struct {
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
	fieldParser   *transform.FieldPathParser
}

type ManipulationIntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewManipulationIntegration(deps ManipulationIntegrationDependencies) (*ManipulationIntegration, error) {
	integration := &ManipulationIntegration{
		binder:      deps.ParameterBinder,
		fieldParser: transform.NewFieldPathParser(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_SetField, integration.SetField).
		AddPerItem(IntegrationActionType_DeleteField, integration.DeleteField)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *ManipulationIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type SetFieldParams struct {
	FieldPath string `json:"field_path"`
	Value     any    `json:"value"`
}

type DeleteFieldParams struct {
	FieldPath string `json:"field_path"`
}

func (i *ManipulationIntegration) SetField(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SetFieldParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.FieldPath == "" {
		return nil, fmt.Errorf("field_path cannot be empty")
	}

	// Create a copy of the item
	enhancedItem := make(map[string]any)
	if itemMap, ok := item.(map[string]any); ok {
		for k, v := range itemMap {
			enhancedItem[k] = v
		}
	} else {
		return nil, fmt.Errorf("item must be a map[string]any")
	}

	// Try to get the existing value to determine type conversion
	// If the field doesn't exist or is nil, we'll skip type conversion and just set the value
	value, err := i.fieldParser.GetValue(enhancedItem, p.FieldPath)
	if err == nil && value != nil {
		// Handle type conversions when set value is a JSON-encoded string
		if err := i.convertValueType(value, &p.Value); err != nil {
			return nil, fmt.Errorf("failed to convert value type: %w", err)
		}
	}

	// Set the value using the field path parser
	err = i.fieldParser.SetValue(enhancedItem, p.FieldPath, p.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to set field '%s': %w", p.FieldPath, err)
	}

	return enhancedItem, nil
}

// convertValueType attempts to convert setValue to match the type of realValue
func (i *ManipulationIntegration) convertValueType(realValue any, setValue *any) error {
	if setValue == nil || *setValue == nil {
		return nil
	}

	realValueType := reflect.TypeOf(realValue)
	setValueType := reflect.TypeOf(*setValue)

	log.Debug().
		Str("realValueType", realValueType.String()).
		Str("setValueType", setValueType.String()).
		Msg("converting value types")

	// If types already match, no conversion needed
	if realValueType.Kind() == setValueType.Kind() {
		return nil
	}

	// Handle conversion from JSON-encoded string to actual type
	if setValueType.Kind() == reflect.String {
		setValueStr, ok := (*setValue).(string)
		if !ok {
			return fmt.Errorf("failed to cast setValue to string")
		}

		switch realValueType.Kind() {
		case reflect.Slice, reflect.Array:
			// Try to unmarshal JSON string to slice
			var result []any
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to array/slice: %w", err)
			}
			*setValue = result
			log.Debug().Interface("converted", result).Msg("converted JSON string to slice")
			return nil

		case reflect.Map:
			// Try to unmarshal JSON string to map
			var result map[string]any
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to map: %w", err)
			}
			*setValue = result
			log.Debug().Interface("converted", result).Msg("converted JSON string to map")
			return nil

		case reflect.Bool:
			// Try to unmarshal JSON string to bool
			var result bool
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to bool: %w", err)
			}
			*setValue = result
			log.Debug().Bool("converted", result).Msg("converted JSON string to bool")
			return nil

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// Try to unmarshal JSON string to number
			var result int64
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to int: %w", err)
			}
			*setValue = result
			log.Debug().Int64("converted", result).Msg("converted JSON string to int")
			return nil

		case reflect.Float32, reflect.Float64:
			// Try to unmarshal JSON string to float
			var result float64
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to float: %w", err)
			}
			*setValue = result
			log.Debug().Float64("converted", result).Msg("converted JSON string to float")
			return nil

		case reflect.String:
			// Both are strings, no conversion needed
			return nil
		}
	}

	// Handle numeric conversions
	if isNumericKind(realValueType.Kind()) && isNumericKind(setValueType.Kind()) {
		return nil // Let the field parser handle numeric conversions
	}

	log.Debug().Msg("no conversion performed, types may be incompatible")
	return nil
}

// isNumericKind returns true if the kind is a numeric type
func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func (i *ManipulationIntegration) DeleteField(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteFieldParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.FieldPath == "" {
		return nil, fmt.Errorf("field_path cannot be empty")
	}

	// Create a copy of the item
	enhancedItem := make(map[string]any)
	if itemMap, ok := item.(map[string]any); ok {
		for k, v := range itemMap {
			enhancedItem[k] = v
		}
	} else {
		return nil, fmt.Errorf("item must be a map[string]any")
	}

	// Delete the value using the field path parser
	err = i.fieldParser.DeleteValue(enhancedItem, p.FieldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to delete field '%s': %w", p.FieldPath, err)
	}

	return enhancedItem, nil
}
