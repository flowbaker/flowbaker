package flowbaker

import (
	"encoding/json"
	"fmt"
)

// NewPublishEventRequest creates a new publish event request
func NewPublishEventRequest(eventType EventType, eventData interface{}) (*PublishEventRequest, error) {
	eventDataJSON, err := json.Marshal(eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	return &PublishEventRequest{
		EventType: eventType,
		EventData: eventDataJSON,
	}, nil
}

// NewNodeExecutionEvent creates a new node execution event
func NewNodeExecutionEvent(executionID, nodeID, nodeType string, input map[string]interface{}) NodeExecutionEvent {
	return NodeExecutionEvent{
		ExecutionID: executionID,
		NodeID:      nodeID,
		NodeType:    nodeType,
		Input:       input,
	}
}

// NewWorkflowExecutionEvent creates a new workflow execution completion event
func NewWorkflowExecutionEvent(executionID, workspaceID, workflowID string, nodeExecutions []NodeExecution) WorkflowExecutionEvent {
	return WorkflowExecutionEvent{
		ExecutionID:    executionID,
		WorkspaceID:    workspaceID,
		WorkflowID:     workflowID,
		NodeExecutions: nodeExecutions,
	}
}

// NewCompleteExecutionRequest creates a new complete execution request
func NewCompleteExecutionRequest(executionID, workspaceID, workflowID, triggerNodeID string) *CompleteExecutionRequest {
	return &CompleteExecutionRequest{
		ExecutionID:       executionID,
		WorkspaceID:       workspaceID,
		WorkflowID:        workflowID,
		TriggerNodeID:     triggerNodeID,
		NodeExecutions:    []NodeExecution{},
		HistoryEntries:    []NodeExecutionEntry{},
		IsTestingWorkflow: false,
	}
}