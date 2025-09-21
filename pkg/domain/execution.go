package domain

import "time"

type NodeItems struct {
	FromNodeID string `json:"from_node_id"`
	Items      []Item `json:"items"`
}

type NodeExecutionEntry struct {
	NodeID          string
	ItemsByInputID  map[string]NodeItems
	ItemsByOutputID map[string]NodeItems
	EventType       EventType
	Error           string
	Timestamp       int64
	ExecutionOrder  int
}

type NodeExecution struct {
	ID                     string
	NodeID                 string
	IntegrationType        IntegrationType
	IntegrationActionType  IntegrationActionType
	StartedAt              time.Time
	EndedAt                time.Time
	ExecutionOrder         int64
	InputItemsCount        InputItemsCount
	InputItemsSizeInBytes  InputItemsSizeInBytes
	OutputItemsCount       OutputItemsCount
	OutputItemsSizeInBytes OutputItemsSizeInBytes
}

type InputItemsCount map[string]int64
type InputItemsSizeInBytes map[string]int64
type OutputItemsCount map[int64]int64
type OutputItemsSizeInBytes map[int64]int64
type AgentNodeExecution struct {
	NodeID          string                `json:"node_id"`
	NodeType        string                `json:"node_type"`
	NodeName        string                `json:"node_name"`
	ExecutionTime   time.Time             `json:"execution_time"`
	Success         bool                  `json:"success"`
	Error           string                `json:"error,omitempty"`
	ItemsByInputID  map[string]NodeItems  `json:"items_by_input_id"`
	ItemsByOutputID map[string]NodeItems  `json:"items_by_output_id"`
	ToolName        string                `json:"tool_name,omitempty"`
	ActionType      IntegrationActionType `json:"action_type,omitempty"`
	ExecutionCount  int                   `json:"execution_count"`
}
