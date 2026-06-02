package expressions

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/flowbaker/flowbaker/pkg/expressions/kangaroo/types"
	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/ext"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type CELBinder struct {
	env            *cel.Env
	exprRegex      *regexp.Regexp
	logger         zerolog.Logger
	defaultTimeout time.Duration

	progCacheMu sync.RWMutex
	progCache   map[string]cel.Program
}

type CELBinderOptions struct {
	Logger         zerolog.Logger
	DefaultTimeout time.Duration
}

func DefaultCELBinderOptions() CELBinderOptions {
	return CELBinderOptions{
		Logger:         zerolog.Nop(),
		DefaultTimeout: 5 * time.Second,
	}
}

func NewCELBinder(opts CELBinderOptions) (*CELBinder, error) {
	if opts.DefaultTimeout == 0 {
		opts.DefaultTimeout = 5 * time.Second
	}

	envOpts := []cel.EnvOption{
		cel.Variable("item", cel.DynType),
		ext.Strings(),
		ext.Lists(),
	}
	envOpts = append(envOpts, customEnvOptions()...)

	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build CEL env: %w", err)
	}

	opts.Logger.Info().
		Dur("defaultTimeout", opts.DefaultTimeout).
		Msg("CEL binder initialized")

	return &CELBinder{
		env:            env,
		exprRegex:      regexp.MustCompile(`\{\{([\s\S]*?)\}\}`),
		logger:         opts.Logger,
		defaultTimeout: opts.DefaultTimeout,
		progCache:      make(map[string]cel.Program),
	}, nil
}

func (b *CELBinder) BindToStruct(ctx context.Context, item any, target any, settings map[string]any) error {
	if err := b.validateInputs(target, settings); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	bound, err := b.bindValue(ctx, item, settings)
	if err != nil {
		return fmt.Errorf("binding failed: %w", err)
	}

	raw, err := json.Marshal(bound)
	if err != nil {
		return fmt.Errorf("failed to marshal bound data: %w", err)
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("failed to unmarshal to target struct: %w", err)
	}

	return nil
}

func (b *CELBinder) BindToStructWithJSON(ctx context.Context, item any, target any, settings map[string]any) error {
	return b.BindToStruct(ctx, item, target, settings)
}

func (b *CELBinder) BindString(ctx context.Context, item any, str string) (any, error) {
	return b.bindString(ctx, item, str)
}

func (b *CELBinder) BindValue(ctx context.Context, item any, value any) (any, error) {
	bound, err := b.bindValue(ctx, item, value)
	if err != nil {
		return nil, fmt.Errorf("binding failed: %w", err)
	}

	raw, err := json.Marshal(bound)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bound data: %w", err)
	}

	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bound data: %w", err)
	}

	return out, nil
}

func (b *CELBinder) Evaluate(expression string, exprCtx *types.ExpressionContext) (*types.EvaluationResult, error) {
	start := time.Now()
	scope := scopeFromContext(exprCtx)

	value, err := b.evaluateExpression(context.Background(), scope, expression)
	elapsed := time.Since(start).Microseconds()

	if err != nil {
		return &types.EvaluationResult{
			Success:   false,
			Error:     err.Error(),
			ErrorType: "runtime",
			Metadata:  &types.Metadata{ExecutionTime: elapsed},
		}, nil
	}

	return &types.EvaluationResult{
		Success:  true,
		Value:    value,
		Metadata: &types.Metadata{ExecutionTime: elapsed},
	}, nil
}

func (b *CELBinder) Close() error {
	b.logger.Info().Msg("CEL binder closed")
	return nil
}

func (b *CELBinder) validateInputs(target any, settings map[string]any) error {
	if target == nil || settings == nil {
		return fmt.Errorf("target and settings cannot be nil")
	}

	if reflect.ValueOf(target).Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	return nil
}

func (b *CELBinder) bindValue(ctx context.Context, item any, value any) (any, error) {
	switch v := value.(type) {
	case string:
		return b.bindString(ctx, item, v)
	case map[string]any:
		return b.bindMap(ctx, item, v)
	case []any:
		return b.bindSlice(ctx, item, v)
	default:
		return value, nil
	}
}

func (b *CELBinder) bindString(ctx context.Context, item any, str string) (any, error) {
	matches := b.exprRegex.FindAllStringSubmatch(str, -1)
	if len(matches) == 0 {
		return str, nil
	}

	scope := scopeFromItem(item)

	if len(matches) == 1 && matches[0][0] == str {
		expression := strings.TrimSpace(matches[0][1])
		return b.evaluateExpression(ctx, scope, expression)
	}

	result := str
	for _, match := range matches {
		full := match[0]
		expr := strings.TrimSpace(match[1])

		value, err := b.evaluateExpression(ctx, scope, expr)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression '%s': %w", expr, err)
		}

		result = strings.ReplaceAll(result, full, valueToString(value))
	}

	return result, nil
}

