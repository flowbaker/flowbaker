package domain

import (
	"context"
)

type WorkflowExecutionContextKey struct{}

type ExecutionHistoryRecorder interface {
	AddNodeExecution(execution NodeExecution)
	AddNodeExecutionEntry(entry NodeExecutionEntry)
}

type WorkflowExecutionContext struct {
	WorkspaceID         string
	WorkflowID          string
	WorkflowExecutionID string
	EnableEvents        bool
	ResponsePayload     Payload
	ResponseHeaders     map[string][]string
	ResponseStatusCode  int
	HistoryRecorder     ExecutionHistoryRecorder
	ToolTracker         *ToolExecutionTracker
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

func NewContextWithWorkflowExecutionContext(ctx context.Context, workspaceID, workflowID, workflowExecutionID string, enableEvents bool) context.Context {
	return NewWorkflowExecutionContextBuilder().
		WithWorkspaceID(workspaceID).
		WithWorkflowID(workflowID).
		WithWorkflowExecutionID(workflowExecutionID).
		WithEvents(enableEvents).
		Build(ctx)
}

func NewContextWithWorkflowExecutionContextAndRecorder(ctx context.Context, workspaceID, workflowID, workflowExecutionID string, enableEvents bool, recorder ExecutionHistoryRecorder) context.Context {
	return NewWorkflowExecutionContextBuilder().
		WithWorkspaceID(workspaceID).
		WithWorkflowID(workflowID).
		WithWorkflowExecutionID(workflowExecutionID).
		WithEvents(enableEvents).
		WithHistoryRecorder(recorder).
		Build(ctx)
}

func GetWorkflowExecutionContext(ctx context.Context) (*WorkflowExecutionContext, bool) {
	workflowExecutionContext, ok := ctx.Value(WorkflowExecutionContextKey{}).(*WorkflowExecutionContext)

	return workflowExecutionContext, ok
}
