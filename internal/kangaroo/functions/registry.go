// Package functions provides the default function registry for Kangaroo expressions
package functions

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja/unistring"
	"github.com/google/uuid"

	"github.com/flowbaker/flowbaker/internal/kangaroo/types"
)

// DefaultFunctionRegistry provides a comprehensive set of secure functions
type DefaultFunctionRegistry struct {
	functions map[string]*types.SafeFunction
	mu        sync.RWMutex
	converter *types.ValueConverter
}

// NewDefaultFunctionRegistry creates a new function registry with default functions
func NewDefaultFunctionRegistry() *DefaultFunctionRegistry {
	registry := &DefaultFunctionRegistry{
		functions: make(map[string]*types.SafeFunction),
		converter: types.NewValueConverter(),
	}
	registry.registerDefaultFunctions()
	return registry
}

// Register registers a new safe function
func (r *DefaultFunctionRegistry) Register(fn *types.SafeFunction) error {
	if fn == nil || fn.Name == "" {
		return fmt.Errorf("invalid function: name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.functions[fn.Name] = fn
	return nil
}

// Unregister removes a function from the registry
func (r *DefaultFunctionRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.functions, name)
}

// Get retrieves a function by name
func (r *DefaultFunctionRegistry) Get(name string) (*types.SafeFunction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.functions[name]
	return fn, ok
}

// Has checks if a function exists
func (r *DefaultFunctionRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.functions[name]
	return ok
}

// List returns all functions, optionally filtered by category
func (r *DefaultFunctionRegistry) List(category string) []*types.SafeFunction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*types.SafeFunction
	for _, fn := range r.functions {
		if category == "" || fn.Category == category {
			result = append(result, fn)
		}
	}

	// Sort by name for consistent results
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// GetNames returns all function names
func (r *DefaultFunctionRegistry) GetNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.functions))
	for name := range r.functions {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// GetCategories returns all unique categories
func (r *DefaultFunctionRegistry) GetCategories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categorySet := make(map[string]bool)
	for _, fn := range r.functions {
		if fn.Category != "" {
			categorySet[fn.Category] = true
		}
	}

	categories := make([]string, 0, len(categorySet))
	for category := range categorySet {
		categories = append(categories, category)
	}

	sort.Strings(categories)
	return categories
}

// Clear removes all functions
func (r *DefaultFunctionRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.functions = make(map[string]*types.SafeFunction)
}

// GetStats returns registry statistics
func (r *DefaultFunctionRegistry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	functionsByCategory := make(map[string]int)
	asyncFunctions := 0
	deprecatedFunctions := 0

	for _, fn := range r.functions {
		if fn.Category != "" {
			functionsByCategory[fn.Category]++
		}
		if fn.IsAsync {
			asyncFunctions++
		}
		if fn.Deprecated {
			deprecatedFunctions++
		}
	}

	return map[string]interface{}{
		"totalFunctions":      len(r.functions),
		"functionsByCategory": functionsByCategory,
		"asyncFunctions":      asyncFunctions,
		"deprecatedFunctions": deprecatedFunctions,
	}
}

// registerDefaultFunctions registers all default functions
func (r *DefaultFunctionRegistry) registerDefaultFunctions() {
	r.registerStringFunctions()
	r.registerArrayFunctions()
	r.registerObjectFunctions()
	r.registerMathFunctions()
	r.registerDateFunctions()
	r.registerJsonFunctions()
	r.registerWorkflowFunctions()
	r.registerCryptoFunctions()
	r.registerArrayUtilityFunctions()
	r.registerStringUtilityFunctions()
	r.registerConditionalFunctions()
}

