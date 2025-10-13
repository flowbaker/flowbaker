package domain

import "context"

type ExecutionObserver interface {
	Subscribe(handler ExecutionEventHandler)
	Notify(ctx context.Context, event ExecutionEvent) error
}

type ExecutionEventHandler interface {
	HandleEvent(ctx context.Context, event ExecutionEvent) error
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