func (b *CELBinder) bindMap(ctx context.Context, item any, m map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(m))
	for k, v := range m {
		bound, err := b.bindValue(ctx, item, v)
		if err != nil {
			return nil, fmt.Errorf("failed to bind key '%s': %w", k, err)
		}
		out[k] = bound
	}
	return out, nil
}

func (b *CELBinder) bindSlice(ctx context.Context, item any, s []any) ([]any, error) {
	out := make([]any, len(s))
	for i, v := range s {
		bound, err := b.bindValue(ctx, item, v)
		if err != nil {
			return nil, fmt.Errorf("failed to bind index %d: %w", i, err)
		}
		out[i] = bound
	}
	return out, nil
}

func (b *CELBinder) evaluateExpression(ctx context.Context, scope map[string]any, expression string) (any, error) {
	prog, err := b.programFor(expression)
	if err != nil {
		return nil, err
	}

	val, _, err := prog.ContextEval(ctx, scope)
	if err != nil {
		b.logger.Warn().
			Err(err).
			Str("expression", expression).
			Msg("CEL evaluation failed")
		return nil, fmt.Errorf("evaluation error: %w", err)
	}

	return celToGo(val), nil
}

func (b *CELBinder) programFor(expression string) (cel.Program, error) {
	b.progCacheMu.RLock()
	if prog, ok := b.progCache[expression]; ok {
		b.progCacheMu.RUnlock()
		return prog, nil
	}
	b.progCacheMu.RUnlock()

	ast, iss := b.env.Compile(expression)
	if iss != nil && iss.Err() != nil {
		return nil, fmt.Errorf("compile error: %w", iss.Err())
	}

	prog, err := b.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program error: %w", err)
	}

	b.progCacheMu.Lock()
	b.progCache[expression] = prog
	b.progCacheMu.Unlock()

	return prog, nil
}

func scopeFromContext(exprCtx *types.ExpressionContext) map[string]any {
	scope := make(map[string]any, 1)
	if exprCtx != nil {
		scope["item"] = normalize(exprCtx.Item)
		for k, v := range exprCtx.Variables {
			scope[k] = normalize(v)
		}
	}
	if _, ok := scope["item"]; !ok {
		scope["item"] = nil
	}
	return scope
}

func scopeFromItem(item any) map[string]any {
	return map[string]any{"item": normalize(item)}
}

