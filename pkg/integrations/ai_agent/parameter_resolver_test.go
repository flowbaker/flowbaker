package ai_agent

import (
	"flowbaker/internal/domain"
	"fmt"
	"testing"
)

func TestAIParameterResolver_ResolveParameters(t *testing.T) {
	resolver := NewAIParameterResolver()

	tests := []struct {
		name                  string
		workflowNode          *domain.WorkflowNode
		presetSettings        map[string]interface{}
		aiProvidedArguments   map[string]interface{}
		expectedResolvedCount int
		expectedAISourceCount int
		expectError           bool
	}{
		{
			name: "simple parameter - AI provided",
			workflowNode: &domain.WorkflowNode{
				ID:              "test-node",
				ProvidedByAgent: []string{"title"},
			},
			presetSettings: map[string]interface{}{
				"title": "Preset Title",
				"body":  "Preset Body",
			},
			aiProvidedArguments: map[string]interface{}{
				"title": "AI Generated Title",
			},
			expectedResolvedCount: 2, // title (AI) + body (preset)
			expectedAISourceCount: 1, // only title from AI
		},
		{
			name: "nested array path - AI provided",
			workflowNode: &domain.WorkflowNode{
				ID:              "test-node",
				ProvidedByAgent: []string{"assignees[0].login"},
			},
			presetSettings: map[string]interface{}{
				"assignees": []interface{}{
					map[string]interface{}{
						"login": "preset-user",
					},
				},
				"title": "Preset Title",
			},
			aiProvidedArguments: map[string]interface{}{
				"assignees": []interface{}{
					map[string]interface{}{
						"login": "ai-user",
					},
				},
			},
			expectedResolvedCount: 2, // assignees + title
			expectedAISourceCount: 2, // assignees[0].login from AI + assignees (full object) from AI
		},
		{
			name: "AI parameter missing - fallback to preset",
			workflowNode: &domain.WorkflowNode{
				ID:              "test-node",
				ProvidedByAgent: []string{"missing_param"},
			},
			presetSettings: map[string]interface{}{
				"missing_param": "Fallback Value",
				"other_param":   "Other Value",
			},
			aiProvidedArguments: map[string]interface{}{
				// AI doesn't provide missing_param
			},
			expectedResolvedCount: 2, // missing_param (fallback) + other_param
			expectedAISourceCount: 0, // no AI parameters used
		},
		{
			name: "complex nested paths with arrays",
			workflowNode: &domain.WorkflowNode{
				ID: "test-node",
				ProvidedByAgent: []string{
					"body.content[0].text",
					"labels[1].name",
				},
			},
			presetSettings: map[string]interface{}{
				"body": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"text": "Preset content",
						},
					},
				},
				"labels": []interface{}{
					map[string]interface{}{"name": "preset-label-0"},
					map[string]interface{}{"name": "preset-label-1"},
				},
				"title": "Preset Title",
			},
			aiProvidedArguments: map[string]interface{}{
				"body": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"text": "AI generated content",
						},
					},
				},
				"labels": []interface{}{
					map[string]interface{}{"name": "ai-label-0"},
					map[string]interface{}{"name": "ai-label-1"},
				},
			},
			expectedResolvedCount: 3, // body + labels + title
			expectedAISourceCount: 4, // body.content[0].text + labels[1].name + body (full) + labels (full) from AI
		},
		{
			name: "no ProvidedByAgent - all preset",
			workflowNode: &domain.WorkflowNode{
				ID:              "test-node",
				ProvidedByAgent: []string{}, // empty
			},
			presetSettings: map[string]interface{}{
				"title": "Preset Title",
				"body":  "Preset Body",
			},
			aiProvidedArguments: map[string]interface{}{
				"title":            "AI Title", // should be ignored since not in ProvidedByAgent
				"unexpected_param": "AI Unexpected",
			},
			expectedResolvedCount: 3, // title (preset) + body (preset) + unexpected_param (AI)
			expectedAISourceCount: 2, // title (AI - not in ProvidedByAgent) + unexpected_param (AI)
		},
		{
			name: "invalid path in ProvidedByAgent",
			workflowNode: &domain.WorkflowNode{
				ID:              "test-node",
				ProvidedByAgent: []string{"invalid..path", "valid_param"},
			},
			presetSettings: map[string]interface{}{
				"valid_param": "Preset Value",
			},
			aiProvidedArguments: map[string]interface{}{
				"valid_param": "AI Value",
			},
			expectedResolvedCount: 1, // only valid_param
			expectedAISourceCount: 1, // valid_param from AI
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParameterResolutionContext{
				WorkflowNode:        tt.workflowNode,
				PresetSettings:      tt.presetSettings,
				AIProvidedArguments: tt.aiProvidedArguments,
				ToolName:            "test_tool",
			}

			result, err := resolver.ResolveParameters(ctx)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result.ResolvedSettings) != tt.expectedResolvedCount {
				t.Errorf("Expected %d resolved settings, got %d", tt.expectedResolvedCount, len(result.ResolvedSettings))
			}

			// Count AI-sourced parameters
			aiSourceCount := 0
			for _, resolution := range result.ResolutionLog {
				if resolution.Source == ParameterSourceAI {
					aiSourceCount++
				}
			}

			if aiSourceCount != tt.expectedAISourceCount {
				t.Errorf("Expected %d AI-sourced parameters, got %d", tt.expectedAISourceCount, aiSourceCount)
			}
		})
	}
}