// registerStringFunctions registers string manipulation functions
func (r *DefaultFunctionRegistry) registerStringFunctions() {
	stringFunctions := []*types.SafeFunction{
		{
			Name:        "toString",
			Description: "Converts value to string representation",
			Category:    types.CategoryString,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return "", nil
				}
				return r.converter.ToString(args[0]), nil
			},
		},
		{
			Name:        "toLowerCase",
			Description: "Converts string to lowercase",
			Category:    types.CategoryString,
			MinArgs:     0, // 0 when called as method, 1 when called as function
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return "", nil
				}
				return strings.ToLower(r.converter.ToString(args[0])), nil
			},
		},
		{
			Name:        "toUpperCase",
			Description: "Converts string to uppercase",
			Category:    types.CategoryString,
			MinArgs:     0,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return "", nil
				}
				return strings.ToUpper(r.converter.ToString(args[0])), nil
			},
		},
		{
			Name:        "trim",
			Description: "Removes whitespace from both ends",
			Category:    types.CategoryString,
			MinArgs:     0,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return "", nil
				}
				return strings.TrimSpace(r.converter.ToString(args[0])), nil
			},
		},
		{
			Name:        "split",
			Description: "Splits string into array",
			Category:    types.CategoryString,
			MinArgs:     0,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return []string{}, nil
				}

				str := r.converter.ToString(args[0])
				separator := ""
				if len(args) > 1 {
					separator = r.converter.ToString(args[1])
				}

				if separator == "" {
					// Split into characters
					return strings.Split(str, ""), nil
				}

				return strings.Split(str, separator), nil
			},
		},
		{
			Name:        "replace",
			Description: "Replaces text in string",
			Category:    types.CategoryString,
			MinArgs:     3,
			MaxArgs:     3,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) < 3 {
					return nil, fmt.Errorf("replace requires 3 arguments")
				}

				str := r.converter.ToString(args[0])
				search := r.converter.ToString(args[1])
				replace := r.converter.ToString(args[2])

				return strings.ReplaceAll(str, search, replace), nil
			},
		},
		{
			Name:        "substring",
			Description: "Extracts substring",
			Category:    types.CategoryString,
			MinArgs:     2,
			MaxArgs:     3,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("substring requires at least 2 arguments")
				}

				str := r.converter.ToString(args[0])
				start := r.converter.ToInt(args[1])

				if start < 0 {
					start = 0
				}
				if start >= len(str) {
					return "", nil
				}

				if len(args) > 2 {
					end := r.converter.ToInt(args[2])
					if end <= start {
						return "", nil
					}
					if end > len(str) {
						end = len(str)
					}
					return str[start:end], nil
				}

				return str[start:], nil
			},
		},
		{
			Name:        "includes",
			Description: "Checks if string contains substring",
			Category:    types.CategoryString,
			MinArgs:     1,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("includes requires 2 arguments")
				}

				str := r.converter.ToString(args[0])
				search := r.converter.ToString(args[1])

				return strings.Contains(str, search), nil
			},
		},
		{
			Name:        "startsWith",
			Description: "Checks if string starts with prefix",
			Category:    types.CategoryString,
			MinArgs:     1,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("startsWith requires 2 arguments")
				}

				str := r.converter.ToString(args[0])
				prefix := r.converter.ToString(args[1])

				return strings.HasPrefix(str, prefix), nil
			},
		},
		{
			Name:        "endsWith",
			Description: "Checks if string ends with suffix",
			Category:    types.CategoryString,
			MinArgs:     1,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("endsWith requires 2 arguments")
				}

				str := r.converter.ToString(args[0])
				suffix := r.converter.ToString(args[1])

				return strings.HasSuffix(str, suffix), nil
			},
		},
	}

	for _, fn := range stringFunctions {
		r.Register(fn)
	}
}

