package domain

import (
	"context"
)

type WorkflowExecutionContextKey struct{}

type WorkflowExecutionContext struct {
	WorkspaceID         string
	WorkflowID          string
	WorkflowExecutionID string
	EnableEvents        bool
	ResponsePayload     Payload
	ResponseHeaders     map[string][]string
	ResponseStatusCode  int
	ExecutionObserver   ExecutionObserver
	IsReExecution       bool
	IsFromErrorTrigger  bool
	IsTesting           bool
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
	WorkspaceID         string
	WorkflowID          string
	WorkflowExecutionID string
	EnableEvents        bool
	Observer            ExecutionObserver
	IsReExecution       bool
	IsFromErrorTrigger  bool
	IsTesting           bool
}

func NewContextWithWorkflowExecutionContext(ctx context.Context, params NewContextWithWorkflowExecutionContextParams) context.Context {
	workflowExecutionContext := &WorkflowExecutionContext{
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
	}

	return context.WithValue(ctx, WorkflowExecutionContextKey{}, workflowExecutionContext)
}

func GetWorkflowExecutionContext(ctx context.Context) (*WorkflowExecutionContext, bool) {
	workflowExecutionContext, ok := ctx.Value(WorkflowExecutionContextKey{}).(*WorkflowExecutionContext)

	return workflowExecutionContext, ok
}
