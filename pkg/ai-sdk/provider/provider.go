package provider

import (
	"context"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

// LanguageModel defines the interface that all LLM providers must implement
type LanguageModel interface {
	// Generate produces a complete response (blocking)
	Generate(ctx context.Context, req GenerateRequest) (*types.GenerateResponse, error)

	// Stream produces a streaming response via channel
	Stream(ctx context.Context, req GenerateRequest) (<-chan types.StreamEvent, <-chan error)

	// ID returns the unique identifier for this model
	ID() string

	// Capabilities returns the capabilities of this model
	Capabilities() Capabilities

	ProviderName() string
}

// GenerateRequest contains all parameters for generating text
type GenerateRequest struct {
	// Messages is the conversation history
	Messages []types.Message `json:"messages"`

	// System is an optional system prompt
	System string `json:"system,omitempty"`

	// Tools is a list of tools available to the model
	Tools []types.Tool `json:"tools,omitempty"`
}

// Capabilities describes what a model can do
type Capabilities struct {
	// SupportsTools indicates if the model supports function/tool calling
	SupportsTools bool `json:"supports_tools"`

	// SupportsStreaming indicates if the model supports streaming responses
	SupportsStreaming bool `json:"supports_streaming"`

	// SupportsVision indicates if the model supports image inputs
	SupportsVision bool `json:"supports_vision"`

	// MaxContextTokens is the maximum context window size
	MaxContextTokens int `json:"max_context_tokens"`

	// MaxOutputTokens is the maximum output tokens
	MaxOutputTokens int `json:"max_output_tokens"`
}