// registerArrayFunctions registers array manipulation functions
func (r *DefaultFunctionRegistry) registerArrayFunctions() {
	arrayFunctions := []*types.SafeFunction{
		{
			Name:        "length",
			Description: "Gets array or string length",
			Category:    types.CategoryArray,
			MinArgs:     0,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return 0, nil
				}

				switch v := args[0].(type) {
				case []interface{}:
					return len(v), nil
				case string:
					return len(v), nil
				default:
					return 0, nil
				}
			},
		},
		{
			Name:        "join",
			Description: "Joins array elements into string",
			Category:    types.CategoryArray,
			MinArgs:     0,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return "", nil
				}

				separator := ","
				if len(args) > 1 {
					separator = r.converter.ToString(args[1])
				}

				arr := r.converter.ToArray(args[0])
				strArr := make([]string, len(arr))
				for i, v := range arr {
					strArr[i] = r.converter.ToString(v)
				}

				return strings.Join(strArr, separator), nil
			},
		},
		{
			Name:        "slice",
			Description: "Returns portion of array",
			Category:    types.CategoryArray,
			MinArgs:     0,
			MaxArgs:     3,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return []interface{}{}, nil
				}

				arr := r.converter.ToArray(args[0])
				if len(arr) == 0 {
					return []interface{}{}, nil
				}

				start := 0
				if len(args) > 1 {
					start = r.converter.ToInt(args[1])
					if start < 0 {
						start = 0
					}
					if start >= len(arr) {
						return []interface{}{}, nil
					}
				}

				end := len(arr)
				if len(args) > 2 {
					end = r.converter.ToInt(args[2])
					if end > len(arr) {
						end = len(arr)
					}
					if end <= start {
						return []interface{}{}, nil
					}
				}

				return arr[start:end], nil
			},
		},
		{
			Name:        "first",
			Description: "Gets first element of array",
			Category:    types.CategoryArray,
			MinArgs:     0,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return nil, nil
				}

				arr := r.converter.ToArray(args[0])
				if len(arr) == 0 {
					return nil, nil
				}

				return arr[0], nil
			},
		},
		{
			Name:        "last",
			Description: "Gets last element of array",
			Category:    types.CategoryArray,
			MinArgs:     0,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return nil, nil
				}

				arr := r.converter.ToArray(args[0])
				if len(arr) == 0 {
					return nil, nil
				}

				return arr[len(arr)-1], nil
			},
		},
		{
			Name:        "reverse",
			Description: "Returns reversed copy of array",
			Category:    types.CategoryArray,
			MinArgs:     0,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return []interface{}{}, nil
				}

				arr := r.converter.ToArray(args[0])
				result := make([]interface{}, len(arr))
				for i, v := range arr {
					result[len(arr)-1-i] = v
				}

				return result, nil
			},
		},
		{
			Name:        "filter",
			Description: "Filters array elements using callback function",
			Category:    types.CategoryArray,
			MinArgs:     2, // array and callback
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				// This is handled specially in the executor for arrow functions
				return nil, fmt.Errorf("filter requires an arrow function callback")
			},
		},
		{
			Name:        "map",
			Description: "Maps array elements using callback function",
			Category:    types.CategoryArray,
			MinArgs:     2,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				return nil, fmt.Errorf("map requires an arrow function callback")
			},
		},
		{
			Name:        "find",
			Description: "Finds first array element matching callback",
			Category:    types.CategoryArray,
			MinArgs:     2,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				return nil, fmt.Errorf("find requires an arrow function callback")
			},
		},
	}

	for _, fn := range arrayFunctions {
		r.Register(fn)
	}
}

// registerObjectFunctions registers object manipulation functions
func (r *DefaultFunctionRegistry) registerObjectFunctions() {
	objectFunctions := []*types.SafeFunction{
		{
			Name:        "Object.keys",
			Description: "Gets object property names",
			Category:    types.CategoryObject,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				obj := r.converter.ToMap(args[0])
				if obj == nil {
					return []interface{}{}, nil
				}

				keys := make([]interface{}, 0, len(obj))
				for k := range obj {
					keys = append(keys, k)
				}

				return keys, nil
			},
		},
		{
			Name:        "Object.values",
			Description: "Gets object property values",
			Category:    types.CategoryObject,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				obj := r.converter.ToMap(args[0])
				if obj == nil {
					return []interface{}{}, nil
				}

				values := make([]interface{}, 0, len(obj))
				for _, v := range obj {
					values = append(values, v)
				}

				return values, nil
			},
		},
		{
			Name:        "Object.entries",
			Description: "Gets object key-value pairs",
			Category:    types.CategoryObject,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				obj := r.converter.ToMap(args[0])
				if obj == nil {
					return []interface{}{}, nil
				}

				entries := make([]interface{}, 0, len(obj))
				for k, v := range obj {
					entries = append(entries, []interface{}{k, v})
				}

				return entries, nil
			},
		},
	}

	for _, fn := range objectFunctions {
		r.Register(fn)
	}
}

