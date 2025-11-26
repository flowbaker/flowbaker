package types

// Usage represents token usage information
type Usage struct {
	PromptTokens       int `json:"prompt_tokens"`
	CompletionTokens   int `json:"completion_tokens"`
	TotalTokens        int `json:"total_tokens"`
	ReasoningTokens    int `json:"reasoning_tokens,omitempty"`
	CachedInputTokens  int `json:"cached_input_tokens,omitempty"`
}

// Add calculates the sum of two usage instances
func (u Usage) Add(other Usage) Usage {
	return Usage{
		PromptTokens:      u.PromptTokens + other.PromptTokens,
		CompletionTokens:  u.CompletionTokens + other.CompletionTokens,
		TotalTokens:       u.TotalTokens + other.TotalTokens,
		ReasoningTokens:   u.ReasoningTokens + other.ReasoningTokens,
		CachedInputTokens: u.CachedInputTokens + other.CachedInputTokens,
	}
}
