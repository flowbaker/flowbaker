// Package core provides the AST execution functionality for the Kangaroo expression evaluator
package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja/ast"
	"github.com/rs/zerolog/log"

	"github.com/flowbaker/flowbaker/pkg/expressions/kangaroo/types"
)

// ASTExecutor provides safe AST execution without eval()
type ASTExecutor struct {
	functionRegistry types.FunctionRegistry
	arrayOperations  ArrayOperations
	executionStack   []ExecutionStackFrame
	startTime        time.Time
	options          ExecutionOptions
	converter        *types.ValueConverter

	// Execution statistics
	stats struct {
		totalExecutions int64
		totalTime       int64
		errors          int64
	}
	mu sync.RWMutex
}

// ExecutionOptions configures AST execution behavior
type ExecutionOptions struct {
	Timeout         int64 // milliseconds
	MaxStackDepth   int
	CollectMetrics  bool
	EnableDebugging bool
	ErrorHandler    func(error, ast.Node, *types.ExpressionContext) (interface{}, error)
}

// ExecutionStackFrame represents a stack frame during execution
type ExecutionStackFrame struct {
	Node         ast.Node
	Depth        int
	StartTime    time.Time
	FunctionName string
}

// ArrayOperations interface for callback support
type ArrayOperations interface {
	Filter(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) ([]interface{}, error)
	Map(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) ([]interface{}, error)
	Find(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) (interface{}, error)
	Some(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) (bool, error)
	Every(array []interface{}, callbackNode ast.Node, context *types.ExpressionContext) (bool, error)
	Reduce(array []interface{}, callbackNode ast.Node, initialValue interface{}, context *types.ExpressionContext) (interface{}, error)
}

// NewASTExecutor creates a new AST executor
func NewASTExecutor(functionRegistry types.FunctionRegistry, options ExecutionOptions) *ASTExecutor {
	executor := &ASTExecutor{
		functionRegistry: functionRegistry,
		converter:        types.NewValueConverter(),
		options: ExecutionOptions{
			Timeout:         5000,
			MaxStackDepth:   50,
			CollectMetrics:  false,
			EnableDebugging: false,
		},
	}

	// Override with provided options
	if options.Timeout > 0 {
		executor.options.Timeout = options.Timeout
	}
	if options.MaxStackDepth > 0 {
		executor.options.MaxStackDepth = options.MaxStackDepth
	}
	executor.options.CollectMetrics = options.CollectMetrics
	executor.options.EnableDebugging = options.EnableDebugging
	executor.options.ErrorHandler = options.ErrorHandler

	// Initialize array operations
	executor.arrayOperations = NewDefaultArrayOperations(executor)

	return executor
}

// SetArrayOperations sets array operations implementation for callback support
func (e *ASTExecutor) SetArrayOperations(arrayOperations ArrayOperations) {
	e.arrayOperations = arrayOperations
}

// Execute executes an AST node safely and returns the result
func (e *ASTExecutor) Execute(node ast.Node, context *types.ExpressionContext) (*types.EvaluationResult, error) {
	e.startTime = time.Now()
	e.executionStack = []ExecutionStackFrame{}

	e.mu.Lock()
	e.stats.totalExecutions++
	e.mu.Unlock()

	defer func() {
		executionTime := time.Since(e.startTime).Microseconds()
		e.mu.Lock()
		e.stats.totalTime += executionTime
		e.mu.Unlock()
	}()

	// Check for timeout at start
	if err := e.checkTimeout(); err != nil {
		return &types.EvaluationResult{
			Success:   false,
			Error:     err.Error(),
			ErrorType: types.ErrorTypeTimeout,
		}, nil
	}

	value, err := e.executeNode(node, context)
	executionTime := time.Since(e.startTime).Microseconds()

	if err != nil {
		e.mu.Lock()
		e.stats.errors++
		e.mu.Unlock()

		// Use custom error handler if provided
		if e.options.ErrorHandler != nil {
			if handledResult, handlerErr := e.options.ErrorHandler(err, node, context); handlerErr == nil {
				return &types.EvaluationResult{
					Success: true,
					Value:   handledResult,
					Metadata: &types.Metadata{
						ExecutionTime: executionTime,
					},
				}, nil
			}
		}

		return &types.EvaluationResult{
			Success:   false,
			Error:     err.Error(),
			ErrorType: e.categorizeError(err),
			Metadata: &types.Metadata{
				ExecutionTime: executionTime,
			},
		}, nil
	}

	return &types.EvaluationResult{
		Success: true,
		Value:   value,
		Metadata: &types.Metadata{
			ExecutionTime:     executionTime,
			Complexity:        e.calculateNodeComplexity(node),
			AccessedVariables: e.extractAccessedVariables(node, context),
			CalledFunctions:   e.extractCalledFunctions(node),
		},
	}, nil
}