// registerMathFunctions registers math functions
func (r *DefaultFunctionRegistry) registerMathFunctions() {
	mathFunctions := []*types.SafeFunction{
		{
			Name:        "Math.round",
			Description: "Rounds to nearest integer",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				num, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil // JavaScript behavior for invalid numbers
				}
				return math.Round(num), nil
			},
		},
		{
			Name:        "Math.floor",
			Description: "Rounds down to integer",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				num, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil
				}
				return math.Floor(num), nil
			},
		},
		{
			Name:        "Math.ceil",
			Description: "Rounds up to integer",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				num, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil
				}
				return math.Ceil(num), nil
			},
		},
		{
			Name:        "Math.abs",
			Description: "Returns absolute value",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				num, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil
				}
				return math.Abs(num), nil
			},
		},
		{
			Name:        "Math.max",
			Description: "Returns largest number",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     -1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return math.Inf(-1), nil
				}

				max, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil
				}
				for i := 1; i < len(args); i++ {
					val, err := r.converter.ToNumber(args[i])
					if err != nil {
						return math.NaN(), nil
					}
					if val > max {
						max = val
					}
				}

				return max, nil
			},
		},
		{
			Name:        "Math.min",
			Description: "Returns smallest number",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     -1,
			Fn: func(args ...interface{}) (interface{}, error) {
				if len(args) == 0 {
					return math.Inf(1), nil
				}

				min, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil
				}
				for i := 1; i < len(args); i++ {
					val, err := r.converter.ToNumber(args[i])
					if err != nil {
						return math.NaN(), nil
					}
					if val < min {
						min = val
					}
				}

				return min, nil
			},
		},
		{
			Name:        "Math.pow",
			Description: "Returns base raised to exponent",
			Category:    types.CategoryMath,
			MinArgs:     2,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				base, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil
				}
				exponent, err := r.converter.ToNumber(args[1])
				if err != nil {
					return math.NaN(), nil
				}
				return math.Pow(base, exponent), nil
			},
		},
		{
			Name:        "Math.sqrt",
			Description: "Returns square root",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				num, err := r.converter.ToNumber(args[0])
				if err != nil {
					return math.NaN(), nil
				}
				return math.Sqrt(num), nil
			},
		},
		{
			Name:        "Math.random",
			Description: "Returns random number between 0 and 1",
			Category:    types.CategoryMath,
			MinArgs:     0,
			MaxArgs:     0,
			Fn: func(args ...interface{}) (interface{}, error) {
				return rand.Float64(), nil
			},
		},
		{
			Name:        "parseInt",
			Description: "Parses string to integer",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				base := 10
				if len(args) > 1 {
					base = r.converter.ToInt(args[1])
				}

				// JavaScript-like parseInt: parse from start until invalid character
				// Remove leading whitespace
				str = strings.TrimLeft(str, " \t\n\r")
				if str == "" {
					return 0.0, nil
				}

				// Extract valid digits from the beginning
				validStr := ""
				sign := ""

				// Handle sign
				if strings.HasPrefix(str, "-") {
					sign = "-"
					str = str[1:]
				} else if strings.HasPrefix(str, "+") {
					str = str[1:]
				}

				// Extract valid digits based on base
				for _, char := range str {
					if base == 10 {
						if char >= '0' && char <= '9' {
							validStr += string(char)
						} else {
							break
						}
					} else if base == 16 {
						if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
							validStr += string(char)
						} else {
							break
						}
					} else if base == 8 {
						if char >= '0' && char <= '7' {
							validStr += string(char)
						} else {
							break
						}
					} else {
						// For other bases, use digit validation
						if (char >= '0' && char <= '9' && int(char-'0') < base) ||
							(char >= 'a' && char <= 'z' && int(char-'a'+10) < base) ||
							(char >= 'A' && char <= 'Z' && int(char-'A'+10) < base) {
							validStr += string(char)
						} else {
							break
						}
					}
				}

				if validStr == "" {
					return 0.0, nil
				}

				val, err := strconv.ParseInt(sign+validStr, base, 64)
				if err != nil {
					return 0.0, nil
				}

				return float64(val), nil
			},
		},
		{
			Name:        "parseFloat",
			Description: "Parses string to float",
			Category:    types.CategoryMath,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				val, err := strconv.ParseFloat(str, 64)
				if err != nil {
					return math.NaN(), nil // JavaScript behavior
				}

				return val, nil
			},
		},
		{
			Name:        "Math.PI",
			Description: "Pi constant",
			Category:    types.CategoryMath,
			MinArgs:     0,
			MaxArgs:     0,
			Fn: func(args ...interface{}) (interface{}, error) {
				return math.Pi, nil
			},
		},
		{
			Name:        "Math.E",
			Description: "Euler's number",
			Category:    types.CategoryMath,
			MinArgs:     0,
			MaxArgs:     0,
			Fn: func(args ...interface{}) (interface{}, error) {
				return math.E, nil
			},
		},
	}

	for _, fn := range mathFunctions {
		r.Register(fn)
	}
}

