package types

// Warning represents a warning from the provider
type Warning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ProviderMetadata contains provider-specific metadata
type ProviderMetadata map[string]interface{}

// GenerateResponse represents a response from text generation
type GenerateResponse struct {
	Content          string           `json:"content"`
	ToolCalls        []ToolCall       `json:"tool_calls,omitempty"`
	Usage            Usage            `json:"usage"`
	FinishReason     string           `json:"finish_reason"`
	Model            string           `json:"model"`
	Warnings         []Warning        `json:"warnings,omitempty"`
	ProviderMetadata ProviderMetadata `json:"provider_metadata,omitempty"`
}

// FinishReason constants
const (
	FinishReasonStop              = "stop"
	FinishReasonLength            = "length"
	FinishReasonToolCalls         = "tool_calls"
	FinishReasonContentFilter     = "content_filter"
	FinishReasonError             = "error"
	FinishReasonHumanIntervention = "human_intervention"
)
