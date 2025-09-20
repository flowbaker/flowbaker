package domain

import (
	"sync"
	"time"
)

type NodeType string

const (
	NodeTypeLLM    NodeType = "llm"
	NodeTypeMemory NodeType = "memory"
	NodeTypeTool   NodeType = "tool"
)

type ExecutionIdentifier struct {
	NodeID          string
	NodeType        NodeType
	IntegrationType IntegrationType
	ActionType      IntegrationActionType
}

type AgentExecution struct {
	Identifier  ExecutionIdentifier
	ExecutedAt  time.Time
	CompletedAt time.Time
	Result      interface{}
	Error       error
	InputItems  []Item
	OutputItems []Item
}

type AgentExecutionTracker struct {
	executions map[string][]*AgentExecution
	mu         sync.RWMutex
}

func NewAgentExecutionTracker() *AgentExecutionTracker {
	return &AgentExecutionTracker{
		executions: make(map[string][]*AgentExecution),
	}
}

func (t *AgentExecutionTracker) RecordExecution(execution *AgentExecution) {
	t.mu.Lock()
	defer t.mu.Unlock()

	nodeID := execution.Identifier.NodeID
	t.executions[nodeID] = append(t.executions[nodeID], execution)
}

func (t *AgentExecutionTracker) GetExecutionCount(nodeID string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.executions[nodeID])
}


func (t *AgentExecutionTracker) GetAggregatedExecution(nodeID string) *AgentExecution {
	t.mu.RLock()
	defer t.mu.RUnlock()

	executions := t.executions[nodeID]
	if len(executions) == 0 {
		return nil
	}

	aggregated := &AgentExecution{
		Identifier:  executions[0].Identifier,
		ExecutedAt:  executions[0].ExecutedAt,
		CompletedAt: executions[len(executions)-1].CompletedAt,
	}

	for _, exec := range executions {
		aggregated.InputItems = append(aggregated.InputItems, exec.InputItems...)
		aggregated.OutputItems = append(aggregated.OutputItems, exec.OutputItems...)
	}

	return aggregated
}


func (t *AgentExecutionTracker) GetExecutedNodeIDsByType(nodeType NodeType) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var nodeIDs []string
	for nodeID, executions := range t.executions {
		if len(executions) > 0 && executions[0].Identifier.NodeType == nodeType {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	return nodeIDs
}



func (t *AgentExecutionTracker) ForEachToolExecution(fn func(nodeID string, execution *AgentExecution)) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for nodeID, executions := range t.executions {
		if len(executions) > 0 && executions[0].Identifier.NodeType == NodeTypeTool {
			aggregated := t.GetAggregatedExecution(nodeID)
			if aggregated != nil {
				fn(nodeID, aggregated)
			}
		}
	}
}

type ExecutionStatistics struct {
	TotalLLMCalls    int                       `json:"total_llm_calls"`
	TotalMemoryCalls int                       `json:"total_memory_calls"`
	TotalToolCalls   int                       `json:"total_tool_calls"`
	ToolCallDetails  map[string]ToolCallDetail `json:"tool_call_details"`
}

type ToolCallDetail struct {
	NodeID          string                `json:"node_id"`
	IntegrationType IntegrationType       `json:"integration_type"`
	ActionType      IntegrationActionType `json:"action_type"`
	CallCount       int                   `json:"call_count"`
}

func (t *AgentExecutionTracker) GetExecutionStatistics() *ExecutionStatistics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &ExecutionStatistics{
		TotalLLMCalls:    0,
		TotalMemoryCalls: 0,
		TotalToolCalls:   0,
		ToolCallDetails:  make(map[string]ToolCallDetail),
	}

	for nodeID, executions := range t.executions {
		if len(executions) == 0 {
			continue
		}

		nodeType := executions[0].Identifier.NodeType
		count := len(executions)

		switch nodeType {
		case NodeTypeLLM:
			stats.TotalLLMCalls += count
		case NodeTypeMemory:
			stats.TotalMemoryCalls += count
		case NodeTypeTool:
			stats.TotalToolCalls += count
			stats.ToolCallDetails[nodeID] = ToolCallDetail{
				NodeID:          nodeID,
				IntegrationType: executions[0].Identifier.IntegrationType,
				ActionType:      executions[0].Identifier.ActionType,
				CallCount:       count,
			}
		}
	}

	return stats
}