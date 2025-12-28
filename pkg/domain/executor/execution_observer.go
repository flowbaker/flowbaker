package executor

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ReExecutableEvent interface {
	SetIsReExecution(isReExecution bool) domain.ExecutionEvent
}

type NodeExecutionStartedEvent struct {
	NodeID        string
	Timestamp     time.Time
	IsReExecution bool
}

func (NodeExecutionStartedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionStarted
}

func (e NodeExecutionStartedEvent) SetIsReExecution(isReExecution bool) domain.ExecutionEvent {
	e.IsReExecution = isReExecution

	return e
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
	IsReExecution              bool
}

func (NodeExecutionCompletedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionCompleted
}

func (e NodeExecutionCompletedEvent) SetIsReExecution(isReExecution bool) domain.ExecutionEvent {
	e.IsReExecution = isReExecution

	return e
}

type NodeExecutionFailedEvent struct {
	NodeID         string
	ItemsByInputID map[string]domain.NodeItems
	Error          error
	Timestamp      time.Time
	IsReExecution  bool
}

func (NodeExecutionFailedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionFailed
}

func (e NodeExecutionFailedEvent) SetIsReExecution(isReExecution bool) domain.ExecutionEvent {
	e.IsReExecution = isReExecution

	return e
}

type WorkflowExecutionCompletedEvent struct {
	Timestamp time.Time
}

func (WorkflowExecutionCompletedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeWorkflowExecutionCompleted
}

type executionObserver struct {
	handlers       []domain.ExecutionEventHandler
	streamHandlers []domain.StreamEventHandler
}

func NewExecutionObserver() *executionObserver {
	return &executionObserver{
		handlers:       []domain.ExecutionEventHandler{},
		streamHandlers: []domain.StreamEventHandler{},
	}
}

func (o *executionObserver) Subscribe(handler domain.ExecutionEventHandler) {
	o.handlers = append(o.handlers, handler)
}

func (o *executionObserver) SubscribeStream(handler domain.StreamEventHandler) {
	o.streamHandlers = append(o.streamHandlers, handler)
}

func (o *executionObserver) Notify(ctx context.Context, event domain.ExecutionEvent) error {
	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if ok {
		if executionContext.IsReExecution {
			reExecutableEvent, ok := event.(ReExecutableEvent)
			if ok {
				event = reExecutableEvent.SetIsReExecution(true)
			}
		}
	}

	for _, handler := range o.handlers {
		if err := handler.HandleEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (o *executionObserver) NotifyStream(ctx context.Context, event domain.StreamEvent) error {
	for _, handler := range o.streamHandlers {
		if err := handler.HandleStreamEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
