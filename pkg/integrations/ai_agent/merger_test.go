package ai_agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathAccessor_Get(t *testing.T) {
	accessor := NewPathAccessor()

	tests := []struct {
		name     string
		source   map[string]any
		path     string
		expected any
		exists   bool
	}{
		{
			name:     "simple path",
			source:   map[string]any{"name": "test"},
			path:     "name",
			expected: "test",
			exists:   true,
		},
		{
			name:     "nested path",
			source:   map[string]any{"config": map[string]any{"timeout": 30}},
			path:     "config.timeout",
			expected: 30,
			exists:   true,
		},
		{
			name: "array index path",
			source: map[string]any{
				"items": []any{
					map[string]any{"name": "first"},
					map[string]any{"name": "second"},
				},
			},
			path:     "items[0].name",
			expected: "first",
			exists:   true,
		},
		{
			name: "array index path second element",
			source: map[string]any{
				"items": []any{
					map[string]any{"name": "first"},
					map[string]any{"name": "second"},
				},
			},
			path:     "items[1].name",
			expected: "second",
			exists:   true,
		},
		{
			name: "deeply nested array path",
			source: map[string]any{
				"data": map[string]any{
					"users": []any{
						map[string]any{
							"messages": []any{
								map[string]any{"text": "hello"},
								map[string]any{"text": "world"},
							},
						},
					},
				},
			},
			path:     "data.users[0].messages[1].text",
			expected: "world",
			exists:   true,
		},
		{
			name:     "missing simple path",
			source:   map[string]any{"name": "test"},
			path:     "missing",
			expected: nil,
			exists:   false,
		},
		{
			name:     "missing nested path",
			source:   map[string]any{"config": map[string]any{"timeout": 30}},
			path:     "config.missing",
			expected: nil,
			exists:   false,
		},
		{
			name: "array index out of bounds",
			source: map[string]any{
				"items": []any{map[string]any{"name": "first"}},
			},
			path:     "items[5].name",
			expected: nil,
			exists:   false,
		},
		{
			name:     "empty path",
			source:   map[string]any{"name": "test"},
			path:     "",
			expected: nil,
			exists:   false,
		},
		{
			name: "get array itself",
			source: map[string]any{
				"items": []any{"a", "b", "c"},
			},
			path:     "items",
			expected: []any{"a", "b", "c"},
			exists:   true,
		},
		{
			name: "nested array in array",
			source: map[string]any{
				"users": []any{
					map[string]any{
						"messages": []any{
							map[string]any{"id": 1},
							map[string]any{"id": 2},
						},
					},
				},
			},
			path:     "users[0].messages",
			expected: []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
			exists:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, exists := accessor.Get(tt.source, tt.path)
			assert.Equal(t, tt.exists, exists)
			if tt.exists {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPathAccessor_Set(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]any
		path     string
		value    any
		expected map[string]any
	}{
		{
			name:     "set simple path",
			initial:  map[string]any{},
			path:     "name",
			value:    "test",
			expected: map[string]any{"name": "test"},
		},
		{
			name:     "override simple path",
			initial:  map[string]any{"name": "old"},
			path:     "name",
			value:    "new",
			expected: map[string]any{"name": "new"},
		},
		{
			name:     "set nested path",
			initial:  map[string]any{"config": map[string]any{}},
			path:     "config.timeout",
			value:    30,
			expected: map[string]any{"config": map[string]any{"timeout": 30}},
		},
		{
			name: "set array element",
			initial: map[string]any{
				"items": []any{
					map[string]any{"name": "old"},
				},
			},
			path:  "items[0].name",
			value: "new",
			expected: map[string]any{
				"items": []any{
					map[string]any{"name": "new"},
				},
			},
		},
		{
			name:    "set entire array",
			initial: map[string]any{},
			path:    "items",
			value:   []any{"a", "b", "c"},
			expected: map[string]any{
				"items": []any{"a", "b", "c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor := NewPathAccessor()
			err := accessor.Set(tt.initial, tt.path, tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tt.initial)
		})
	}
}

func TestArrayAppendStrategy_Merge(t *testing.T) {
	strategy := &ArrayAppendStrategy{}

	tests := []struct {
		name     string
		existing any
		incoming any
		expected any
	}{
		{
			name:     "merge simple arrays",
			existing: []any{"existing1", "existing2"},
			incoming: []any{"ai1", "ai2"},
			expected: []any{"ai1", "ai2", "existing1", "existing2"},
		},
		{
			name: "merge object arrays",
			existing: []any{
				map[string]any{"key": "existing", "value": "val1"},
			},
			incoming: []any{
				map[string]any{"key": "ai_key", "value": "ai_val"},
			},
			expected: []any{
				map[string]any{"key": "ai_key", "value": "ai_val"},
				map[string]any{"key": "existing", "value": "val1"},
			},
		},
		{
			name:     "incoming not array returns incoming",
			existing: []any{"existing"},
			incoming: "not an array",
			expected: "not an array",
		},
		{
			name:     "existing not array returns incoming",
			existing: "not an array",
			incoming: []any{"ai1", "ai2"},
			expected: []any{"ai1", "ai2"},
		},
		{
			name:     "merge empty arrays",
			existing: []any{},
			incoming: []any{"ai1"},
			expected: []any{"ai1"},
		},
		{
			name:     "merge with empty incoming array",
			existing: []any{"existing"},
			incoming: []any{},
			expected: []any{"existing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.Merge(tt.existing, tt.incoming)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplaceStrategy_Merge(t *testing.T) {
	strategy := &ReplaceStrategy{}

	tests := []struct {
		name     string
		existing any
		incoming any
		expected any
	}{
		{
			name:     "replace array",
			existing: []any{"existing1", "existing2"},
			incoming: []any{"new1", "new2"},
			expected: []any{"new1", "new2"},
		},
		{
			name:     "replace string",
			existing: "old",
			incoming: "new",
			expected: "new",
		},
		{
			name:     "replace with nil",
			existing: "something",
			incoming: nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.Merge(tt.existing, tt.incoming)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSettingsMerger_Merge(t *testing.T) {
	tests := []struct {
		name            string
		userSettings    map[string]any
		providedByAgent []string
		aiValues        map[string]any
		expected        map[string]any
	}{
		{
			name: "simple property merge",
			userSettings: map[string]any{
				"existing": "value",
			},
			providedByAgent: []string{"url"},
			aiValues: map[string]any{
				"url": "https://example.com",
			},
			expected: map[string]any{
				"existing": "value",
				"url":      "https://example.com",
			},
		},
		{
			name: "array merge - simple path",
			userSettings: map[string]any{
				"http_auth_type": "no-credential",
				"query_params": []any{
					map[string]any{"key": "existing", "value": "val"},
				},
			},
			providedByAgent: []string{"url", "query_params"},
			aiValues: map[string]any{
				"url": "https://example.com",
				"query_params": []any{
					map[string]any{"key": "q", "value": "search"},
				},
			},
			expected: map[string]any{
				"http_auth_type": "no-credential",
				"url":            "https://example.com",
				"query_params": []any{
					map[string]any{"key": "q", "value": "search"},
					map[string]any{"key": "existing", "value": "val"},
				},
			},
		},
		{
			name: "array merge - nested path",
			userSettings: map[string]any{
				"users": []any{
					map[string]any{
						"name": "John",
						"messages": []any{
							map[string]any{"text": "existing"},
						},
					},
				},
			},
			providedByAgent: []string{"users[0].messages"},
			aiValues: map[string]any{
				"users[0].messages": []any{
					map[string]any{"text": "new message"},
				},
			},
			expected: map[string]any{
				"users": []any{
					map[string]any{
						"name": "John",
						"messages": []any{
							map[string]any{"text": "new message"},
							map[string]any{"text": "existing"},
						},
					},
				},
			},
		},
		{
			name: "multiple properties merge",
			userSettings: map[string]any{
				"headers": []any{
					map[string]any{"key": "X-Existing", "value": "123"},
				},
				"query_params": []any{
					map[string]any{"key": "page", "value": "1"},
				},
			},
			providedByAgent: []string{"url", "headers", "query_params"},
			aiValues: map[string]any{
				"url": "https://api.example.com",
				"headers": []any{
					map[string]any{"key": "Authorization", "value": "Bearer token"},
				},
				"query_params": []any{
					map[string]any{"key": "q", "value": "search"},
				},
			},
			expected: map[string]any{
				"url": "https://api.example.com",
				"headers": []any{
					map[string]any{"key": "Authorization", "value": "Bearer token"},
					map[string]any{"key": "X-Existing", "value": "123"},
				},
				"query_params": []any{
					map[string]any{"key": "q", "value": "search"},
					map[string]any{"key": "page", "value": "1"},
				},
			},
		},
		{
			name: "skip missing ai values",
			userSettings: map[string]any{
				"existing": "value",
			},
			providedByAgent: []string{"url", "missing"},
			aiValues: map[string]any{
				"url": "https://example.com",
			},
			expected: map[string]any{
				"existing": "value",
				"url":      "https://example.com",
			},
		},
		{
			name: "preserve non-provided settings",
			userSettings: map[string]any{
				"auth_type": "bearer",
				"timeout":   30,
				"retries":   3,
			},
			providedByAgent: []string{"url"},
			aiValues: map[string]any{
				"url": "https://example.com",
			},
			expected: map[string]any{
				"auth_type": "bearer",
				"timeout":   30,
				"retries":   3,
				"url":       "https://example.com",
			},
		},
		{
			name:            "empty provided by agent",
			userSettings:    map[string]any{"existing": "value"},
			providedByAgent: []string{},
			aiValues:        map[string]any{"url": "https://example.com"},
			expected:        map[string]any{"existing": "value"},
		},
		{
			name: "non-array value replaces",
			userSettings: map[string]any{
				"url": "old-url",
			},
			providedByAgent: []string{"url"},
			aiValues: map[string]any{
				"url": "new-url",
			},
			expected: map[string]any{
				"url": "new-url",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewSettingsMerger()
			result := merger.Merge(tt.userSettings, tt.providedByAgent, tt.aiValues)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSettingsMerger_Merge_WithReplaceStrategy(t *testing.T) {
	merger := NewSettingsMerger(WithArrayStrategy(&ReplaceStrategy{}))

	userSettings := map[string]any{
		"items": []any{"existing1", "existing2"},
	}
	aiValues := map[string]any{
		"items": []any{"new1", "new2"},
	}

	result := merger.Merge(userSettings, []string{"items"}, aiValues)

	// With replace strategy, existing items should be replaced, not appended
	assert.Equal(t, []any{"new1", "new2"}, result["items"])
}

func TestSettingsMerger_Merge_DoesNotMutateOriginal(t *testing.T) {
	merger := NewSettingsMerger()

	original := map[string]any{
		"items": []any{"a", "b"},
	}

	aiValues := map[string]any{
		"items": []any{"c", "d"},
	}

	result := merger.Merge(original, []string{"items"}, aiValues)

	// Original should not be mutated
	assert.Equal(t, []any{"a", "b"}, original["items"])

	// Result should have merged arrays
	assert.Equal(t, []any{"c", "d", "a", "b"}, result["items"])
}

func TestSettingsMerger_GetValueAtPath(t *testing.T) {
	merger := NewSettingsMerger()

	source := map[string]any{
		"users": []any{
			map[string]any{
				"messages": []any{
					map[string]any{"text": "hello"},
				},
			},
		},
	}

	value, exists := merger.GetValueAtPath(source, "users[0].messages[0].text")
	assert.True(t, exists)
	assert.Equal(t, "hello", value)
}

func TestSettingsMerger_SetValueAtPath(t *testing.T) {
	merger := NewSettingsMerger()

	target := map[string]any{
		"config": map[string]any{},
	}

	err := merger.SetValueAtPath(target, "config.timeout", 30)
	require.NoError(t, err)
	assert.Equal(t, 30, target["config"].(map[string]any)["timeout"])
}

func TestDeepCopy(t *testing.T) {
	original := map[string]any{
		"string": "value",
		"number": 42,
		"nested": map[string]any{
			"key": "nested_value",
		},
		"array": []any{
			map[string]any{"id": 1},
			map[string]any{"id": 2},
		},
	}

	// Use Merge with empty providedByAgent to effectively just deep copy
	merger := NewSettingsMerger()
	result := merger.Merge(original, []string{}, map[string]any{})

	// Modify original
	original["string"] = "modified"
	original["nested"].(map[string]any)["key"] = "modified_nested"
	original["array"].([]any)[0].(map[string]any)["id"] = 999

	// Result should not be affected
	assert.Equal(t, "value", result["string"])
	assert.Equal(t, "nested_value", result["nested"].(map[string]any)["key"])
	assert.Equal(t, 1, result["array"].([]any)[0].(map[string]any)["id"])
}
