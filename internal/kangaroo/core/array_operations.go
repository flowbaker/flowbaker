package core

import (
	"fmt"
	"math"

	"github.com/flowbaker/flowbaker/internal/kangaroo/types"

	"github.com/dop251/goja/ast"
	"github.com/rs/zerolog/log"
)

// DefaultArrayOperations provides default implementation of ArrayOperations
type DefaultArrayOperations struct {
	executor *ASTExecutor
}

// NewDefaultArrayOperations creates a new default array operations
func NewDefaultArrayOperations(executor *ASTExecutor) *DefaultArrayOperations {
	return &DefaultArrayOperations{
		executor: executor,
	}
}

// Filter filters array elements using arrow function callback
func (ao *DefaultArrayOperations) Filter(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) ([]interface{}, error) {
	log.Debug().
		Int("arrayLength", len(array)).
		Str("callbackNodeType", fmt.Sprintf("%T", callbackNode)).
		Msg("Filter: Starting filter operation")

	arrowFn, ok := callbackNode.(*ast.ArrowFunctionLiteral)
	if !ok {
		log.Error().Str("nodeType", fmt.Sprintf("%T", callbackNode)).Msg("Filter: Node is not an arrow function")
		return nil, fmt.Errorf("filter requires an arrow function")
	}

	log.Debug().Int("paramCount", len(arrowFn.ParameterList.List)).Msg("Filter: Arrow function validated")

	var result []interface{}

	for _, item := range array {
		// Create new context with current item bound to parameter
		itemContext := &types.ExpressionContext{
			Item:      context.Item,
			Variables: make(map[string]interface{}),
		}

		// Copy existing variables
		for k, v := range context.Variables {
			itemContext.Variables[k] = v
		}

		// Bind arrow function parameter to current array item
		if len(arrowFn.ParameterList.List) > 0 {
			if ident, ok := arrowFn.ParameterList.List[0].Target.(*ast.Identifier); ok {
				paramName := ident.Name.String()
				itemContext.Variables[paramName] = item
			}
		}

		// Execute arrow function body
		bodyResult, err := ao.executor.executeNode(arrowFn.Body, itemContext)
		if err != nil {
			log.Error().Err(err).Msg("Filter: Error executing arrow function body")
			return nil, err
		}

		log.Debug().
			Interface("item", item).
			Interface("bodyResult", bodyResult).
			Str("bodyResultType", fmt.Sprintf("%T", bodyResult)).
			Bool("isTruthy", ao.isTruthy(bodyResult)).
			Msg("Filter: Processing item")

		// Convert result to boolean
		if ao.isTruthy(bodyResult) {
			result = append(result, item)
		}
	}

	return result, nil
}

// Map maps array elements using arrow function callback
func (ao *DefaultArrayOperations) Map(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) ([]interface{}, error) {
	arrowFn, ok := callbackNode.(*ast.ArrowFunctionLiteral)
	if !ok {
		return nil, fmt.Errorf("map requires an arrow function")
	}

	result := make([]interface{}, len(array))

	for i, item := range array {
		// Create new context with current item bound to parameter
		itemContext := &types.ExpressionContext{
			Item:      context.Item,
			Variables: make(map[string]interface{}),
		}

		// Copy existing variables
		for k, v := range context.Variables {
			itemContext.Variables[k] = v
		}

		// Bind arrow function parameters
		if len(arrowFn.ParameterList.List) > 0 {
			if ident, ok := arrowFn.ParameterList.List[0].Target.(*ast.Identifier); ok {
				paramName := ident.Name.String()
				itemContext.Variables[paramName] = item
			}
		}
		if len(arrowFn.ParameterList.List) > 1 {
			if ident, ok := arrowFn.ParameterList.List[1].Target.(*ast.Identifier); ok {
				indexParamName := ident.Name.String()
				itemContext.Variables[indexParamName] = float64(i) // JavaScript uses numbers for indices
			}
		}

		// Execute arrow function body
		bodyResult, err := ao.executor.executeNode(arrowFn.Body, itemContext)
		if err != nil {
			return nil, err
		}

		result[i] = bodyResult
	}

	return result, nil
}

// Find finds first array element matching callback
func (ao *DefaultArrayOperations) Find(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) (interface{}, error) {
	arrowFn, ok := callbackNode.(*ast.ArrowFunctionLiteral)
	if !ok {
		return nil, fmt.Errorf("find requires an arrow function")
	}

	for _, item := range array {
		// Create new context with current item bound to parameter
		itemContext := &types.ExpressionContext{
			Item:      context.Item,
			Variables: make(map[string]interface{}),
		}

		// Copy existing variables
		for k, v := range context.Variables {
			itemContext.Variables[k] = v
		}

		// Bind arrow function parameter to current array item
		if len(arrowFn.ParameterList.List) > 0 {
			if ident, ok := arrowFn.ParameterList.List[0].Target.(*ast.Identifier); ok {
				paramName := ident.Name.String()
				itemContext.Variables[paramName] = item
			}
		}

		// Execute arrow function body
		bodyResult, err := ao.executor.executeNode(arrowFn.Body, itemContext)
		if err != nil {
			return nil, err
		}

		// Return first item that matches
		if ao.isTruthy(bodyResult) {
			return item, nil
		}
	}

	return nil, nil // Not found
}

