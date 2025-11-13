// Package core provides the core parsing functionality for the Kangaroo expression evaluator
package core

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"

	"github.com/flowbaker/flowbaker/pkg/expressions/kangaroo/types"
)

// ASTParser provides secure JavaScript expression parsing using Goja
type ASTParser struct {
	parseCache   sync.Map
	cacheSize    int64
	maxCacheSize int64
	mu           sync.RWMutex
}

// NewASTParser creates a new AST parser with caching
func NewASTParser() *ASTParser {
	return &ASTParser{
		parseCache:   sync.Map{},
		maxCacheSize: 1000,
	}
}

// Parse parses a JavaScript expression into an enhanced AST with metadata
func (p *ASTParser) Parse(expression string) (*types.ParsedExpression, error) {
	if expression == "" {
		return nil, fmt.Errorf("empty expression")
	}

	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return nil, fmt.Errorf("empty expression after trim")
	}

	// Check cache first
	cacheKey := p.getCacheKey(trimmed)
	if cached, ok := p.parseCache.Load(cacheKey); ok {
		if cached == nil {
			// Previously failed to parse - return cached error
			return nil, fmt.Errorf("expression previously failed to parse")
		}
		if parsedExpr, ok := cached.(*types.ParsedExpression); ok {
			return parsedExpr, nil
		}
	}

	// Parse the expression
	result, err := p.parseInternal(trimmed)
	if err != nil {
		// Cache the error result as nil
		p.setCached(cacheKey, nil)
		return nil, err
	}

	// Cache successful result
	p.setCached(cacheKey, result)

	return result, nil
}

// ParseTemplate parses multiple expressions from a template string
func (p *ASTParser) ParseTemplate(template string) ([]*types.TemplateMatch, error) {
	matches := p.ExtractTemplateExpressions(template)
	return matches, nil
}

// HasTemplateExpressions checks if a string contains template expressions
func (p *ASTParser) HasTemplateExpressions(text string) bool {
	// Use [\s\S] to explicitly match any character including newlines
	matched, _ := regexp.MatchString(`\{\{[\s\S]*?\}\}`, text)
	return matched
}

// ExtractTemplateExpressions extracts template expressions from a string
func (p *ASTParser) ExtractTemplateExpressions(template string) []*types.TemplateMatch {
	// Use [\s\S] to match any character including newlines (non-greedy with *?)
	re := regexp.MustCompile(`\{\{([\s\S]*?)\}\}`)
	matches := re.FindAllStringSubmatchIndex(template, -1)

	var expressions []*types.TemplateMatch
	for _, match := range matches {
		if len(match) >= 4 {
			fullMatch := template[match[0]:match[1]]
			expression := strings.TrimSpace(template[match[2]:match[3]])
			startIndex := match[0]
			endIndex := match[1]
			multiline := strings.Contains(expression, "\n")

			if expression != "" {
				expressions = append(expressions, &types.TemplateMatch{
					FullMatch:  fullMatch,
					Expression: expression,
					StartIndex: startIndex,
					EndIndex:   endIndex,
					Multiline:  multiline,
				})
			}
		}
	}

	return expressions
}

// ReplaceTemplateExpressions replaces template expressions with processed values
func ReplaceTemplateExpressions(template string, replacer func(expression string, match *types.TemplateMatch) string) string {
	parser := NewASTParser()
	matches := parser.ExtractTemplateExpressions(template)

	// Process matches in reverse order to maintain indices
	result := template
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		replacement := replacer(match.Expression, match)
		result = result[:match.StartIndex] + replacement + result[match.EndIndex:]
	}

	return result
}