// GetStats returns execution statistics
func (e *ASTExecutor) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"totalExecutions": e.stats.totalExecutions,
		"totalTime":       e.stats.totalTime,
		"errors":          e.stats.errors,
	}
}

// ResetStats resets execution statistics
func (e *ASTExecutor) ResetStats() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.totalExecutions = 0
	e.stats.totalTime = 0
	e.stats.errors = 0
}

// executeNode executes a specific AST node based on its type
func (e *ASTExecutor) executeNode(node ast.Node, context *types.ExpressionContext) (interface{}, error) {

	// Check execution limits
	if err := e.checkTimeout(); err != nil {
		return nil, err
	}
	if err := e.checkStackDepth(); err != nil {
		return nil, err
	}

	// Push to execution stack
	stackFrame := ExecutionStackFrame{
		Node:      node,
		Depth:     len(e.executionStack),
		StartTime: time.Now(),
	}
	e.executionStack = append(e.executionStack, stackFrame)
	defer func() {
		e.executionStack = e.executionStack[:len(e.executionStack)-1]
	}()

	switch n := node.(type) {
	case *ast.StringLiteral:
		return e.converter.NormalizeValue(n.Value), nil
	case *ast.NumberLiteral:
		return n.Value, nil
	case *ast.BooleanLiteral:
		return n.Value, nil
	case *ast.NullLiteral:
		return nil, nil
	case *ast.Identifier:
		return e.executeIdentifier(n, context)
	case *ast.DotExpression:
		return e.executeDotExpression(n, context)
	case *ast.BracketExpression:
		return e.executeBracketExpression(n, context)
	case *ast.CallExpression:
		return e.executeCallExpression(n, context)
	case *ast.BinaryExpression:
		return e.executeBinaryExpression(n, context)
	case *ast.ConditionalExpression:
		return e.executeConditionalExpression(n, context)
	case *ast.UnaryExpression:
		return e.executeUnaryExpression(n, context)
	case *ast.ArrayLiteral:
		return e.executeArrayLiteral(n, context)
	case *ast.ObjectLiteral:
		return e.executeObjectLiteral(n, context)
	case *ast.ExpressionBody:
		return e.executeNode(n.Expression, context)
	default:
		return nil, fmt.Errorf("unsupported node type: %T", node)
	}
}

// executeIdentifier executes identifier nodes (variable references)
func (e *ASTExecutor) executeIdentifier(node *ast.Identifier, context *types.ExpressionContext) (interface{}, error) {
	name := node.Name.String()

	// Check context variables first
	if context != nil {
		switch name {
		case "item":
			return context.Item, nil
		case "items":
			return context.Items, nil
		default:
			if context.Variables != nil {
				if value, exists := context.Variables[name]; exists {
					return value, nil
				}
			}
		}
	}

	// Check built-in constants
	switch name {
	case "undefined":
		return nil, nil
	case "null":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "Infinity":
		return math.Inf(1), nil
	case "NaN":
		return math.NaN(), nil
	case "Math":
		// Return a simple object with only constants for property access
		return map[string]interface{}{
			"PI": math.Pi,
			"E":  math.E,
		}, nil
	default:
		// If identifier is not in context, it's undefined (JavaScript behavior)
		return nil, nil
	}
}

// executeDotExpression executes property access expressions (obj.prop)
func (e *ASTExecutor) executeDotExpression(node *ast.DotExpression, context *types.ExpressionContext) (interface{}, error) {
	object, err := e.executeNode(node.Left, context)
	if err != nil {
		return nil, err
	}

	if object == nil {
		return nil, nil
	}

	var property string
	property = node.Identifier.Name.String()

	return e.propertyAccess(object, property)
}

