package ai_agent

import (
	"fmt"
	"reflect"

	"flowbaker/internal/domain"

	"github.com/rs/zerolog/log"
)

// AIParameterResolver handles merging of AI-provided parameters with preset values
// using PropertyPathManager for nested path support
type AIParameterResolver struct {
	pathManager *domain.PropertyPathManager
}

// NewAIParameterResolver creates a new parameter resolver
func NewAIParameterResolver() *AIParameterResolver {
	return &AIParameterResolver{
		pathManager: domain.NewPropertyPathManager(),
	}
}

// ParameterResolutionContext provides context for parameter resolution
type ParameterResolutionContext struct {
	// WorkflowNode contains the ProvidedByAgent configuration
	WorkflowNode *domain.WorkflowNode
	// PresetSettings are the configured values from the workflow node
	PresetSettings map[string]interface{}
	// AIProvidedArguments are the parameters provided by the AI in the tool call
	AIProvidedArguments map[string]interface{}
	// ToolName for logging and debugging
	ToolName string
}

// ParameterResolutionResult contains the resolved parameters and metadata
type ParameterResolutionResult struct {
	// ResolvedSettings contains the final merged parameters
	ResolvedSettings map[string]interface{}
	// ResolutionLog contains information about which sources were used for each parameter
	ResolutionLog []ParameterResolution
}

// ParameterResolution tracks how a specific parameter was resolved
type ParameterResolution struct {
	Path             string          `json:"path"`
	Source           ParameterSource `json:"source"`
	Value            interface{}     `json:"value"`
	ProvidedByAgent  bool            `json:"provided_by_agent"`
	AIValueAvailable bool            `json:"ai_value_available"`
}

// ParameterSource indicates where a parameter value came from
type ParameterSource string

const (
	ParameterSourceAI      ParameterSource = "ai_provided"  // Value from AI tool call
	ParameterSourcePreset  ParameterSource = "preset_value" // Value from workflow configuration
	ParameterSourceMissing ParameterSource = "missing"      // Parameter not found in either source
)

// ResolveParameters merges AI-provided arguments with preset values based on ProvidedByAgent paths
func (r *AIParameterResolver) ResolveParameters(ctx ParameterResolutionContext) (*ParameterResolutionResult, error) {
	log.Debug().
		Str("tool_name", ctx.ToolName).
		Int("provided_by_agent_count", len(ctx.WorkflowNode.ProvidedByAgent)).
		Int("preset_settings_count", len(ctx.PresetSettings)).
		Int("ai_arguments_count", len(ctx.AIProvidedArguments)).
		Msg("Starting parameter resolution")

	result := &ParameterResolutionResult{
		ResolvedSettings: make(map[string]interface{}),
		ResolutionLog:    []ParameterResolution{},
	}

	// Step 1: Start with all preset settings as the base
	for key, value := range ctx.PresetSettings {
		// Skip special internal keys
		if r.isSpecialKey(key) {
			continue
		}
		result.ResolvedSettings[key] = value
	}

	// Step 2: Process ProvidedByAgent paths - these should use AI values when available
	for _, path := range ctx.WorkflowNode.ProvidedByAgent {
		if !r.pathManager.IsValidPath(path) {
			log.Warn().
				Str("tool_name", ctx.ToolName).
				Str("path", path).
				Msg("Invalid path in ProvidedByAgent, skipping")
			continue
		}

		resolution := r.resolveAgentProvidedPath(path, ctx)
		result.ResolutionLog = append(result.ResolutionLog, resolution)

		// Apply the resolved value to the result
		if resolution.Source != ParameterSourceMissing {
			err := r.setValueAtPath(result.ResolvedSettings, path, resolution.Value)
			if err != nil {
				log.Error().
					Err(err).
					Str("path", path).
					Str("tool_name", ctx.ToolName).
					Msg("Failed to set resolved parameter value")
				continue
			}
		}
	}

	// Step 3: Log any AI arguments that are NOT in ProvidedByAgent (these should be ignored)
	for key, value := range ctx.AIProvidedArguments {
		if !r.isPathInProvidedByAgent(key, ctx.WorkflowNode.ProvidedByAgent) && !r.isSpecialKey(key) {
			// This AI parameter is not marked as provided by agent - it should be ignored
			log.Warn().
				Str("tool_name", ctx.ToolName).
				Str("parameter", key).
				Interface("ai_value", value).
				Msg("AI provided parameter not in ProvidedByAgent list - ignoring AI value, using preset")

			// Don't override the preset value - just log that we're ignoring the AI value
			if _, exists := result.ResolvedSettings[key]; exists {
				result.ResolutionLog = append(result.ResolutionLog, ParameterResolution{
					Path:             key,
					Source:           ParameterSourcePreset,
					Value:            result.ResolvedSettings[key],
					ProvidedByAgent:  false,
					AIValueAvailable: true, // AI provided it but we ignore it
				})
			}
		}
	}

	log.Info().
		Str("tool_name", ctx.ToolName).
		Int("final_parameter_count", len(result.ResolvedSettings)).
		Int("resolution_entries", len(result.ResolutionLog)).
		Msg("Parameter resolution completed")

	return result, nil
}

