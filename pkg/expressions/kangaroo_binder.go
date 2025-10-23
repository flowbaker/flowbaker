package expressions

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/pkg/expressions/kangaroo"
	"github.com/flowbaker/flowbaker/pkg/expressions/kangaroo/types"
	"github.com/rs/zerolog"
)

// KangarooBinder implements expression binding using local Kangaroo runtime
type KangarooBinder struct {
	evaluator      *kangaroo.Kangaroo
	exprRegex      *regexp.Regexp
	logger         zerolog.Logger
	defaultTimeout time.Duration
}

// KangarooBinderOptions configures the local Kangaroo binder
type KangarooBinderOptions struct {
	Logger          zerolog.Logger
	DefaultTimeout  time.Duration
	KangarooOptions *types.EvaluatorOptions
}

// DefaultLocalKangarooBinderOptions returns sensible defaults
func DefaultKangarooBinderOptions() KangarooBinderOptions {
	return KangarooBinderOptions{
		Logger:          zerolog.Nop(),
		DefaultTimeout:  5 * time.Second,
		KangarooOptions: types.DefaultEvaluatorOptions(),
	}
}

// NewKangarooBinder creates a new local Kangaroo expression binder
func NewKangarooBinder(opts KangarooBinderOptions) (*KangarooBinder, error) {
	if opts.DefaultTimeout == 0 {
		opts.DefaultTimeout = 5 * time.Second
	}
	if opts.KangarooOptions == nil {
		opts.KangarooOptions = types.DefaultEvaluatorOptions()
	}

	// Create local Kangaroo evaluator
	evaluator := kangaroo.NewKangaroo(opts.KangarooOptions)

	binder := &KangarooBinder{
		evaluator:      evaluator,
		exprRegex:      regexp.MustCompile(`\{\{(.*?)\}\}`),
		logger:         opts.Logger,
		defaultTimeout: opts.DefaultTimeout,
	}

	opts.Logger.Info().
		Dur("defaultTimeout", opts.DefaultTimeout).
		Msg("Local Kangaroo binder initialized successfully")

	return binder, nil
}

// BindToStructWithJSON binds expressions in userNodeSettings to the target struct using item data
// This method implements the IntegrationParameterBinder interface
func (b *KangarooBinder) BindToStructWithJSON(ctx context.Context, item any, target any, userNodeSettings map[string]any) error {
	return b.BindToStruct(ctx, item, target, userNodeSettings)
}

// BindToStruct binds Kangaroo expressions in userNodeSettings to the target struct using item data
func (b *KangarooBinder) BindToStruct(ctx context.Context, item any, target any, userNodeSettings map[string]any) error {
	// Validate inputs
	if err := b.validateInputs(target, userNodeSettings); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Process expressions recursively
	boundData, err := b.bindValue(ctx, item, userNodeSettings)
	if err != nil {
		return fmt.Errorf("binding failed: %w", err)
	}

	// Marshal to JSON then unmarshal to target struct
	jsonData, err := json.Marshal(boundData)
	if err != nil {
		return fmt.Errorf("failed to marshal bound data: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal to target struct: %w", err)
	}

	return nil
}

// BindString processes a string that may contain expressions and returns the result
func (b *KangarooBinder) BindString(ctx context.Context, item any, str string) (any, error) {
	return b.bindString(ctx, item, str)
}

func (b *KangarooBinder) BindValue(ctx context.Context, item any, value any) (any, error) {
	boundData, err := b.bindValue(ctx, item, value)
	if err != nil {
		return nil, fmt.Errorf("binding failed: %w", err)
	}

	jsonData, err := json.Marshal(boundData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bound data: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bound data: %w", err)
	}

	return result, nil
}

// Evaluate evaluates a single expression directly
func (b *KangarooBinder) Evaluate(expression string, context *types.ExpressionContext) (*types.EvaluationResult, error) {
	if b.evaluator == nil {
		return &types.EvaluationResult{
			Success:   false,
			Error:     "kangaroo evaluator is nil",
			ErrorType: "runtime",
		}, nil
	}
	return b.evaluator.Evaluate(expression, context)
}

