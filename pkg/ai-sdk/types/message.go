package types

import "time"

// Message represents a single message in a conversation
type Message struct {
	Role        MessageRole            `json:"role" bson:"role"`
	Content     string                 `json:"content" bson:"content"`
	ToolCalls   []ToolCall             `json:"tool_calls,omitempty" bson:"tool_calls,omitempty"`
	ToolResults []ToolResult           `json:"tool_results,omitempty" bson:"tool_results,omitempty"`
	Timestamp   time.Time              `json:"timestamp" bson:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
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
	ID        string                 `json:"id" bson:"id"`
	Name      string                 `json:"name" bson:"name"`
	Arguments map[string]interface{} `json:"arguments" bson:"arguments"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	ToolCallID string `json:"tool_call_id" bson:"tool_call_id"`
	Content    string `json:"content" bson:"content"`
	IsError    bool   `json:"is_error,omitempty" bson:"is_error,omitempty"`
}

// Conversation represents a complete conversation
type Conversation struct {
	ID        string                 `json:"id" bson:"id"`
	SessionID string                 `json:"session_id" bson:"session_id"`
	UserID    string                 `json:"user_id,omitempty" bson:"user_id,omitempty"`
	Messages  []Message              `json:"messages" bson:"messages"`
	CreatedAt time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" bson:"updated_at"`
	Status    ConversationStatus     `json:"status" bson:"status"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
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
