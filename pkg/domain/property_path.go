package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PropertyPathSegment represents a single segment in a property path
type PropertyPathSegment struct {
	Key   string `json:"key"`
	Index *int   `json:"index,omitempty"` // nil for non-array properties
}

// PropertyPathManager provides utilities for managing property paths in workflows
// It handles building, parsing, and manipulating paths for nested properties including arrays
//
// Path format examples:
// - Simple: "message"
// - Nested: "config.timeout"
// - Array: "items[0].name"
// - Deep: "data.users[2].address.street"
type PropertyPathManager struct{}

// NewPropertyPathManager creates a new PropertyPathManager instance
func NewPropertyPathManager() *PropertyPathManager {
	return &PropertyPathManager{}
}

// BuildPath creates a dot-notation path from components
// Parameters:
//   - parentPath: existing path to build upon (can be empty)
//   - propertyKey: the property key to append (can be empty if only adding index)
//   - index: optional array index to append
//
// Examples:
//   - BuildPath("", "message", nil) -> "message"
//   - BuildPath("config", "timeout", nil) -> "config.timeout"
//   - BuildPath("items", "name", &0) -> "items[0].name"
//   - BuildPath("items[0]", "metadata", nil) -> "items[0].metadata"
func (p *PropertyPathManager) BuildPath(parentPath, propertyKey string, index *int) string {
	path := strings.TrimSpace(parentPath)

	// Add array index if provided
	if index != nil && *index >= 0 && path != "" {
		path = fmt.Sprintf("%s[%d]", path, *index)
	}

	// Add property key if provided
	if propertyKey != "" {
		propertyKey = strings.TrimSpace(propertyKey)
		if propertyKey != "" {
			if path != "" {
				path = fmt.Sprintf("%s.%s", path, propertyKey)
			} else {
				path = propertyKey
			}
		}
	}

	return path
}

// BuildPathFromSegments creates a path from an array of segments
func (p *PropertyPathManager) BuildPathFromSegments(segments []PropertyPathSegment) string {
	if len(segments) == 0 {
		return ""
	}

	var parts []string
	for _, segment := range segments {
		part := segment.Key
		if segment.Index != nil && *segment.Index >= 0 {
			part = fmt.Sprintf("%s[%d]", part, *segment.Index)
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, ".")
}

// ParsePath breaks down a dot-notation path into segments
// Returns an array of PropertyPathSegment with keys and optional indices
func (p *PropertyPathManager) ParsePath(path string) ([]PropertyPathSegment, error) {
	if path == "" {
		return []PropertyPathSegment{}, nil
	}

	path = strings.TrimSpace(path)
	if path == "" {
		return []PropertyPathSegment{}, nil
	}

	// Basic format validation without calling IsValidPath to avoid circular dependency
	// Check for invalid characters (allow underscore, hyphen, and alphanumeric)
	validChars := regexp.MustCompile(`^[a-zA-Z0-9._\-\[\]]+$`)
	if !validChars.MatchString(path) {
		return nil, fmt.Errorf("invalid characters in path: '%s'", path)
	}

	// Check for empty segments or invalid dot placement
	invalidPatterns := regexp.MustCompile(`\.\.|\.\[|^\.|\.+$`)
	if invalidPatterns.MatchString(path) {
		return nil, fmt.Errorf("invalid dot placement in path: '%s'", path)
	}

	// Split by dots, but we need to be careful about array notation
	parts := strings.Split(path, ".")
	var segments []PropertyPathSegment

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check for array notation using regex
		arrayRegex := regexp.MustCompile(`^(.+?)\[(\d+)\]$`)
		matches := arrayRegex.FindStringSubmatch(part)

		if len(matches) == 3 {
			// This part has array notation
			key := matches[1]
			indexStr := matches[2]

			// Validate array index
			if indexStr == "" {
				return nil, fmt.Errorf("empty array index in path '%s'", path)
			}

			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index '%s' in path '%s'", indexStr, path)
			}

			if index < 0 {
				return nil, fmt.Errorf("negative array index '%s' in path '%s'", indexStr, path)
			}

			segments = append(segments, PropertyPathSegment{
				Key:   key,
				Index: &index,
			})
		} else {
			// Check for invalid array notation patterns
			if strings.Contains(part, "[") || strings.Contains(part, "]") {
				return nil, fmt.Errorf("invalid array notation in path segment '%s'", part)
			}

			// Regular property
			segments = append(segments, PropertyPathSegment{
				Key: part,
			})
		}
	}

	return segments, nil
}

