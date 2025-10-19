package domain

import (
	"context"
	"fmt"
	"sync"
)

type EventPublisher interface {
	PublishEvent(ctx context.Context, event Event) error
}

type EventHandler func(ctx context.Context, event []byte) error

type EventListener interface {
	Listen(ctx context.Context, eventType EventType, handler EventHandler) error
}

type EventOrderContextKey struct{}

type EventOrderContext struct {
	mtx   sync.Mutex
	order int
}

func NewContextWithEventOrder(ctx context.Context) context.Context {
	eventOrderContext := &EventOrderContext{
		order: 0,
		mtx:   sync.Mutex{},
	}

	return context.WithValue(ctx, EventOrderContextKey{}, eventOrderContext)
}

func GetEventOrderContext(ctx context.Context) (*EventOrderContext, bool) {
	order, ok := ctx.Value(EventOrderContextKey{}).(*EventOrderContext)
	if !ok {
		return nil, false
	}

	return order, true
}

func (c *EventOrderContext) GetNextOrder() int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.order++
	return c.order
}

type OrderedEvent interface {
	Event
	GetEventOrder() int
	SetEventOrder(order int)
}

type Event interface {
	GetType() EventType
}

type EventType string

const (
	NodeExecuted               EventType = "node_executed"
	NodeFailed                 EventType = "node_failed"
	NodeExecutionStarted       EventType = "node_execution_started"
	WorkflowExecutionCompleted EventType = "workflow_execution_completed"
)

type NodeExecutionStartedEvent struct {
	WorkflowID          string `json:"workflow_id"`
	NodeID              string `json:"node_id"`
	Timestamp           int64  `json:"timestamp"`
	WorkflowExecutionID string `json:"workflow_execution_id"`
	EventOrder          int    `json:"event_order"`
	IsReExecution       bool   `json:"is_re_execution"`
}

func (e *NodeExecutionStartedEvent) GetType() EventType {
	return NodeExecutionStarted
}

func (e *NodeExecutionStartedEvent) GetEventOrder() int {
	return e.EventOrder
}

func (e *NodeExecutionStartedEvent) SetEventOrder(order int) {
	e.EventOrder = order
}

type NodeExecutedEvent struct {
	WorkflowID          string               `json:"workflow_id"`
	NodeID              string               `json:"node_id"`
	ItemsByInputID      map[string]NodeItems `json:"items_by_input_id"`
	ItemsByOutputID     map[string]NodeItems `json:"items_by_output_id"`
	Timestamp           int64                `json:"timestamp"`
	ExecutionOrder      int                  `json:"execution_order"`
	WorkflowExecutionID string               `json:"workflow_execution_id"`
	EventOrder          int                  `json:"event_order"`
	IsReExecution       bool                 `json:"is_re_execution"`
}

func (e *NodeExecutedEvent) GetType() EventType {
	return NodeExecuted
}

func (e *NodeExecutedEvent) GetEventOrder() int {
	return e.EventOrder
}

func (e *NodeExecutedEvent) SetEventOrder(order int) {
	e.EventOrder = order
}

type NodeFailedEvent struct {
	WorkflowID          string               `json:"workflow_id"`
	NodeID              string               `json:"node_id"`
	Error               string               `json:"error"`
	WorkflowExecutionID string               `json:"workflow_execution_id"`
	ExecutionOrder      int                  `json:"execution_order"`
	Timestamp           int64                `json:"timestamp"`
	ItemsByInputID      map[string]NodeItems `json:"items_by_input_id"`
	ItemsByOutputID     map[string]NodeItems `json:"items_by_output_id"`
	EventOrder          int                  `json:"event_order"`
	IsReExecution       bool                 `json:"is_re_execution"`
}

func (e *NodeFailedEvent) GetType() EventType {
	return NodeFailed
}

func (e *NodeFailedEvent) GetEventOrder() int {
	return e.EventOrder
}

func (e *NodeFailedEvent) SetEventOrder(order int) {
	e.EventOrder = order
}

type ExecuteWorkflowRequestEvent struct {
	WorkflowID string `json:"workflow_id"`
	FromNodeID string `json:"from_node_id"`
}

func (e ExecuteWorkflowRequestEvent) GetType() EventType {
	return "execute_workflow_request"
}

type WorkflowExecutionCompletedEvent struct {
	WorkflowID          string `json:"workflow_id"`
	WorkflowExecutionID string `json:"workflow_execution_id"`
	Timestamp           int64  `json:"timestamp"`
	EventOrder          int    `json:"event_order"`
}

func (e *WorkflowExecutionCompletedEvent) GetType() EventType {
	return WorkflowExecutionCompleted
}

func (e *WorkflowExecutionCompletedEvent) GetEventOrder() int {
	return e.EventOrder
}

func (e *WorkflowExecutionCompletedEvent) SetEventOrder(order int) {
	e.EventOrder = order
}

type OrderedEventPublisher struct {
	eventPublisher EventPublisher
}

func NewOrderedEventPublisher(eventPublisher EventPublisher) *OrderedEventPublisher {
	return &OrderedEventPublisher{
		eventPublisher: eventPublisher,
	}
}

func (p *OrderedEventPublisher) PublishEvent(ctx context.Context, event Event) error {
	orderedEvent, isOrderedEvent := event.(OrderedEvent)
	if !isOrderedEvent {
		return p.eventPublisher.PublishEvent(ctx, event)
	}

	eventOrderContext, ok := GetEventOrderContext(ctx)
	if !ok {
		return fmt.Errorf("failed to get event order context")
	}

	nextOrder := eventOrderContext.GetNextOrder()
	orderedEvent.SetEventOrder(nextOrder)

	return p.eventPublisher.PublishEvent(ctx, event)
}
