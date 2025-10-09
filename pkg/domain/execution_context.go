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
	// ExecutionObserver is stored as interface{} to avoid import cycles with pkg/domain/executor.
	// Cast to *executor.ExecutionObserver when needed.
	ExecutionObserver interface{}
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

func NewContextWithWorkflowExecutionContext(ctx context.Context, workspaceID, workflowID, workflowExecutionID string, enableEvents bool, observer interface{}) context.Context {
	workflowExecutionContext := &WorkflowExecutionContext{
		WorkspaceID:         workspaceID,
		WorkflowID:          workflowID,
		WorkflowExecutionID: workflowExecutionID,
		EnableEvents:        enableEvents,
		ResponsePayload:     nil,
		ResponseHeaders:     map[string][]string{},
		ResponseStatusCode:  200,
		ExecutionObserver:   observer,
	}

	return context.WithValue(ctx, WorkflowExecutionContextKey{}, workflowExecutionContext)
}

func GetWorkflowExecutionContext(ctx context.Context) (*WorkflowExecutionContext, bool) {
	workflowExecutionContext, ok := ctx.Value(WorkflowExecutionContextKey{}).(*WorkflowExecutionContext)

	return workflowExecutionContext, ok
}
