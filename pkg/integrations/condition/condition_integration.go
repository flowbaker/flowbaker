package condition

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"fmt"
	"regexp"
	"strings"
	"time"
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

// Internal format structures
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

		// Evaluate conditions
		result := false
		for _, condition := range conditionParams.Conditions {
			conditionResult := EvaluateCondition(condition)
			if conditionParams.ConditionRelation == ConditionRelationAnd {
				result = result && conditionResult
			} else {
				result = result || conditionResult
			}
		}

		// Add item to the output based on the result
		if result {
			outputItems[0] = append(outputItems[0], item)
		} else {
			outputItems[1] = append(outputItems[1], item)
		}
	}

	// Prepare output payloads
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

func EvaluateCondition(conditions Conditions) bool {
	if conditions.Condition1 == "" && conditions.Condition2 == "" {
		return false
	}

	parts := strings.SplitN(conditions.ConditionType, ".", 2)

	var dataType, conditionType string
	if len(parts) == 2 {
		dataType = parts[0]
		conditionType = parts[1]
	} else {
		// Handle the case where there's no dot in the string
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
		return false
	}
}

func evaluateStringCondition(conditionType string, conditions Conditions) bool {
	switch conditionType {
	case "is_equal":
		return conditions.Condition1 == conditions.Condition2
	case "is_not_equal":
		return conditions.Condition1 != conditions.Condition2
	case "contains":
		return strings.Contains(conditions.Condition1, conditions.Condition2)
	case "does_not_contain":
		return !strings.Contains(conditions.Condition1, conditions.Condition2)
	case "starts_with":
		return strings.HasPrefix(conditions.Condition1, conditions.Condition2)
	case "ends_with":
		return strings.HasSuffix(conditions.Condition1, conditions.Condition2)
	case "does_not_start_with":
		return !strings.HasPrefix(conditions.Condition1, conditions.Condition2)
	case "does_not_end_with":
		return !strings.HasSuffix(conditions.Condition1, conditions.Condition2)
	case "is_empty":
		return conditions.Condition1 == ""
	case "is_not_empty":
		return conditions.Condition1 != ""
	case "matches_regex":
		matched, err := regexp.MatchString(conditions.Condition2, conditions.Condition1)
		if err != nil {
			return false
		}
		return matched
	case "does_not_match_regex":
		matched, err := regexp.MatchString(conditions.Condition2, conditions.Condition1)
		if err != nil {
			return false
		}
		return !matched
	default:
		return false
	}
}

func evaluateNumberCondition(conditionType string, conditions Conditions) bool {
	// Parse string values to float64 for number comparison
	num1, err1 := parseFloat64(conditions.Condition1)
	num2, err2 := parseFloat64(conditions.Condition2)

	switch conditionType {
	case "exists":
		return conditions.Condition1 != ""
	case "does_not_exist":
		return conditions.Condition1 == ""
	}

	// If parsing failed and we're not checking existence, return false
	if err1 != nil || err2 != nil {
		return false
	}

	switch conditionType {
	case "is_equal":
		return num1 == num2
	case "is_not_equal":
		return num1 != num2
	case "is_greater_than":
		return num1 > num2
	case "is_less_than":
		return num1 < num2
	case "is_greater_than_or_equal":
		return num1 >= num2
	case "is_less_than_or_equal":
		return num1 <= num2
	default:
		return false
	}
}

func evaluateBooleanCondition(conditionType string, conditions Conditions) bool {
	switch conditionType {
	case "is_equal":
		return conditions.Condition1 == conditions.Condition2
	case "is_not_equal":
		return conditions.Condition1 != conditions.Condition2
	case "is_true":
		return strings.ToLower(conditions.Condition1) == "true"
	case "is_false":
		return strings.ToLower(conditions.Condition1) == "false"
	default:
		return false
	}
}

func evaluateDateCondition(conditionType string, conditions Conditions) bool {
	// Parse string values to time.Time for date comparison
	date1, err1 := time.Parse(time.RFC3339, conditions.Condition1)
	date2, err2 := time.Parse(time.RFC3339, conditions.Condition2)

	switch conditionType {
	case "exists":
		return conditions.Condition1 != ""
	case "does_not_exist":
		return conditions.Condition1 == ""
	}

	// If parsing failed and we're not checking existence, return false
	if err1 != nil || err2 != nil {
		return false
	}

	switch conditionType {
	case "is_equal":
		return date1.Equal(date2)
	case "is_not_equal":
		return !date1.Equal(date2)
	case "is_after":
		return date1.After(date2)
	case "is_before":
		return date1.Before(date2)
	case "is_after_or_equal":
		return date1.After(date2) || date1.Equal(date2)
	case "is_before_or_equal":
		return date1.Before(date2) || date1.Equal(date2)
	default:
		return false
	}
}

func evaluateArrayCondition(conditionType string, conditions Conditions) bool {
	var arr []interface{}
	err := json.Unmarshal([]byte(conditions.Condition1), &arr)
	if err != nil {
		// If it's not a valid JSON array, return false for all cases except does_not_exist
		return conditionType == "array does not exist"
	}

	switch conditionType {
	case "exists":
		return true
	case "does_not_exist":
		return false
	case "is_empty":
		return len(arr) == 0
	case "is_not_empty":
		return len(arr) > 0
	case "contains":
		return arrayContains(arr, conditions.Condition2)
	case "does_not_contain":
		return !arrayContains(arr, conditions.Condition2)
	case "length_equals":
		targetLen, err := parseFloat64(conditions.Condition2)
		if err != nil {
			return false
		}
		return float64(len(arr)) == targetLen
	case "length_greater_than":
		targetLen, err := parseFloat64(conditions.Condition2)
		if err != nil {
			return false
		}
		return float64(len(arr)) > targetLen
	case "length_less_than":
		targetLen, err := parseFloat64(conditions.Condition2)
		if err != nil {
			return false
		}
		return float64(len(arr)) < targetLen
	default:
		return false
	}
}

func evaluateObjectCondition(conditionType string, condition Conditions) bool {
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(condition.Condition1), &obj)
	if err != nil {
		// If it's not a valid JSON object, return false for all cases except does_not_exist
		return conditionType == "object does not exist"
	}

	switch conditionType {
	case "exists":
		return true
	case "does_not_exist":
		return false
	case "has_key":
		_, exists := obj[condition.Condition2]
		return exists
	case "does_not_have_key":
		_, exists := obj[condition.Condition2]
		return !exists
	case "key_equals":
		// Condition2 should be in format "key:value"
		parts := strings.SplitN(condition.Condition2, ":", 2)
		if len(parts) != 2 {
			return false
		}
		val, exists := obj[parts[0]]
		if !exists {
			return false
		}
		// Convert value to string for comparison
		valStr, ok := val.(string)
		if !ok {
			valBytes, err := json.Marshal(val)
			if err != nil {
				return false
			}
			valStr = string(valBytes)
		}
		return valStr == parts[1]
	case "key_not_equals":
		parts := strings.SplitN(condition.Condition2, ":", 2)
		if len(parts) != 2 {
			return false
		}
		val, exists := obj[parts[0]]
		if !exists {
			return true
		}
		// Convert value to string for comparison
		valStr, ok := val.(string)
		if !ok {
			valBytes, err := json.Marshal(val)
			if err != nil {
				return false
			}
			valStr = string(valBytes)
		}
		return valStr != parts[1]
	default:
		return false
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
