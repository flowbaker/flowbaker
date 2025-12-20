package types

import "time"

// StreamEvent is the base interface for all streaming events
type StreamEvent interface {
	GetType() StreamEventType
	GetTimestamp() time.Time
}

// StreamEventType identifies the type of streaming event
type StreamEventType string

const (
	// Lifecycle events
	EventTypeStreamStart StreamEventType = "stream-start"
	EventTypeStreamEnd   StreamEventType = "stream-end"
	EventTypeStreamError StreamEventType = "stream-error"

	// Content events
	EventTypeTextDelta    StreamEventType = "text-delta"
	EventTypeTextComplete StreamEventType = "text-complete"

	// Tool events
	EventTypeToolCallStart    StreamEventType = "tool-call-start"
	EventTypeToolCallDelta    StreamEventType = "tool-call-delta"
	EventTypeToolCallComplete StreamEventType = "tool-call-complete"

	// Metadata events
	EventTypeUsage        StreamEventType = "usage"
	EventTypeFinishReason StreamEventType = "finish-reason"
	EventTypeModelInfo    StreamEventType = "model-info"

	// Reasoning events (for o1, o3, etc.)
	EventTypeReasoningDelta    StreamEventType = "reasoning-delta"
	EventTypeReasoningComplete StreamEventType = "reasoning-complete"

	// Provider-specific events
	EventTypeWarning          StreamEventType = "warning"
	EventTypeProviderMetadata StreamEventType = "provider-metadata"

	// Agent-specific events
	EventTypeAgentStepStart        StreamEventType = "agent-step-start"
	EventTypeAgentStepComplete     StreamEventType = "agent-step-complete"
	EventTypeToolExecutionStart    StreamEventType = "tool-execution-start"
	EventTypeToolExecutionComplete StreamEventType = "tool-execution-complete"

	// Plan-specific events
	EventTypePlanCreated       StreamEventType = "plan-created"
	EventTypePlanStepStarted   StreamEventType = "plan-step-started"
	EventTypePlanStepCompleted StreamEventType = "plan-step-completed"
	EventTypePlanStepFailed    StreamEventType = "plan-step-failed"
	EventTypePlanUpdated       StreamEventType = "plan-updated"
	EventTypePlanCompleted     StreamEventType = "plan-completed"

	// User input events (HITL)
	EventTypeUserInputRequested StreamEventType = "user-input-requested"
)

// Base event struct with common fields
type baseEvent struct {
	eventType StreamEventType
	timestamp time.Time
}

func (e *baseEvent) GetType() StreamEventType {
	return e.eventType
}

func (e *baseEvent) GetTimestamp() time.Time {
	return e.timestamp
}

func newBaseEvent(eventType StreamEventType) baseEvent {
	return baseEvent{
		eventType: eventType,
		timestamp: time.Now(),
	}
}

// StreamStartEvent signals the beginning of a stream
type StreamStartEvent struct {
	baseEvent
	Model             string `json:"model"`
	RequestID         string `json:"request_id,omitempty"`
	SystemFingerprint string `json:"system_fingerprint,omitempty"`
}

// StreamEndEvent signals the end of a stream with final metadata
type StreamEndEvent struct {
	baseEvent
	FinishReason string `json:"finish_reason"`
	Usage        Usage  `json:"usage"`
}