// registerDateFunctions registers date functions
func (r *DefaultFunctionRegistry) registerDateFunctions() {
	dateFunctions := []*types.SafeFunction{
		{
			Name:        "Date.now",
			Description: "Current timestamp in milliseconds",
			Category:    types.CategoryDate,
			MinArgs:     0,
			MaxArgs:     0,
			Fn: func(args ...interface{}) (interface{}, error) {
				return float64(time.Now().UnixMilli()), nil
			},
		},
		{
			Name:        "Date.parse",
			Description: "Parse date string to timestamp",
			Category:    types.CategoryDate,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				dateStr := r.converter.ToString(args[0])

				// Try multiple date formats
				formats := []string{
					time.RFC3339,
					"2006-01-02T15:04:05Z",
					"2006-01-02T15:04:05",
					"2006-01-02 15:04:05",
					"2006-01-02",
					"01/02/2006",
					"2006/01/02",
				}

				for _, format := range formats {
					if t, err := time.Parse(format, dateStr); err == nil {
						return float64(t.UnixMilli()), nil
					}
				}

				return math.NaN(), nil
			},
		},
		{
			Name:        "Date.today",
			Description: "Get today's date in YYYY-MM-DD format",
			Category:    types.CategoryDate,
			MinArgs:     0,
			MaxArgs:     0,
			Fn: func(args ...interface{}) (interface{}, error) {
				return time.Now().Format("2006-01-02"), nil
			},
		},
		{
			Name:        "Date.addDays",
			Description: "Add days to a date",
			Category:    types.CategoryDate,
			MinArgs:     2,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				dateStr := r.converter.ToString(args[0])
				days := r.converter.ToInt(args[1])

				t, err := time.Parse(time.RFC3339, dateStr)
				if err != nil {
					// Try parsing as YYYY-MM-DD
					if t, err = time.Parse("2006-01-02", dateStr); err != nil {
						return nil, fmt.Errorf("invalid date format")
					}
				}

				result := t.AddDate(0, 0, days)
				return result.Format(time.RFC3339), nil
			},
		},
		{
			Name:        "Date.diffDays",
			Description: "Get difference in days between two dates",
			Category:    types.CategoryDate,
			MinArgs:     2,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				date1Str := r.converter.ToString(args[0])
				date2Str := r.converter.ToString(args[1])

				t1, err := time.Parse(time.RFC3339, date1Str)
				if err != nil {
					if t1, err = time.Parse("2006-01-02", date1Str); err != nil {
						return nil, fmt.Errorf("invalid date1 format")
					}
				}

				t2, err := time.Parse(time.RFC3339, date2Str)
				if err != nil {
					if t2, err = time.Parse("2006-01-02", date2Str); err != nil {
						return nil, fmt.Errorf("invalid date2 format")
					}
				}

				diff := t2.Sub(t1)
				return float64(diff.Hours() / 24), nil
			},
		},
	}

	for _, fn := range dateFunctions {
		r.Register(fn)
	}
}