// GetParentPath returns the parent path of a given path
// Examples:
//   - GetParentPath("config.timeout") -> "config"
//   - GetParentPath("items[0].name") -> "items[0]"
//   - GetParentPath("items[0]") -> "items"
//   - GetParentPath("message") -> ""
func (p *PropertyPathManager) GetParentPath(path string) string {
	if path == "" {
		return ""
	}

	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	// Special case: if path ends with array notation and no dot after, return the base
	// e.g., "items[0]" -> "items"
	arrayOnlyRegex := regexp.MustCompile(`^(.+)\[\d+\]$`)
	if matches := arrayOnlyRegex.FindStringSubmatch(path); len(matches) == 2 {
		// Check if there's a dot after the base name
		base := matches[1]
		if !strings.Contains(base, ".") {
			return base
		}
	}

	// Find the last dot that's not inside array brackets
	lastDotIndex := -1
	bracketDepth := 0

	for i := len(path) - 1; i >= 0; i-- {
		char := path[i]
		switch char {
		case ']':
			bracketDepth++
		case '[':
			bracketDepth--
		case '.':
			if bracketDepth == 0 {
				lastDotIndex = i
				goto found
			}
		}
	}
found:

	if lastDotIndex > 0 {
		return path[:lastDotIndex]
	}

	return ""
}

// GetLeafProperty returns the final property name from a path
// Examples:
//   - GetLeafProperty("config.timeout") -> "timeout"
//   - GetLeafProperty("items[0].name") -> "name"
//   - GetLeafProperty("message") -> "message"
func (p *PropertyPathManager) GetLeafProperty(path string) string {
	if path == "" {
		return ""
	}

	// Simple approach: find the last dot, then extract the key after it
	// Handle array notation properly
	lastDotIndex := -1
	bracketDepth := 0

	for i := len(path) - 1; i >= 0; i-- {
		char := path[i]
		switch char {
		case ']':
			bracketDepth++
		case '[':
			bracketDepth--
		case '.':
			if bracketDepth == 0 {
				lastDotIndex = i
				goto foundLeaf
			}
		}
	}
foundLeaf:

	// Extract the final part after the last dot
	var finalPart string
	if lastDotIndex >= 0 && lastDotIndex < len(path)-1 {
		finalPart = path[lastDotIndex+1:]
	} else {
		finalPart = path
	}

	// Remove array notation if present - extract just the key
	arrayRegex := regexp.MustCompile(`^(.+?)\[\d+\]$`)
	if matches := arrayRegex.FindStringSubmatch(finalPart); len(matches) == 2 {
		return matches[1]
	}

	return finalPart
}

// IsValidPath validates if a path string is well-formed
func (p *PropertyPathManager) IsValidPath(path string) bool {
	if path == "" {
		return true // Empty path is valid
	}

	// Check for invalid characters (allow underscore, hyphen, and alphanumeric)
	validChars := regexp.MustCompile(`^[a-zA-Z0-9._\-\[\]]+$`)
	if !validChars.MatchString(path) {
		return false
	}

	// Check for invalid dot placement (but allow dots after array indices like items[0].name)
	if strings.Contains(path, "..") || strings.HasPrefix(path, ".") || strings.HasSuffix(path, ".") {
		return false
	}

	// Check for dots immediately before brackets (invalid)
	if strings.Contains(path, ".[") {
		return false
	}

	// Validate balanced brackets and valid array indices using a simpler approach
	// Find all bracket pairs and validate them
	bracketPairs := regexp.MustCompile(`\[([^\[\]]*)\]`)
	matches := bracketPairs.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		if len(match) != 2 {
			return false
		}

		indexStr := match[1]
		if indexStr == "" {
			return false // Empty brackets
		}

		index, err := strconv.Atoi(indexStr)
		if err != nil || index < 0 {
			return false // Invalid index
		}

		// Reasonable bounds check (prevent memory issues)
		if index > 999999 {
			return false // Index too large
		}
	}

	// Check for unmatched brackets
	openCount := strings.Count(path, "[")
	closeCount := strings.Count(path, "]")
	if openCount != closeCount {
		return false
	}

	return true
}

