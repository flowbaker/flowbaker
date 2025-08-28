// Package kangaroo provides a secure JavaScript-like expression evaluator for Go
package kangaroo

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"flowbaker/internal/kangaroo/core"
	"flowbaker/internal/kangaroo/functions"
	"flowbaker/internal/kangaroo/types"
)

// Kangaroo is the main expression evaluator class
type Kangaroo struct {
	parser           *core.ASTParser
	functionRegistry *functions.DefaultFunctionRegistry
	options          *types.EvaluatorOptions

	// Performance tracking
	stats struct {
		totalEvaluations      int64
		successfulEvaluations int64
		failedEvaluations     int64
		averageExecutionTime  int64
	}

	mu sync.RWMutex
}

// NewKangaroo creates a new Kangaroo expression evaluator
func NewKangaroo(options *types.EvaluatorOptions) *Kangaroo {
	if options == nil {
		options = &types.EvaluatorOptions{
			MaxComplexity:   100,
			MaxDepth:        10,
			EnableDebugging: false,
			StrictMode:      true,
			Timeout:         5000,
			EnableCaching:   true,
			MaxCacheSize:    1000,
			CollectMetrics:  false,
		}
	}

	// Fill in defaults for missing options
	if options.MaxComplexity == 0 {
		options.MaxComplexity = 100
	}
	if options.MaxDepth == 0 {
		options.MaxDepth = 10
	}
	if options.Timeout == 0 {
		options.Timeout = 5000
	}
	if options.MaxCacheSize == 0 {
		options.MaxCacheSize = 1000
	}

	// Initialize core components
	functionRegistry := functions.NewDefaultFunctionRegistry()
	parser := core.NewASTParser()

	evaluator := &Kangaroo{
		parser:           parser,
		functionRegistry: functionRegistry,
		options:          options,
	}

	// Register custom functions
	for _, fn := range options.CustomFunctions {
		functionRegistry.Register(&fn)
	}

	return evaluator
}

// Evaluate evaluates an expression with the given context
func (k *Kangaroo) Evaluate(expression string, context *types.ExpressionContext) (*types.EvaluationResult, error) {
	startTime := time.Now()

	k.mu.Lock()
	k.stats.totalEvaluations++
	k.mu.Unlock()

	defer func() {
		executionTime := time.Since(startTime).Microseconds()
		k.updatePerformanceMetrics(executionTime)
	}()

	if expression == "" {
		return &types.EvaluationResult{
			Success: true,
			Value:   "",
		}, nil
	}

	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return &types.EvaluationResult{
			Success: true,
			Value:   "",
		}, nil
	}

	// Check if this is a template with {{}} expressions
	if k.parser.HasTemplateExpressions(trimmed) {
		return k.evaluateTemplate(trimmed, context)
	} else {
		// Try direct expression evaluation
		result, err := k.evaluateExpression(trimmed, context)
		if err != nil {
			return nil, err
		}

		// If it failed due to syntax error and looks like plain text (not JavaScript), treat as literal string
		if !result.Success && result.ErrorType == types.ErrorTypeSyntax && k.looksLikePlainText(trimmed) {
			return &types.EvaluationResult{
				Success: true,
				Value:   trimmed,
			}, nil
		}

		return result, nil
	}
}

// looksLikePlainText determines if a string looks like plain text rather than malformed JavaScript
func (k *Kangaroo) looksLikePlainText(text string) bool {
	// If it contains programming constructs, it's likely intended to be JavaScript
	jsPatterns := []string{
		"(", ")", // Function calls
		".", "[", "]", // Property/array access
		"===", "==", "!=", "!==", // Comparison operators
		"&&", "||", // Logical operators
		"+", "-", "*", "/", "%", // Arithmetic operators (but only if not at start/end)
		"?", ":", // Ternary operator
		"{", "}", // Objects
		";",       // Statements
		"'", "\"", // String literals (quotes)
	}

	for _, pattern := range jsPatterns {
		if strings.Contains(text, pattern) {
			return false
		}
	}

	// If it's mostly letters, spaces, and basic punctuation, it's likely plain text
	return true
}

