package tool

import (
	"context"
	"fmt"
)

// NewUserInputTool creates a tool that requests input from the user during execution.
// This tool works with the pause/resume pattern in ChatStream:
// 1. AI requests user input by calling this tool
// 2. Agent detects the tool call and pauses the stream
// 3. Frontend receives UserInputRequestedEvent
// 4. User provides input via frontend
// 5. Frontend resumes with a new ChatRequest including ToolResults
func NewUserInputTool() Tool {
	parameters := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The question or prompt to show the user",
			},
			"input_type": map[string]any{
				"type":        "string",
				"enum":        []string{"text", "credential-choice"},
				"description": "Type of input expected from user (default: text)",
			},
			"options": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"description": "Available options (required for choice type)",
			},
			"metadata": map[string]any{
				"type":        "object",
				"description": "Additional context or metadata for the request",
			},
		},
		"required": []string{"prompt"},
	}

	return &userInputTool{
		FuncTool: &FuncTool{
			name:        "request_user_input",
			description: "Request input from the user during execution. Use this when you need the user to provide information, confirm an action, or make a choice. The conversation will pause until the user responds.",
			parameters:  parameters,
			fn:          func(args string) (string, error) { return "", nil },
		},
	}
}

// userInputTool implements the Tool interface
type userInputTool struct {
	*FuncTool
}

// Execute implements the Tool interface
// This tool should not be executed directly - it triggers the pause/resume pattern
func (t *userInputTool) Execute(ctx context.Context, args string) (string, error) {
	return "", fmt.Errorf("request_user_input tool should not be executed directly; it requires ChatStream with pause/resume pattern")
}
