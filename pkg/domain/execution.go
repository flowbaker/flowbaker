package domain

import "time"

type NodeItems struct {
	FromNodeID string `json:"from_node_id"`
	Items      []Item `json:"items"`
}

type NodeExecutionEntry struct {
	NodeID             string
	ItemsByInputIndex  map[int]NodeItems
	ItemsByOutputIndex map[int]NodeItems
	EventType          EventType
	Error              string
	Timestamp          int64
	ExecutionOrder     int
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

type InputItemsCount map[int]int64
type InputItemsSizeInBytes map[int]int64
type OutputItemsCount map[int]int64
type OutputItemsSizeInBytes map[int]int64