// ContainsPath checks if a path exists in a list of paths
func (p *PropertyPathManager) ContainsPath(path string, paths []string) bool {
	if path == "" || len(paths) == 0 {
		return false
	}

	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

// AddPath adds a path to a list if not already present
func (p *PropertyPathManager) AddPath(path string, paths []string) []string {
	if path == "" || !p.IsValidPath(path) {
		return paths
	}

	if !p.ContainsPath(path, paths) {
		return append(paths, path)
	}
	return paths
}

// RemovePath removes a path from a list
func (p *PropertyPathManager) RemovePath(path string, paths []string) []string {
	if path == "" || len(paths) == 0 {
		return paths
	}

	var result []string
	for _, p := range paths {
		if p != path {
			result = append(result, p)
		}
	}
	return result
}

// UpdatePathsOnArrayRemove updates paths when an array item is removed
// This adjusts indices in paths that reference array items after the removed index
//
// Parameters:
//   - arrayPath: base path of the array (e.g., "items")
//   - removedIndex: index of the removed item
//   - paths: current array of paths
//
// Returns: updated array with adjusted indices and removed paths for deleted items
func (p *PropertyPathManager) UpdatePathsOnArrayRemove(arrayPath string, removedIndex int, paths []string) []string {
	if arrayPath == "" || removedIndex < 0 || len(paths) == 0 {
		return paths
	}

	var result []string
	arrayPrefix := arrayPath + "["

	for _, path := range paths {
		if !strings.HasPrefix(path, arrayPrefix) {
			// Path doesn't involve this array, keep as-is
			result = append(result, path)
			continue
		}

		// Extract the index from the path using regex
		pattern := fmt.Sprintf(`^%s\[(\d+)\](.*)$`, regexp.QuoteMeta(arrayPath))
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(path)

		if len(matches) != 3 {
			// Couldn't parse the path, keep as-is
			result = append(result, path)
			continue
		}

		index, err := strconv.Atoi(matches[1])
		if err != nil {
			// Invalid index, keep as-is
			result = append(result, path)
			continue
		}

		restOfPath := matches[2]

		if index == removedIndex {
			// This path refers to the removed item, don't include it
			continue
		} else if index > removedIndex {
			// Adjust the index down by 1
			newPath := fmt.Sprintf("%s[%d]%s", arrayPath, index-1, restOfPath)
			result = append(result, newPath)
		} else {
			// Index is before the removed item, keep unchanged
			result = append(result, path)
		}
	}

	return result
}

// FilterPathsByPrefix returns all paths that start with the given prefix
func (p *PropertyPathManager) FilterPathsByPrefix(prefix string, paths []string) []string {
	if prefix == "" {
		return paths
	}

	var result []string
	for _, path := range paths {
		if strings.HasPrefix(path, prefix) {
			result = append(result, path)
		}
	}
	return result
}

// GetArrayItemPaths returns all paths that reference a specific array item
// Example: GetArrayItemPaths("items", 0, paths) returns paths like ["items[0].name", "items[0].value"]
func (p *PropertyPathManager) GetArrayItemPaths(arrayPath string, index int, paths []string) []string {
	prefix := fmt.Sprintf("%s[%d]", arrayPath, index)
	return p.FilterPathsByPrefix(prefix, paths)
}