// AnalyzeComplexity analyzes expression complexity
func (p *ASTParser) AnalyzeComplexity(expression string) (map[string]interface{}, error) {
	parsed, err := p.Parse(expression)
	if err != nil {
		return nil, err
	}

	breakdown := make(map[string]int)
	functionCalls := 0
	propertyAccesses := 0
	maxDepth := 0

	p.walkAST(parsed.AST, func(node ast.Node, depth int) {
		maxDepth = max(maxDepth, depth)

		nodeType := getNodeTypeName(node)
		breakdown[nodeType]++

		switch node.(type) {
		case *ast.CallExpression:
			functionCalls++
		case *ast.DotExpression, *ast.BracketExpression:
			propertyAccesses++
		}
	}, 0)

	estimatedTime := p.estimateExecutionTime(parsed.Complexity)

	return map[string]interface{}{
		"score":            parsed.Complexity,
		"breakdown":        breakdown,
		"maxDepth":         maxDepth,
		"functionCalls":    functionCalls,
		"propertyAccesses": propertyAccesses,
		"estimatedTime":    estimatedTime,
	}, nil
}

// ClearCache clears the parse cache
func (p *ASTParser) ClearCache() {
	p.parseCache = sync.Map{}
	p.mu.Lock()
	p.cacheSize = 0
	p.mu.Unlock()
}

// GetCacheStats returns cache statistics
func (p *ASTParser) GetCacheStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"size":    p.cacheSize,
		"maxSize": p.maxCacheSize,
	}
}

// parseInternal performs the actual parsing
func (p *ASTParser) parseInternal(expression string) (*types.ParsedExpression, error) {
	// Wrap expression to make it parseable as a complete program
	wrappedExpression := fmt.Sprintf("(%s)", expression)

	// Parse using Goja parser
	program, err := parser.ParseFile(nil, "", wrappedExpression, 0)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Extract the actual expression from the wrapped program
	exprNode, err := p.extractExpression(program)
	if err != nil {
		return nil, fmt.Errorf("failed to extract expression: %w", err)
	}

	// Analyze the expression
	dependencies := p.extractDependencies(exprNode)
	functions := p.extractFunctionCalls(exprNode)
	complexity := p.calculateComplexity(exprNode)
	isSimple := p.isSimpleExpression(exprNode)
	hasTemplates := p.HasTemplateExpressions(expression)
	depth := p.calculateDepth(exprNode)
	estimatedMemoryUsage := p.estimateMemoryUsage(exprNode)

	return &types.ParsedExpression{
		AST:                  exprNode,
		Dependencies:         dependencies,
		Functions:            functions,
		Complexity:           complexity,
		IsSimple:             isSimple,
		HasTemplates:         hasTemplates,
		Depth:                depth,
		EstimatedMemoryUsage: estimatedMemoryUsage,
	}, nil
}

// extractExpression extracts the actual expression node from a wrapped program
func (p *ASTParser) extractExpression(program *ast.Program) (ast.Node, error) {
	if len(program.Body) != 1 {
		return nil, fmt.Errorf("expected single statement")
	}

	stmt, ok := program.Body[0].(*ast.ExpressionStatement)
	if !ok {
		return nil, fmt.Errorf("expected expression statement")
	}

	return stmt.Expression, nil
}

// extractDependencies extracts variable dependencies from the AST
func (p *ASTParser) extractDependencies(node ast.Node) []string {
	dependencies := make(map[string]bool)

	p.walkAST(node, func(currentNode ast.Node, depth int) {
		if ident, ok := currentNode.(*ast.Identifier); ok {
			name := ident.Name.String()
			if p.isContextVariable(name) {
				dependencies[name] = true
			}
		}

		if dotExpr, ok := currentNode.(*ast.DotExpression); ok {
			if ident, ok := dotExpr.Left.(*ast.Identifier); ok {
				name := ident.Name.String()
				if p.isContextVariable(name) {
					dependencies[name] = true
				}
			}
		}
	}, 0)

	var result []string
	for dep := range dependencies {
		result = append(result, dep)
	}
	return result
}