// Parse parses an expression and returns metadata
func (k *Kangaroo) Parse(expression string) (*types.ParsedExpression, error) {
	if expression == "" {
		return nil, fmt.Errorf("empty expression")
	}

	return k.parser.Parse(strings.TrimSpace(expression))
}

// ExtractDependencies extracts variable dependencies from expression
func (k *Kangaroo) ExtractDependencies(expression string) ([]string, error) {
	if expression == "" {
		return []string{}, nil
	}

	dependencies := make(map[string]bool)

	// Handle template expressions
	var expressions []string
	if k.parser.HasTemplateExpressions(expression) {
		matches := k.parser.ExtractTemplateExpressions(expression)
		expressions = make([]string, 0, len(matches))
		for _, match := range matches {
			expressions = append(expressions, match.Expression)
		}
	} else {
		expressions = []string{expression}
	}

	for _, expr := range expressions {
		parsed, err := k.parser.Parse(expr)
		if err != nil {
			continue // Skip invalid expressions
		}

		for _, dep := range parsed.Dependencies {
			dependencies[dep] = true
		}
	}

	var result []string
	for dep := range dependencies {
		result = append(result, dep)
	}

	return result, nil
}

// AddFunction adds a custom function to the registry
func (k *Kangaroo) AddFunction(fn *types.SafeFunction) {
	k.functionRegistry.Register(fn)
}

// RemoveFunction removes a function from the registry
func (k *Kangaroo) RemoveFunction(name string) {
	k.functionRegistry.Unregister(name)
}

// ListFunctions lists all available functions
func (k *Kangaroo) ListFunctions(category string) []*types.SafeFunction {
	return k.functionRegistry.List(category)
}

// GetPerformanceStats returns evaluator performance statistics
func (k *Kangaroo) GetPerformanceStats() map[string]interface{} {
	k.mu.RLock()
	defer k.mu.RUnlock()

	return map[string]interface{}{
		"totalEvaluations":      k.stats.totalEvaluations,
		"successfulEvaluations": k.stats.successfulEvaluations,
		"failedEvaluations":     k.stats.failedEvaluations,
		"averageExecutionTime":  k.stats.averageExecutionTime,
	}
}

// ResetStats resets all performance statistics
func (k *Kangaroo) ResetStats() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.stats.totalEvaluations = 0
	k.stats.successfulEvaluations = 0
	k.stats.failedEvaluations = 0
	k.stats.averageExecutionTime = 0

	k.parser.ClearCache()
}

// ClearCaches clears all caches
func (k *Kangaroo) ClearCaches() {
	k.parser.ClearCache()
}

// evaluateTemplate evaluates a template string with multiple expressions
func (k *Kangaroo) evaluateTemplate(template string, context *types.ExpressionContext) (*types.EvaluationResult, error) {
	var processedExpressions []types.ProcessedExpression

	result := core.ReplaceTemplateExpressions(template, func(expression string, match *types.TemplateMatch) string {
		evalResult, err := k.evaluateExpression(expression, context)
		if err != nil {
			return fmt.Sprintf("[ERROR: %s]", err.Error())
		}

		processedExpressions = append(processedExpressions, types.ProcessedExpression{
			Original:   expression,
			Evaluated:  evalResult.Value,
			StartIndex: match.StartIndex,
			EndIndex:   match.EndIndex,
		})

		if !evalResult.Success {
			return fmt.Sprintf("[ERROR: %s]", evalResult.Error)
		}

		if evalResult.Value == nil {
			return ""
		}

		return k.toString(evalResult.Value)
	})

	return &types.EvaluationResult{
		Success: true,
		Value:   result,
	}, nil
}