// validateInputs ensures the inputs are valid before processing
func (b *KangarooBinder) validateInputs(target any, settings map[string]any) error {
	if target == nil || settings == nil {
		return fmt.Errorf("target and settings cannot be nil")
	}

	if reflect.ValueOf(target).Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	return nil
}

// bindValue recursively processes values and binds expressions
func (b *KangarooBinder) bindValue(ctx context.Context, item any, value any) (any, error) {
	switch v := value.(type) {
	case string:
		return b.bindString(ctx, item, v)
	case map[string]any:
		return b.bindMap(ctx, item, v)
	case []any:
		return b.bindSlice(ctx, item, v)
	default:
		// Return non-string values as-is
		return value, nil
	}
}

// bindString processes a string that may contain expressions
func (b *KangarooBinder) bindString(ctx context.Context, item any, str string) (any, error) {
	matches := b.exprRegex.FindAllStringSubmatch(str, -1)
	if len(matches) == 0 {
		return str, nil
	}

	// Check if entire string is a single expression
	if len(matches) == 1 && matches[0][0] == str {
		// Single expression - evaluate and convert to string for JSON compatibility
		expression := strings.TrimSpace(matches[0][1])
		value, err := b.evaluateExpression(ctx, item, expression)
		if err != nil {
			return nil, err
		}
		// Convert to string to ensure JSON unmarshaling compatibility
		return b.valueToString(value), nil
	}

	// Multiple expressions or mixed content - interpolate as string
	result := str
	for _, match := range matches {
		fullMatch := match[0]
		expression := strings.TrimSpace(match[1])

		value, err := b.evaluateExpression(ctx, item, expression)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression '%s': %w", expression, err)
		}

		// Convert value to string for interpolation
		strValue := b.valueToString(value)
		result = strings.ReplaceAll(result, fullMatch, strValue)
	}

	return result, nil
}

// bindMap recursively processes a map
func (b *KangarooBinder) bindMap(ctx context.Context, item any, m map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(m))

	for key, value := range m {
		boundValue, err := b.bindValue(ctx, item, value)
		if err != nil {
			return nil, fmt.Errorf("failed to bind key '%s': %w", key, err)
		}
		result[key] = boundValue
	}

	return result, nil
}

// bindSlice recursively processes a slice
func (b *KangarooBinder) bindSlice(ctx context.Context, item any, s []any) ([]any, error) {
	result := make([]any, len(s))

	for i, value := range s {
		boundValue, err := b.bindValue(ctx, item, value)
		if err != nil {
			return nil, fmt.Errorf("failed to bind index %d: %w", i, err)
		}
		result[i] = boundValue
	}

	return result, nil
}

// evaluateExpression evaluates a Kangaroo expression using the local runtime
func (b *KangarooBinder) evaluateExpression(ctx context.Context, item any, expression string) (any, error) {
	// Create execution context
	context := &types.ExpressionContext{
		Item: item,
	}

	// Evaluate expression directly
	if b.evaluator == nil {
		b.logger.Error().Msg("CRITICAL: KangarooBinder evaluator is nil")
		return nil, fmt.Errorf("kangaroo evaluator is nil")
	}
	result, err := b.evaluator.Evaluate(expression, context)
	if err != nil {
		b.logger.Warn().
			Err(err).
			Str("expression", expression).
			Msg("Kangaroo expression evaluation failed")
		return nil, fmt.Errorf("evaluation error: %w", err)
	}

	if !result.Success {
		b.logger.Warn().
			Str("error", result.Error).
			Str("expression", expression).
			Msg("Kangaroo expression evaluation failed")
		return nil, fmt.Errorf("evaluation failed: %s", result.Error)
	}

	return result.Value, nil
}

// valueToString converts any value to its string representation
func (b *KangarooBinder) valueToString(value any) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		// For complex types, use JSON representation
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(jsonBytes)
	}
}

// Close performs cleanup operations
func (b *KangarooBinder) Close() error {
	b.logger.Info().Msg("Local Kangaroo binder closed")
	return nil
}