// extractFunctionCalls extracts function calls from the AST
func (p *ASTParser) extractFunctionCalls(node ast.Node) []string {
	functions := make(map[string]bool)

	p.walkAST(node, func(currentNode ast.Node, depth int) {
		if callExpr, ok := currentNode.(*ast.CallExpression); ok {
			// Direct function call: func()
			if ident, ok := callExpr.Callee.(*ast.Identifier); ok {
				functions[ident.Name.String()] = true
			}

			// Method call: obj.method() or Namespace.method()
			if dotExpr, ok := callExpr.Callee.(*ast.DotExpression); ok {
				methodName := p.getMethodName(dotExpr)
				fullMethodName := p.getFullMethodName(dotExpr)

				if methodName != "" {
					functions[methodName] = true
				}
				if fullMethodName != "" && fullMethodName != methodName {
					functions[fullMethodName] = true
				}
			}
		}
	}, 0)

	var result []string
	for fn := range functions {
		result = append(result, fn)
	}
	return result
}

// calculateComplexity calculates expression complexity score
func (p *ASTParser) calculateComplexity(node ast.Node) float64 {
	complexity := 0.0

	p.walkAST(node, func(currentNode ast.Node, depth int) {
		switch currentNode.(type) {
		case *ast.CallExpression:
			complexity += 3.0 // Function calls are expensive
		case *ast.DotExpression, *ast.BracketExpression:
			complexity += 1.0
		case *ast.BinaryExpression:
			complexity += 1.0
		case *ast.ConditionalExpression:
			complexity += 4.0 // Ternary operators add branching complexity
		case *ast.ArrayLiteral:
			if arrLit, ok := currentNode.(*ast.ArrayLiteral); ok {
				complexity += 2.0 + float64(len(arrLit.Value))*0.5
			}
		case *ast.ObjectLiteral:
			if objLit, ok := currentNode.(*ast.ObjectLiteral); ok {
				complexity += 2.0 + float64(len(objLit.Value))*0.5
			}
		case *ast.ArrowFunctionLiteral:
			complexity += 5.0 // Arrow functions add significant complexity
		default:
			complexity += 0.5
		}
	}, 0)

	return complexity
}

// calculateDepth calculates maximum nesting depth
func (p *ASTParser) calculateDepth(node ast.Node) int {
	maxDepth := 0

	p.walkAST(node, func(currentNode ast.Node, depth int) {
		maxDepth = max(maxDepth, depth)
	}, 0)

	return maxDepth
}

// estimateMemoryUsage estimates memory usage for expression evaluation
func (p *ASTParser) estimateMemoryUsage(node ast.Node) int64 {
	estimatedBytes := int64(0)

	p.walkAST(node, func(currentNode ast.Node, depth int) {
		switch n := currentNode.(type) {
		case *ast.StringLiteral:
			estimatedBytes += int64(len(n.Value) * 2) // UTF-16
		case *ast.NumberLiteral, *ast.BooleanLiteral, *ast.NullLiteral:
			estimatedBytes += 8
		case *ast.ArrayLiteral:
			estimatedBytes += 64 // Base array overhead
		case *ast.ObjectLiteral:
			estimatedBytes += 128 // Base object overhead
		case *ast.CallExpression:
			estimatedBytes += 32 // Function call overhead
		default:
			estimatedBytes += 16 // General node overhead
		}
	}, 0)

	return estimatedBytes
}

// isSimpleExpression determines if expression is simple
func (p *ASTParser) isSimpleExpression(node ast.Node) bool {
	isSimple := true
	hasComplexOperation := false

	p.walkAST(node, func(currentNode ast.Node, depth int) {
		switch currentNode.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression,
			*ast.StringLiteral, *ast.NumberLiteral, *ast.BooleanLiteral, *ast.NullLiteral,
			*ast.BinaryExpression:
			// These are allowed in simple expressions
		case *ast.CallExpression, *ast.ConditionalExpression:
			hasComplexOperation = true
			isSimple = false
		default:
			isSimple = false
		}
	}, 0)

	return isSimple && !hasComplexOperation
}