// resolveAgentProvidedPath resolves a single path marked as provided by agent
func (r *AIParameterResolver) resolveAgentProvidedPath(path string, ctx ParameterResolutionContext) ParameterResolution {
	// Try to get value from AI arguments first (since this path is marked as provided by agent)
	aiValue, aiAvailable := r.getValueAtPath(ctx.AIProvidedArguments, path)

	if aiAvailable && aiValue != nil {
		// AI provided the value - use it
		return ParameterResolution{
			Path:             path,
			Source:           ParameterSourceAI,
			Value:            aiValue,
			ProvidedByAgent:  true,
			AIValueAvailable: true,
		}
	}

	// AI didn't provide the value - try fallback to preset
	presetValue, presetAvailable := r.getValueAtPath(ctx.PresetSettings, path)

	if presetAvailable && presetValue != nil {
		log.Warn().
			Str("tool_name", ctx.ToolName).
			Str("path", path).
			Msg("Agent-provided parameter missing from AI - using preset fallback")

		return ParameterResolution{
			Path:             path,
			Source:           ParameterSourcePreset,
			Value:            presetValue,
			ProvidedByAgent:  true,
			AIValueAvailable: false,
		}
	}

	// Neither AI nor preset has this value
	log.Error().
		Str("tool_name", ctx.ToolName).
		Str("path", path).
		Msg("Agent-provided parameter missing from both AI and preset values")

	return ParameterResolution{
		Path:             path,
		Source:           ParameterSourceMissing,
		Value:            nil,
		ProvidedByAgent:  true,
		AIValueAvailable: false,
	}
}

// getValueAtPath extracts a value from a nested map using a property path
func (r *AIParameterResolver) getValueAtPath(data map[string]interface{}, path string) (interface{}, bool) {
	segments, err := r.pathManager.ParsePath(path)
	if err != nil {
		return nil, false
	}

	current := data

	for i, segment := range segments {
		// Get the value for this segment key
		value, exists := current[segment.Key]
		if !exists {
			return nil, false
		}

		// If this is the last segment, return the value
		if i == len(segments)-1 {
			// Handle array index access for final segment
			if segment.Index != nil {
				return r.getArrayElement(value, *segment.Index)
			}
			return value, true
		}

		// For intermediate segments, navigate deeper
		if segment.Index != nil {
			// Handle array access
			arrayValue, arrayOk := r.getArrayElement(value, *segment.Index)
			if !arrayOk {
				return nil, false
			}

			// Convert to map for next iteration
			if nextMap, ok := arrayValue.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil, false
			}
		} else {
			// Handle object access
			if nextMap, ok := value.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil, false
			}
		}
	}

	return nil, false
}

