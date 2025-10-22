package condition

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ConditionIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewConditionIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &ConditionIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *ConditionIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewConditionIntegration(ConditionIntegrationDependencies{
		ParameterBinder: c.binder,
	})
}

type ConditionIntegration struct {
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
}

type ConditionIntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewConditionIntegration(deps ConditionIntegrationDependencies) (*ConditionIntegration, error) {
	integration := &ConditionIntegration{
		binder: deps.ParameterBinder,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemRoutable(IntegrationActionType_IfElse, integration.IfElse).
		AddPerItemRoutable(IntegrationActionType_Switch, integration.Switch)

	integration.actionManager = actionManager

	return integration, nil
}

type Condition struct {
	Value1        any    `json:"value1"`
	Value2        any    `json:"value2"`
	ConditionType string `json:"condition_type"`
}

type ConditionParams struct {
	Conditions        []Condition       `json:"conditions"`
	ConditionRelation ConditionRelation `json:"relation_type"`
}

type ConditionRelation string

const (
	ConditionRelationAnd ConditionRelation = "and"
	ConditionRelationOr  ConditionRelation = "or"
)

type SwitchParams struct {
	ValueType string        `json:"value_type"`
	Value     any           `json:"value,omitempty"`
	Routes    []SwitchValue `json:"routes,omitempty"`
}

type SwitchValue struct {
	Key             string `json:"Name"`
	Value           any    `json:"value,omitempty"`
	ValueComparison string `json:"value_comparison,omitempty"`
}

func (i *ConditionIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *ConditionIntegration) IfElse(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.RoutableOutput, error) {
	conditionParams := ConditionParams{}
	err := i.binder.BindToStruct(ctx, item, &conditionParams, params.IntegrationParams.Settings)
	if err != nil {
		return domain.RoutableOutput{}, err
	}

	result := conditionParams.ConditionRelation == ConditionRelationAnd
	if conditionParams.ConditionRelation == ConditionRelationOr {
		result = false
	}

	conditionType := conditionParams.Conditions[0].ConditionType
	parts := strings.SplitN(conditionType, ".", 2)

	valueType := parts[0]
	comparisonType := parts[1]

	for _, condition := range conditionParams.Conditions {
		conditionResult, err := EvaluateCondition(valueType, EvaluateConditionParams{
			Value1:         condition.Value1,
			Value2:         condition.Value2,
			ComparisonType: comparisonType,
		})
		if err != nil {
			return domain.RoutableOutput{}, fmt.Errorf("failed to evaluate condition: %w", err)
		}

		if conditionParams.ConditionRelation == ConditionRelationAnd {
			result = result && conditionResult
		} else {
			result = result || conditionResult
		}
	}

	outputIndex := 1
	if result {
		outputIndex = 0
	}

	return domain.RoutableOutput{
		Item:        item,
		OutputIndex: outputIndex,
	}, nil
}

func (i *ConditionIntegration) Switch(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.RoutableOutput, error) {
	p := SwitchParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.RoutableOutput{}, err
	}

	if len(p.Routes) == 0 {
		return domain.RoutableOutput{}, fmt.Errorf("no routes defined")
	}

	for routeIndex, route := range p.Routes {
		matches, err := EvaluateCondition(p.ValueType, EvaluateConditionParams{
			Value1:         route.Value,
			Value2:         p.Value,
			ComparisonType: route.ValueComparison,
		})
		if err != nil {
			return domain.RoutableOutput{}, err
		}

		if matches {
			enhancedItem := make(map[string]any)
			for k, v := range item.(map[string]any) {
				enhancedItem[k] = v
			}

			switchResult := make(map[string]any)
			switchResult["matched_route_key"] = route.Key
			switchResult["matched_route_index"] = routeIndex
			switchResult["comparison_type"] = route.ValueComparison
			switchResult["value"] = p.Value
			switchResult["comparison_value"] = route.Value

			enhancedItem["__switch_result__"] = switchResult

			return domain.RoutableOutput{
				Item:        enhancedItem,
				OutputIndex: routeIndex,
			}, nil
		}
	}

	return domain.RoutableOutput{}, fmt.Errorf("no matching route found for input value")
}

type EvaluateConditionParams struct {
	Value1         any
	Value2         any
	ComparisonType string
}

func EvaluateCondition(valueType string, params EvaluateConditionParams) (bool, error) {
	switch valueType {
	case "string":
		return evaluateStringCondition(params)
	case "number":
		return evaluateNumberCondition(params)
	case "boolean":
		return evaluateBooleanCondition(params)
	case "date":
		return evaluateDateCondition(params)
	case "array":
		return evaluateArrayCondition(params)
	case "object":
		return evaluateObjectCondition(params)
	default:
		return false, fmt.Errorf("unknown condition data type: %s", params.ComparisonType)
	}
}

