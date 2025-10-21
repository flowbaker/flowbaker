package switchintegration

import (
	"context"
	"fmt"
	"reflect"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type SwitchIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewSwitchIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &SwitchIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *SwitchIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewSwitchIntegration(SwitchIntegrationDependencies{
		Binder: c.binder,
	})
}

type SwitchIntegration struct {
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
}

type SwitchIntegrationDependencies struct {
	Binder domain.IntegrationParameterBinder
}

type SwitchParams struct {
	ConditionType      string        `json:"condition_type"`
	ConditionString    string        `json:"condition_string,omitempty"`
	ConditionNumber    float64       `json:"condition_number,omitempty"`
	ConditionBoolean   bool          `json:"condition_boolean,omitempty"`
	ConditionDate      string        `json:"condition_date,omitempty"`
	ConditionArray     interface{}   `json:"condition_array,omitempty"`
	ConditionObject    interface{}   `json:"condition_object,omitempty"`
	ConditionDeepEqual interface{}   `json:"condition_deep_equal,omitempty"`
	Routes             []SwitchRoute `json:"routes"`
}

type SwitchRoute struct {
	Name           string      `json:"Name"`
	RouteString    string      `json:"route_string,omitempty"`
	RouteNumber    float64     `json:"route_number,omitempty"`
	RouteBoolean   bool        `json:"route_boolean,omitempty"`
	RouteDate      string      `json:"route_date,omitempty"`
	RouteArray     interface{} `json:"route_array,omitempty"`
	RouteObject    interface{} `json:"route_object,omitempty"`
	RouteDeepEqual interface{} `json:"route_deep_equal,omitempty"`
}

func NewSwitchIntegration(deps SwitchIntegrationDependencies) (*SwitchIntegration, error) {
	integration := &SwitchIntegration{
		binder: deps.Binder,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemRoutable(IntegrationActionType_Switch, integration.Switch)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *SwitchIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	fmt.Println("Executing switch integration")
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *SwitchIntegration) Switch(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.RoutableOutput, error) {
	p := SwitchParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.RoutableOutput{}, err
	}

	if len(p.Routes) == 0 {
		return domain.RoutableOutput{}, fmt.Errorf("no routes defined")
	}

	fmt.Println("Switch params: ", p)

	for routeIndex, route := range p.Routes {
		matches, err := i.evaluateCondition(p.ConditionType, p, route)
		if err != nil {
			return domain.RoutableOutput{}, err
		}

		if matches {
			enhancedItem := make(map[string]interface{})

			if itemMap, ok := item.(map[string]interface{}); ok {
				for k, v := range itemMap {
					enhancedItem[k] = v
				}
			} else {
				enhancedItem["original_item"] = item
			}

			enhancedItem["switch_result"] = map[string]interface{}{
				"matched_route":       route.Name,
				"matched_route_index": routeIndex,
				"condition_type":      p.ConditionType,
			}

			switch p.ConditionType {
			case "string":
				enhancedItem["switch_result"].(map[string]interface{})["matched_value"] = route.RouteString
			case "number":
				enhancedItem["switch_result"].(map[string]interface{})["matched_value"] = route.RouteNumber
			case "boolean":
				enhancedItem["switch_result"].(map[string]interface{})["matched_value"] = route.RouteBoolean
			case "date":
				enhancedItem["switch_result"].(map[string]interface{})["matched_value"] = route.RouteDate
			case "array":
				enhancedItem["switch_result"].(map[string]interface{})["matched_value"] = route.RouteArray
			case "object":
				enhancedItem["switch_result"].(map[string]interface{})["matched_value"] = route.RouteObject
			case "deep_equal":
				enhancedItem["switch_result"].(map[string]interface{})["matched_value"] = route.RouteDeepEqual
			}

			return domain.RoutableOutput{
				Item:        enhancedItem,
				OutputIndex: routeIndex,
			}, nil
		}
	}

	return domain.RoutableOutput{}, fmt.Errorf("no matching route found for input value")
}

func (i *SwitchIntegration) evaluateCondition(conditionType string, params SwitchParams, route SwitchRoute) (bool, error) {
	switch conditionType {
	case "string":
		return i.evaluateStringCondition(params.ConditionString, route.RouteString)
	case "number":
		return i.evaluateNumberCondition(params.ConditionNumber, route.RouteNumber)
	case "boolean":
		return i.evaluateBooleanCondition(params.ConditionBoolean, route.RouteBoolean)
	case "date":
		return i.evaluateDateCondition(params.ConditionDate, route.RouteDate)
	case "array":
		return i.evaluateArrayCondition(params.ConditionArray, route.RouteArray)
	case "object":
		return i.evaluateObjectCondition(params.ConditionObject, route.RouteObject)
	case "deep_equal":
		return i.evaluateDeepEqualCondition(params.ConditionDeepEqual, route.RouteDeepEqual)
	default:
		return false, fmt.Errorf("unsupported condition type: %s", conditionType)
	}
}

func (i *SwitchIntegration) evaluateStringCondition(inputValue interface{}, routeValue string) (bool, error) {
	inputStr, ok := inputValue.(string)
	if !ok {
		return false, fmt.Errorf("input value is not a string")
	}

	return inputStr == routeValue, nil
}

func (i *SwitchIntegration) evaluateNumberCondition(inputValue interface{}, routeValue float64) (bool, error) {
	var inputNum float64

	switch v := inputValue.(type) {
	case float64:
		inputNum = v
	case float32:
		inputNum = float64(v)
	case int:
		inputNum = float64(v)
	case int64:
		inputNum = float64(v)
	case int32:
		inputNum = float64(v)
	default:
		return false, fmt.Errorf("input value is not a number")
	}

	return inputNum == routeValue, nil
}

func (i *SwitchIntegration) evaluateBooleanCondition(inputValue interface{}, routeValue bool) (bool, error) {
	inputBool, ok := inputValue.(bool)
	if !ok {
		return false, fmt.Errorf("input value is not a boolean")
	}

	return inputBool == routeValue, nil
}

func (i *SwitchIntegration) evaluateDateCondition(inputValue interface{}, routeValue string) (bool, error) {
	inputDate, ok := inputValue.(string)
	if !ok {
		return false, fmt.Errorf("input value is not a string")
	}

	return inputDate == routeValue, nil
}

func (i *SwitchIntegration) evaluateArrayCondition(inputValue, routeValue interface{}) (bool, error) {
	return reflect.DeepEqual(inputValue, routeValue), nil
}

func (i *SwitchIntegration) evaluateObjectCondition(inputValue, routeValue interface{}) (bool, error) {
	return reflect.DeepEqual(inputValue, routeValue), nil
}

func (i *SwitchIntegration) evaluateDeepEqualCondition(inputValue, routeValue interface{}) (bool, error) {
	return reflect.DeepEqual(inputValue, routeValue), nil
}
