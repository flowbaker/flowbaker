package domain

import (
	"sync"
	"time"
)

// ToolIdentifier provides a structured way to identify tools
type ToolIdentifier struct {
	NodeID          string
	IntegrationType IntegrationType
	ActionType      IntegrationActionType
}

// ToolExecution represents a single tool execution with metadata
type ToolExecution struct {
	Identifier  ToolIdentifier
	ExecutedAt  time.Time
	CompletedAt time.Time
	Result      interface{}
	Error       error
	InputItems  []Item
	OutputItems []Item
}

// ToolExecutionTracker tracks tool executions during workflow execution
type ToolExecutionTracker struct {
	executions map[string]*ToolExecution // key is NodeID
	mu         sync.RWMutex
}

// NewToolExecutionTracker creates a new tool execution tracker
func NewToolExecutionTracker() *ToolExecutionTracker {
	return &ToolExecutionTracker{
		executions: make(map[string]*ToolExecution),
	}
}

// RecordExecution records a tool execution
func (t *ToolExecutionTracker) RecordExecution(execution *ToolExecution) {
	t.mu.Lock()
	defer t.mu.Unlock()

	nodeID := execution.Identifier.NodeID
	if existingExecution, exists := t.executions[nodeID]; exists {
		existingExecution.InputItems = append(existingExecution.InputItems, execution.InputItems...)
		existingExecution.OutputItems = append(existingExecution.OutputItems, execution.OutputItems...)
		existingExecution.CompletedAt = execution.CompletedAt
	} else {
		t.executions[nodeID] = execution
	}
}

// GetExecutedNodeIDs returns the node IDs of all executed tools
func (t *ToolExecutionTracker) GetExecutedNodeIDs() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nodeIDs := make([]string, 0, len(t.executions))
	for nodeID := range t.executions {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}

// ForEachExecution executes a function for each recorded execution
func (t *ToolExecutionTracker) ForEachExecution(fn func(nodeID string, execution *ToolExecution)) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for nodeID, execution := range t.executions {
		fn(nodeID, execution)
	}
}