func TestAIParameterResolver_GetValueAtPath(t *testing.T) {
	resolver := NewAIParameterResolver()

	testData := map[string]interface{}{
		"simple": "value",
		"nested": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": "deep_value",
			},
		},
		"array": []interface{}{
			map[string]interface{}{
				"item": "item0",
			},
			map[string]interface{}{
				"item": "item1",
			},
		},
		"complex": map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{
							"name": "user0",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
		expectOk bool
	}{
		{
			name:     "simple path",
			path:     "simple",
			expected: "value",
			expectOk: true,
		},
		{
			name:     "nested path",
			path:     "nested.level2.level3",
			expected: "deep_value",
			expectOk: true,
		},
		{
			name:     "array path",
			path:     "array[0].item",
			expected: "item0",
			expectOk: true,
		},
		{
			name:     "array path - second item",
			path:     "array[1].item",
			expected: "item1",
			expectOk: true,
		},
		{
			name:     "complex nested array path",
			path:     "complex.data[0].users[0].name",
			expected: "user0",
			expectOk: true,
		},
		{
			name:     "non-existent path",
			path:     "non.existent.path",
			expected: nil,
			expectOk: false,
		},
		{
			name:     "out of bounds array",
			path:     "array[5].item",
			expected: nil,
			expectOk: false,
		},
		{
			name:     "invalid path format",
			path:     "invalid..path",
			expected: nil,
			expectOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := resolver.getValueAtPath(testData, tt.path)

			if ok != tt.expectOk {
				t.Errorf("Expected ok=%t, got ok=%t", tt.expectOk, ok)
			}

			if tt.expectOk && result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAIParameterResolver_SetValueAtPath(t *testing.T) {
	resolver := NewAIParameterResolver()

	tests := []struct {
		name           string
		initialData    map[string]interface{}
		path           string
		value          interface{}
		expectError    bool
		expectedResult map[string]interface{}
	}{
		{
			name:        "simple path",
			initialData: map[string]interface{}{},
			path:        "simple",
			value:       "new_value",
			expectError: false,
			expectedResult: map[string]interface{}{
				"simple": "new_value",
			},
		},
		{
			name: "override existing simple",
			initialData: map[string]interface{}{
				"simple": "old_value",
			},
			path:        "simple",
			value:       "new_value",
			expectError: false,
			expectedResult: map[string]interface{}{
				"simple": "new_value",
			},
		},
		{
			name:        "nested path creation",
			initialData: map[string]interface{}{},
			path:        "nested.level2.level3",
			value:       "deep_value",
			expectError: false,
			expectedResult: map[string]interface{}{
				"nested": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": "deep_value",
					},
				},
			},
		},
		{
			name: "array path - now supported",
			initialData: map[string]interface{}{
				"items": []interface{}{},
			},
			path:        "items[0].name",
			value:       "item_name",
			expectError: false,
			expectedResult: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"name": "item_name",
					},
				},
			},
		},
		{
			name:        "empty path",
			initialData: map[string]interface{}{},
			path:        "",
			value:       "value",
			expectError: true,
		},
		{
			name:        "invalid path",
			initialData: map[string]interface{}{},
			path:        "invalid..path",
			value:       "value",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of initial data
			data := make(map[string]interface{})
			for k, v := range tt.initialData {
				data[k] = v
			}

			err := resolver.setValueAtPath(data, tt.path, tt.value)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify the result matches expectations
			if !compareValues(data, tt.expectedResult) {
				t.Errorf("Result doesn't match expected.\nGot: %+v\nExpected: %+v", data, tt.expectedResult)
			}
		})
	}
}

func TestAIParameterResolver_IsSpecialKey(t *testing.T) {
	resolver := NewAIParameterResolver()

	tests := []struct {
		key      string
		expected bool
	}{
		{"credential_id", true},
		{"node_id", true},
		{"integration_type", true},
		{"action_type", true},
		{"_tool_call", true},
		{"_tool_name", true},
		{"_tool_call_id", true},
		{"normal_param", false},
		{"title", false},
		{"body", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := resolver.isSpecialKey(tt.key)
			if result != tt.expected {
				t.Errorf("isSpecialKey(%q) = %t, expected %t", tt.key, result, tt.expected)
			}
		})
	}
}

func TestAIParameterResolver_GetResolutionSummary(t *testing.T) {
	resolver := NewAIParameterResolver()

	result := &ParameterResolutionResult{
		ResolvedSettings: map[string]interface{}{
			"param1": "value1",
			"param2": "value2",
		},
		ResolutionLog: []ParameterResolution{
			{Path: "param1", Source: ParameterSourceAI},
			{Path: "param2", Source: ParameterSourcePreset},
			{Path: "param3", Source: ParameterSourceMissing},
		},
	}

	summary := resolver.GetResolutionSummary(result)

	// Check that summary contains expected information
	expectedSubstrings := []string{
		"Parameter Resolution Summary",
		"AI Provided: 1",
		"Preset Values: 1",
		"Missing: 1",
		"WARNING", // Should warn about missing parameters
	}

	for _, expected := range expectedSubstrings {
		if !containsSubstring(summary, expected) {
			t.Errorf("Summary missing expected substring: %q\nSummary: %s", expected, summary)
		}
	}
}

// Helper functions

func compareValues(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle maps
	aMap, aIsMap := a.(map[string]interface{})
	bMap, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		if len(aMap) != len(bMap) {
			return false
		}
		for k, v := range aMap {
			if bv, exists := bMap[k]; !exists || !compareValues(v, bv) {
				return false
			}
		}
		return true
	}

	// Handle slices
	aSlice, aIsSlice := a.([]interface{})
	bSlice, bIsSlice := b.([]interface{})
	if aIsSlice && bIsSlice {
		if len(aSlice) != len(bSlice) {
			return false
		}
		for i, v := range aSlice {
			if !compareValues(v, bSlice[i]) {
				return false
			}
		}
		return true
	}

	// For non-comparable types, convert to string and compare
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
