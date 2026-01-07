package ai_agent

import (
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/rs/zerolog/log"
)

// SettingsMerger merges AI-provided values with user settings
type SettingsMerger struct {
	accessor      *PathAccessor
	arrayStrategy MergeStrategy
}

// MergerOption configures a SettingsMerger
type MergerOption func(*SettingsMerger)

// WithArrayStrategy sets the strategy for merging array values
func WithArrayStrategy(strategy MergeStrategy) MergerOption {
	return func(m *SettingsMerger) {
		m.arrayStrategy = strategy
	}
}

// NewSettingsMerger creates a new SettingsMerger with optional configuration
func NewSettingsMerger(opts ...MergerOption) *SettingsMerger {
	m := &SettingsMerger{
		accessor:      NewPathAccessor(),
		arrayStrategy: &ArrayAppendStrategy{},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Merge combines user settings with AI-provided values
// For paths in providedByAgent, AI values take precedence
// For array values, the configured strategy is applied (default: append existing after incoming)
func (m *SettingsMerger) Merge(
	userSettings map[string]any,
	providedByAgent []string,
	aiValues map[string]any,
) map[string]any {
	result := deepCopy(userSettings).(map[string]any)

	for _, path := range providedByAgent {
		aiValue, exists := aiValues[path]
		if !exists {
			continue
		}

		// Apply merge strategy if existing value found
		if existingValue, found := m.accessor.Get(userSettings, path); found {
			aiValue = m.arrayStrategy.Merge(existingValue, aiValue)
		}

		if err := m.accessor.Set(result, path, aiValue); err != nil {
			log.Warn().Str("path", path).Err(err).Msg("failed to set value at path")
		}
	}

	return result
}

// GetValueAtPath retrieves a value at the specified path (convenience method)
func (m *SettingsMerger) GetValueAtPath(source map[string]any, path string) (any, bool) {
	return m.accessor.Get(source, path)
}

// SetValueAtPath sets a value at the specified path (convenience method)
func (m *SettingsMerger) SetValueAtPath(target map[string]any, path string, value any) error {
	return m.accessor.Set(target, path, value)
}

func deepCopy(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			result[k] = deepCopy(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = deepCopy(v)
		}
		return result
	default:
		return v
	}
}

// PathAccessor provides Get/Set operations for nested property paths
type PathAccessor struct {
	pathManager *domain.PropertyPathManager
}

// NewPathAccessor creates a new PathAccessor instance
func NewPathAccessor() *PathAccessor {
	return &PathAccessor{
		pathManager: domain.NewPropertyPathManager(),
	}
}

// Get retrieves a value at the specified path from the source map
func (a *PathAccessor) Get(source map[string]any, path string) (any, bool) {
	segments, err := a.pathManager.ParsePath(path)
	if err != nil || len(segments) == 0 {
		return nil, false
	}
	return a.getAtSegments(source, segments)
}

func (a *PathAccessor) getAtSegments(current any, segments []domain.PropertyPathSegment) (any, bool) {
	if len(segments) == 0 {
		return current, true
	}

	segment := segments[0]

	switch t := current.(type) {
	case map[string]any:
		value, exists := t[segment.Key]
		if !exists {
			return nil, false
		}
		if segment.Index != nil {
			arr, ok := value.([]any)
			if !ok || *segment.Index >= len(arr) {
				return nil, false
			}
			return a.getAtSegments(arr[*segment.Index], segments[1:])
		}
		return a.getAtSegments(value, segments[1:])
	case []any:
		if segment.Index == nil {
			return nil, false
		}
		if *segment.Index >= len(t) {
			return nil, false
		}
		return a.getAtSegments(t[*segment.Index], segments[1:])
	default:
		return nil, false
	}
}

// Set sets a value at the specified path in the target map
func (a *PathAccessor) Set(target map[string]any, path string, value any) error {
	segments, err := a.pathManager.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	return a.setAtSegments(target, segments, value)
}

func (a *PathAccessor) setAtSegments(target any, segments []domain.PropertyPathSegment, value any) error {
	if len(segments) == 0 {
		return nil
	}

	segment := segments[0]
	isLastSegment := len(segments) == 1

	switch t := target.(type) {
	case map[string]any:
		return a.setInMap(t, segment, segments[1:], value, isLastSegment)
	case []any:
		return a.setInArray(t, segment, segments[1:], value, isLastSegment)
	default:
		return fmt.Errorf("unexpected target type: %T", target)
	}
}

func (a *PathAccessor) setInMap(target map[string]any, segment domain.PropertyPathSegment, remainingSegments []domain.PropertyPathSegment, value any, isLastSegment bool) error {
	key := segment.Key

	if segment.Index != nil {
		existing, exists := target[key]
		var arr []any
		if exists {
			if a, ok := existing.([]any); ok {
				arr = a
			} else {
				arr = []any{}
			}
		} else {
			arr = []any{}
		}

		arr = a.ensureArrayIndex(arr, *segment.Index)
		target[key] = arr

		if isLastSegment {
			arr = append(arr, value)
			target[key] = arr
			return nil
		}

		if arr[*segment.Index] == nil {
			arr[*segment.Index] = make(map[string]any)
		}
		return a.setAtSegments(arr[*segment.Index], remainingSegments, value)
	}

	if isLastSegment {
		target[key] = value
		return nil
	}

	existing, exists := target[key]
	if !exists {
		if len(remainingSegments) > 0 && remainingSegments[0].Index != nil {
			target[key] = []any{}
		} else {
			target[key] = make(map[string]any)
		}
		existing = target[key]
	}

	return a.setAtSegments(existing, remainingSegments, value)
}

func (a *PathAccessor) setInArray(target []any, segment domain.PropertyPathSegment, remainingSegments []domain.PropertyPathSegment, value any, isLastSegment bool) error {
	if segment.Index == nil {
		return fmt.Errorf("expected array index for segment %s", segment.Key)
	}

	index := *segment.Index
	target = a.ensureArrayIndex(target, index)

	if isLastSegment {
		target[index] = value
		return nil
	}

	if target[index] == nil {
		target[index] = make(map[string]any)
	}

	return a.setAtSegments(target[index], remainingSegments, value)
}

func (a *PathAccessor) ensureArrayIndex(arr []any, index int) []any {
	for len(arr) <= index {
		arr = append(arr, make(map[string]any))
	}
	return arr
}

// MergeStrategy defines how to merge two values
type MergeStrategy interface {
	Merge(existing, incoming any) any
}

// ArrayAppendStrategy appends existing array items after incoming items
// Result: [incoming items..., existing items...]
type ArrayAppendStrategy struct{}

func (s *ArrayAppendStrategy) Merge(existing, incoming any) any {
	incomingArr, isIncomingArray := incoming.([]any)
	if !isIncomingArray {
		return incoming
	}

	existingArr, isExistingArray := existing.([]any)
	if !isExistingArray {
		return incoming
	}

	result := make([]any, 0, len(incomingArr)+len(existingArr))
	result = append(result, incomingArr...)
	result = append(result, existingArr...)
	return result
}

// ReplaceStrategy simply replaces existing with incoming
type ReplaceStrategy struct{}

func (s *ReplaceStrategy) Merge(existing, incoming any) any {
	return incoming
}
