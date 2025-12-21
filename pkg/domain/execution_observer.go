package domain

import "context"

type ExecutionObserver interface {
	Subscribe(handler ExecutionEventHandler)
	SubscribeStream(handler StreamEventHandler)
	Notify(ctx context.Context, event ExecutionEvent) error
	NotifyStream(ctx context.Context, event StreamEvent) error
}

type ExecutionEventHandler interface {
	HandleEvent(ctx context.Context, event ExecutionEvent) error
}

type StreamEventHandler interface {
	HandleStreamEvent(ctx context.Context, event StreamEvent) error
}

type ExecutionEventType string

const (
	ExecutionEventTypeNodeExecutionStarted       ExecutionEventType = "node_execution_started"
	ExecutionEventTypeNodeExecutionCompleted     ExecutionEventType = "node_execution_completed"
	ExecutionEventTypeNodeExecutionFailed        ExecutionEventType = "node_execution_failed"
	ExecutionEventTypeWorkflowExecutionCompleted ExecutionEventType = "workflow_execution_completed"
)

type ExecutionEvent interface {
	GetEventType() ExecutionEventType
}

type StreamEventType string

const (
	StreamEventTypeAIChatStream StreamEventType = "ai_chat_stream"
)

type StreamEvent interface {
	GetEventType() StreamEventType
}