// Some checks if any array element matches callback
func (ao *DefaultArrayOperations) Some(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) (bool, error) {
	arrowFn, ok := callbackNode.(*ast.ArrowFunctionLiteral)
	if !ok {
		return false, fmt.Errorf("some requires an arrow function")
	}

	for _, item := range array {
		// Create new context with current item bound to parameter
		itemContext := &types.ExpressionContext{
			Item:      context.Item,
			Variables: make(map[string]interface{}),
		}

		// Copy existing variables
		for k, v := range context.Variables {
			itemContext.Variables[k] = v
		}

		// Bind arrow function parameter to current array item
		if len(arrowFn.ParameterList.List) > 0 {
			if ident, ok := arrowFn.ParameterList.List[0].Target.(*ast.Identifier); ok {
				paramName := ident.Name.String()
				itemContext.Variables[paramName] = item
			}
		}

		// Execute arrow function body
		bodyResult, err := ao.executor.executeNode(arrowFn.Body, itemContext)
		if err != nil {
			return false, err
		}

		// Return true if any item matches
		if ao.isTruthy(bodyResult) {
			return true, nil
		}
	}

	return false, nil
}

// Every checks if all array elements match callback
func (ao *DefaultArrayOperations) Every(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) (bool, error) {
	arrowFn, ok := callbackNode.(*ast.ArrowFunctionLiteral)
	if !ok {
		return false, fmt.Errorf("every requires an arrow function")
	}

	for _, item := range array {
		// Create new context with current item bound to parameter
		itemContext := &types.ExpressionContext{
			Item:      context.Item,
			Variables: make(map[string]interface{}),
		}

		// Copy existing variables
		for k, v := range context.Variables {
			itemContext.Variables[k] = v
		}

		// Bind arrow function parameter to current array item
		if len(arrowFn.ParameterList.List) > 0 {
			if ident, ok := arrowFn.ParameterList.List[0].Target.(*ast.Identifier); ok {
				paramName := ident.Name.String()
				itemContext.Variables[paramName] = item
			}
		}

		// Execute arrow function body
		bodyResult, err := ao.executor.executeNode(arrowFn.Body, itemContext)
		if err != nil {
			return false, err
		}

		// Return false if any item doesn't match
		if !ao.isTruthy(bodyResult) {
			return false, nil
		}
	}

	return true, nil
}

// Reduce reduces array to single value using callback
func (ao *DefaultArrayOperations) Reduce(array []interface{}, callbackNode ast.Node, initialValue interface{}, context *types.ExpressionContext) (interface{}, error) {
	arrowFn, ok := callbackNode.(*ast.ArrowFunctionLiteral)
	if !ok {
		return nil, fmt.Errorf("reduce requires an arrow function")
	}

	accumulator := initialValue
	startIndex := 0

	// If no initial value provided, use first array element
	if initialValue == nil {
		if len(array) == 0 {
			return nil, fmt.Errorf("reduce of empty array with no initial value")
		}
		accumulator = array[0]
		startIndex = 1
	}

	for i := startIndex; i < len(array); i++ {
		item := array[i]

		// Create new context with current item bound to parameters
		itemContext := &types.ExpressionContext{
			Item:      context.Item,
			Variables: make(map[string]interface{}),
		}

		// Copy existing variables
		for k, v := range context.Variables {
			itemContext.Variables[k] = v
		}

		// Bind arrow function parameters (accumulator, currentValue, index)
		if len(arrowFn.ParameterList.List) > 0 {
			if ident, ok := arrowFn.ParameterList.List[0].Target.(*ast.Identifier); ok {
				accParamName := ident.Name.String()
				itemContext.Variables[accParamName] = accumulator
			}
		}
		if len(arrowFn.ParameterList.List) > 1 {
			if ident, ok := arrowFn.ParameterList.List[1].Target.(*ast.Identifier); ok {
				itemParamName := ident.Name.String()
				itemContext.Variables[itemParamName] = item
			}
		}
		if len(arrowFn.ParameterList.List) > 2 {
			if ident, ok := arrowFn.ParameterList.List[2].Target.(*ast.Identifier); ok {
				indexParamName := ident.Name.String()
				itemContext.Variables[indexParamName] = float64(i)
			}
		}

		// Execute arrow function body
		bodyResult, err := ao.executor.executeNode(arrowFn.Body, itemContext)
		if err != nil {
			return nil, err
		}

		accumulator = bodyResult
	}

	return accumulator, nil
}

// isTruthy converts a value to boolean using JavaScript truthiness rules
func (ao *DefaultArrayOperations) isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0 && !math.IsNaN(v)
	case int:
		return v != 0
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return true
	}
}
