package executor

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

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

type NodeExecutionStartedEvent struct {
	NodeID    string
	Timestamp time.Time
}

func (NodeExecutionStartedEvent) GetEventType() ExecutionEventType {
	return ExecutionEventTypeNodeExecutionStarted
}

type NodeExecutionCompletedEvent struct {
	NodeID                     string
	SourceNodePayloadByInputID SourceNodePayloadByInputID
	IntegrationOutput          domain.IntegrationOutput
	ItemsByInputID             map[string]domain.NodeItems
	ItemsByOutputID            map[string]domain.NodeItems
	ExecutionOrder             int64
	StartedAt                  time.Time
	EndedAt                    time.Time
	IntegrationType            domain.IntegrationType
	IntegrationActionType      domain.IntegrationActionType
	Timestamp                  time.Time
}

func (NodeExecutionCompletedEvent) GetEventType() ExecutionEventType {
	return ExecutionEventTypeNodeExecutionCompleted
}

type NodeExecutionFailedEvent struct {
	NodeID           string
	PayloadByInputID SourceNodePayloadByInputID
	ItemsByInputID   map[string]domain.NodeItems
	Error            error
	Timestamp        time.Time
}

func (NodeExecutionFailedEvent) GetEventType() ExecutionEventType {
	return ExecutionEventTypeNodeExecutionFailed
}

type WorkflowExecutionCompletedEvent struct {
	Timestamp time.Time
}

func (WorkflowExecutionCompletedEvent) GetEventType() ExecutionEventType {
	return ExecutionEventTypeWorkflowExecutionCompleted
}

type ExecutionEventHandler interface {
	HandleEvent(ctx context.Context, event ExecutionEvent) error
}

type ExecutionObserver struct {
	handlers []ExecutionEventHandler
}

func NewExecutionObserver() *ExecutionObserver {
	return &ExecutionObserver{
		handlers: []ExecutionEventHandler{},
	}
}

func (o *ExecutionObserver) Subscribe(handler ExecutionEventHandler) {
	o.handlers = append(o.handlers, handler)
}

func (o *ExecutionObserver) Notify(ctx context.Context, event ExecutionEvent) error {
	for _, handler := range o.handlers {
		if err := handler.HandleEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
