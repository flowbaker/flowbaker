package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type PeekableValueResolver struct {
	executorManager domain.ExecutorIntegrationManager
	cache           map[string]string
}

func NewPeekableValueResolver(executorManager domain.ExecutorIntegrationManager) *PeekableValueResolver {
	return &PeekableValueResolver{
		executorManager: executorManager,
		cache:           make(map[string]string),
	}
}

type PeekableResolutionContext struct {
	PropertyKey     string
	DisplayValue    interface{}
	PeekableType    domain.IntegrationPeekableType
	IntegrationType domain.IntegrationType
	CredentialID    string
	WorkspaceID     string
	WorkflowNode    *domain.WorkflowNode
	DependentValues map[string]interface{}
}

type PeekableResolutionResult struct {
	ResolvedValue    interface{}
	DisplayValue     interface{}
	ResolutionMethod string
	Error            error
}

func (r *PeekableValueResolver) ResolveValue(ctx context.Context, resolutionCtx PeekableResolutionContext) *PeekableResolutionResult {

	displayStr := r.convertToString(resolutionCtx.DisplayValue)
	if displayStr == "" {
		return &PeekableResolutionResult{
			ResolvedValue:    resolutionCtx.DisplayValue,
			DisplayValue:     resolutionCtx.DisplayValue,
			ResolutionMethod: "empty_value",
		}
	}

	cacheKey := fmt.Sprintf("%s:%s", resolutionCtx.PeekableType, displayStr)
	if cachedValue, exists := r.cache[cacheKey]; exists {

		return &PeekableResolutionResult{
			ResolvedValue:    cachedValue,
			DisplayValue:     resolutionCtx.DisplayValue,
			ResolutionMethod: "cache",
		}
	}

	r.cache[cacheKey] = displayStr

	return &PeekableResolutionResult{
		ResolvedValue:    displayStr,
		DisplayValue:     resolutionCtx.DisplayValue,
		ResolutionMethod: "direct_return",
	}
}

func (r *PeekableValueResolver) findMatchingPeekValue(displayValue string, peekResults []domain.PeekResultItem) string {
	displayLower := strings.ToLower(displayValue)

	for _, item := range peekResults {
		if strings.ToLower(item.Key) == displayLower {
			return item.Value
		}
		if strings.ToLower(item.Value) == displayLower {
			return item.Value
		}
	}

	return ""
}

func (r *PeekableValueResolver) convertToString(value interface{}) string {
	if value == nil {
		return ""
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return ""
	}

	var result string
	if err := json.Unmarshal(jsonBytes, &result); err == nil {
		return result
	}

	return string(jsonBytes)
}

func (r *PeekableValueResolver) IdentifyPeekableFields(
	workflowNode *domain.WorkflowNode,
	integrationType domain.IntegrationType,
	actionType domain.IntegrationActionType,
) (map[string]domain.IntegrationPeekableType, error) {
	integration, err := r.executorManager.GetIntegration(context.Background(), integrationType)
	if err != nil {
		return nil, fmt.Errorf("failed to get integration schema: %w", err)
	}

	peekableFields := make(map[string]domain.IntegrationPeekableType)

	action := r.findActionByType(integration.Actions, actionType)
	if action == nil {
		return peekableFields, nil
	}

	for _, prop := range action.Properties {
		if !prop.Peekable || prop.PeekableType == "" {
			continue
		}

		if r.isProvidedByAgent(prop.Key, workflowNode.ProvidedByAgent) {
			peekableFields[prop.Key] = prop.PeekableType
		}
	}

	return peekableFields, nil
}

func (r *PeekableValueResolver) ClearCache() {
	r.cache = make(map[string]string)
}

func (r *PeekableValueResolver) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size":    len(r.cache),
		"entries": r.cache,
	}
}

func (r *PeekableValueResolver) findActionByType(actions []domain.IntegrationAction, actionType domain.IntegrationActionType) *domain.IntegrationAction {
	for _, action := range actions {
		if action.ActionType == actionType {
			return &action
		}
	}
	return nil
}

func (r *PeekableValueResolver) isProvidedByAgent(propertyKey string, providedByAgent []string) bool {
	for _, agentPath := range providedByAgent {
		if agentPath == propertyKey {
			return true
		}
	}
	return false
}