// registerJsonFunctions registers JSON functions
func (r *DefaultFunctionRegistry) registerJsonFunctions() {
	jsonFunctions := []*types.SafeFunction{
		{
			Name:        "JSON.parse",
			Description: "Parse JSON string",
			Category:    types.CategoryJSON,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				var result interface{}
				if err := json.Unmarshal([]byte(str), &result); err != nil {
					return nil, nil // JavaScript behavior - return null for invalid JSON
				}
				return result, nil
			},
		},
		{
			Name:        "JSON.stringify",
			Description: "Convert object to JSON string",
			Category:    types.CategoryJSON,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				bytes, err := json.Marshal(args[0])
				if err != nil {
					return "null", nil // JavaScript behavior
				}
				return string(bytes), nil
			},
		},
	}

	for _, fn := range jsonFunctions {
		r.Register(fn)
	}
}

// registerWorkflowFunctions registers workflow-specific functions
func (r *DefaultFunctionRegistry) registerWorkflowFunctions() {
	workflowFunctions := []*types.SafeFunction{
		{
			Name:        "isEmpty",
			Description: "Check if value is empty",
			Category:    types.CategoryUtility,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				value := args[0]

				if value == nil {
					return true, nil
				}

				switch v := value.(type) {
				case string:
					return strings.TrimSpace(v) == "", nil
				case unistring.String:
					return r.converter.IsEmpty(v), nil
				case []interface{}:
					return len(v) == 0, nil
				case map[string]interface{}:
					return len(v) == 0, nil
				default:
					return false, nil
				}
			},
		},
		{
			Name:        "hasField",
			Description: "Check if object has field",
			Category:    types.CategoryUtility,
			MinArgs:     2,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				obj := r.converter.ToMap(args[0])
				if obj == nil {
					return false, nil
				}

				fieldName := r.converter.ToString(args[1])
				_, exists := obj[fieldName]
				return exists, nil
			},
		},
	}

	for _, fn := range workflowFunctions {
		r.Register(fn)
	}
}

// registerCryptoFunctions registers cryptographic functions
func (r *DefaultFunctionRegistry) registerCryptoFunctions() {
	cryptoFunctions := []*types.SafeFunction{
		{
			Name:        "Crypto.uuid",
			Description: "Generate UUID v4",
			Category:    types.CategoryCrypto,
			MinArgs:     0,
			MaxArgs:     0,
			Fn: func(args ...interface{}) (interface{}, error) {
				// Simple UUID v4 implementation
				uuid := uuid.New().String()
				return uuid, nil
			},
		},
		{
			Name:        "Crypto.base64Encode",
			Description: "Base64 encode string",
			Category:    types.CategoryCrypto,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				return base64.StdEncoding.EncodeToString([]byte(str)), nil
			},
		},
		{
			Name:        "Crypto.base64Decode",
			Description: "Base64 decode string",
			Category:    types.CategoryCrypto,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				decoded, err := base64.StdEncoding.DecodeString(str)
				if err != nil {
					return "", nil // Return empty string on error
				}
				return string(decoded), nil
			},
		},
	}

	for _, fn := range cryptoFunctions {
		r.Register(fn)
	}
}

// registerArrayUtilityFunctions registers array utility functions
func (r *DefaultFunctionRegistry) registerArrayUtilityFunctions() {
	arrayUtilityFunctions := []*types.SafeFunction{
		{
			Name:        "Array.flatten",
			Description: "Flatten nested arrays",
			Category:    types.CategoryArray,
			MinArgs:     1,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				arr := r.converter.ToArray(args[0])
				depth := 1
				if len(args) > 1 {
					depth = r.converter.ToInt(args[1])
				}

				return flattenArray(arr, depth), nil
			},
		},
		{
			Name:        "Array.filterUnique",
			Description: "Remove duplicate values",
			Category:    types.CategoryArray,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				arr := r.converter.ToArray(args[0])
				seen := make(map[string]bool)
				var result []interface{}

				for _, item := range arr {
					key := r.converter.ToString(item)
					if !seen[key] {
						seen[key] = true
						result = append(result, item)
					}
				}

				return result, nil
			},
		},
		{
			Name:        "Array.chunk",
			Description: "Split array into chunks",
			Category:    types.CategoryArray,
			MinArgs:     2,
			MaxArgs:     2,
			Fn: func(args ...interface{}) (interface{}, error) {
				arr := r.converter.ToArray(args[0])
				size := r.converter.ToInt(args[1])

				if size <= 0 {
					return []interface{}{}, nil
				}

				var chunks []interface{}
				for i := 0; i < len(arr); i += size {
					end := i + size
					if end > len(arr) {
						end = len(arr)
					}
					chunks = append(chunks, arr[i:end])
				}

				return chunks, nil
			},
		},
	}

	for _, fn := range arrayUtilityFunctions {
		r.Register(fn)
	}
}

