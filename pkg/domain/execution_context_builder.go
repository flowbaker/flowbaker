package domain

import "context"

// WorkflowExecutionContextBuilder provides a fluent interface for building WorkflowExecutionContext
type WorkflowExecutionContextBuilder struct {
	workspaceID         string
	workflowID          string
	workflowExecutionID string
	enableEvents        bool
	historyRecorder     ExecutionHistoryRecorder
	responseStatusCode  int
}

// NewWorkflowExecutionContextBuilder creates a new builder instance
func NewWorkflowExecutionContextBuilder() *WorkflowExecutionContextBuilder {
	return &WorkflowExecutionContextBuilder{
		responseStatusCode: 200, // default status code
	}
}

// WithWorkspaceID sets the workspace ID
func (b *WorkflowExecutionContextBuilder) WithWorkspaceID(id string) *WorkflowExecutionContextBuilder {
	b.workspaceID = id
	return b
}

// WithWorkflowID sets the workflow ID
func (b *WorkflowExecutionContextBuilder) WithWorkflowID(id string) *WorkflowExecutionContextBuilder {
	b.workflowID = id
	return b
}

// WithWorkflowExecutionID sets the workflow execution ID
func (b *WorkflowExecutionContextBuilder) WithWorkflowExecutionID(id string) *WorkflowExecutionContextBuilder {
	b.workflowExecutionID = id
	return b
}

// WithEvents enables or disables events
func (b *WorkflowExecutionContextBuilder) WithEvents(enable bool) *WorkflowExecutionContextBuilder {
	b.enableEvents = enable
	return b
}

// WithHistoryRecorder sets the history recorder
func (b *WorkflowExecutionContextBuilder) WithHistoryRecorder(recorder ExecutionHistoryRecorder) *WorkflowExecutionContextBuilder {
	b.historyRecorder = recorder
	return b
}

// WithEventOrder adds event ordering to the context
func (b *WorkflowExecutionContextBuilder) WithEventOrder() *WorkflowExecutionContextBuilder {
	// This will be applied during Build()
	return b
}


// Build creates the WorkflowExecutionContext and adds it to the provided context
func (b *WorkflowExecutionContextBuilder) Build(ctx context.Context) context.Context {
	// Add event order context first if needed
	ctx = NewContextWithEventOrder(ctx)
	
	workflowExecutionContext := &WorkflowExecutionContext{
		WorkspaceID:         b.workspaceID,
		WorkflowID:          b.workflowID,
		WorkflowExecutionID: b.workflowExecutionID,
		EnableEvents:        b.enableEvents,
		ResponsePayload:     nil,
		ResponseHeaders:     map[string][]string{},
		ResponseStatusCode:  b.responseStatusCode,
		HistoryRecorder:     b.historyRecorder,
		ToolTracker:         NewToolExecutionTracker(),
	}

	return context.WithValue(ctx, WorkflowExecutionContextKey{}, workflowExecutionContext)
}