func normalize(v any) any {
	if v == nil {
		return nil
	}
	switch v.(type) {
	case map[string]any, []any, string, bool, float64, int, int64, uint, uint64:
		return v
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}

func celToGo(v ref.Val) any {
	if v == nil {
		return nil
	}
	if _, isNull := v.(celtypes.Null); isNull {
		return nil
	}
	if m, ok := v.(traits.Mapper); ok {
		out := make(map[string]any)
		it := m.Iterator()
		for it.HasNext() == celtypes.True {
			k := it.Next()
			kv := celToGo(k)
			ks, isStr := kv.(string)
			if !isStr {
				ks = fmt.Sprintf("%v", kv)
			}
			vv, _ := m.Find(k)
			out[ks] = celToGo(vv)
		}
		return out
	}
	if l, ok := v.(traits.Lister); ok {
		size, _ := l.Size().Value().(int64)
		out := make([]any, size)
		for i := int64(0); i < size; i++ {
			out[i] = celToGo(l.Get(celtypes.Int(i)))
		}
		return out
	}
	out, err := v.ConvertToNative(reflect.TypeOf((*any)(nil)).Elem())
	if err == nil {
		return out
	}
	return v.Value()
}

func customEnvOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("capitalize",
			cel.MemberOverload("string_capitalize",
				[]*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(strCapitalize))),
		cel.Function("truncate",
			cel.MemberOverload("string_truncate_int",
				[]*cel.Type{cel.StringType, cel.IntType}, cel.StringType,
				cel.BinaryBinding(strTruncate))),
		cel.Function("first",
			cel.MemberOverload("list_first",
				[]*cel.Type{cel.ListType(cel.DynType)}, cel.DynType,
				cel.UnaryBinding(listFirst))),
		cel.Function("last",
			cel.MemberOverload("list_last",
				[]*cel.Type{cel.ListType(cel.DynType)}, cel.DynType,
				cel.UnaryBinding(listLast))),
		cel.Function("sum",
			cel.MemberOverload("list_sum",
				[]*cel.Type{cel.ListType(cel.DynType)}, cel.DoubleType,
				cel.UnaryBinding(listSum))),

		cel.Function("JSON.parse",
			cel.Overload("JSON.parse_string",
				[]*cel.Type{cel.StringType}, cel.DynType,
				cel.UnaryBinding(jsonParse))),
		cel.Function("JSON.stringify",
			cel.Overload("JSON.stringify_dyn",
				[]*cel.Type{cel.DynType}, cel.StringType,
				cel.UnaryBinding(jsonStringify))),

		cel.Function("Math.max",
			cel.Overload("Math.max_dyn_dyn",
				[]*cel.Type{cel.DynType, cel.DynType}, cel.DynType,
				cel.BinaryBinding(mathMaxBinary))),
		cel.Function("Math.min",
			cel.Overload("Math.min_dyn_dyn",
				[]*cel.Type{cel.DynType, cel.DynType}, cel.DynType,
				cel.BinaryBinding(mathMinBinary))),
		cel.Function("Math.abs",
			cel.Overload("Math.abs_dyn",
				[]*cel.Type{cel.DynType}, cel.DoubleType,
				cel.UnaryBinding(mathAbs))),
		cel.Function("Math.round",
			cel.Overload("Math.round_dyn",
				[]*cel.Type{cel.DynType}, cel.IntType,
				cel.UnaryBinding(mathRound))),
		cel.Function("Math.floor",
			cel.Overload("Math.floor_dyn",
				[]*cel.Type{cel.DynType}, cel.IntType,
				cel.UnaryBinding(mathFloor))),
		cel.Function("Math.ceil",
			cel.Overload("Math.ceil_dyn",
				[]*cel.Type{cel.DynType}, cel.IntType,
				cel.UnaryBinding(mathCeil))),
		cel.Function("Math.sqrt",
			cel.Overload("Math.sqrt_dyn",
				[]*cel.Type{cel.DynType}, cel.DoubleType,
				cel.UnaryBinding(mathSqrt))),

		cel.Function("Crypto.uuid",
			cel.Overload("Crypto.uuid_zero",
				[]*cel.Type{}, cel.StringType,
				cel.FunctionBinding(cryptoUUID))),

		cel.Function("Object.keys",
			cel.Overload("Object.keys_dyn",
				[]*cel.Type{cel.DynType}, cel.ListType(cel.DynType),
				cel.UnaryBinding(objectKeys))),
		cel.Function("Object.values",
			cel.Overload("Object.values_dyn",
				[]*cel.Type{cel.DynType}, cel.ListType(cel.DynType),
				cel.UnaryBinding(objectValues))),

		cel.Function("keys",
			cel.MemberOverload("map_keys",
				[]*cel.Type{cel.MapType(cel.DynType, cel.DynType)}, cel.ListType(cel.DynType),
				cel.UnaryBinding(objectKeys))),
		cel.Function("values",
			cel.MemberOverload("map_values",
				[]*cel.Type{cel.MapType(cel.DynType, cel.DynType)}, cel.ListType(cel.DynType),
				cel.UnaryBinding(objectValues))),
		cel.Function("entries",
			cel.MemberOverload("map_entries",
				[]*cel.Type{cel.MapType(cel.DynType, cel.DynType)}, cel.ListType(cel.ListType(cel.DynType)),
				cel.UnaryBinding(mapEntries))),
	}
}

func strCapitalize(v ref.Val) ref.Val {
	s, ok := v.Value().(string)
	if !ok || s == "" {
		return v
	}
	return celtypes.String(strings.ToUpper(s[:1]) + s[1:])
}

func strTruncate(s, n ref.Val) ref.Val {
	str, ok1 := s.Value().(string)
	if !ok1 {
		return s
	}
	nn, ok2 := n.Value().(int64)
	if !ok2 {
		return s
	}
	if nn < 0 {
		nn = 0
	}
	if int64(len(str)) <= nn {
		return celtypes.String(str)
	}
	return celtypes.String(str[:nn])
}

func listFirst(v ref.Val) ref.Val {
	l, ok := v.(traits.Lister)
	if !ok {
		return celtypes.NullValue
	}
	size, _ := l.Size().Value().(int64)
	if size == 0 {
		return celtypes.NullValue
	}
	return l.Get(celtypes.Int(0))
}

func listLast(v ref.Val) ref.Val {
	l, ok := v.(traits.Lister)
	if !ok {
		return celtypes.NullValue
	}
	size, _ := l.Size().Value().(int64)
	if size == 0 {
		return celtypes.NullValue
	}
	return l.Get(celtypes.Int(size - 1))
}

