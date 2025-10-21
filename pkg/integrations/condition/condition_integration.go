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
	binder domain.IntegrationParameterBinder

	actionFuncs map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error)
}

type ConditionIntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewConditionIntegration(deps ConditionIntegrationDependencies) (*ConditionIntegration, error) {
	integration := &ConditionIntegration{
		binder: deps.ParameterBinder,
	}

	actionFuncs := map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error){
		IntegrationActionType_IfStreams: integration.ifStreams,
	}

	integration.actionFuncs = actionFuncs

	return integration, nil
}

type Conditions struct {
	Condition1    string `json:"condition1"`
	Condition2    string `json:"condition2"`
	ConditionType string `json:"condition_type"`
}

type ConditionParams struct {
	Conditions        []Conditions      `json:"conditions"`
	ConditionRelation ConditionRelation `json:"relation_type"`
}

type ConditionRelation string

const (
	ConditionRelationAnd ConditionRelation = "and"
	ConditionRelationOr  ConditionRelation = "or"
)

func (i *ConditionIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	actionFunc, ok := i.actionFuncs[params.ActionType]
	if !ok {
		return domain.IntegrationOutput{}, fmt.Errorf("action not found")
	}

	return actionFunc(ctx, params)
}

func (i *ConditionIntegration) ifStreams(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := []domain.Item{}
	for _, items := range itemsByInputID {
		allItems = append(allItems, items...)
	}

	outputItems := [][]domain.Item{{}, {}}

	for _, item := range allItems {
		conditionParams := ConditionParams{}
		err := i.binder.BindToStruct(ctx, item, &conditionParams, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		result := false
		for _, condition := range conditionParams.Conditions {
			conditionResult, err := EvaluateCondition(condition)
			if err != nil {
				return domain.IntegrationOutput{}, fmt.Errorf("failed to evaluate condition: %w", err)
			}
			if conditionParams.ConditionRelation == ConditionRelationAnd {
				result = result && conditionResult
			} else {
				result = result || conditionResult
			}
		}

		if result {
			outputItems[0] = append(outputItems[0], item)
		} else {
			outputItems[1] = append(outputItems[1], item)
		}
	}

	resultByOutputID := make([]domain.Payload, 0)
	for _, items := range outputItems {
		payload, err := json.Marshal(items)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to marshal output items: %w", err)
		}
		resultByOutputID = append(resultByOutputID, domain.Payload(payload))
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: resultByOutputID,
	}, nil
}