// StreamErrorEvent represents an error during streaming
type StreamErrorEvent struct {
	baseEvent
	Error       error  `json:"error"`
	Code        string `json:"code,omitempty"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// TextDeltaEvent contains an incremental text chunk
type TextDeltaEvent struct {
	baseEvent
	Delta string `json:"delta"`
	Index int    `json:"index"` // For multiple completions
}

// TextCompleteEvent signals that text generation is complete
type TextCompleteEvent struct {
	baseEvent
	FullText string `json:"full_text"`
}

// ToolCallStartEvent signals the beginning of a tool call
type ToolCallStartEvent struct {
	baseEvent
	ID    string `json:"id"`
	Name  string `json:"name"`
	Index int    `json:"index"` // Index of tool call in array
}

// ToolCallDeltaEvent contains incremental tool call arguments
type ToolCallDeltaEvent struct {
	baseEvent
	ID            string `json:"id"`
	ArgumentDelta string `json:"argument_delta"`
	Index         int    `json:"index"`
}

// ToolCallCompleteEvent signals a tool call is complete with full arguments
type ToolCallCompleteEvent struct {
	baseEvent
	ToolCall ToolCall `json:"tool_call"`
	Index    int      `json:"index"`
}

// UsageEvent contains token usage information
type UsageEvent struct {
	baseEvent
	Usage Usage `json:"usage"`
}

// FinishReasonEvent indicates why generation stopped
type FinishReasonEvent struct {
	baseEvent
	Reason string `json:"reason"`
}

// ModelInfoEvent provides model identification and capabilities
type ModelInfoEvent struct {
	baseEvent
	ModelID      string `json:"model_id"`
	ModelVersion string `json:"model_version,omitempty"`
}

// ReasoningDeltaEvent contains incremental reasoning tokens (for o1/o3 models)
type ReasoningDeltaEvent struct {
	baseEvent
	Delta string `json:"delta"`
}

// ReasoningCompleteEvent signals reasoning is complete
type ReasoningCompleteEvent struct {
	baseEvent
	FullReasoning string `json:"full_reasoning"`
	Summary       string `json:"summary,omitempty"`
}

// WarningEvent represents a non-fatal warning from the provider
type WarningEvent struct {
	baseEvent
	Warning Warning `json:"warning"`
}

// ProviderMetadataEvent contains raw provider-specific data
type ProviderMetadataEvent struct {
	baseEvent
	Metadata ProviderMetadata `json:"metadata"`
}

// AgentStepStartEvent signals the start of an agent iteration
type AgentStepStartEvent struct {
	baseEvent
	StepNumber int    `json:"step_number"`
	Message    string `json:"message,omitempty"`
}

// AgentStepCompleteEvent signals completion of an agent iteration
type AgentStepCompleteEvent struct {
	baseEvent
	StepNumber   int          `json:"step_number"`
	Content      string       `json:"content"`
	ToolCalls    []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults  []ToolResult `json:"tool_results,omitempty"`
	Usage        Usage        `json:"usage"`
	FinishReason string       `json:"finish_reason"`
}

// ToolExecutionStartEvent signals the start of tool execution
type ToolExecutionStartEvent struct {
	baseEvent
	ToolCall ToolCall `json:"tool_call"`
}

// ToolExecutionCompleteEvent signals completion of tool execution
type ToolExecutionCompleteEvent struct {
	baseEvent
	ToolCall   ToolCall   `json:"tool_call"`
	ToolResult ToolResult `json:"tool_result"`
}

// Constructor functions for each event type

func NewStreamStartEvent(model, requestID, systemFingerprint string) *StreamStartEvent {
	return &StreamStartEvent{
		baseEvent:         newBaseEvent(EventTypeStreamStart),
		Model:             model,
		RequestID:         requestID,
		SystemFingerprint: systemFingerprint,
	}
}

func NewStreamEndEvent(finishReason string, usage Usage) *StreamEndEvent {
	return &StreamEndEvent{
		baseEvent:    newBaseEvent(EventTypeStreamEnd),
		FinishReason: finishReason,
		Usage:        usage,
	}
}

func NewStreamErrorEvent(err error, code, message string, recoverable bool) *StreamErrorEvent {
	return &StreamErrorEvent{
		baseEvent:   newBaseEvent(EventTypeStreamError),
		Error:       err,
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
	}
}

func NewTextDeltaEvent(delta string, index int) *TextDeltaEvent {
	return &TextDeltaEvent{
		baseEvent: newBaseEvent(EventTypeTextDelta),
		Delta:     delta,
		Index:     index,
	}
}

func NewTextCompleteEvent(fullText string) *TextCompleteEvent {
	return &TextCompleteEvent{
		baseEvent: newBaseEvent(EventTypeTextComplete),
		FullText:  fullText,
	}
}

func NewToolCallStartEvent(id, name string, index int) *ToolCallStartEvent {
	return &ToolCallStartEvent{
		baseEvent: newBaseEvent(EventTypeToolCallStart),
		ID:        id,
		Name:      name,
		Index:     index,
	}
}

func NewToolCallDeltaEvent(id, argumentDelta string, index int) *ToolCallDeltaEvent {
	return &ToolCallDeltaEvent{
		baseEvent:     newBaseEvent(EventTypeToolCallDelta),
		ID:            id,
		ArgumentDelta: argumentDelta,
		Index:         index,
	}
}

func NewToolCallCompleteEvent(toolCall ToolCall, index int) *ToolCallCompleteEvent {
	return &ToolCallCompleteEvent{
		baseEvent: newBaseEvent(EventTypeToolCallComplete),
		ToolCall:  toolCall,
		Index:     index,
	}
}

func NewUsageEvent(usage Usage) *UsageEvent {
	return &UsageEvent{
		baseEvent: newBaseEvent(EventTypeUsage),
		Usage:     usage,
	}
}

func NewFinishReasonEvent(reason string) *FinishReasonEvent {
	return &FinishReasonEvent{
		baseEvent: newBaseEvent(EventTypeFinishReason),
		Reason:    reason,
	}
}

func NewModelInfoEvent(modelID, modelVersion string) *ModelInfoEvent {
	return &ModelInfoEvent{
		baseEvent:    newBaseEvent(EventTypeModelInfo),
		ModelID:      modelID,
		ModelVersion: modelVersion,
	}
}

func NewReasoningDeltaEvent(delta string) *ReasoningDeltaEvent {
	return &ReasoningDeltaEvent{
		baseEvent: newBaseEvent(EventTypeReasoningDelta),
		Delta:     delta,
	}
}

func NewReasoningCompleteEvent(fullReasoning, summary string) *ReasoningCompleteEvent {
	return &ReasoningCompleteEvent{
		baseEvent:     newBaseEvent(EventTypeReasoningComplete),
		FullReasoning: fullReasoning,
		Summary:       summary,
	}
}

func NewWarningEvent(warning Warning) *WarningEvent {
	return &WarningEvent{
		baseEvent: newBaseEvent(EventTypeWarning),
		Warning:   warning,
	}
}

func NewProviderMetadataEvent(metadata ProviderMetadata) *ProviderMetadataEvent {
	return &ProviderMetadataEvent{
		baseEvent: newBaseEvent(EventTypeProviderMetadata),
		Metadata:  metadata,
	}
}

func NewAgentStepStartEvent(stepNumber int, message string) *AgentStepStartEvent {
	return &AgentStepStartEvent{
		baseEvent:  newBaseEvent(EventTypeAgentStepStart),
		StepNumber: stepNumber,
		Message:    message,
	}
}

func NewAgentStepCompleteEvent(stepNumber int, content string, toolCalls []ToolCall, toolResults []ToolResult, usage Usage, finishReason string) *AgentStepCompleteEvent {
	return &AgentStepCompleteEvent{
		baseEvent:    newBaseEvent(EventTypeAgentStepComplete),
		StepNumber:   stepNumber,
		Content:      content,
		ToolCalls:    toolCalls,
		ToolResults:  toolResults,
		Usage:        usage,
		FinishReason: finishReason,
	}
}

func NewToolExecutionStartEvent(toolCall ToolCall) *ToolExecutionStartEvent {
	return &ToolExecutionStartEvent{
		baseEvent: newBaseEvent(EventTypeToolExecutionStart),
		ToolCall:  toolCall,
	}
}

func NewToolExecutionCompleteEvent(toolCall ToolCall, toolResult ToolResult) *ToolExecutionCompleteEvent {
	return &ToolExecutionCompleteEvent{
		baseEvent:  newBaseEvent(EventTypeToolExecutionComplete),
		ToolCall:   toolCall,
		ToolResult: toolResult,
	}
}

// PlanCreatedEvent signals that a plan has been created
type PlanCreatedEvent struct {
	baseEvent
	Plan Plan `json:"plan"`
}

// PlanStepStartedEvent signals that a plan step has started
type PlanStepStartedEvent struct {
	baseEvent
	PlanID string   `json:"plan_id"`
	Step   PlanStep `json:"step"`
}

// PlanStepCompletedEvent signals that a plan step has completed successfully
type PlanStepCompletedEvent struct {
	baseEvent
	PlanID string   `json:"plan_id"`
	Step   PlanStep `json:"step"`
}

// PlanStepFailedEvent signals that a plan step has failed
type PlanStepFailedEvent struct {
	baseEvent
	PlanID string   `json:"plan_id"`
	Step   PlanStep `json:"step"`
	Error  string   `json:"error"`
}

// PlanUpdatedEvent signals that a plan has been modified
type PlanUpdatedEvent struct {
	baseEvent
	Plan   Plan   `json:"plan"`
	Change string `json:"change"` // Description of what changed
}

// PlanCompletedEvent signals that all plan steps are completed
type PlanCompletedEvent struct {
	baseEvent
	Plan Plan `json:"plan"`
}

func NewPlanCreatedEvent(plan Plan) *PlanCreatedEvent {
	return &PlanCreatedEvent{
		baseEvent: newBaseEvent(EventTypePlanCreated),
		Plan:      plan,
	}
}

func NewPlanStepStartedEvent(planID string, step PlanStep) *PlanStepStartedEvent {
	return &PlanStepStartedEvent{
		baseEvent: newBaseEvent(EventTypePlanStepStarted),
		PlanID:    planID,
		Step:      step,
	}
}

func NewPlanStepCompletedEvent(planID string, step PlanStep) *PlanStepCompletedEvent {
	return &PlanStepCompletedEvent{
		baseEvent: newBaseEvent(EventTypePlanStepCompleted),
		PlanID:    planID,
		Step:      step,
	}
}

func NewPlanStepFailedEvent(planID string, step PlanStep, errorMsg string) *PlanStepFailedEvent {
	return &PlanStepFailedEvent{
		baseEvent: newBaseEvent(EventTypePlanStepFailed),
		PlanID:    planID,
		Step:      step,
		Error:     errorMsg,
	}
}

func NewPlanUpdatedEvent(plan Plan, change string) *PlanUpdatedEvent {
	return &PlanUpdatedEvent{
		baseEvent: newBaseEvent(EventTypePlanUpdated),
		Plan:      plan,
		Change:    change,
	}
}

func NewPlanCompletedEvent(plan Plan) *PlanCompletedEvent {
	return &PlanCompletedEvent{
		baseEvent: newBaseEvent(EventTypePlanCompleted),
		Plan:      plan,
	}
}

// UserInputRequestedEvent signals that user input has been requested (HITL)
type UserInputRequestedEvent struct {
	baseEvent
	ToolCallID string                 `json:"tool_call_id"` // ID of the tool call to respond to
	Prompt     string                 `json:"prompt"`
	InputType  string                 `json:"input_type"`
	Options    []string               `json:"options,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func NewUserInputRequestedEvent(toolCallID, prompt, inputType string, options []string, metadata map[string]interface{}) *UserInputRequestedEvent {
	return &UserInputRequestedEvent{
		baseEvent:  newBaseEvent(EventTypeUserInputRequested),
		ToolCallID: toolCallID,
		Prompt:     prompt,
		InputType:  inputType,
		Options:    options,
		Metadata:   metadata,
	}
}
