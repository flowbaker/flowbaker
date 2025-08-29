package domain

import (
	"testing"
)

func TestPropertyPathManager_BuildPath(t *testing.T) {
	pm := NewPropertyPathManager()

	tests := []struct {
		name         string
		parentPath   string
		propertyKey  string
		index        *int
		expected     string
	}{
		{
			name:        "simple property",
			parentPath:  "",
			propertyKey: "message",
			index:       nil,
			expected:    "message",
		},
		{
			name:        "nested property",
			parentPath:  "config",
			propertyKey: "timeout",
			index:       nil,
			expected:    "config.timeout",
		},
		{
			name:        "array with index only",
			parentPath:  "items",
			propertyKey: "",
			index:       intPtr(0),
			expected:    "items[0]",
		},
		{
			name:        "array with property",
			parentPath:  "items",
			propertyKey: "name",
			index:       intPtr(0),
			expected:    "items[0].name",
		},
		{
			name:        "nested array property",
			parentPath:  "items[0]",
			propertyKey: "metadata",
			index:       nil,
			expected:    "items[0].metadata",
		},
		{
			name:        "deep nesting",
			parentPath:  "data.users[2].address",
			propertyKey: "street",
			index:       nil,
			expected:    "data.users[2].address.street",
		},
		{
			name:        "empty parent and key",
			parentPath:  "",
			propertyKey: "",
			index:       nil,
			expected:    "",
		},
		{
			name:        "negative index ignored",
			parentPath:  "items",
			propertyKey: "name",
			index:       intPtr(-1),
			expected:    "items.name", // negative index should be ignored
		},
		{
			name:        "whitespace handling",
			parentPath:  "  config  ",
			propertyKey: "  timeout  ",
			index:       nil,
			expected:    "config.timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.BuildPath(tt.parentPath, tt.propertyKey, tt.index)
			if result != tt.expected {
				t.Errorf("BuildPath() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestPropertyPathManager_ParsePath(t *testing.T) {
	pm := NewPropertyPathManager()

	tests := []struct {
		name        string
		path        string
		expected    []PropertyPathSegment
		shouldError bool
	}{
		{
			name: "simple property",
			path: "message",
			expected: []PropertyPathSegment{
				{Key: "message"},
			},
		},
		{
			name: "nested property",
			path: "config.timeout",
			expected: []PropertyPathSegment{
				{Key: "config"},
				{Key: "timeout"},
			},
		},
		{
			name: "array property",
			path: "items[0].name",
			expected: []PropertyPathSegment{
				{Key: "items", Index: intPtr(0)},
				{Key: "name"},
			},
		},
		{
			name: "multiple arrays",
			path: "data[1].users[2].name",
			expected: []PropertyPathSegment{
				{Key: "data", Index: intPtr(1)},
				{Key: "users", Index: intPtr(2)},
				{Key: "name"},
			},
		},
		{
			name: "deep nesting",
			path: "level1.level2.level3.value",
			expected: []PropertyPathSegment{
				{Key: "level1"},
				{Key: "level2"},
				{Key: "level3"},
				{Key: "value"},
			},
		},
		{
			name:     "empty path",
			path:     "",
			expected: []PropertyPathSegment{},
		},
		{
			name:     "whitespace only",
			path:     "   ",
			expected: []PropertyPathSegment{},
		},
		{
			name:        "invalid array index",
			path:        "items[abc].name",
			shouldError: true,
		},
		{
			name:        "negative array index",
			path:        "items[-1].name",
			shouldError: true,
		},
		{
			name:        "empty array index",
			path:        "items[].name",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pm.ParsePath(tt.path)
			
			if tt.shouldError {
				if err == nil {
					t.Errorf("ParsePath() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParsePath() unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ParsePath() returned %d segments, expected %d", len(result), len(tt.expected))
				return
			}

			for i, segment := range result {
				expected := tt.expected[i]
				if segment.Key != expected.Key {
					t.Errorf("Segment %d: got key %q, expected %q", i, segment.Key, expected.Key)
				}
				
				if (segment.Index == nil) != (expected.Index == nil) {
					t.Errorf("Segment %d: index presence mismatch", i)
				} else if segment.Index != nil && expected.Index != nil && *segment.Index != *expected.Index {
					t.Errorf("Segment %d: got index %d, expected %d", i, *segment.Index, *expected.Index)
				}
			}
		})
	}
}

func TestPropertyPathManager_GetParentPath(t *testing.T) {
	pm := NewPropertyPathManager()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "nested property",
			path:     "config.timeout",
			expected: "config",
		},
		{
			name:     "array property",
			path:     "items[0].name",
			expected: "items[0]",
		},
		{
			name:     "deep nesting",
			path:     "level1.level2.level3.value",
			expected: "level1.level2.level3",
		},
		{
			name:     "simple property",
			path:     "message",
			expected: "",
		},
		{
			name:     "array only",
			path:     "items[0]",
			expected: "items",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "complex array path",
			path:     "data.users[2].address.street",
			expected: "data.users[2].address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.GetParentPath(tt.path)
			if result != tt.expected {
				t.Errorf("GetParentPath() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestPropertyPathManager_GetLeafProperty(t *testing.T) {
	pm := NewPropertyPathManager()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple property",
			path:     "message",
			expected: "message",
		},
		{
			name:     "nested property",
			path:     "config.timeout",
			expected: "timeout",
		},
		{
			name:     "array property",
			path:     "items[0].name",
			expected: "name",
		},
		{
			name:     "deep nesting",
			path:     "level1.level2.level3.value",
			expected: "value",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.GetLeafProperty(tt.path)
			if result != tt.expected {
				t.Errorf("GetLeafProperty() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestPropertyPathManager_IsValidPath(t *testing.T) {
	pm := NewPropertyPathManager()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Valid paths
		{name: "empty path", path: "", expected: true},
		{name: "simple property", path: "message", expected: true},
		{name: "nested property", path: "config.timeout", expected: true},
		{name: "array property", path: "items[0].name", expected: true},
		{name: "multiple arrays", path: "data[1].users[2].name", expected: true},
		{name: "underscore in name", path: "user_name", expected: true},
		{name: "hyphen in name", path: "user-name", expected: true},
		{name: "numbers in name", path: "config2.timeout1", expected: true},
		{name: "large valid index", path: "items[999999]", expected: true},

		// Invalid paths
		{name: "double dots", path: "config..timeout", expected: false},
		{name: "leading dot", path: ".config", expected: false},
		{name: "trailing dot", path: "config.", expected: false},
		{name: "dot before bracket", path: "items.[0]", expected: false},
		{name: "unmatched opening bracket", path: "items[0.name", expected: false},
		{name: "unmatched closing bracket", path: "items0].name", expected: false},
		{name: "empty brackets", path: "items[].name", expected: false},
		{name: "invalid characters", path: "config@timeout", expected: false},
		{name: "negative index", path: "items[-1]", expected: false},
		{name: "non-numeric index", path: "items[abc]", expected: false},
		{name: "index too large", path: "items[1000000]", expected: false},
		{name: "multiple dots", path: "config...timeout", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.IsValidPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsValidPath(%q) = %t, expected %t", tt.path, result, tt.expected)
			}
		})
	}
}

func TestPropertyPathManager_UpdatePathsOnArrayRemove(t *testing.T) {
	pm := NewPropertyPathManager()

	tests := []struct {
		name         string
		arrayPath    string
		removedIndex int
		inputPaths   []string
		expected     []string
	}{
		{
			name:         "remove first item",
			arrayPath:    "items",
			removedIndex: 0,
			inputPaths: []string{
				"items[0].name",
				"items[1].name",
				"items[2].name",
				"other.property",
			},
			expected: []string{
				"items[0].name", // was items[1]
				"items[1].name", // was items[2]
				"other.property",
			},
		},
		{
			name:         "remove middle item",
			arrayPath:    "items",
			removedIndex: 1,
			inputPaths: []string{
				"items[0].name",
				"items[1].name",
				"items[2].name",
				"items[3].value",
			},
			expected: []string{
				"items[0].name", // unchanged
				"items[1].name", // was items[2]
				"items[2].value", // was items[3]
			},
		},
		{
			name:         "remove last item",
			arrayPath:    "items",
			removedIndex: 2,
			inputPaths: []string{
				"items[0].name",
				"items[1].name",
				"items[2].name",
			},
			expected: []string{
				"items[0].name", // unchanged
				"items[1].name", // unchanged
				// items[2].name removed
			},
		},
		{
			name:         "nested array paths",
			arrayPath:    "data.items",
			removedIndex: 0,
			inputPaths: []string{
				"data.items[0].title",
				"data.items[1].title",
				"data.items[0].metadata.created",
				"data.other.field",
			},
			expected: []string{
				"data.items[0].title",              // was data.items[1].title
				"data.other.field",                 // unchanged
			},
		},
		{
			name:         "empty paths array",
			arrayPath:    "items",
			removedIndex: 0,
			inputPaths:   []string{},
			expected:     []string{},
		},
		{
			name:         "no matching paths",
			arrayPath:    "items",
			removedIndex: 0,
			inputPaths:   []string{"config.timeout", "user.name"},
			expected:     []string{"config.timeout", "user.name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.UpdatePathsOnArrayRemove(tt.arrayPath, tt.removedIndex, tt.inputPaths)
			
			if len(result) != len(tt.expected) {
				t.Errorf("UpdatePathsOnArrayRemove() returned %d paths, expected %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Expected: %v", tt.expected)
				return
			}

			for i, path := range result {
				if path != tt.expected[i] {
					t.Errorf("Path %d: got %q, expected %q", i, path, tt.expected[i])
				}
			}
		})
	}
}

func TestPropertyPathManager_PathManipulation(t *testing.T) {
	pm := NewPropertyPathManager()

	t.Run("ContainsPath", func(t *testing.T) {
		paths := []string{"config.timeout", "items[0].name", "user.email"}
		
		if !pm.ContainsPath("config.timeout", paths) {
			t.Error("ContainsPath should return true for existing path")
		}
		
		if pm.ContainsPath("config.missing", paths) {
			t.Error("ContainsPath should return false for non-existing path")
		}
		
		if pm.ContainsPath("", paths) {
			t.Error("ContainsPath should return false for empty path")
		}
	})

	t.Run("AddPath", func(t *testing.T) {
		paths := []string{"config.timeout"}
		
		// Add new path
		result := pm.AddPath("items[0].name", paths)
		if len(result) != 2 || !pm.ContainsPath("items[0].name", result) {
			t.Error("AddPath should add new valid path")
		}
		
		// Add duplicate path
		result = pm.AddPath("config.timeout", result)
		if len(result) != 2 {
			t.Error("AddPath should not add duplicate path")
		}
		
		// Add invalid path
		result = pm.AddPath("invalid..path", result)
		if len(result) != 2 {
			t.Error("AddPath should not add invalid path")
		}
		
		// Add empty path
		result = pm.AddPath("", result)
		if len(result) != 2 {
			t.Error("AddPath should not add empty path")
		}
	})

	t.Run("RemovePath", func(t *testing.T) {
		paths := []string{"config.timeout", "items[0].name", "user.email"}
		
		// Remove existing path
		result := pm.RemovePath("config.timeout", paths)
		if len(result) != 2 || pm.ContainsPath("config.timeout", result) {
			t.Error("RemovePath should remove existing path")
		}
		
		// Remove non-existing path
		result = pm.RemovePath("missing.path", paths)
		if len(result) != 3 {
			t.Error("RemovePath should not change array when removing non-existing path")
		}
	})
}

func TestPropertyPathManager_FilteringUtilities(t *testing.T) {
	pm := NewPropertyPathManager()
	
	paths := []string{
		"config.timeout",
		"items[0].name",
		"items[0].value", 
		"items[1].name",
		"user.email",
		"user.profile.avatar",
	}

	t.Run("FilterPathsByPrefix", func(t *testing.T) {
		// Filter by "items" prefix
		result := pm.FilterPathsByPrefix("items", paths)
		expected := []string{"items[0].name", "items[0].value", "items[1].name"}
		
		if len(result) != len(expected) {
			t.Errorf("FilterPathsByPrefix returned %d paths, expected %d", len(result), len(expected))
		}
		
		for _, expectedPath := range expected {
			if !pm.ContainsPath(expectedPath, result) {
				t.Errorf("FilterPathsByPrefix missing expected path: %s", expectedPath)
			}
		}
		
		// Filter by "user.profile" prefix
		result = pm.FilterPathsByPrefix("user.profile", paths)
		if len(result) != 1 || result[0] != "user.profile.avatar" {
			t.Errorf("FilterPathsByPrefix should return only user.profile.avatar")
		}
	})

	t.Run("GetArrayItemPaths", func(t *testing.T) {
		// Get paths for items[0]
		result := pm.GetArrayItemPaths("items", 0, paths)
		expected := []string{"items[0].name", "items[0].value"}
		
		if len(result) != len(expected) {
			t.Errorf("GetArrayItemPaths returned %d paths, expected %d", len(result), len(expected))
		}
		
		for _, expectedPath := range expected {
			if !pm.ContainsPath(expectedPath, result) {
				t.Errorf("GetArrayItemPaths missing expected path: %s", expectedPath)
			}
		}
		
		// Get paths for items[1] 
		result = pm.GetArrayItemPaths("items", 1, paths)
		if len(result) != 1 || result[0] != "items[1].name" {
			t.Errorf("GetArrayItemPaths should return only items[1].name")
		}
		
		// Get paths for non-existing array item
		result = pm.GetArrayItemPaths("items", 5, paths)
		if len(result) != 0 {
			t.Errorf("GetArrayItemPaths should return empty array for non-existing index")
		}
	})
}

func TestPropertyPathManager_BuildPathFromSegments(t *testing.T) {
	pm := NewPropertyPathManager()

	tests := []struct {
		name     string
		segments []PropertyPathSegment
		expected string
	}{
		{
			name: "simple segments",
			segments: []PropertyPathSegment{
				{Key: "config"},
				{Key: "timeout"},
			},
			expected: "config.timeout",
		},
		{
			name: "segments with array",
			segments: []PropertyPathSegment{
				{Key: "items", Index: intPtr(0)},
				{Key: "name"},
			},
			expected: "items[0].name",
		},
		{
			name: "complex path",
			segments: []PropertyPathSegment{
				{Key: "data"},
				{Key: "users", Index: intPtr(2)},
				{Key: "profile"},
				{Key: "settings", Index: intPtr(1)},
				{Key: "value"},
			},
			expected: "data.users[2].profile.settings[1].value",
		},
		{
			name:     "empty segments",
			segments: []PropertyPathSegment{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.BuildPathFromSegments(tt.segments)
			if result != tt.expected {
				t.Errorf("BuildPathFromSegments() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// Helper function to create int pointers for tests
func intPtr(i int) *int {
	return &i
}