// executeBracketExpression executes bracket notation property access (obj[prop])
func (e *ASTExecutor) executeBracketExpression(node *ast.BracketExpression, context *types.ExpressionContext) (interface{}, error) {
	object, err := e.executeNode(node.Left, context)
	if err != nil {
		return nil, err
	}

	if object == nil {
		return nil, nil
	}

	property, err := e.executeNode(node.Member, context)
	if err != nil {
		return nil, err
	}

	log.Debug().Interface("property", property).Interface("object", object).Msg("bracket access")

	return e.propertyAccess(object, property)
}

// executeCallExpression executes function call expressions
func (e *ASTExecutor) executeCallExpression(node *ast.CallExpression, context *types.ExpressionContext) (interface{}, error) {
	if ident, ok := node.Callee.(*ast.Identifier); ok {
		// Direct function call: func()
		args := make([]interface{}, len(node.ArgumentList))
		for i, arg := range node.ArgumentList {
			val, err := e.executeNode(arg, context)
			if err != nil {
				return nil, err
			}
			args[i] = val
		}
		return e.executeFunction(ident.Name.String(), args, context)
	}

	if dotExpr, ok := node.Callee.(*ast.DotExpression); ok {
		// Method call: obj.method() or Namespace.method()
		methodName := e.getMethodName(dotExpr)
		fullMethodName := e.getFullMethodName(dotExpr)

		if methodName == "" {
			return nil, fmt.Errorf("invalid method call")
		}

		// Check if it's a qualified method like Object.keys() or Math.round()
		if fullMethodName != "" && e.functionRegistry.Has(fullMethodName) {
			args := make([]interface{}, len(node.ArgumentList))
			for i, arg := range node.ArgumentList {
				val, err := e.executeNode(arg, context)
				if err != nil {
					return nil, err
				}
				args[i] = val
			}
			return e.executeFunction(fullMethodName, args, context)
		}

		// Check if this is a callback method that needs special handling
		object, err := e.executeNode(dotExpr.Left, context)
		if err != nil {
			return nil, err
		}

		if e.isCallbackMethod(methodName) {
			if arr, ok := object.([]interface{}); ok {
				if e.arrayOperations == nil {
					return nil, fmt.Errorf("array operations not initialized")
				}

				// For callback methods, handle AST nodes specially
				if len(node.ArgumentList) == 0 {
					return nil, fmt.Errorf("%s requires a callback argument", methodName)
				}

				callbackArg := node.ArgumentList[0]

				// Check if the callback is an arrow function AST node
				if _, ok := callbackArg.(*ast.ArrowFunctionLiteral); ok {
					log.Debug().
						Str("methodName", methodName).
						Str("callbackType", fmt.Sprintf("%T", callbackArg)).
						Msg("Executor: Routing callback method")
					switch methodName {
					case "filter":
						log.Debug().Msg("Executor: Calling arrayOperations.Filter")
						return e.arrayOperations.Filter(arr, callbackArg, context)
					case "map":
						return e.arrayOperations.Map(arr, callbackArg, context)
					case "find":
						return e.arrayOperations.Find(arr, callbackArg, context)
					case "some":
						return e.arrayOperations.Some(arr, callbackArg, context)
					case "every":
						return e.arrayOperations.Every(arr, callbackArg, context)
					case "reduce":
						initialValue := interface{}(nil)
						if len(node.ArgumentList) > 1 {
							initialValue, err = e.executeNode(node.ArgumentList[1], context)
							if err != nil {
								return nil, err
							}
						}
						return e.arrayOperations.Reduce(arr, callbackArg, initialValue, context)
					default:
						return nil, fmt.Errorf("unsupported callback method: %s", methodName)
					}
				} else {
					return nil, fmt.Errorf("%s requires an arrow function callback", methodName)
				}
			}
		} else {
			// Regular method call - evaluate all arguments
			args := make([]interface{}, len(node.ArgumentList))
			for i, arg := range node.ArgumentList {
				val, err := e.executeNode(arg, context)
				if err != nil {
					return nil, err
				}
				args[i] = val
			}
			return e.executeMethod(object, methodName, args)
		}
	}

	return nil, fmt.Errorf("unsupported function call type")
}