func evaluateStringCondition(params EvaluateConditionParams) (bool, error) {
	value1str, ok := params.Value1.(string)
	if !ok {
		return false, fmt.Errorf("value1 is not a string")
	}

	value2str, ok := params.Value2.(string)
	if !ok {
		return false, fmt.Errorf("value2 is not a string")
	}

	switch ConditionTypeString(params.ComparisonType) {
	case ConditionTypeString_Exists:
		return value1str != "", nil
	case ConditionTypeString_DoesNotExist:
		return value1str == "", nil
	case ConditionTypeString_IsEqual:
		return value1str == value2str, nil
	case ConditionTypeString_IsNotEqual:
		return value1str != value2str, nil
	case ConditionTypeString_Contains:
		return strings.Contains(value1str, value2str), nil
	case ConditionTypeString_DoesNotContain:
		return !strings.Contains(value1str, value2str), nil
	case ConditionTypeString_StartsWith:
		return strings.HasPrefix(value1str, value2str), nil
	case ConditionTypeString_EndsWith:
		return strings.HasSuffix(value1str, value2str), nil
	case ConditionTypeString_DoesNotStartWith:
		return !strings.HasPrefix(value1str, value2str), nil
	case ConditionTypeString_DoesNotEndWith:
		return !strings.HasSuffix(value1str, value2str), nil
	case ConditionTypeString_IsEmpty:
		return value1str == "", nil
	case ConditionTypeString_IsNotEmpty:
		return value1str != "", nil
	case ConditionTypeString_MatchesRegex:
		matched, err := regexp.MatchString(value2str, value1str)
		if err != nil {
			return false, fmt.Errorf("regex parse error in matches_regex: %w", err)
		}
		return matched, nil
	case ConditionTypeString_DoesNotMatchRegex:
		matched, err := regexp.MatchString(value2str, value1str)
		if err != nil {
			return false, fmt.Errorf("regex parse error in does_not_match_regex: %w", err)
		}
		return !matched, nil
	default:
		return false, fmt.Errorf("unknown string condition type: %s", params.ComparisonType)
	}
}

func evaluateNumberCondition(params EvaluateConditionParams) (bool, error) {
	value1num, ok := params.Value1.(float64)
	if !ok {
		return false, fmt.Errorf("value1 is not a number")
	}

	value2num, ok := params.Value2.(float64)
	if !ok {
		return false, fmt.Errorf("value2 is not a number")
	}

	switch ConditionTypeNumber(params.ComparisonType) {
	case ConditionTypeNumber_IsEqual:
		return value1num == value2num, nil
	case ConditionTypeNumber_IsNotEqual:
		return value1num != value2num, nil
	case ConditionTypeNumber_IsGreaterThan:
		return value1num > value2num, nil
	case ConditionTypeNumber_IsLessThan:
		return value1num < value2num, nil
	case ConditionTypeNumber_IsGreaterThanOrEqual:
		return value1num >= value2num, nil
	case ConditionTypeNumber_IsLessThanOrEqual:
		return value1num <= value2num, nil
	default:
		return false, fmt.Errorf("unknown number condition type: %s", params.ComparisonType)
	}
}

func evaluateBooleanCondition(params EvaluateConditionParams) (bool, error) {
	value1bool, ok := params.Value1.(bool)
	if !ok {
		return false, fmt.Errorf("value1 is not a boolean")
	}

	value2bool, ok := params.Value2.(bool)
	if !ok {
		return false, fmt.Errorf("value2 is not a boolean")
	}

	switch ConditionTypeBoolean(params.ComparisonType) {
	case ConditionTypeBoolean_IsEqual:
		return value1bool == value2bool, nil
	case ConditionTypeBoolean_IsNotEqual:
		return value1bool != value2bool, nil
	case ConditionTypeBoolean_IsTrue:
		return value1bool, nil
	case ConditionTypeBoolean_IsFalse:
		return !value1bool, nil
	default:
		return false, fmt.Errorf("unknown boolean condition type: %s", params.ComparisonType)
	}
}

