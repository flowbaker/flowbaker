package executor

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ReExecutableEvent interface {
	SetIsReExecution(isReExecution bool) domain.ExecutionEvent
}

type FromErrorTriggerEvent interface {
	SetIsFromErrorTrigger(isFromErrorTrigger bool) domain.ExecutionEvent
}

type TestingEvent interface {
	SetIsTesting(isTesting bool) domain.ExecutionEvent
}

type NodeExecutionStartedEvent struct {
	NodeID             string
	Timestamp          time.Time
	IsReExecution      bool
	IsTesting          bool
	IsFromErrorTrigger bool
}

func (NodeExecutionStartedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionStarted
}

func (e NodeExecutionStartedEvent) SetIsTesting(isTesting bool) domain.ExecutionEvent {
	e.IsTesting = isTesting

	return e
}

func (e NodeExecutionStartedEvent) SetIsReExecution(isReExecution bool) domain.ExecutionEvent {
	e.IsReExecution = isReExecution

	return e
}

func (e NodeExecutionStartedEvent) SetIsFromErrorTrigger(isFromErrorTrigger bool) domain.ExecutionEvent {
	e.IsFromErrorTrigger = isFromErrorTrigger

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
	IsFromErrorTrigger         bool
	IsTesting                  bool
}

func (NodeExecutionCompletedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionCompleted
}

func (e NodeExecutionCompletedEvent) SetIsReExecution(isReExecution bool) domain.ExecutionEvent {
	e.IsReExecution = isReExecution

	return e
}

func (e NodeExecutionCompletedEvent) SetIsFromErrorTrigger(isFromErrorTrigger bool) domain.ExecutionEvent {
	e.IsFromErrorTrigger = isFromErrorTrigger

	return e
}

func (e NodeExecutionCompletedEvent) SetIsTesting(isTesting bool) domain.ExecutionEvent {
	e.IsTesting = isTesting

	return e
}

type NodeExecutionFailedEvent struct {
	NodeID             string
	ItemsByInputID     map[string]domain.NodeItems
	Error              error
	Timestamp          time.Time
	IsReExecution      bool
	IsFromErrorTrigger bool
	IsTesting          bool
}

func (NodeExecutionFailedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeNodeExecutionFailed
}

func (e NodeExecutionFailedEvent) SetIsReExecution(isReExecution bool) domain.ExecutionEvent {
	e.IsReExecution = isReExecution

	return e
}

func (e NodeExecutionFailedEvent) SetIsFromErrorTrigger(isFromErrorTrigger bool) domain.ExecutionEvent {
	e.IsFromErrorTrigger = isFromErrorTrigger

	return e
}

func (e NodeExecutionFailedEvent) SetIsTesting(isTesting bool) domain.ExecutionEvent {
	e.IsTesting = isTesting

	return e
}

type WorkflowExecutionCompletedEvent struct {
	Timestamp          time.Time
	IsTesting          bool
	IsFromErrorTrigger bool
}

func (WorkflowExecutionCompletedEvent) GetEventType() domain.ExecutionEventType {
	return domain.ExecutionEventTypeWorkflowExecutionCompleted
}

func (e WorkflowExecutionCompletedEvent) SetIsTesting(isTesting bool) domain.ExecutionEvent {
	e.IsTesting = isTesting

	return e
}

func (e WorkflowExecutionCompletedEvent) SetIsFromErrorTrigger(isFromErrorTrigger bool) domain.ExecutionEvent {
	e.IsFromErrorTrigger = isFromErrorTrigger

	return e
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
	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if ok {
		if executionContext.IsReExecution {
			reExecutableEvent, ok := event.(ReExecutableEvent)
			if ok {
				event = reExecutableEvent.SetIsReExecution(true)
			}
		}

		if executionContext.IsFromErrorTrigger {
			fromErrorTriggerEvent, ok := event.(FromErrorTriggerEvent)
			if ok {
				event = fromErrorTriggerEvent.SetIsFromErrorTrigger(true)
			}
		}

		if executionContext.IsTesting {
			testingEvent, ok := event.(TestingEvent)
			if ok {
				event = testingEvent.SetIsTesting(true)
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