// executeBinaryExpression executes binary expressions (+, -, *, /, ==, &&, ||, etc.)
func (e *ASTExecutor) executeBinaryExpression(node *ast.BinaryExpression, context *types.ExpressionContext) (interface{}, error) {
	left, err := e.executeNode(node.Left, context)
	if err != nil {
		return nil, err
	}

	// Handle short-circuiting operators
	operatorStr := node.Operator.String()
	if operatorStr == "&&" && !e.converter.ToBool(left) {
		return left, nil // Short-circuit
	}
	if operatorStr == "||" && e.converter.ToBool(left) {
		return left, nil // Short-circuit
	}

	right, err := e.executeNode(node.Right, context)
	if err != nil {
		return nil, err
	}

	switch node.Operator.String() {
	case "+":
		return e.addValues(left, right), nil
	case "-":
		leftNum, _ := e.converter.ToNumber(left)
		rightNum, _ := e.converter.ToNumber(right)
		return leftNum - rightNum, nil
	case "*":
		leftNum, _ := e.converter.ToNumber(left)
		rightNum, _ := e.converter.ToNumber(right)
		return leftNum * rightNum, nil
	case "/":
		rightNum, _ := e.converter.ToNumber(right)
		if rightNum == 0 {
			return math.Inf(1), nil // JavaScript behavior
		}
		leftNum, _ := e.converter.ToNumber(left)
		return leftNum / rightNum, nil
	case "%":
		leftNum, _ := e.converter.ToNumber(left)
		rightNum, _ := e.converter.ToNumber(right)
		return math.Mod(leftNum, rightNum), nil
	case "**":
		leftNum, _ := e.converter.ToNumber(left)
		rightNum, _ := e.converter.ToNumber(right)
		return math.Pow(leftNum, rightNum), nil
	case "==":
		return e.looseEquals(left, right), nil
	case "!=":
		return !e.looseEquals(left, right), nil
	case "===":
		return e.strictEquals(left, right), nil
	case "!==":
		return !e.strictEquals(left, right), nil
	case "<":
		return e.converter.CompareValues(left, right, "<")
	case "<=":
		return e.converter.CompareValues(left, right, "<=")
	case ">":
		return e.converter.CompareValues(left, right, ">")
	case ">=":
		return e.converter.CompareValues(left, right, ">=")
	case "&&":
		if !e.converter.ToBool(left) {
			return left, nil // Short-circuit
		}
		return right, nil
	case "||":
		if e.converter.ToBool(left) {
			return left, nil // Short-circuit
		}
		return right, nil
	case "??":
		if left != nil {
			return e.converter.NormalizeValue(left), nil // Nullish coalescing
		}
		return e.converter.NormalizeValue(right), nil
	case "in":
		return e.inOperator(left, right), nil
	default:
		return nil, fmt.Errorf("unsupported binary operator: %s", node.Operator.String())
	}
}

// executeLogicalExpression executes logical expressions (&&, ||, ??)

// executeConditionalExpression executes ternary expressions (test ? consequent : alternate)
func (e *ASTExecutor) executeConditionalExpression(node *ast.ConditionalExpression, context *types.ExpressionContext) (interface{}, error) {
	test, err := e.executeNode(node.Test, context)
	if err != nil {
		return nil, err
	}

	if e.converter.ToBool(test) {
		result, err := e.executeNode(node.Consequent, context)
		if err != nil {
			return nil, err
		}
		return e.converter.NormalizeValue(result), nil
	} else {
		result, err := e.executeNode(node.Alternate, context)
		if err != nil {
			return nil, err
		}
		return e.converter.NormalizeValue(result), nil
	}
}

