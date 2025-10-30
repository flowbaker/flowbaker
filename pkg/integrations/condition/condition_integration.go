package condition

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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
		AddPerItemRoutable(IntegrationActionType_ConditionalDispatch, integration.ConditionalDispatch)

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

type ConditionalDispatchParams struct {
	ValueType string                     `json:"value_type"`
	Value     any                        `json:"value,omitempty"`
	Routes    []ConditionalDispatchValue `json:"routes,omitempty"`
}

type ConditionalDispatchValue struct {
	Key             string `json:"key"`
	Value           any    `json:"value,omitempty"`
	ValueComparison string `json:"value_comparison,omitempty"`
}

type IfElseResult struct {
	OutputIndex    int    `json:"output_index"`
	ValueType      string `json:"value_type"`
	Value1         any    `json:"value1"`
	Value2         any    `json:"value2"`
	ComparisonType string `json:"comparison_type"`
}

type ConditionalDispatchResult struct {
	MatchedRouteKey   string `json:"matched_route_key"`
	MatchedRouteIndex int    `json:"matched_route_index"`
	ComparisonType    string `json:"comparison_type"`
	Value             any    `json:"value"`
	ComparisonValue   any    `json:"comparison_value"`
}

func (i *ConditionIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *ConditionIntegration) IfElse(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.RoutableOutput, error) {
	p := ConditionParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.RoutableOutput{}, err
	}

	result := false
	if p.ConditionRelation == ConditionRelationAnd {
		result = true
	}

	if len(p.Conditions) == 0 {
		return domain.RoutableOutput{}, fmt.Errorf("no conditions found")
	}

	conditionType := p.Conditions[0].ConditionType
	if conditionType == "" {
		return domain.RoutableOutput{}, fmt.Errorf("no condition type found")
	}

	parts := strings.SplitN(conditionType, ".", 2)
	if len(parts) != 2 {
		return domain.RoutableOutput{}, fmt.Errorf("invalid condition type: %s", conditionType)
	}

	valueType := parts[0]
	comparisonType := parts[1]

	for _, condition := range p.Conditions {
		conditionResult, err := EvaluateCondition(valueType, EvaluateConditionParams{
			Value1:         condition.Value1,
			Value2:         condition.Value2,
			ComparisonType: comparisonType,
		})
		if err != nil {
			return domain.RoutableOutput{}, fmt.Errorf("failed to evaluate condition: %w", err)
		}

		if p.ConditionRelation == ConditionRelationAnd {
			result = result && conditionResult
		} else {
			result = result || conditionResult
		}
	}

	outputIndex := 1
	if result {
		outputIndex = 0
	}

	enhancedItem := make(map[string]any)
	for k, v := range item.(map[string]any) {
		enhancedItem[k] = v
	}

	// now returns only the first condition value1 and value2
	ifElseResult := IfElseResult{
		OutputIndex:    outputIndex,
		ValueType:      valueType,
		Value1:         p.Conditions[0].Value1,
		Value2:         p.Conditions[0].Value2,
		ComparisonType: comparisonType,
	}

	enhancedItem["__if_else_result__"] = ifElseResult

	return domain.RoutableOutput{
		Item:        enhancedItem,
		OutputIndex: outputIndex,
	}, nil
}

func (i *ConditionIntegration) ConditionalDispatch(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.RoutableOutput, error) {
	p := ConditionalDispatchParams{}

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

			conditionalDispatchResult := ConditionalDispatchResult{
				MatchedRouteKey:   route.Key,
				MatchedRouteIndex: routeIndex,
				ComparisonType:    route.ValueComparison,
				Value:             p.Value,
				ComparisonValue:   route.Value,
			}

			enhancedItem["__conditional_dispatch_result__"] = conditionalDispatchResult

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
	case "tag":
		return evaluateTagCondition(params)
	default:
		return false, fmt.Errorf("unknown condition data type: %s", params.ComparisonType)
	}
}

