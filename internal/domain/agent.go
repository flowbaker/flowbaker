package domain

import (
	"context"
	"time"
)

// Message represents a single message in a conversation
type ConversationMessage struct {
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// ToolCall represents a tool call request from the LLM
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// Tool represents a tool available to the LLM
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ModelResponse represents the response from an LLM
type ModelResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// AgentConversation represents a stored AI agent conversation
type AgentConversation struct {
	ID             string                 `bson:"_id" json:"id"`
	WorkspaceID    string                 `bson:"workspace_id" json:"workspace_id"`
	SessionID      string                 `bson:"session_id" json:"session_id"`
	ConversationID string                 `bson:"conversation_id" json:"conversation_id"`
	Messages       []AgentMessage         `bson:"messages" json:"messages"`
	CreatedAt      time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time              `bson:"updated_at" json:"updated_at"`
	UserPrompt     string                 `bson:"user_prompt" json:"user_prompt"`
	FinalResponse  string                 `bson:"final_response" json:"final_response"`
	ToolsUsed      []string               `bson:"tools_used" json:"tools_used"`
	Status         string                 `bson:"status" json:"status"` // completed, failed, interrupted
	Metadata       map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

// AgentMessage represents a single message in an agent conversation
type AgentMessage struct {
	Role        string                 `bson:"role" json:"role"` // user, assistant, tool
	Content     string                 `bson:"content" json:"content"`
	ToolCalls   []AgentToolCall        `bson:"tool_calls,omitempty" json:"tool_calls,omitempty"`
	ToolResults []AgentToolResult      `bson:"tool_results,omitempty" json:"tool_results,omitempty"`
	Timestamp   time.Time              `bson:"timestamp" json:"timestamp"`
	Metadata    map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

// AgentToolCall represents a tool call in a conversation
type AgentToolCall struct {
	ID        string                 `bson:"id" json:"id"`
	Name      string                 `bson:"name" json:"name"`
	Arguments map[string]interface{} `bson:"arguments" json:"arguments"`
}

// AgentToolResult represents a tool execution result
type AgentToolResult struct {
	ToolCallID string `bson:"tool_call_id" json:"tool_call_id"`
	Content    string `bson:"content" json:"content"`
}

// GenerateRequest represents a request to generate a response
type GenerateRequest struct {
	Messages     []ConversationMessage `json:"messages"`
	SystemPrompt string                `json:"system_prompt,omitempty"`
	Tools        []Tool                `json:"tools,omitempty"`
	Temperature  float32               `json:"temperature,omitempty"`
	MaxTokens    int                   `json:"max_tokens,omitempty"`
	Model        string                `json:"model,omitempty"`
}

type IntegrationLLM interface {
	GenerateWithConversation(ctx context.Context, req GenerateRequest) (ModelResponse, error)
}

type IntegrationMemory interface {
	StoreConversation(ctx context.Context, request StoreConversationRequest) error
	RetrieveConversations(ctx context.Context, filter ConversationFilter) ([]AgentConversation, error)
}

type StoreConversationRequest struct {
	Conversation *AgentConversation `json:"conversation"`
	Embedding    []float32          `json:"embedding"`
	TTL          int                `json:"ttl"`
	PartitionKey string             `json:"partition_key"`
}

type ConversationFilter struct {
	SessionID    string    `json:"session_id"`
	WorkspaceID  string    `json:"workspace_id"`
	Limit        int       `json:"limit"`
	Since        time.Time `json:"since"`
	Status       string    `json:"status"`
	IncludeTools bool      `json:"include_tools"`
}