// registerStringUtilityFunctions registers string utility functions
func (r *DefaultFunctionRegistry) registerStringUtilityFunctions() {
	stringUtilityFunctions := []*types.SafeFunction{
		{
			Name:        "String.capitalize",
			Description: "Capitalize first letter",
			Category:    types.CategoryString,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				if str == "" {
					return str, nil
				}

				return strings.ToUpper(str[:1]) + strings.ToLower(str[1:]), nil
			},
		},
		{
			Name:        "String.reverse",
			Description: "Reverse string",
			Category:    types.CategoryString,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				runes := []rune(str)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return string(runes), nil
			},
		},
		{
			Name:        "String.truncate",
			Description: "Truncate string with suffix",
			Category:    types.CategoryString,
			MinArgs:     2,
			MaxArgs:     3,
			Fn: func(args ...interface{}) (interface{}, error) {
				str := r.converter.ToString(args[0])
				length := r.converter.ToInt(args[1])
				suffix := "..."
				if len(args) > 2 {
					suffix = r.converter.ToString(args[2])
				}

				if len(str) <= length {
					return str, nil
				}

				return str[:length] + suffix, nil
			},
		},
	}

	for _, fn := range stringUtilityFunctions {
		r.Register(fn)
	}
}

// registerConditionalFunctions registers conditional functions
func (r *DefaultFunctionRegistry) registerConditionalFunctions() {
	conditionalFunctions := []*types.SafeFunction{
		{
			Name:        "$if",
			Description: "Conditional expression",
			Category:    types.CategoryConditional,
			MinArgs:     2,
			MaxArgs:     3,
			Fn: func(args ...interface{}) (interface{}, error) {
				condition := r.converter.ToBool(args[0])
				trueValue := r.converter.NormalizeValue(args[1])
				falseValue := interface{}(nil)
				if len(args) > 2 {
					falseValue = r.converter.NormalizeValue(args[2])
				}

				if condition {
					return trueValue, nil
				}
				return falseValue, nil
			},
		},
		{
			Name:        "$and",
			Description: "Logical AND",
			Category:    types.CategoryConditional,
			MinArgs:     1,
			MaxArgs:     -1,
			Fn: func(args ...interface{}) (interface{}, error) {
				for _, arg := range args {
					if !r.converter.ToBool(arg) {
						return false, nil
					}
				}
				return true, nil
			},
		},
		{
			Name:        "$or",
			Description: "Logical OR",
			Category:    types.CategoryConditional,
			MinArgs:     1,
			MaxArgs:     -1,
			Fn: func(args ...interface{}) (interface{}, error) {
				for _, arg := range args {
					if r.converter.ToBool(arg) {
						return true, nil
					}
				}
				return false, nil
			},
		},
		{
			Name:        "$not",
			Description: "Logical NOT",
			Category:    types.CategoryConditional,
			MinArgs:     1,
			MaxArgs:     1,
			Fn: func(args ...interface{}) (interface{}, error) {
				return !r.converter.ToBool(args[0]), nil
			},
		},
	}

	for _, fn := range conditionalFunctions {
		r.Register(fn)
	}
}

func flattenArray(arr []interface{}, depth int) []interface{} {
	if depth <= 0 {
		return arr
	}

	var result []interface{}
	for _, item := range arr {
		if subArr, ok := item.([]interface{}); ok {
			result = append(result, flattenArray(subArr, depth-1)...)
		} else {
			result = append(result, item)
		}
	}

	return result
}