// executeUnaryExpression executes unary expressions (+, -, !, typeof)
func (e *ASTExecutor) executeUnaryExpression(node *ast.UnaryExpression, context *types.ExpressionContext) (interface{}, error) {
	operand, err := e.executeNode(node.Operand, context)
	if err != nil {
		return nil, err
	}

	switch node.Operator.String() {
	case "+":
		operandNum, _ := e.converter.ToNumber(operand)
		return operandNum, nil
	case "-":
		operandNum, _ := e.converter.ToNumber(operand)
		return -operandNum, nil
	case "!":
		return !e.converter.ToBool(operand), nil
	case "typeof":
		return e.converter.GetJavaScriptType(operand), nil
	case "void":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported unary operator: %s", node.Operator.String())
	}
}

// executeArrayLiteral executes array literals ([1, 2, 3])
func (e *ASTExecutor) executeArrayLiteral(node *ast.ArrayLiteral, context *types.ExpressionContext) (interface{}, error) {
	result := make([]interface{}, len(node.Value))

	for i, element := range node.Value {
		if element != nil {
			val, err := e.executeNode(element, context)
			if err != nil {
				return nil, err
			}
			result[i] = val
		} else {
			result[i] = nil
		}
	}

	return result, nil
}

// executeObjectLiteral executes object literals ({key: value})
func (e *ASTExecutor) executeObjectLiteral(node *ast.ObjectLiteral, context *types.ExpressionContext) (interface{}, error) {
	result := make(map[string]interface{})

	for _, prop := range node.Value {
		var key string

		// Cast to PropertyKeyed to access Key and Value
		property, ok := prop.(*ast.PropertyKeyed)
		if !ok {
			return nil, fmt.Errorf("unsupported property type")
		}

		switch k := property.Key.(type) {
		case *ast.Identifier:
			key = k.Name.String()
		case *ast.StringLiteral:
			key = k.Value.String()
		case *ast.NumberLiteral:
			key = fmt.Sprintf("%g", k.Value)
		default:
			// Computed property name
			keyVal, err := e.executeNode(property.Key, context)
			if err != nil {
				return nil, err
			}
			key = e.converter.ToString(keyVal)
		}

		value, err := e.executeNode(property.Value, context)
		if err != nil {
			return nil, err
		}

		result[key] = value
	}

	return result, nil
}

// executeFunction executes a registered function
func (e *ASTExecutor) executeFunction(name string, args []interface{}, context *types.ExpressionContext) (interface{}, error) {
	fn, ok := e.functionRegistry.Get(name)
	if !ok {
		return nil, fmt.Errorf("function '%s' is not defined", name)
	}

	if err := e.validateFunctionCall(fn, args, false); err != nil {
		return nil, err
	}

	// Update stack frame with function name
	if len(e.executionStack) > 0 {
		e.executionStack[len(e.executionStack)-1].FunctionName = name
	}

	return fn.Fn(args...)
}

// executeMethod executes a method call on an object
func (e *ASTExecutor) executeMethod(object interface{}, methodName string, args []interface{}) (interface{}, error) {
	fn, ok := e.functionRegistry.Get(methodName)
	if !ok {
		return nil, fmt.Errorf("method '%s' is not defined", methodName)
	}

	// For method calls, pass the object as the first argument
	allArgs := append([]interface{}{object}, args...)
	if err := e.validateFunctionCall(fn, allArgs, true); err != nil {
		return nil, err
	}

	return fn.Fn(allArgs...)
}

// Helper methods

func (e *ASTExecutor) validateFunctionCall(fn *types.SafeFunction, args []interface{}, isMethodCall bool) error {
	effectiveMinArgs := fn.MinArgs
	if isMethodCall && effectiveMinArgs > 0 {
		effectiveMinArgs = effectiveMinArgs - 1
	}

	effectiveMaxArgs := fn.MaxArgs

	// Check argument count
	if len(args) < effectiveMinArgs {
		return fmt.Errorf("function '%s' requires at least %d arguments, got %d", fn.Name, effectiveMinArgs, len(args))
	}

	if effectiveMaxArgs >= 0 && len(args) > effectiveMaxArgs {
		return fmt.Errorf("function '%s' accepts at most %d arguments, got %d", fn.Name, effectiveMaxArgs, len(args))
	}

	return nil
}