func evaluateStringCondition(params EvaluateConditionParams) (bool, error) {
	value1str, err := convertToString(params.Value1)
	if err != nil {
		return false, fmt.Errorf("value1 is not a string: %w", err)
	}

	value2str, err := convertToString(params.Value2)
	if err != nil {
		return false, fmt.Errorf("value2 is not a string: %w", err)
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
		isEmpty := checkIsEmpty(value1str)
		if isEmpty {
			return true, nil
		} else {
			return value1str == "", nil
		}
	case ConditionTypeString_IsNotEmpty:
		isEmpty := checkIsEmpty(value1str)
		if isEmpty {
			return false, nil
		} else {
			return value1str != "", nil
		}
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
	value1num, err := convertToFloat64(params.Value1)
	if err != nil {
		return false, fmt.Errorf("value1 is not a number: %w", err)
	}

	value2num, err := convertToFloat64(params.Value2)
	if err != nil {
		return false, fmt.Errorf("value2 is not a number: %w", err)
	}

	switch ConditionTypeNumber(params.ComparisonType) {
	case ConditionTypeNumber_Exists:
		return value1num != 0, nil
	case ConditionTypeNumber_DoesNotExist:
		return value1num == 0, nil
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
	value1bool, err := convertToBool(params.Value1)
	if err != nil {
		return false, fmt.Errorf("value1 is not a boolean: %w", err)
	}

	value2bool, err := convertToBool(params.Value2)
	if err != nil {
		return false, fmt.Errorf("value2 is not a boolean: %w", err)
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
	value1date, err := convertToTime(params.Value1)
	if err != nil {
		return false, fmt.Errorf("value1 is not a date: %w", err)
	}

	value2date, err := convertToTime(params.Value2)
	if err != nil {
		return false, fmt.Errorf("value2 is not a date: %w", err)
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
	value1arr, err := convertToArray(params.Value1)
	if err != nil {
		return false, fmt.Errorf("value1 is not an array: %w", err)
	}

	value2arr, err := convertToArray(params.Value2)
	if err != nil {
		return false, fmt.Errorf("cannot convert value2 to array: %w", err)
	}

	value2str, err := convertToString(params.Value2)
	if err != nil {
		return false, fmt.Errorf("cannot convert value2 to string: %w", err)
	}

	switch ConditionTypeArray(params.ComparisonType) {
	case ConditionTypeArray_Exists:
		return true, nil
	case ConditionTypeArray_DoesNotExist:
		return false, nil
	case ConditionTypeArray_IsEmpty:
		isEmpty := checkIsEmpty(value1arr)
		if isEmpty {
			return true, nil
		} else {
			return len(value1arr) == 0, nil
		}

	case ConditionTypeArray_IsNotEmpty:
		isEmpty := checkIsEmpty(value1arr)
		if isEmpty {
			return false, nil
		} else {
			return len(value1arr) > 0, nil
		}
	case ConditionTypeArray_Contains:
		return arrayContains(value1arr, value2str), nil
	case ConditionTypeArray_DoesNotContains:
		return !arrayContains(value1arr, value2str), nil
	case ConditionTypeArray_LengthEquals:
		return len(value1arr) == len(value2arr), nil
	case ConditionTypeArray_LengthGreaterThan:
		return len(value1arr) > len(value2arr), nil
	case ConditionTypeArray_LengthLessThan:
		return len(value1arr) < len(value2arr), nil
	default:
		return false, fmt.Errorf("unknown array condition type: %s", params.ComparisonType)
	}
}

func evaluateTagCondition(params EvaluateConditionParams) (bool, error) {
	value1Array, err := convertToArray(params.Value1)
	if err != nil {
		return false, fmt.Errorf("value1 is not an array: %w", err)
	}

	value2Array, err := convertToArray(params.Value2)
	if err != nil {
		return false, fmt.Errorf("value2 is not an array: %w", err)
	}

	value1ArrayString := make([]string, len(value1Array))
	for i, v := range value1Array {
		if str, ok := v.(string); ok {
			value1ArrayString[i] = str
		} else {
			value1ArrayString[i] = fmt.Sprintf("%v", v)
		}
	}

	value2ArrayString := make([]string, len(value2Array))
	for i, v := range value2Array {
		if str, ok := v.(string); ok {
			value2ArrayString[i] = str
		} else {
			value2ArrayString[i] = fmt.Sprintf("%v", v)
		}
	}

	switch ConditionTypeTag(params.ComparisonType) {
	case ConditionTypeTag_Exists:
		return len(value1Array) > 0, nil
	case ConditionTypeTag_DoesNotExist:
		return len(value1Array) == 0, nil
	case ConditionTypeTag_IsEqual:
		return stringSlicesEqual(value1ArrayString, value2ArrayString), nil
	case ConditionTypeTag_IsNotEqual:
		return !stringSlicesEqual(value1ArrayString, value2ArrayString), nil
	case ConditionTypeTag_Contains:
		return stringSliceContainsAll(value1ArrayString, value2ArrayString), nil
	case ConditionTypeTag_ContainsAny:
		return stringSliceContainsAny(value1ArrayString, value2ArrayString), nil
	case ConditionTypeTag_DoesNotContain:
		return !stringSliceContainsAny(value1ArrayString, value2ArrayString), nil
	}
	return false, fmt.Errorf("unknown tag condition type: %s", params.ComparisonType)
}

func evaluateObjectCondition(params EvaluateConditionParams) (bool, error) {
	value1obj, ok := params.Value1.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("value1 is not an object")
	}

	value1objstr, ok := params.Value1.(string)
	if !ok {
		// Try to convert to string
		v1str, err := convertToString(params.Value1)
		if err != nil {
			return false, fmt.Errorf("value1 is not a string")
		}
		value1objstr = v1str
	}

	value2objstr, ok := params.Value2.(string)
	if !ok {
		// Try to convert to string
		v2str, err := convertToString(params.Value2)
		if err != nil {
			return false, fmt.Errorf("value2 is not a string")
		}
		value2objstr = v2str
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

func checkIsEmpty(value any) bool {
	str, err := convertToString(value)
	if err != nil {
		return false
	}
	if str == "null" || str == "undefined" {
		return true
	}

	if len(str) == 0 {
		return true
	}

	return str == ""
}

func arrayContains(arr []interface{}, target string) bool {
	for _, item := range arr {
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

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringSliceContainsAll(haystack, needles []string) bool {
	for _, needle := range needles {
		found := false
		for _, item := range haystack {
			if item == needle {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func stringSliceContainsAny(haystack, needles []string) bool {
	for _, needle := range needles {
		for _, item := range haystack {
			if item == needle {
				return true
			}
		}
	}
	return false
}

func convertToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

func convertToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cannot convert %T to string", value)
		}
		return string(bytes), nil
	}
}

func convertToBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int, int32, int64:
		return fmt.Sprintf("%v", v) != "0", nil
	case float32, float64:
		return fmt.Sprintf("%v", v) != "0", nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

func convertToTime(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
			"15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("cannot parse time string: %s", v)
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time.Time", value)
	}
}

func convertToArray(value interface{}) ([]any, error) {
	switch v := value.(type) {
	case []any:
		return v, nil
	case string:
		if len(v) == 0 {
			return []any{}, nil
		}

		var arr []any

		err := json.Unmarshal([]byte(v), &arr)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal string to []any: %w", err)
		}

		return arr, nil
	case nil:
		return []any{}, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to []any", value)
	}
}
