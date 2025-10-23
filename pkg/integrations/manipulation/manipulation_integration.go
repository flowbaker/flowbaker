package manipulation

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

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
		AddPerItem(IntegrationActionType_SetMultipleFields, integration.SetMultipleFields).
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
	FieldType string `json:"field_type"`
}

type FieldUpdate struct {
	FieldPath string `json:"field_path"`
	Value     any    `json:"value"`
	FieldType string `json:"field_type"`
}

type SetMultipleFieldsParams struct {
	Fields []FieldUpdate `json:"fields"`
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

	enhancedItem := make(map[string]any)
	if itemMap, ok := item.(map[string]any); ok {
		for k, v := range itemMap {
			enhancedItem[k] = v
		}
	} else {
		return nil, fmt.Errorf("item must be a map[string]any")
	}

	if p.FieldType != "" && p.FieldType != "auto" {
		if err := i.convertValueToExplicitType(&p.Value, p.FieldType); err != nil {
			return nil, fmt.Errorf("failed to convert value to type '%s': %w", p.FieldType, err)
		}
	} else {
		value, err := i.fieldParser.GetValue(enhancedItem, p.FieldPath)
		if err == nil && value != nil {
			if err := i.convertValueType(value, &p.Value); err != nil {
				return nil, fmt.Errorf("failed to convert value type: %w", err)
			}
		}
	}

	err = i.fieldParser.SetValue(enhancedItem, p.FieldPath, p.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to set field '%s': %w", p.FieldPath, err)
	}

	return enhancedItem, nil
}

func (i *ManipulationIntegration) SetMultipleFields(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SetMultipleFieldsParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if len(p.Fields) == 0 {
		return nil, fmt.Errorf("fields array cannot be empty")
	}

	enhancedItem := make(map[string]any)
	if itemMap, ok := item.(map[string]any); ok {
		for k, v := range itemMap {
			enhancedItem[k] = v
		}
	} else {
		return nil, fmt.Errorf("item must be a map[string]any")
	}

	for idx, field := range p.Fields {
		if field.FieldPath == "" {
			return nil, fmt.Errorf("field_path cannot be empty for field at index %d", idx)
		}

		if field.FieldType != "" && field.FieldType != "auto" {
			if err := i.convertValueToExplicitType(&field.Value, field.FieldType); err != nil {
				return nil, fmt.Errorf("failed to convert value for field '%s': %w", field.FieldPath, err)
			}
		} else {
			value, err := i.fieldParser.GetValue(enhancedItem, field.FieldPath)
			if err == nil && value != nil {
				if err := i.convertValueType(value, &field.Value); err != nil {
					return nil, fmt.Errorf("failed to convert value for field '%s': %w", field.FieldPath, err)
				}
			}
		}

		err = i.fieldParser.SetValue(enhancedItem, field.FieldPath, field.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to set field '%s': %w", field.FieldPath, err)
		}
	}

	return enhancedItem, nil
}

func (i *ManipulationIntegration) convertValueToExplicitType(setValue *any, targetType string) error {
	if setValue == nil || *setValue == nil {
		return nil
	}

	log.Debug().
		Str("targetType", targetType).
		Interface("value", *setValue).
		Msg("converting value to explicit type")

	setValueType := reflect.TypeOf(*setValue)
	setValueKind := setValueType.Kind()

	switch targetType {
	case "string":
		if setValueKind == reflect.String {
			return nil
		}
		*setValue = fmt.Sprintf("%v", *setValue)
		return nil

	case "number":
		if isNumericKind(setValueKind) {
			return nil
		}
		if setValueKind == reflect.String {
			setValueStr := (*setValue).(string)
			setValueStr = strings.TrimSpace(setValueStr)
			result, err := strconv.ParseFloat(setValueStr, 64)
			if err != nil {
				return fmt.Errorf("failed to parse string as number: %w", err)
			}
			*setValue = result
			return nil
		}
		return fmt.Errorf("cannot convert type %s to number", setValueKind)

	case "boolean":
		if setValueKind == reflect.Bool {
			return nil
		}
		if setValueKind == reflect.String {
			setValueStr := (*setValue).(string)
			setValueStr = strings.TrimSpace(strings.ToLower(setValueStr))
			result, err := strconv.ParseBool(setValueStr)
			if err != nil {
				return fmt.Errorf("failed to parse string as boolean: %w", err)
			}
			*setValue = result
			return nil
		}
		return fmt.Errorf("cannot convert type %s to boolean", setValueKind)

	case "array":
		if setValueKind == reflect.Slice || setValueKind == reflect.Array {
			return nil
		}
		if setValueKind == reflect.String {
			setValueStr := (*setValue).(string)
			var result []any
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to parse string as array: %w", err)
			}
			*setValue = result
			return nil
		}
		return fmt.Errorf("cannot convert type %s to array", setValueKind)

	case "object":
		if setValueKind == reflect.Map {
			return nil
		}
		if setValueKind == reflect.String {
			setValueStr := (*setValue).(string)
			var result map[string]any
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to parse string as object: %w", err)
			}
			*setValue = result
			return nil
		}
		return fmt.Errorf("cannot convert type %s to object", setValueKind)

	default:
		return fmt.Errorf("unknown target type: %s", targetType)
	}
}

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

	if realValueType.Kind() == setValueType.Kind() {
		return nil
	}

	if setValueType.Kind() == reflect.String {
		setValueStr, ok := (*setValue).(string)
		if !ok {
			return fmt.Errorf("failed to cast setValue to string")
		}

		switch realValueType.Kind() {
		case reflect.Slice, reflect.Array:
			var result []any
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to array/slice: %w", err)
			}
			*setValue = result
			log.Debug().Interface("converted", result).Msg("converted JSON string to slice")
			return nil

		case reflect.Map:
			var result map[string]any
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to map: %w", err)
			}
			*setValue = result
			log.Debug().Interface("converted", result).Msg("converted JSON string to map")
			return nil

		case reflect.Bool:
			var result bool
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to bool: %w", err)
			}
			*setValue = result
			log.Debug().Bool("converted", result).Msg("converted JSON string to bool")
			return nil

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var result int64
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to int: %w", err)
			}
			*setValue = result
			log.Debug().Int64("converted", result).Msg("converted JSON string to int")
			return nil

		case reflect.Float32, reflect.Float64:
			var result float64
			if err := json.Unmarshal([]byte(setValueStr), &result); err != nil {
				return fmt.Errorf("failed to unmarshal string to float: %w", err)
			}
			*setValue = result
			log.Debug().Float64("converted", result).Msg("converted JSON string to float")
			return nil

		case reflect.String:
			return nil
		}
	}

	if isNumericKind(realValueType.Kind()) && isNumericKind(setValueType.Kind()) {
		return nil
	}

	log.Debug().Msg("no conversion performed, types may be incompatible")
	return nil
}

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

	enhancedItem := make(map[string]any)
	if itemMap, ok := item.(map[string]any); ok {
		for k, v := range itemMap {
			enhancedItem[k] = v
		}
	} else {
		return nil, fmt.Errorf("item must be a map[string]any")
	}

	err = i.fieldParser.DeleteValue(enhancedItem, p.FieldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to delete field '%s': %w", p.FieldPath, err)
	}

	return enhancedItem, nil
}