// walkAST walks the AST and calls visitor function for each node
func (p *ASTParser) walkAST(node ast.Node, visitor func(ast.Node, int), depth int) {
	visitor(node, depth)

	// Walk child nodes based on node type
	switch n := node.(type) {
	case *ast.BinaryExpression:
		p.walkAST(n.Left, visitor, depth+1)
		p.walkAST(n.Right, visitor, depth+1)
	case *ast.ConditionalExpression:
		p.walkAST(n.Test, visitor, depth+1)
		p.walkAST(n.Consequent, visitor, depth+1)
		p.walkAST(n.Alternate, visitor, depth+1)
	case *ast.CallExpression:
		p.walkAST(n.Callee, visitor, depth+1)
		for _, arg := range n.ArgumentList {
			p.walkAST(arg, visitor, depth+1)
		}
	case *ast.DotExpression:
		p.walkAST(n.Left, visitor, depth+1)
	case *ast.BracketExpression:
		p.walkAST(n.Left, visitor, depth+1)
		p.walkAST(n.Member, visitor, depth+1)
	case *ast.ArrayLiteral:
		for _, elem := range n.Value {
			if elem != nil {
				p.walkAST(elem, visitor, depth+1)
			}
		}
	case *ast.ObjectLiteral:
		for _, prop := range n.Value {
			if keyedProp, ok := prop.(*ast.PropertyKeyed); ok {
				p.walkAST(keyedProp.Key, visitor, depth+1)
				p.walkAST(keyedProp.Value, visitor, depth+1)
			}
		}
	case *ast.UnaryExpression:
		p.walkAST(n.Operand, visitor, depth+1)
	}
}

// isContextVariable checks if identifier is a context variable
func (p *ASTParser) isContextVariable(name string) bool {
	contextVars := map[string]bool{
		"item":      true,
		"inputs":    true,
		"outputs":   true,
		"node":      true,
		"execution": true,
		// Built-in constants
		"true":      false,
		"false":     false,
		"null":      false,
		"undefined": false,
		"Infinity":  false,
		"NaN":       false,
	}

	if isContext, exists := contextVars[name]; exists {
		return isContext
	}
	return false
}

// getMethodName gets method name from dot expression
func (p *ASTParser) getMethodName(dotExpr *ast.DotExpression) string {
	return dotExpr.Identifier.Name.String()
}

// getFullMethodName gets full method name (e.g., "Math.round") from dot expression
func (p *ASTParser) getFullMethodName(dotExpr *ast.DotExpression) string {
	methodName := p.getMethodName(dotExpr)
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

// estimateExecutionTime estimates execution time based on complexity
func (p *ASTParser) estimateExecutionTime(complexity float64) float64 {
	result := complexity * 0.05
	if result < 0.1 {
		return 0.1
	}
	return result
}

// getCacheKey generates cache key for parsed expressions
func (p *ASTParser) getCacheKey(expression string) string {
	return expression
}

// setCached sets value in cache with size management
func (p *ASTParser) setCached(key string, value *types.ParsedExpression) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cacheSize >= p.maxCacheSize {
		// Simple cache eviction - in production, use LRU
		p.parseCache = sync.Map{}
		p.cacheSize = 0
	}

	p.parseCache.Store(key, value)
	p.cacheSize++
}

// getNodeTypeName returns the string representation of a node type
func getNodeTypeName(node ast.Node) string {
	switch node.(type) {
	case *ast.Identifier:
		return "Identifier"
	case *ast.StringLiteral:
		return "StringLiteral"
	case *ast.NumberLiteral:
		return "NumberLiteral"
	case *ast.BooleanLiteral:
		return "BooleanLiteral"
	case *ast.NullLiteral:
		return "NullLiteral"
	case *ast.BinaryExpression:
		return "BinaryExpression"
	case *ast.ConditionalExpression:
		return "ConditionalExpression"
	case *ast.CallExpression:
		return "CallExpression"
	case *ast.DotExpression:
		return "DotExpression"
	case *ast.BracketExpression:
		return "BracketExpression"
	case *ast.ArrayLiteral:
		return "ArrayLiteral"
	case *ast.ObjectLiteral:
		return "ObjectLiteral"
	case *ast.UnaryExpression:
		return "UnaryExpression"
	case *ast.ArrowFunctionLiteral:
		return "ArrowFunctionLiteral"
	case *ast.ExpressionBody:
		return "ExpressionBody"
	default:
		return "Unknown"
	}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