func EvaluateCondition(conditions Conditions) (bool, error) {
	if conditions.Condition1 == "" && conditions.Condition2 == "" {
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

func evaluateStringCondition(conditionType string, conditions Conditions) (bool, error) {
	switch conditionType {
	case "is_equal":
		return conditions.Condition1 == conditions.Condition2, nil
	case "is_not_equal":
		return conditions.Condition1 != conditions.Condition2, nil
	case "contains":
		return strings.Contains(conditions.Condition1, conditions.Condition2), nil
	case "does_not_contain":
		return !strings.Contains(conditions.Condition1, conditions.Condition2), nil
	case "starts_with":
		return strings.HasPrefix(conditions.Condition1, conditions.Condition2), nil
	case "ends_with":
		return strings.HasSuffix(conditions.Condition1, conditions.Condition2), nil
	case "does_not_start_with":
		return !strings.HasPrefix(conditions.Condition1, conditions.Condition2), nil
	case "does_not_end_with":
		return !strings.HasSuffix(conditions.Condition1, conditions.Condition2), nil
	case "is_empty":
		return conditions.Condition1 == "", nil
	case "is_not_empty":
		return conditions.Condition1 != "", nil
	case "matches_regex":
		matched, err := regexp.MatchString(conditions.Condition2, conditions.Condition1)
		if err != nil {
			return false, fmt.Errorf("regex parse error in matches_regex: %w", err)
		}
		return matched, nil
	case "does_not_match_regex":
		matched, err := regexp.MatchString(conditions.Condition2, conditions.Condition1)
		if err != nil {
			return false, fmt.Errorf("regex parse error in does_not_match_regex: %w", err)
		}
		return !matched, nil
	default:
		return false, fmt.Errorf("unknown string condition type: %s", conditionType)
	}
}

func evaluateNumberCondition(conditionType string, conditions Conditions) (bool, error) {
	switch conditionType {
	case "exists":
		return conditions.Condition1 != "", nil
	case "does_not_exist":
		return conditions.Condition1 == "", nil
	}

	num1, err1 := parseFloat64(conditions.Condition1)
	num2, err2 := parseFloat64(conditions.Condition2)

	if err1 != nil {
		return false, fmt.Errorf("failed to parse condition1 as number '%s': %w", conditions.Condition1, err1)
	}
	if err2 != nil {
		return false, fmt.Errorf("failed to parse condition2 as number '%s': %w", conditions.Condition2, err2)
	}

	switch conditionType {
	case "is_equal":
		return num1 == num2, nil
	case "is_not_equal":
		return num1 != num2, nil
	case "is_greater_than":
		return num1 > num2, nil
	case "is_less_than":
		return num1 < num2, nil
	case "is_greater_than_or_equal":
		return num1 >= num2, nil
	case "is_less_than_or_equal":
		return num1 <= num2, nil
	default:
		return false, fmt.Errorf("unknown number condition type: %s", conditionType)
	}
}

func evaluateBooleanCondition(conditionType string, conditions Conditions) (bool, error) {
	switch conditionType {
	case "is_equal":
		return conditions.Condition1 == conditions.Condition2, nil
	case "is_not_equal":
		return conditions.Condition1 != conditions.Condition2, nil
	case "is_true":
		return strings.ToLower(conditions.Condition1) == "true", nil
	case "is_false":
		return strings.ToLower(conditions.Condition1) == "false", nil
	default:
		return false, fmt.Errorf("unknown boolean condition type: %s", conditionType)
	}
}

func evaluateDateCondition(conditionType string, conditions Conditions) (bool, error) {
	switch conditionType {
	case "exists":
		return conditions.Condition1 != "", nil
	case "does_not_exist":
		return conditions.Condition1 == "", nil
	}

	date1, err1 := time.Parse(time.RFC3339, conditions.Condition1)
	date2, err2 := time.Parse(time.RFC3339, conditions.Condition2)

	if err1 != nil {
		return false, fmt.Errorf("failed to parse condition1 as date '%s': %w", conditions.Condition1, err1)
	}
	if err2 != nil {
		return false, fmt.Errorf("failed to parse condition2 as date '%s': %w", conditions.Condition2, err2)
	}

	switch conditionType {
	case "is_equal":
		return date1.Equal(date2), nil
	case "is_not_equal":
		return !date1.Equal(date2), nil
	case "is_after":
		return date1.After(date2), nil
	case "is_before":
		return date1.Before(date2), nil
	case "is_after_or_equal":
		return date1.After(date2) || date1.Equal(date2), nil
	case "is_before_or_equal":
		return date1.Before(date2) || date1.Equal(date2), nil
	default:
		return false, fmt.Errorf("unknown date condition type: %s", conditionType)
	}
}

func evaluateArrayCondition(conditionType string, conditions Conditions) (bool, error) {
	switch conditionType {
	case "does_not_exist":
		var arr []interface{}
		err := json.Unmarshal([]byte(conditions.Condition1), &arr)
		return err != nil, nil
	}

	var arr []interface{}
	err := json.Unmarshal([]byte(conditions.Condition1), &arr)
	if err != nil {
		return false, fmt.Errorf("failed to parse condition1 as JSON array '%s': %w", conditions.Condition1, err)
	}

	switch conditionType {
	case "exists":
		return true, nil
	case "is_empty":
		return len(arr) == 0, nil
	case "is_not_empty":
		return len(arr) > 0, nil
	case "contains":
		return arrayContains(arr, conditions.Condition2), nil
	case "does_not_contain":
		return !arrayContains(arr, conditions.Condition2), nil
	case "length_equals":
		targetLen, err := parseFloat64(conditions.Condition2)
		if err != nil {
			return false, fmt.Errorf("failed to parse length value '%s': %w", conditions.Condition2, err)
		}
		return float64(len(arr)) == targetLen, nil
	case "length_greater_than":
		targetLen, err := parseFloat64(conditions.Condition2)
		if err != nil {
			return false, fmt.Errorf("failed to parse length value '%s': %w", conditions.Condition2, err)
		}
		return float64(len(arr)) > targetLen, nil
	case "length_less_than":
		targetLen, err := parseFloat64(conditions.Condition2)
		if err != nil {
			return false, fmt.Errorf("failed to parse length value '%s': %w", conditions.Condition2, err)
		}
		return float64(len(arr)) < targetLen, nil
	default:
		return false, fmt.Errorf("unknown array condition type: %s", conditionType)
	}
}

func evaluateObjectCondition(conditionType string, condition Conditions) (bool, error) {
	switch conditionType {
	case "does_not_exist":
		var obj map[string]interface{}
		err := json.Unmarshal([]byte(condition.Condition1), &obj)
		return err != nil, nil
	}

	var obj map[string]interface{}
	err := json.Unmarshal([]byte(condition.Condition1), &obj)
	if err != nil {
		return false, fmt.Errorf("failed to parse condition1 as JSON object '%s': %w", condition.Condition1, err)
	}

	switch conditionType {
	case "exists":
		return true, nil
	case "has_key":
		_, exists := obj[condition.Condition2]
		return exists, nil
	case "does_not_have_key":
		_, exists := obj[condition.Condition2]
		return !exists, nil
	case "key_equals":
		parts := strings.SplitN(condition.Condition2, ":", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("key_equals condition requires format 'key:value', got: %s", condition.Condition2)
		}
		val, exists := obj[parts[0]]
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
	case "key_not_equals":
		parts := strings.SplitN(condition.Condition2, ":", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("key_not_equals condition requires format 'key:value', got: %s", condition.Condition2)
		}
		val, exists := obj[parts[0]]
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
		return false, fmt.Errorf("unknown object condition type: %s", conditionType)
	}
}

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
