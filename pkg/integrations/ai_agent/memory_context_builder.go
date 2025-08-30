package ai_agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/internal/domain"

	"github.com/rs/zerolog/log"
)

// MemoryContextBuilder handles formatting memory data into context for LLM prompts
type MemoryContextBuilder interface {
	// BuildContext formats raw memory data into structured context
	BuildContext(memoryData string) (string, error)

	// EnhanceSystemPrompt adds memory context to a system prompt
	EnhanceSystemPrompt(basePrompt, memoryContext string) string

	// FormatConversationSummary creates a summary of a conversation for context
	FormatConversationSummary(conversation *domain.AgentConversation) string

	// FormatMultipleConversations formats multiple conversations into context
	FormatMultipleConversations(conversations []domain.AgentConversation) string
}

// DefaultMemoryContextBuilder implements MemoryContextBuilder
type DefaultMemoryContextBuilder struct {
	config ConversationMemoryConfig
}

// NewMemoryContextBuilder creates a new memory context builder
func NewMemoryContextBuilder(config ConversationMemoryConfig) MemoryContextBuilder {
	return &DefaultMemoryContextBuilder{
		config: config,
	}
}

// BuildContext formats raw memory data into structured context
func (b *DefaultMemoryContextBuilder) BuildContext(memoryData string) (string, error) {
	if memoryData == "" {
		return "", nil
	}

	// Try to parse as single conversation
	var conversation domain.AgentConversation
	if err := json.Unmarshal([]byte(memoryData), &conversation); err == nil {
		return b.FormatConversationSummary(&conversation), nil
	}

	// Try to parse as conversation array
	var conversations []domain.AgentConversation
	if err := json.Unmarshal([]byte(memoryData), &conversations); err == nil {
		return b.FormatMultipleConversations(conversations), nil
	}

	// If parsing fails, treat as raw text
	log.Debug().Msg("Memory data is not structured conversation data, treating as raw context")
	return b.formatRawMemoryData(memoryData), nil
}

// EnhanceSystemPrompt adds memory context to a system prompt
func (b *DefaultMemoryContextBuilder) EnhanceSystemPrompt(basePrompt, memoryContext string) string {
	if memoryContext == "" {
		return basePrompt
	}

	// Check if context is too long
	if len(memoryContext) > b.config.MaxContextLength {
		log.Debug().
			Int("memory_context_length", len(memoryContext)).
			Int("max_length", b.config.MaxContextLength).
			Msg("Memory context too long, truncating")

		memoryContext = b.truncateContext(memoryContext)
	}

	// Add memory context section to system prompt
	enhancedPrompt := basePrompt + "\n\n" +
		"## Previous Conversation Context\n" +
		"Here is relevant context from previous conversations with this agent:\n\n" +
		memoryContext + "\n\n" +
		"Use this context to provide better assistance and maintain continuity, " +
		"but focus primarily on the current user request."

	return enhancedPrompt
}

// FormatConversationSummary creates a summary of a conversation for context
func (b *DefaultMemoryContextBuilder) FormatConversationSummary(conversation *domain.AgentConversation) string {
	if conversation == nil {
		return ""
	}

	var summary strings.Builder

	// Header with timestamp
	summary.WriteString(fmt.Sprintf("**Conversation from %s:**\n",
		conversation.CreatedAt.Format("2006-01-02 15:04")))

	// User prompt
	if conversation.UserPrompt != "" {
		summary.WriteString(fmt.Sprintf("User Request: %s\n", conversation.UserPrompt))
	}

	// Final response (truncated if too long)
	if conversation.FinalResponse != "" {
		response := conversation.FinalResponse
		if len(response) > 200 {
			response = response[:200] + "..."
		}
		summary.WriteString(fmt.Sprintf("Agent Response: %s\n", response))
	}

	// Tools used
	if len(conversation.ToolsUsed) > 0 && b.config.IncludeToolUsage {
		summary.WriteString(fmt.Sprintf("Tools Used: %s\n",
			strings.Join(conversation.ToolsUsed, ", ")))
	}

	// Key metadata
	if conversation.Metadata != nil {
		if executionTime, ok := conversation.Metadata["execution_time"]; ok {
			summary.WriteString(fmt.Sprintf("Execution Time: %.2fs\n", executionTime))
		}

		if toolFailures, ok := conversation.Metadata["tool_failures"]; ok {
			if failures, ok := toolFailures.(float64); ok && failures > 0 {
				summary.WriteString(fmt.Sprintf("Tool Failures: %.0f\n", failures))
			}
		}
	}

	// Status
	if conversation.Status != "" && conversation.Status != "completed" {
		summary.WriteString(fmt.Sprintf("Status: %s\n", conversation.Status))
	}

	summary.WriteString("\n")
	return summary.String()
}

// FormatMultipleConversations formats multiple conversations into context
func (b *DefaultMemoryContextBuilder) FormatMultipleConversations(conversations []domain.AgentConversation) string {
	if len(conversations) == 0 {
		return ""
	}

	var context strings.Builder
	context.WriteString("## Recent Conversations\n\n")

	// Limit the number of conversations to include
	maxConversations := b.config.ConversationCount
	if maxConversations <= 0 {
		maxConversations = 5
	}

	count := 0
	for i := len(conversations) - 1; i >= 0 && count < maxConversations; i-- {
		conversation := conversations[i]
		context.WriteString(b.FormatConversationSummary(&conversation))
		count++
	}

	return context.String()
}

// formatRawMemoryData formats unstructured memory data
func (b *DefaultMemoryContextBuilder) formatRawMemoryData(memoryData string) string {
	if len(memoryData) > 500 {
		return fmt.Sprintf("## Previous Context\n%s...\n\n", memoryData[:500])
	}
	return fmt.Sprintf("## Previous Context\n%s\n\n", memoryData)
}

// truncateContext truncates memory context to fit within limits
func (b *DefaultMemoryContextBuilder) truncateContext(context string) string {
	if len(context) <= b.config.MaxContextLength {
		return context
	}

	// Try to truncate at sentence boundaries
	truncated := context[:b.config.MaxContextLength]
	lastSentence := strings.LastIndex(truncated, ". ")
	if lastSentence > b.config.MaxContextLength/2 {
		truncated = truncated[:lastSentence+1]
	}

	return truncated + "\n\n[Context truncated due to length...]"
}

// ParseConversationFromMemory safely parses memory data into a conversation
func ParseConversationFromMemory(memoryData string) (*domain.AgentConversation, error) {
	if memoryData == "" {
		return nil, fmt.Errorf("empty memory data")
	}

	var conversation domain.AgentConversation
	if err := json.Unmarshal([]byte(memoryData), &conversation); err != nil {
		return nil, fmt.Errorf("failed to parse conversation from memory: %w", err)
	}

	return &conversation, nil
}
