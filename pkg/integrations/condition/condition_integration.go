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

// Internal format structures
type Condition struct {
	Value1        string `json:"value1"`
	Value2        string `json:"value2"`
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
	ValueType    string        `json:"value_type"`
	ValueString  string        `json:"value_string,omitempty"`
	ValueNumber  float64       `json:"value_number,omitempty"`
	ValueBoolean bool          `json:"value_boolean,omitempty"`
	ValueDate    string        `json:"value_date,omitempty"`
	ValueArray   interface{}   `json:"value_array,omitempty"`
	ValueObject  interface{}   `json:"value_object,omitempty"`
	Routes       []SwitchValue `json:"routes,omitempty"`
}

// TODO: THERE ARE MANY ANY TYPES HERE, WE NEED TO HANDLE THEM CORRECTLY
type SwitchValue struct {
	Key                    string               `json:"Name"`
	ValueString            any                  `json:"value_string,omitempty"`
	ValueStringComparison  ConditionTypeString  `json:"value_string_comparison,omitempty"`
	ValueNumber            any                  `json:"value_number,omitempty"`
	ValueNumberComparison  ConditionTypeNumber  `json:"value_number_comparison,omitempty"`
	ValueBoolean           any                  `json:"value_boolean,omitempty"`
	ValueBooleanComparison ConditionTypeBoolean `json:"value_boolean_comparison,omitempty"`
	ValueDate              any                  `json:"value_date,omitempty"`
	ValueDateComparison    ConditionTypeDate    `json:"value_date_comparison,omitempty"`
	// TODO: Handle data format for array
	ValueArray           interface{}        `json:"value_array,omitempty"`
	ValueArrayComparison ConditionTypeArray `json:"value_array_comparison,omitempty"`
	// TODO: Handle data format for object
	ValueObject           interface{}         `json:"value_object,omitempty"`
	ValueObjectComparison ConditionTypeObject `json:"value_object_comparison,omitempty"`
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

	for _, condition := range conditionParams.Conditions {
		conditionResult, err := EvaluateCondition(condition)
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

func EvaluateCondition(conditions Condition) (bool, error) {
	if conditions.Value1 == "" && conditions.Value2 == "" {
		return false, nil
	}

	parts := strings.SplitN(conditions.ConditionType, ".", 2)

	var dataType, conditionType string
	if len(parts) == 2 {
		dataType = parts[0]
		conditionType = parts[1]
	} else {
		dataType = conditions.ConditionType
		conditionType = ""
	}

	switch dataType {
	case "string":
		return evaluateStringCondition(conditionType, conditions)
	case "number":
		return evaluateNumberCondition(conditionType, conditions)
	case "boolean":
		return evaluateBooleanCondition(conditionType, conditions)
	case "date":
		return evaluateDateCondition(conditionType, conditions)
	case "array":
		return evaluateArrayCondition(conditionType, conditions)
	case "object":
		return evaluateObjectCondition(conditionType, conditions)
	default:
		return false, fmt.Errorf("unknown condition data type: %s", dataType)
	}
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
		matches, err := i.evaluateValueComparison(p, route)
		if err != nil {
			return domain.RoutableOutput{}, err
		}

		if matches {
			enhancedItem := make(map[string]any)
			for k, v := range item.(map[string]any) {
				enhancedItem[k] = v
			}

			switchResult, ok := enhancedItem["__switch_result"].(map[string]any)
			if !ok {
				return domain.RoutableOutput{}, fmt.Errorf("switch_result is not a map")
			}

			switchResult["matched_route_key"] = route.Key
			switchResult["matched_route_index"] = routeIndex
			switchResult["comparison_type"] = p.ValueType

			switch p.ValueType {
			case "string":
				switchResult["value"] = p.ValueString
				switchResult["comparison_value"] = route.ValueString
			case "number":
				switchResult["value"] = p.ValueNumber
				switchResult["comparison_value"] = route.ValueNumber
			case "boolean":
				switchResult["value"] = p.ValueBoolean
				switchResult["comparison_value"] = route.ValueBoolean
			case "date":
				switchResult["value"] = p.ValueDate
				switchResult["comparison_value"] = route.ValueDate
			case "array":
				switchResult["value"] = p.ValueArray
				switchResult["comparison_value"] = route.ValueArray
			case "object":
				switchResult["value"] = p.ValueObject
				switchResult["comparison_value"] = route.ValueObject
			}

			return domain.RoutableOutput{
				Item:        enhancedItem,
				OutputIndex: routeIndex,
			}, nil
		}
	}

	return domain.RoutableOutput{}, fmt.Errorf("no matching route found for input value")
}

func (i *ConditionIntegration) evaluateValueComparison(params SwitchParams, route SwitchValue) (bool, error) {
	// Determine which field is actually filled in the route
	actualValueType, err := i.determineActualValueType(route)
	if err != nil {
		return false, err
	}

	// Use the actual value type from route instead of the declared valueType
	switch actualValueType {
	case "string":
		if route.ValueStringComparison == "" {
			return false, fmt.Errorf("string comparison type is required")
		}
		return evaluateStringCondition(string(route.ValueStringComparison), Condition{
			Value1: route.ValueString.(string),
			Value2: params.ValueString,
		})
	case "number":
		if route.ValueNumberComparison == "" {
			return false, fmt.Errorf("number comparison type is required")
		}
		return evaluateNumberCondition(string(route.ValueNumberComparison), Condition{
			Value1: fmt.Sprintf("%f", params.ValueNumber),
			Value2: fmt.Sprintf("%f", route.ValueNumber),
		})
	case "boolean":
		if route.ValueBooleanComparison == "" {
			return false, fmt.Errorf("boolean comparison type is required")
		}
		return evaluateBooleanCondition(string(route.ValueBooleanComparison), Condition{
			Value1: fmt.Sprintf("%t", params.ValueBoolean),
			Value2: fmt.Sprintf("%t", route.ValueBoolean),
		})
	case "date":
		if route.ValueDateComparison == "" {
			return false, fmt.Errorf("date comparison type is required")
		}
		return evaluateDateCondition(string(route.ValueDateComparison), Condition{
			Value1: params.ValueDate,
			Value2: route.ValueDate.(string),
		})
	default:
		return false, fmt.Errorf("no valid value found in route or unsupported type: %s", actualValueType)
	}
}

func (i *ConditionIntegration) determineActualValueType(route SwitchValue) (string, error) {
	// Check which field has a non-empty value by looking at comparison types first
	// This is more reliable since user has to set comparison type to use a field

	if route.ValueStringComparison != "" {
		return "string", nil
	}
	if route.ValueNumberComparison != "" {
		return "number", nil
	}
	if route.ValueBooleanComparison != "" {
		return "boolean", nil
	}
	if route.ValueDateComparison != "" {
		return "date", nil
	}

	// Fallback: check actual values (less reliable due to empty string issues)
	if route.ValueString != "" {
		return "string", nil
	}
	if route.ValueNumber != "" {
		return "number", nil
	}
	if route.ValueDate != "" {
		return "date", nil
	}

	return "", fmt.Errorf("no comparison type set for route '%s'", route.Key)
}

func evaluateStringCondition(conditionType string, conditions Condition) (bool, error) {
	switch ConditionTypeString(conditionType) {
	case ConditionTypeString_Exists:
		return conditions.Value1 != "", nil
	case ConditionTypeString_DoesNotExist:
		return conditions.Value1 == "", nil
	case ConditionTypeString_IsEqual:
		return conditions.Value1 == conditions.Value2, nil
	case ConditionTypeString_IsNotEqual:
		return conditions.Value1 != conditions.Value2, nil
	case ConditionTypeString_Contains:
		return strings.Contains(conditions.Value1, conditions.Value2), nil
	case ConditionTypeString_DoesNotContain:
		return !strings.Contains(conditions.Value1, conditions.Value2), nil
	case ConditionTypeString_StartsWith:
		return strings.HasPrefix(conditions.Value1, conditions.Value2), nil
	case ConditionTypeString_EndsWith:
		return strings.HasSuffix(conditions.Value1, conditions.Value2), nil
	case ConditionTypeString_DoesNotStartWith:
		return !strings.HasPrefix(conditions.Value1, conditions.Value2), nil
	case ConditionTypeString_DoesNotEndWith:
		return !strings.HasSuffix(conditions.Value1, conditions.Value2), nil
	case ConditionTypeString_IsEmpty:
		return conditions.Value1 == "", nil
	case ConditionTypeString_IsNotEmpty:
		return conditions.Value1 != "", nil
	case ConditionTypeString_MatchesRegex:
		matched, err := regexp.MatchString(conditions.Value2, conditions.Value1)
		if err != nil {
			return false, fmt.Errorf("regex parse error in matches_regex: %w", err)
		}
		return matched, nil
	case ConditionTypeString_DoesNotMatchRegex:
		matched, err := regexp.MatchString(conditions.Value2, conditions.Value1)
		if err != nil {
			return false, fmt.Errorf("regex parse error in does_not_match_regex: %w", err)
		}
		return !matched, nil
	default:
		return false, fmt.Errorf("unknown string condition type: %s", conditionType)
	}
}

func evaluateNumberCondition(conditionType string, conditions Condition) (bool, error) {
	switch ConditionTypeNumber(conditionType) {
	case ConditionTypeNumber_Exists:
		return conditions.Value1 != "", nil
	case ConditionTypeNumber_DoesNotExist:
		return conditions.Value1 == "", nil
	}

	// Parse string values to float64 for number comparison
	num1, err1 := parseFloat64(conditions.Value1)
	num2, err2 := parseFloat64(conditions.Value2)

	// If parsing failed, return error
	if err1 != nil {
		return false, fmt.Errorf("failed to parse condition1 as number '%s': %w", conditions.Value1, err1)
	}
	if err2 != nil {
		return false, fmt.Errorf("failed to parse condition2 as number '%s': %w", conditions.Value2, err2)
	}

	switch ConditionTypeNumber(conditionType) {
	case ConditionTypeNumber_IsEqual:
		return num1 == num2, nil
	case ConditionTypeNumber_IsNotEqual:
		return num1 != num2, nil
	case ConditionTypeNumber_IsGreaterThan:
		return num1 > num2, nil
	case ConditionTypeNumber_IsLessThan:
		return num1 < num2, nil
	case ConditionTypeNumber_IsGreaterThanOrEqual:
		return num1 >= num2, nil
	case ConditionTypeNumber_IsLessThanOrEqual:
		return num1 <= num2, nil
	default:
		return false, fmt.Errorf("unknown number condition type: %s", conditionType)
	}
}

func evaluateBooleanCondition(conditionType string, conditions Condition) (bool, error) {
	switch ConditionTypeBoolean(conditionType) {
	case ConditionTypeBoolean_Exists:
		return conditions.Value1 != "", nil
	case ConditionTypeBoolean_DoesNotExist:
		return conditions.Value1 == "", nil
	case ConditionTypeBoolean_IsEqual:
		return conditions.Value1 == conditions.Value2, nil
	case ConditionTypeBoolean_IsNotEqual:
		return conditions.Value1 != conditions.Value2, nil
	case ConditionTypeBoolean_IsTrue:
		return strings.ToLower(conditions.Value1) == "true", nil
	case ConditionTypeBoolean_IsFalse:
		return strings.ToLower(conditions.Value1) == "false", nil
	default:
		return false, fmt.Errorf("unknown boolean condition type: %s", conditionType)
	}
}

func evaluateDateCondition(conditionType string, conditions Condition) (bool, error) {
	switch ConditionTypeDate(conditionType) {
	case ConditionTypeDate_Exists:
		return conditions.Value1 != "", nil
	case ConditionTypeDate_DoesNotExist:
		return conditions.Value1 == "", nil
	}

	date1, err1 := time.Parse(time.RFC3339, conditions.Value1)
	date2, err2 := time.Parse(time.RFC3339, conditions.Value2)

	if err1 != nil {
		return false, fmt.Errorf("failed to parse condition1 as date '%s': %w", conditions.Value1, err1)
	}
	if err2 != nil {
		return false, fmt.Errorf("failed to parse condition2 as date '%s': %w", conditions.Value2, err2)
	}

	switch ConditionTypeDate(conditionType) {
	case ConditionTypeDate_IsEqual:
		return date1.Equal(date2), nil
	case ConditionTypeDate_IsNotEqual:
		return !date1.Equal(date2), nil
	case ConditionTypeDate_IsAfter:
		return date1.After(date2), nil
	case ConditionTypeDate_IsBefore:
		return date1.Before(date2), nil
	case ConditionTypeDate_IsAfterOrEqual:
		return date1.After(date2) || date1.Equal(date2), nil
	case ConditionTypeDate_IsBeforeOrEqual:
		return date1.Before(date2) || date1.Equal(date2), nil
	default:
		return false, fmt.Errorf("unknown date condition type: %s", conditionType)
	}
}

func evaluateArrayCondition(conditionType string, conditions Condition) (bool, error) {
	switch ConditionTypeArray(conditionType) {
	case ConditionTypeArray_DoesNotExist:
		var arr []interface{}
		err := json.Unmarshal([]byte(conditions.Value1), &arr)
		return err != nil, nil
	}

	var arr []interface{}
	err := json.Unmarshal([]byte(conditions.Value1), &arr)
	if err != nil {
		return false, fmt.Errorf("failed to parse condition1 as JSON array '%s': %w", conditions.Value1, err)
	}

	switch ConditionTypeArray(conditionType) {
	case ConditionTypeArray_Exists:
		return true, nil
	case ConditionTypeArray_IsEmpty:
		return len(arr) == 0, nil
	case ConditionTypeArray_IsNotEmpty:
		return len(arr) > 0, nil
	case ConditionTypeArray_Contains:
		return arrayContains(arr, conditions.Value2), nil
	case ConditionTypeArray_DoesNotContain:
		return !arrayContains(arr, conditions.Value2), nil
	case ConditionTypeArray_LengthEquals:
		targetLen, err := parseFloat64(conditions.Value2)
		if err != nil {
			return false, fmt.Errorf("failed to parse length value '%s': %w", conditions.Value2, err)
		}
		return float64(len(arr)) == targetLen, nil
	case ConditionTypeArray_LengthGreaterThan:
		targetLen, err := parseFloat64(conditions.Value2)
		if err != nil {
			return false, fmt.Errorf("failed to parse length value '%s': %w", conditions.Value2, err)
		}
		return float64(len(arr)) > targetLen, nil
	case ConditionTypeArray_LengthLessThan:
		targetLen, err := parseFloat64(conditions.Value2)
		if err != nil {
			return false, fmt.Errorf("failed to parse length value '%s': %w", conditions.Value2, err)
		}
		return float64(len(arr)) < targetLen, nil
	default:
		return false, fmt.Errorf("unknown array condition type: %s", conditionType)
	}
}

func evaluateObjectCondition(conditionType string, condition Condition) (bool, error) {
	switch ConditionTypeObject(conditionType) {
	case ConditionTypeObject_DoesNotExist:
		var obj map[string]interface{}
		err := json.Unmarshal([]byte(condition.Value1), &obj)
		return err != nil, nil
	}

	var obj map[string]interface{}
	err := json.Unmarshal([]byte(condition.Value1), &obj)
	if err != nil {
		return false, fmt.Errorf("failed to parse condition1 as JSON object '%s': %w", condition.Value1, err)
	}

	switch ConditionTypeObject(conditionType) {
	case ConditionTypeObject_Exists:
		return true, nil
	case ConditionTypeObject_HasKey:
		_, exists := obj[condition.Value2]
		return exists, nil
	case ConditionTypeObject_DoesNotHaveKey:
		_, exists := obj[condition.Value2]
		return !exists, nil
	case ConditionTypeObject_KeyEquals:
		// Condition2 should be in format "key:value"
		parts := strings.SplitN(condition.Value2, ":", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("key_equals condition requires format 'key:value', got: %s", condition.Value2)
		}
		val, exists := obj[parts[0]]
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
		parts := strings.SplitN(condition.Value2, ":", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("key_not_equals condition requires format 'key:value', got: %s", condition.Value2)
		}
		val, exists := obj[parts[0]]
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
		return false, fmt.Errorf("unknown object condition type: %s", conditionType)
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
