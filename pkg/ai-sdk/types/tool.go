package types

// Tool represents a tool/function that can be called by the LLM
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolDefinition is a more detailed tool definition with execution logic
type ToolDefinition struct {
	Tool
	Execute ToolExecuteFunc
}

// ToolExecuteFunc is the function signature for tool execution
type ToolExecuteFunc func(args map[string]any) (any, error)
