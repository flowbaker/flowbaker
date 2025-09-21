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
	AgentNodeExecutions []NodeExecutionEntry
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

func (c *WorkflowExecutionContext) AddAgentNodeExecution(execution NodeExecutionEntry) {
	c.AgentNodeExecutions = append(c.AgentNodeExecutions, execution)
}

func NewContextWithWorkflowExecutionContext(ctx context.Context, workspaceID, workflowID, workflowExecutionID string, enableEvents bool) context.Context {
	workflowExecutionContext := &WorkflowExecutionContext{
		WorkspaceID:         workspaceID,
		WorkflowID:          workflowID,
		WorkflowExecutionID: workflowExecutionID,
		EnableEvents:        enableEvents,
		ResponsePayload:     nil,
		ResponseHeaders:     map[string][]string{},
		ResponseStatusCode:  200,
		AgentNodeExecutions: []NodeExecutionEntry{},
	}

	return context.WithValue(ctx, WorkflowExecutionContextKey{}, workflowExecutionContext)
}

func GetWorkflowExecutionContext(ctx context.Context) (*WorkflowExecutionContext, bool) {
	workflowExecutionContext, ok := ctx.Value(WorkflowExecutionContextKey{}).(*WorkflowExecutionContext)

	return workflowExecutionContext, ok
}
