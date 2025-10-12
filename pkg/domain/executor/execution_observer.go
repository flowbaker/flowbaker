package executor

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type NodeExecutionStartedEvent struct {
	NodeID        string
	Timestamp     time.Time
	IsReExecution bool
}

func (NodeExecutionStartedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionStarted
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
	IsReExecution              bool
}

func (NodeExecutionCompletedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionCompleted
}

type NodeExecutionFailedEvent struct {
	NodeID           string
	PayloadByInputID SourceNodePayloadByInputID
	ItemsByInputID   map[string]domain.NodeItems
	Error            error
	Timestamp        time.Time
	IsReExecution    bool
}

func (NodeExecutionFailedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionFailed
}

type WorkflowExecutionCompletedEvent struct {
	Timestamp time.Time
}

func (WorkflowExecutionCompletedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeWorkflowExecutionCompleted
}

type executionObserver struct {
	handlers []domain.ExecutionEventHandler
}

func NewExecutionObserver() *executionObserver {
	return &executionObserver{
		handlers: []domain.ExecutionEventHandler{},
	}
}

func (o *executionObserver) Subscribe(handler domain.ExecutionEventHandler) {
	o.handlers = append(o.handlers, handler)
}

func (o *executionObserver) Notify(ctx context.Context, event domain.ExecutionEvent) error {
	for _, handler := range o.handlers {
		if err := handler.HandleEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