func listSum(v ref.Val) ref.Val {
	l, ok := v.(traits.Lister)
	if !ok {
		return celtypes.Double(0)
	}
	size, _ := l.Size().Value().(int64)
	var sum float64
	for i := int64(0); i < size; i++ {
		if n, ok := toFloat64(l.Get(celtypes.Int(i))); ok {
			sum += n
		}
	}
	return celtypes.Double(sum)
}

func jsonParse(v ref.Val) ref.Val {
	s, ok := v.Value().(string)
	if !ok {
		return celtypes.NullValue
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return celtypes.NullValue
	}
	return celtypes.DefaultTypeAdapter.NativeToValue(out)
}

func jsonStringify(v ref.Val) ref.Val {
	raw, err := json.Marshal(celToGo(v))
	if err != nil {
		return celtypes.String("null")
	}
	return celtypes.String(string(raw))
}

func mathMaxBinary(a, b ref.Val) ref.Val {
	af, ok1 := toFloat64(a)
	bf, ok2 := toFloat64(b)
	if !ok1 || !ok2 {
		return celtypes.Double(math.NaN())
	}
	if af > bf {
		return a
	}
	return b
}

func mathMinBinary(a, b ref.Val) ref.Val {
	af, ok1 := toFloat64(a)
	bf, ok2 := toFloat64(b)
	if !ok1 || !ok2 {
		return celtypes.Double(math.NaN())
	}
	if af < bf {
		return a
	}
	return b
}

func mathAbs(v ref.Val) ref.Val {
	n, ok := toFloat64(v)
	if !ok {
		return celtypes.Double(math.NaN())
	}
	return celtypes.Double(math.Abs(n))
}

func mathRound(v ref.Val) ref.Val {
	n, ok := toFloat64(v)
	if !ok {
		return celtypes.Int(0)
	}
	return celtypes.Int(int64(math.Round(n)))
}

func mathFloor(v ref.Val) ref.Val {
	n, ok := toFloat64(v)
	if !ok {
		return celtypes.Int(0)
	}
	return celtypes.Int(int64(math.Floor(n)))
}

func mathCeil(v ref.Val) ref.Val {
	n, ok := toFloat64(v)
	if !ok {
		return celtypes.Int(0)
	}
	return celtypes.Int(int64(math.Ceil(n)))
}

func mathSqrt(v ref.Val) ref.Val {
	n, ok := toFloat64(v)
	if !ok {
		return celtypes.Double(math.NaN())
	}
	return celtypes.Double(math.Sqrt(n))
}

func cryptoUUID(args ...ref.Val) ref.Val {
	return celtypes.String(uuid.NewString())
}

func sortedStringKeys(m traits.Mapper) []ref.Val {
	type kv struct {
		s string
		k ref.Val
	}
	pairs := []kv{}
	it := m.Iterator()
	for it.HasNext() == celtypes.True {
		k := it.Next()
		pairs = append(pairs, kv{s: fmt.Sprintf("%v", celToGo(k)), k: k})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].s < pairs[j].s })
	out := make([]ref.Val, len(pairs))
	for i, p := range pairs {
		out[i] = p.k
	}
	return out
}

func objectKeys(v ref.Val) ref.Val {
	m, ok := v.(traits.Mapper)
	if !ok {
		return celtypes.DefaultTypeAdapter.NativeToValue([]any{})
	}
	keys := sortedStringKeys(m)
	out := make([]any, len(keys))
	for i, k := range keys {
		out[i] = celToGo(k)
	}
	return celtypes.DefaultTypeAdapter.NativeToValue(out)
}

func objectValues(v ref.Val) ref.Val {
	m, ok := v.(traits.Mapper)
	if !ok {
		return celtypes.DefaultTypeAdapter.NativeToValue([]any{})
	}
	keys := sortedStringKeys(m)
	out := make([]any, len(keys))
	for i, k := range keys {
		val, _ := m.Find(k)
		out[i] = celToGo(val)
	}
	return celtypes.DefaultTypeAdapter.NativeToValue(out)
}

func mapEntries(v ref.Val) ref.Val {
	m, ok := v.(traits.Mapper)
	if !ok {
		return celtypes.DefaultTypeAdapter.NativeToValue([]any{})
	}
	keys := sortedStringKeys(m)
	out := make([]any, len(keys))
	for i, k := range keys {
		val, _ := m.Find(k)
		out[i] = []any{celToGo(k), celToGo(val)}
	}
	return celtypes.DefaultTypeAdapter.NativeToValue(out)
}

func toFloat64(v ref.Val) (float64, bool) {
	switch n := v.Value().(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

func valueToString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(raw)
	}
}