// setValueAtPath sets a value in a nested map using a property path
func (r *AIParameterResolver) setValueAtPath(data map[string]interface{}, path string, value interface{}) error {
	segments, err := r.pathManager.ParsePath(path)
	if err != nil {
		return fmt.Errorf("invalid path '%s': %w", path, err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// For simple paths (single segment), set directly
	if len(segments) == 1 {
		segment := segments[0]
		if segment.Index != nil {
			return fmt.Errorf("cannot set array index at root level")
		}
		data[segment.Key] = value
		return nil
	}

	// Navigate through the path, creating structure as needed
	current := data

	for i, segment := range segments[:len(segments)-1] {
		// Handle current segment navigation
		if segment.Index != nil {
			// This segment represents an array access
			current, err = r.navigateArraySegment(current, segment, i, segments)
			if err != nil {
				return fmt.Errorf("failed to navigate array segment at '%s': %w", segment.Key, err)
			}
		} else {
			// This segment represents an object access
			current, err = r.navigateObjectSegment(current, segment, i, segments)
			if err != nil {
				return fmt.Errorf("failed to navigate object segment at '%s': %w", segment.Key, err)
			}
		}
	}

	// Set the final value
	finalSegment := segments[len(segments)-1]
	if finalSegment.Index != nil {
		// Final segment is an array element
		return r.setArrayElement(current, finalSegment, value)
	} else {
		// Final segment is an object property
		current[finalSegment.Key] = value
		return nil
	}
}

// navigateArraySegment navigates through an array segment, creating structure if needed
func (r *AIParameterResolver) navigateArraySegment(current map[string]interface{}, segment domain.PropertyPathSegment, segmentIndex int, allSegments []domain.PropertyPathSegment) (map[string]interface{}, error) {
	// Ensure the array exists
	if _, exists := current[segment.Key]; !exists {
		current[segment.Key] = []interface{}{}
	}

	// Get the array
	arrayValue, ok := current[segment.Key].([]interface{})
	if !ok {
		return nil, fmt.Errorf("segment '%s' is not an array", segment.Key)
	}

	// Ensure array is large enough
	requiredSize := *segment.Index + 1
	for len(arrayValue) < requiredSize {
		// Determine what to append based on next segment
		if segmentIndex+1 < len(allSegments)-1 {
			// More segments after this, append object
			arrayValue = append(arrayValue, make(map[string]interface{}))
		} else {
			// This is second to last segment, check final segment
			finalSegment := allSegments[len(allSegments)-1]
			if finalSegment.Index != nil {
				// Final is array, append array
				arrayValue = append(arrayValue, []interface{}{})
			} else {
				// Final is object property, append object
				arrayValue = append(arrayValue, make(map[string]interface{}))
			}
		}
	}

	// Update the array in the current object
	current[segment.Key] = arrayValue

	// Get the element at the specified index
	element := arrayValue[*segment.Index]
	if elementMap, ok := element.(map[string]interface{}); ok {
		return elementMap, nil
	}

	return nil, fmt.Errorf("array element at index %d is not an object", *segment.Index)
}

// navigateObjectSegment navigates through an object segment, creating structure if needed
func (r *AIParameterResolver) navigateObjectSegment(current map[string]interface{}, segment domain.PropertyPathSegment, _ int, _ []domain.PropertyPathSegment) (map[string]interface{}, error) {
	// Ensure the object property exists
	if _, exists := current[segment.Key]; !exists {
		// For object segments (no index), we always create an object, not an array
		// The next segment will handle array creation if needed
		current[segment.Key] = make(map[string]interface{})
	}

	// Navigate to the next level
	value := current[segment.Key]
	if nextMap, ok := value.(map[string]interface{}); ok {
		return nextMap, nil
	} else if _, isArray := value.([]interface{}); isArray {
		return nil, fmt.Errorf("expected object but found array at segment '%s'", segment.Key)
	}

	return nil, fmt.Errorf("segment '%s' is not an object", segment.Key)
}

// setArrayElement sets a value in an array element
func (r *AIParameterResolver) setArrayElement(current map[string]interface{}, segment domain.PropertyPathSegment, value interface{}) error {
	// Ensure the array exists
	if _, exists := current[segment.Key]; !exists {
		current[segment.Key] = []interface{}{}
	}

	// Get the array
	arrayValue, ok := current[segment.Key].([]interface{})
	if !ok {
		return fmt.Errorf("segment '%s' is not an array", segment.Key)
	}

	// Ensure array is large enough
	requiredSize := *segment.Index + 1
	for len(arrayValue) < requiredSize {
		arrayValue = append(arrayValue, nil)
	}

	// Set the value at the specified index
	arrayValue[*segment.Index] = value

	// Update the array in the current object
	current[segment.Key] = arrayValue

	return nil
}

// getArrayElement safely gets an element from an array-like value
func (r *AIParameterResolver) getArrayElement(value interface{}, index int) (interface{}, bool) {
	if index < 0 {
		return nil, false
	}

	switch arr := value.(type) {
	case []interface{}:
		if index < len(arr) {
			return arr[index], true
		}
	case []map[string]interface{}:
		if index < len(arr) {
			return arr[index], true
		}
	default:
		// Try reflection for other slice types
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice && index < rv.Len() {
			return rv.Index(index).Interface(), true
		}
	}

	return nil, false
}

// isPathInProvidedByAgent checks if a path is in the ProvidedByAgent list
func (r *AIParameterResolver) isPathInProvidedByAgent(path string, providedByAgent []string) bool {
	log.Debug().
		Str("path", path).
		Interface("provided_by_agent", providedByAgent).
		Msg("Checking if path is in ProvidedByAgent")

	return r.pathManager.ContainsPath(path, providedByAgent)
}

// isSpecialKey checks if a key should be skipped during parameter resolution
func (r *AIParameterResolver) isSpecialKey(key string) bool {
	specialKeys := []string{
		"credential_id",
		"node_id",
		"integration_type",
		"action_type",
		"_tool_call",
		"_tool_name",
		"_tool_call_id",
	}

	for _, special := range specialKeys {
		if key == special {
			return true
		}
	}
	return false
}

// GetResolutionSummary returns a human-readable summary of the parameter resolution
func (r *AIParameterResolver) GetResolutionSummary(result *ParameterResolutionResult) string {
	summary := fmt.Sprintf("Parameter Resolution Summary (%d total parameters):\n", len(result.ResolutionLog))

	aiCount := 0
	presetCount := 0
	missingCount := 0

	for _, resolution := range result.ResolutionLog {
		switch resolution.Source {
		case ParameterSourceAI:
			aiCount++
		case ParameterSourcePreset:
			presetCount++
		case ParameterSourceMissing:
			missingCount++
		}
	}

	summary += fmt.Sprintf("  - AI Provided: %d\n", aiCount)
	summary += fmt.Sprintf("  - Preset Values: %d\n", presetCount)
	summary += fmt.Sprintf("  - Missing: %d\n", missingCount)

	if missingCount > 0 {
		summary += "\nWARNING: Some parameters marked as 'provided by agent' were missing!\n"
	}

	return summary
}