func (e *ASTExecutor) propertyAccess(object, property interface{}) (interface{}, error) {
	var result interface{}

	// Handle different object types
	switch obj := object.(type) {
	case []interface{}:
		propStr := e.converter.ToString(property)
		if propStr == "length" {
			result = float64(len(obj))
		} else if idx := e.converter.ToArrayIndex(property); idx >= 0 && idx < len(obj) {
			result = obj[idx]
		} else {
			result = nil
		}
	case map[string]interface{}:
		propStr := e.converter.ToString(property)
		result = obj[propStr]
	case string:
		propStr := e.converter.ToString(property)
		if propStr == "length" {
			result = float64(len(obj))
		} else if idx := e.converter.ToArrayIndex(property); idx >= 0 && idx < len(obj) {
			result = string(obj[idx])
		} else {
			result = nil
		}
	default:
		result = nil
	}

	return result, nil
}

func (e *ASTExecutor) isCallbackMethod(methodName string) bool {
	callbackMethods := map[string]bool{
		"filter": true,
		"map":    true,
		"find":   true,
		"some":   true,
		"every":  true,
		"reduce": true,
	}
	return callbackMethods[methodName]
}

func (e *ASTExecutor) getMethodName(dotExpr *ast.DotExpression) string {
	return dotExpr.Identifier.Name.String()
}

func (e *ASTExecutor) getFullMethodName(dotExpr *ast.DotExpression) string {
	methodName := e.getMethodName(dotExpr)
	if methodName == "" {
		return ""
	}

	if ident, ok := dotExpr.Left.(*ast.Identifier); ok {
		objectName := ident.Name.String()
		staticNamespaces := map[string]bool{
			"Object": true, "Math": true, "JSON": true, "Date": true,
			"Array": true, "Crypto": true, "String": true, "Number": true,
		}

		if staticNamespaces[objectName] {
			return fmt.Sprintf("%s.%s", objectName, methodName)
		}
	}

	return ""
}

func (e *ASTExecutor) checkTimeout() error {
	if e.options.Timeout > 0 {
		elapsed := time.Since(e.startTime).Milliseconds()
		if elapsed > e.options.Timeout {
			return fmt.Errorf("execution timeout after %dms", e.options.Timeout)
		}
	}
	return nil
}

func (e *ASTExecutor) checkStackDepth() error {
	if len(e.executionStack) >= e.options.MaxStackDepth {
		return fmt.Errorf("maximum stack depth exceeded (%d)", e.options.MaxStackDepth)
	}
	return nil
}

func (e *ASTExecutor) categorizeError(err error) string {
	message := strings.ToLower(err.Error())

	if strings.Contains(message, "timeout") {
		return types.ErrorTypeTimeout
	} else if strings.Contains(message, "blocked") || strings.Contains(message, "security") {
		return types.ErrorTypeSecurity
	} else if strings.Contains(message, "type") || strings.Contains(message, "argument") {
		return types.ErrorTypeType
	} else if strings.Contains(message, "syntax") || strings.Contains(message, "invalid") {
		return types.ErrorTypeSyntax
	} else {
		return types.ErrorTypeRuntime
	}
}

func (e *ASTExecutor) calculateNodeComplexity(node ast.Node) float64 {
	complexity := 0.0

	var walk func(ast.Node)
	walk = func(n ast.Node) {
		complexity += 0.5

		// Walk child nodes based on type
		switch curr := n.(type) {
		case *ast.BinaryExpression:
			walk(curr.Left)
			walk(curr.Right)
		case *ast.ConditionalExpression:
			walk(curr.Test)
			walk(curr.Consequent)
			walk(curr.Alternate)
		case *ast.CallExpression:
			walk(curr.Callee)
			for _, arg := range curr.ArgumentList {
				walk(arg)
			}
		case *ast.DotExpression:
			walk(curr.Left)
		case *ast.BracketExpression:
			walk(curr.Left)
			walk(curr.Member)
		case *ast.ArrayLiteral:
			for _, elem := range curr.Value {
				if elem != nil {
					walk(elem)
				}
			}
		case *ast.ObjectLiteral:
			for _, prop := range curr.Value {
				if property, ok := prop.(*ast.PropertyKeyed); ok {
					walk(property.Key)
					walk(property.Value)
				}
			}
		case *ast.UnaryExpression:
			walk(curr.Operand)
		}
	}

	walk(node)
	return complexity
}