func evaluateDateCondition(params EvaluateConditionParams) (bool, error) {
	value1date, ok := params.Value1.(time.Time)
	if !ok {
		return false, fmt.Errorf("value1 is not a date")
	}

	value2date, ok := params.Value2.(time.Time)
	if !ok {
		return false, fmt.Errorf("value2 is not a date")
	}

	switch ConditionTypeDate(params.ComparisonType) {
	case ConditionTypeDate_Exists:
		return value1date != time.Time{}, nil
	case ConditionTypeDate_DoesNotExist:
		return value1date.Equal(time.Time{}), nil
	case ConditionTypeDate_IsEqual:
		return value1date.Equal(value2date), nil
	case ConditionTypeDate_IsNotEqual:
		return !value1date.Equal(value2date), nil
	case ConditionTypeDate_IsAfter:
		return value1date.After(value2date), nil
	case ConditionTypeDate_IsBefore:
		return value1date.Before(value2date), nil
	case ConditionTypeDate_IsAfterOrEqual:
		return value1date.After(value2date) || value1date.Equal(value2date), nil
	case ConditionTypeDate_IsBeforeOrEqual:
		return value1date.Before(value2date) || value1date.Equal(value2date), nil
	default:
		return false, fmt.Errorf("unknown date condition type: %s", params.ComparisonType)
	}
}

func evaluateArrayCondition(params EvaluateConditionParams) (bool, error) {
	value1arr, ok := params.Value1.([]interface{})
	if !ok {
		return false, fmt.Errorf("value1 is not an array")
	}

	value2arr, ok := params.Value2.([]interface{})
	if !ok {
		return false, fmt.Errorf("value2 is not an array")
	}

	switch ConditionTypeArray(params.ComparisonType) {
	case ConditionTypeArray_Exists:
		return true, nil
	case ConditionTypeArray_IsEmpty:
		return len(value1arr) == 0, nil
	case ConditionTypeArray_IsNotEmpty:
		return len(value1arr) > 0, nil
	case ConditionTypeArray_Contains:
		value2arrstr, ok := params.Value2.(string)
		if !ok {
			return false, fmt.Errorf("value2 is not a string")
		}
		return arrayContains(value1arr, value2arrstr), nil
	case ConditionTypeArray_LengthGreaterThan:
		return len(value1arr) > len(value2arr), nil
	case ConditionTypeArray_LengthLessThan:
		return len(value1arr) < len(value2arr), nil
	default:
		return false, fmt.Errorf("unknown array condition type: %s", params.ComparisonType)
	}
}

func evaluateObjectCondition(params EvaluateConditionParams) (bool, error) {
	value1obj, ok := params.Value1.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("value1 is not an object")
	}

	value1objstr, ok := params.Value1.(string)
	if !ok {
		return false, fmt.Errorf("value2 is not a string")
	}

	// value2obj, ok := params.Value2.(map[string]interface{})
	// if !ok {
	// 	return false, fmt.Errorf("value2 is not an object")
	// }

	value2objstr, ok := params.Value2.(string)
	if !ok {
		return false, fmt.Errorf("value2 is not a string")
	}

	switch ConditionTypeObject(params.ComparisonType) {
	case ConditionTypeObject_Exists:
		return true, nil
	case ConditionTypeObject_HasKey:
		_, exists := value1obj[value1objstr]
		return exists, nil
	case ConditionTypeObject_DoesNotHaveKey:
		_, exists := value1obj[value1objstr]
		return !exists, nil
	case ConditionTypeObject_KeyEquals:
		// Condition2 should be in format "key:value"
		parts := strings.SplitN(value2objstr, ":", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("key_equals condition requires format 'key:value', got: %s", value2objstr)
		}
		val, exists := value1obj[parts[0]]
		if !exists {
			return false, nil
		}
		// Convert value to string for comparison
		valStr, ok := val.(string)
		if !ok {
			valBytes, err := json.Marshal(val)
			if err != nil {
				return false, fmt.Errorf("failed to marshal object value for comparison: %w", err)
			}
			valStr = string(valBytes)
		}
		return valStr == parts[1], nil
	case ConditionTypeObject_KeyNotEquals:
		parts := strings.SplitN(value2objstr, ":", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("key_not_equals condition requires format 'key:value', got: %s", value2objstr)
		}
		val, exists := value1obj[parts[0]]
		if !exists {
			return true, nil
		}
		// Convert value to string for comparison
		valStr, ok := val.(string)
		if !ok {
			valBytes, err := json.Marshal(val)
			if err != nil {
				return false, fmt.Errorf("failed to marshal object value for comparison: %w", err)
			}
			valStr = string(valBytes)
		}
		return valStr != parts[1], nil
	default:
		return false, fmt.Errorf("unknown object condition type: %s", params.ComparisonType)
	}
}

// Helper function to parse string to float64
func parseFloat64(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func arrayContains(arr []interface{}, target string) bool {
	for _, item := range arr {
		// Convert item to string for comparison
		var itemStr string
		switch v := item.(type) {
		case string:
			itemStr = v
		default:
			itemBytes, err := json.Marshal(v)
			if err != nil {
				continue
			}
			itemStr = string(itemBytes)
		}
		if itemStr == target {
			return true
		}
	}
	return false
}
