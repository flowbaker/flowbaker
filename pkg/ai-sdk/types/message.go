package types

import "time"

// Message represents a single message in a conversation
type Message struct {
	ConversationID string                 `json:"conversation_id"`
	Role           MessageRole            `json:"role"`
	Content        string                 `json:"content"`
	ToolCalls      []ToolCall             `json:"tool_calls,omitempty"`
	ToolResults    []ToolResult           `json:"tool_results,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

// MessageRole defines the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

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

// Conversation represents a complete conversation
type Conversation struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	UserID    string                 `json:"user_id,omitempty"`
	Messages  []Message              `json:"messages"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Status    ConversationStatus     `json:"status"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (c *Conversation) IsInterrupted() bool {
	return c.Status == StatusInterrupted
}

func (c *Conversation) IsCompleted() bool {
	return c.Status == StatusCompleted
}

func (c *Conversation) IsFailed() bool {
	return c.Status == StatusFailed
}

func (c *Conversation) IsActive() bool {
	return c.Status == StatusActive
}

// ConversationStatus defines the status of a conversation
type ConversationStatus string

const (
	StatusActive      ConversationStatus = "active"
	StatusCompleted   ConversationStatus = "completed"
	StatusFailed      ConversationStatus = "failed"
	StatusInterrupted ConversationStatus = "interrupted"
)
