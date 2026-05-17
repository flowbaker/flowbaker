package domain

import (
	"context"
	"time"
)

type WorkflowExecutionContextKey struct{}

type ExecutionSignal interface {
	signalMarker()
}

type PauseSignal struct {
	WakeAt time.Time
}

func (PauseSignal) signalMarker() {}

type WorkflowExecutionContext struct {
	UserID              *string
	WorkspaceID         string
	WorkflowID          string
	WorkflowExecutionID string
	EnableEvents        bool
	InputPayload        Payload
	ResponsePayload     Payload
	ResponseHeaders     map[string][]string
	ResponseStatusCode  int
	ExecutionObserver   ExecutionObserver
	IsReExecution       bool
	IsFromErrorTrigger  bool
	IsTesting           bool
	TriggerNode         WorkflowNode
	signals             []ExecutionSignal
}

func (c *WorkflowExecutionContext) EmitSignal(s ExecutionSignal) {
	c.signals = append(c.signals, s)
}

func (c *WorkflowExecutionContext) DrainSignals() []ExecutionSignal {
	out := c.signals
	c.signals = nil
	return out
}

func (c *WorkflowExecutionContext) SetResponsePayload(payload Payload) {
	c.ResponsePayload = payload
}

func (c *WorkflowExecutionContext) SetResponseHeaders(headers map[string][]string) {
	c.ResponseHeaders = headers
}

func (c *WorkflowExecutionContext) SetResponseStatusCode(statusCode int) {
	c.ResponseStatusCode = statusCode
}

type NewContextWithWorkflowExecutionContextParams struct {
	UserID              *string
	InputPayload        Payload
	WorkspaceID         string
	WorkflowID          string
	WorkflowExecutionID string
	EnableEvents        bool
	Observer            ExecutionObserver
	IsReExecution       bool
	IsFromErrorTrigger  bool
	IsTesting           bool
	TriggerNode         WorkflowNode
}

func NewContextWithWorkflowExecutionContext(ctx context.Context, params NewContextWithWorkflowExecutionContextParams) context.Context {
	workflowExecutionContext := &WorkflowExecutionContext{
		UserID:              params.UserID,
		InputPayload:        params.InputPayload,
		WorkspaceID:         params.WorkspaceID,
		WorkflowID:          params.WorkflowID,
		WorkflowExecutionID: params.WorkflowExecutionID,
		EnableEvents:        params.EnableEvents,
		ResponsePayload:     nil,
		ResponseHeaders:     map[string][]string{},
		ResponseStatusCode:  200,
		ExecutionObserver:   params.Observer,
		IsReExecution:       params.IsReExecution,
		IsFromErrorTrigger:  params.IsFromErrorTrigger,
		IsTesting:           params.IsTesting,
		TriggerNode:         params.TriggerNode,
	}

	return context.WithValue(ctx, WorkflowExecutionContextKey{}, workflowExecutionContext)
}

func GetWorkflowExecutionContext(ctx context.Context) (*WorkflowExecutionContext, bool) {
	workflowExecutionContext, ok := ctx.Value(WorkflowExecutionContextKey{}).(*WorkflowExecutionContext)

	return workflowExecutionContext, ok
}