func (e *ASTExecutor) extractAccessedVariables(node ast.Node, context *types.ExpressionContext) []string {
	variables := make(map[string]bool)

	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if ident, ok := n.(*ast.Identifier); ok {
			name := ident.Name.String()
			if context != nil {
				switch name {
				case "item":
					variables[name] = true
				default:
					if context.Variables != nil {
						if _, exists := context.Variables[name]; exists {
							variables[name] = true
						}
					}
				}
			}
		}

		// Walk child nodes (simplified version)
		switch curr := n.(type) {
		case *ast.BinaryExpression:
			walk(curr.Left)
			walk(curr.Right)
		case *ast.CallExpression:
			walk(curr.Callee)
			for _, arg := range curr.ArgumentList {
				walk(arg)
			}
		case *ast.DotExpression:
			walk(curr.Left)
		}
	}

	walk(node)

	var result []string
	for variable := range variables {
		result = append(result, variable)
	}
	return result
}

func (e *ASTExecutor) extractCalledFunctions(node ast.Node) []string {
	functions := make(map[string]bool)

	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if callExpr, ok := n.(*ast.CallExpression); ok {
			if ident, ok := callExpr.Callee.(*ast.Identifier); ok {
				functions[ident.Name.String()] = true
			}
		}

		// Walk child nodes (simplified)
		switch curr := n.(type) {
		case *ast.CallExpression:
			for _, arg := range curr.ArgumentList {
				walk(arg)
			}
		}
	}

	walk(node)

	var result []string
	for function := range functions {
		result = append(result, function)
	}
	return result
}

// Type conversion and comparison helpers

func (e *ASTExecutor) addValues(left, right interface{}) interface{} {
	// JavaScript-style addition (string concatenation vs numeric addition)
	leftType := e.converter.GetJavaScriptType(left)
	rightType := e.converter.GetJavaScriptType(right)

	if leftType == "string" || rightType == "string" {
		return e.converter.ToString(left) + e.converter.ToString(right)
	}

	leftNum, _ := e.converter.ToNumber(left)
	rightNum, _ := e.converter.ToNumber(right)
	return leftNum + rightNum
}

func (e *ASTExecutor) looseEquals(left, right interface{}) bool {
	// Simplified loose equality (==) implementation
	log.Debug().
		Interface("left", left).
		Interface("right", right).
		Str("leftType", fmt.Sprintf("%T", left)).
		Str("rightType", fmt.Sprintf("%T", right)).
		Msg("Executor: looseEquals comparison")

	if e.strictEquals(left, right) {
		log.Debug().Bool("result", true).Msg("Executor: strictEquals returned true")
		return true
	}

	// Type coercion rules (simplified)
	if left == nil && right == nil {
		return true
	}

	// Handle string type coercion between Go string and unistring.String
	// If both values can be converted to strings, do string comparison
	if e.converter.GetJavaScriptType(left) == "string" || e.converter.GetJavaScriptType(right) == "string" {
		leftStr := e.converter.ToString(left)
		rightStr := e.converter.ToString(right)
		result := leftStr == rightStr
		log.Debug().
			Str("leftStr", leftStr).
			Str("rightStr", rightStr).
			Bool("stringResult", result).
			Msg("Executor: string comparison result")
		return result
	}

	// Convert to same type for comparison
	result, _ := e.converter.CompareValues(left, right, "==")
	leftNum, _ := e.converter.ToNumber(left)
	rightNum, _ := e.converter.ToNumber(right)
	log.Debug().
		Float64("leftFloat", leftNum).
		Float64("rightFloat", rightNum).
		Bool("result", result).
		Msg("Executor: numeric comparison result")
	return result
}

func (e *ASTExecutor) strictEquals(left, right interface{}) bool {
	// Strict equality (===) implementation
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}

	return left == right
}

func (e *ASTExecutor) inOperator(left, right interface{}) bool {
	property := e.converter.ToString(left)

	switch obj := right.(type) {
	case map[string]interface{}:
		_, exists := obj[property]
		return exists
	case []interface{}:
		if index, err := strconv.Atoi(property); err == nil {
			return index >= 0 && index < len(obj)
		}
		return false
	default:
		return false
	}
}
