package provider

import (
	"context"
	"sync"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

// LanguageModel defines the interface that all LLM providers must implement
type LanguageModel interface {
	// Generate produces a complete response (blocking)
	Generate(ctx context.Context, req GenerateRequest) (*types.GenerateResponse, error)

	// Stream creates a streaming request.
	// Returns error immediately if request fails to start (auth, bad params, etc.)
	// Only streaming errors go through ProviderStream.Err()
	Stream(ctx context.Context, req GenerateRequest) (*ProviderStream, error)

	// ID returns the unique identifier for this model
	ID() string

	// Capabilities returns the capabilities of this model
	Capabilities() Capabilities

	ProviderName() string
}

// ProviderStream represents an active streaming response from a provider.
// Events are received through the Events channel.
// After the Events channel closes, call Err() to check for streaming errors.
type ProviderStream struct {
	Events <-chan types.StreamEvent

	mu  sync.RWMutex
	err error
}

// NewProviderStream creates a new ProviderStream with the given events channel.
func NewProviderStream(events <-chan types.StreamEvent) *ProviderStream {
	return &ProviderStream{
		Events: events,
	}
}

// SetError sets the streaming error (thread-safe).
func (s *ProviderStream) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// Err returns any error that occurred during streaming.
// Should be called after Events channel closes.
func (s *ProviderStream) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
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
