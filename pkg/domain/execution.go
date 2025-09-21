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
	// Agent-specific fields
	NodeType        string                `json:"node_type,omitempty"`        // "llm", "memory", "tool"
	NodeName        string                `json:"node_name,omitempty"`
	ToolName        string                `json:"tool_name,omitempty"`
	ActionType      IntegrationActionType `json:"action_type,omitempty"`
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
