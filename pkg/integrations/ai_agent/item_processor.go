package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/internal/domain"

	"github.com/rs/zerolog/log"
)

// ActionExecution represents an executed action
type ActionExecution struct {
	Iteration  int                    `json:"iteration"`
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Success    bool                   `json:"success"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
}

// ItemProcessor handles item processing for AI Agent V2
type ItemProcessor struct {
	parameterBinder domain.IntegrationParameterBinder
}

// NewItemProcessor creates a new item processor
func NewItemProcessor(parameterBinder domain.IntegrationParameterBinder) *ItemProcessor {
	return &ItemProcessor{
		parameterBinder: parameterBinder,
	}
}

// ProcessInputItems processes input items from upstream workflow nodes
func (p *ItemProcessor) ProcessInputItems(ctx context.Context, params domain.IntegrationInput) ([]InputItem, error) {
	var inputItems []InputItem

	// Process each input payload
	for inputID, payload := range params.PayloadByInputID {
		items, err := payload.ToItems()
		if err != nil {
			log.Warn().
				Err(err).
				Str("input_id", inputID).
				Msg("Failed to parse items from payload")
			continue
		}

		// Convert domain items to InputItems
		for i, item := range items {
			inputItem := InputItem{
				InputID: inputID,
				Index:   i,
				Data:    item,
			}
			inputItems = append(inputItems, inputItem)
		}

		log.Debug().
			Str("input_id", inputID).
			Int("item_count", len(items)).
			Msg("Processed input items")
	}

	log.Info().
		Int("total_input_items", len(inputItems)).
		Msg("Input items processed")

	return inputItems, nil
}

// CreateToolCallItems creates items for tool execution
func (p *ItemProcessor) CreateToolCallItems(ctx context.Context, toolCall domain.ToolCall, inputItems []InputItem) (domain.Payload, error) {
	// Create a tool call item
	toolCallItem := map[string]interface{}{
		"_tool_call":    true,
		"_tool_name":    toolCall.Name,
		"_tool_call_id": toolCall.ID,
	}

	// Add tool call arguments
	for key, value := range toolCall.Arguments {
		toolCallItem[key] = value
	}

	// Add context from input items if available
	if len(inputItems) > 0 {
		toolCallItem["_input_context"] = p.extractInputContext(inputItems)

		// If there's only one input item, merge its data
		if len(inputItems) == 1 {
			for key, value := range inputItems[0].Data.(map[string]interface{}) {
				// Don't override tool call arguments
				if _, exists := toolCallItem[key]; !exists {
					toolCallItem[key] = value
				}
			}
		}
	}

	// Create payload with tool call item
	toolCallItems := []interface{}{toolCallItem}
	payloadJSON, err := json.Marshal(toolCallItems)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool call items: %w", err)
	}

	log.Debug().
		Str("tool_name", toolCall.Name).
		Str("tool_call_id", toolCall.ID).
		Interface("item_data", toolCallItem).
		Msg("Created tool call item")

	return domain.Payload(payloadJSON), nil
}

// CreateOutputItems creates output items from conversation result
func (p *ItemProcessor) CreateOutputItems(ctx context.Context, result *ConversationResult, toolExecutions []interface{}) ([]domain.Item, error) {
	outputItems := make([]domain.Item, 0)

	// Create main result item
	resultItem := map[string]interface{}{
		"final_response":    result.FinalResponse,
		"conversation_id":   generateConversationID(),
		"tool_executions":   len(toolExecutions),
		"execution_summary": p.createExecutionSummary(toolExecutions),
	}

	// Add tool execution results if available
	if len(toolExecutions) > 0 {
		resultItem["tools_used"] = p.extractToolNames(toolExecutions)
		resultItem["tool_results"] = toolExecutions
	}

	outputItems = append(outputItems, resultItem)

	return outputItems, nil
}

// ExtractPromptContext extracts relevant context from input items for LLM prompts
func (p *ItemProcessor) ExtractPromptContext(inputItems []InputItem) string {
	if len(inputItems) == 0 {
		return ""
	}

	context := "Available data from upstream nodes:\n"

	for i, item := range inputItems {
		// Limit to first few items to avoid overwhelming the prompt
		if i >= 3 {
			context += fmt.Sprintf("... and %d more items\n", len(inputItems)-3)
			break
		}

		context += fmt.Sprintf("Item %d (%s): %s\n", i+1, item.InputID, p.formatItemForPrompt(item.Data))
	}

	return context
}

// Helper methods

func (p *ItemProcessor) extractInputContext(inputItems []InputItem) map[string]interface{} {
	context := map[string]interface{}{
		"item_count": len(inputItems),
		"input_ids":  make([]string, 0),
	}

	inputIDs := make(map[string]bool)
	for _, item := range inputItems {
		if !inputIDs[item.InputID] {
			context["input_ids"] = append(context["input_ids"].([]string), item.InputID)
			inputIDs[item.InputID] = true
		}
	}

	// Add summary of first item if available
	if len(inputItems) > 0 {
		context["first_item_summary"] = p.summarizeItem(inputItems[0].Data)
	}

	return context
}

func (p *ItemProcessor) summarizeItem(item interface{}) map[string]interface{} {
	summary := map[string]interface{}{
		"type": "unknown",
	}

	if itemMap, ok := item.(map[string]interface{}); ok {
		summary["type"] = "object"
		summary["keys"] = make([]string, 0, len(itemMap))

		for key := range itemMap {
			summary["keys"] = append(summary["keys"].([]string), key)
		}
	}

	return summary
}

func (p *ItemProcessor) formatItemForPrompt(item interface{}) string {
	if itemMap, ok := item.(map[string]interface{}); ok {
		// Try to find meaningful fields to display
		if title, exists := itemMap["title"]; exists {
			return fmt.Sprintf("Title: %v", title)
		}
		if name, exists := itemMap["name"]; exists {
			return fmt.Sprintf("Name: %v", name)
		}
		if description, exists := itemMap["description"]; exists {
			return fmt.Sprintf("Description: %v", description)
		}

		// Fallback to showing keys
		keys := make([]string, 0, len(itemMap))
		for key := range itemMap {
			keys = append(keys, key)
			if len(keys) >= 5 { // Limit to first 5 keys
				break
			}
		}
		return fmt.Sprintf("Object with keys: %v", keys)
	}

	// Fallback to string representation
	return fmt.Sprintf("%v", item)
}

func (p *ItemProcessor) createExecutionSummary(toolExecutions []interface{}) string {
	if len(toolExecutions) == 0 {
		return "No tools executed"
	}

	if len(toolExecutions) == 1 {
		return "1 tool executed successfully"
	}

	return fmt.Sprintf("%d tools executed", len(toolExecutions))
}

func (p *ItemProcessor) extractToolNames(toolExecutions []interface{}) []string {
	names := make([]string, 0, len(toolExecutions))

	for _, exec := range toolExecutions {
		// Try function calling execution struct first
		if fcExec, ok := exec.(FunctionCallExecution); ok {
			names = append(names, fcExec.ToolName)
			continue
		}

		// Try ReAct action execution (for compatibility)
		if actionExec, ok := exec.(ActionExecution); ok {
			names = append(names, actionExec.ToolName)
			continue
		}

		// Fallback to map format
		if execMap, ok := exec.(map[string]interface{}); ok {
			if toolName, exists := execMap["tool_name"]; exists {
				names = append(names, fmt.Sprintf("%v", toolName))
			}
		}
	}

	return names
}

func generateConversationID() string {
	return fmt.Sprintf("conv_%d", getCurrentTimestamp())
}

// InputItem represents an input item from upstream workflow nodes
type InputItem struct {
	InputID string      `json:"input_id"`
	Index   int         `json:"index"`
	Data    interface{} `json:"data"`
}
