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
	ConditionType    string  `json:"condition_type"`
	ConditionString  string  `json:"condition_string,omitempty"`
	ConditionNumber  float64 `json:"condition_number,omitempty"`
	ConditionBoolean bool    `json:"condition_boolean,omitempty"`
	ConditionDate    string  `json:"condition_date,omitempty"`
	// TODO: Handle data format for array
	ConditionArray interface{} `json:"condition_array,omitempty"`
	// TODO: Handle data format for object
	ConditionObject interface{} `json:"condition_object,omitempty"`
	// TODO: Handle data format for deep equal
	ConditionDeepEqual interface{}   `json:"condition_deep_equal,omitempty"`
	Routes             []SwitchRoute `json:"routes"`
}

type SwitchRoute struct {
	Name             string          `json:"Name"`
	RouteString      string          `json:"route_string,omitempty"`
	QueryTypeString  QueryTypeString `json:"query_type_string,omitempty"`
	RouteNumber      float64         `json:"route_number,omitempty"`
	QueryTypeNumber  QueryTypeNumber `json:"query_type_number,omitempty"`
	RouteBoolean     bool            `json:"route_boolean,omitempty"`
	QueryTypeBoolean string          `json:"query_type_boolean,omitempty"`
	RouteDate        string          `json:"route_date,omitempty"`
	QueryTypeDate    string          `json:"query_type_date,omitempty"`
	// TODO: Handle data format for array
	RouteArray     interface{} `json:"route_array,omitempty"`
	QueryTypeArray string      `json:"query_type_array,omitempty"`
	// TODO: Handle data format for object
	RouteObject     interface{} `json:"route_object,omitempty"`
	QueryTypeObject string      `json:"query_type_object,omitempty"`
	// TODO: Handle data format for deep equal
	RouteDeepEqual     interface{} `json:"route_deep_equal,omitempty"`
	QueryTypeDeepEqual string      `json:"query_type_deep_equal,omitempty"`
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
			enhancedItem := make(map[string]any)

			for k, v := range item.(map[string]any) {
				enhancedItem[k] = v
			}

			enhancedItem["switch_result"] = map[string]any{
				"matched_route":       route.Name,
				"matched_route_index": routeIndex,
				"condition_type":      p.ConditionType,
			}

			switch p.ConditionType {
			case "string":
				enhancedItem["switch_result"].(map[string]any)["matched_value"] = route.RouteString
			case "number":
				enhancedItem["switch_result"].(map[string]any)["matched_value"] = route.RouteNumber
			case "boolean":
				enhancedItem["switch_result"].(map[string]any)["matched_value"] = route.RouteBoolean
			case "date":
				enhancedItem["switch_result"].(map[string]any)["matched_value"] = route.RouteDate
			case "array":
				enhancedItem["switch_result"].(map[string]any)["matched_value"] = route.RouteArray
			case "object":
				enhancedItem["switch_result"].(map[string]any)["matched_value"] = route.RouteObject
			case "deep_equal":
				enhancedItem["switch_result"].(map[string]any)["matched_value"] = route.RouteDeepEqual
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
		return i.evaluateStringCondition(params.ConditionString, route.RouteString, route.QueryTypeString)
	case "number":
		return i.evaluateNumberCondition(params.ConditionNumber, route.RouteNumber, route.QueryTypeNumber)
	case "boolean":
		return i.evaluateBooleanCondition(params.ConditionBoolean, route.RouteBoolean)
	case "date":
		return i.evaluateDateCondition(params.ConditionDate, route.RouteDate)
		// TODO: Handle data format for array
	case "array":
		return i.evaluateArrayCondition(params.ConditionArray.([]any), route.RouteArray.([]any))
		// TODO: Handle data format for object
	case "object":
		return i.evaluateObjectCondition(params.ConditionObject.(map[string]any), route.RouteObject.(map[string]any))
	case "deep_equal":
		return i.evaluateDeepEqualCondition(params.ConditionDeepEqual, route.RouteDeepEqual)
	default:
		return false, fmt.Errorf("unsupported condition type: %s", conditionType)
	}
}

func (i *SwitchIntegration) evaluateStringCondition(inputValue string, routeValue string, queryType QueryTypeString) (bool, error) {
	switch queryType {
	case QueryTypeString_Equals:
		return inputValue == routeValue, nil
	case QueryTypeString_NotEquals:
		return inputValue != routeValue, nil
	default:
		return false, fmt.Errorf("unsupported query type: %s", queryType)
	}
}

func (i *SwitchIntegration) evaluateNumberCondition(inputValue float64, routeValue float64, queryType QueryTypeNumber) (bool, error) {
	switch queryType {
	case QueryTypeNumber_Equals:
		return inputValue == routeValue, nil
	case QueryTypeNumber_NotEquals:
		return inputValue != routeValue, nil
	case QueryTypeNumber_GreaterThan:
		return inputValue > routeValue, nil
	case QueryTypeNumber_LessThan:
		return inputValue < routeValue, nil
	default:
		return false, fmt.Errorf("unsupported query type: %s", queryType)
	}
}

func (i *SwitchIntegration) evaluateBooleanCondition(inputValue bool, routeValue bool) (bool, error) {
	return inputValue == routeValue, nil
}

func (i *SwitchIntegration) evaluateDateCondition(inputValue string, routeValue string) (bool, error) {
	return inputValue == routeValue, nil
}

func (i *SwitchIntegration) evaluateArrayCondition(inputValue []any, routeValue []any) (bool, error) {
	return reflect.DeepEqual(inputValue, routeValue), nil
}

func (i *SwitchIntegration) evaluateObjectCondition(inputValue map[string]any, routeValue map[string]any) (bool, error) {
	return reflect.DeepEqual(inputValue, routeValue), nil
}

func (i *SwitchIntegration) evaluateDeepEqualCondition(inputValue any, routeValue any) (bool, error) {
	return reflect.DeepEqual(inputValue, routeValue), nil
}
