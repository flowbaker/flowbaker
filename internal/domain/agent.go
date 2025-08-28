package domain

import "time"

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