// evaluateExpression evaluates a single expression
func (k *Kangaroo) evaluateExpression(expression string, context *types.ExpressionContext) (*types.EvaluationResult, error) {
	// Parse expression
	parsed, err := k.parser.Parse(expression)
	if err != nil {
		k.mu.Lock()
		k.stats.failedEvaluations++
		k.mu.Unlock()

		return &types.EvaluationResult{
			Success:   false,
			Error:     fmt.Sprintf("Invalid syntax: %s", err.Error()),
			ErrorType: types.ErrorTypeSyntax,
		}, nil
	}

	// Check if parsed result is nil
	if parsed == nil {
		log.Error().Msg("CRITICAL: Parser returned nil parsed result")
		return &types.EvaluationResult{
			Success:   false,
			Error:     "parser returned nil result",
			ErrorType: types.ErrorTypeRuntime,
		}, nil
	}

	// Check complexity - add defensive nil check
	if k.options == nil {
		log.Error().Msg("CRITICAL: Kangaroo evaluator options is nil - this should never happen")
		return &types.EvaluationResult{
			Success:   false,
			Error:     "evaluator options not initialized",
			ErrorType: types.ErrorTypeRuntime,
		}, nil
	}

	if parsed.Complexity > k.options.MaxComplexity {
		k.mu.Lock()
		k.stats.failedEvaluations++
		k.mu.Unlock()

		return &types.EvaluationResult{
			Success:   false,
			Error:     fmt.Sprintf("Expression too complex (%.1f > %.1f)", parsed.Complexity, k.options.MaxComplexity),
			ErrorType: types.ErrorTypeComplexity,
		}, nil
	}

	// Check depth
	if float64(parsed.Depth) > float64(k.options.MaxDepth) {
		k.mu.Lock()
		k.stats.failedEvaluations++
		k.mu.Unlock()

		return &types.EvaluationResult{
			Success:   false,
			Error:     fmt.Sprintf("Expression too deep (%d > %d)", parsed.Depth, k.options.MaxDepth),
			ErrorType: types.ErrorTypeComplexity,
		}, nil
	}

	// Create fresh executor for this evaluation (thread-safe)
	executionOptions := core.ExecutionOptions{
		Timeout:         k.options.Timeout,
		MaxStackDepth:   50,
		CollectMetrics:  k.options.CollectMetrics,
		EnableDebugging: k.options.EnableDebugging,
	}
	executor := core.NewASTExecutor(k.functionRegistry, executionOptions)

	// Execute expression
	result, err := executor.Execute(parsed.AST, context)
	if err != nil {
		return nil, err
	}

	if result.Success {
		k.mu.Lock()
		k.stats.successfulEvaluations++
		k.mu.Unlock()
	} else {
		k.mu.Lock()
		k.stats.failedEvaluations++
		k.mu.Unlock()
	}

	return result, nil
}

// updatePerformanceMetrics updates performance tracking
func (k *Kangaroo) updatePerformanceMetrics(executionTime int64) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.stats.totalEvaluations > 1 {
		totalTime := k.stats.averageExecutionTime * (k.stats.totalEvaluations - 1)
		k.stats.averageExecutionTime = (totalTime + executionTime) / k.stats.totalEvaluations
	} else {
		k.stats.averageExecutionTime = executionTime
	}
}

// toString converts a value to string representation
func (k *Kangaroo) toString(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Convenience functions for common use cases

// CreateEvaluator creates a new Kangaroo expression evaluator with default settings
func CreateEvaluator(options *types.EvaluatorOptions) *Kangaroo {
	return NewKangaroo(options)
}

// Evaluate is a convenience function for simple evaluation
func Evaluate(expression string, context *types.ExpressionContext, options *types.EvaluatorOptions) (*types.EvaluationResult, error) {
	evaluator := NewKangaroo(options)
	return evaluator.Evaluate(expression, context)
}

// IsTemplate checks if an expression is a template (contains {{}} syntax)
func IsTemplate(expression string) bool {
	parser := core.NewASTParser()
	return parser.HasTemplateExpressions(expression)
}
