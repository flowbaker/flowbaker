package domain

import "time"

type NodeItems struct {
	FromNodeID string `json:"from_node_id"`
	Items      []Item `json:"items"`
}

type NodeItemsMap map[int]NodeItems

func NewNodeItemsMap(index int, fromNodeID string, items []Item) NodeItemsMap {
	return NodeItemsMap{
		index: {FromNodeID: fromNodeID, Items: items},
	}
}

func (m NodeItemsMap) Set(index int, fromNodeID string, items []Item) NodeItemsMap {
	m[index] = NodeItems{FromNodeID: fromNodeID, Items: items}
	return m
}

type ErrorItem struct {
	ErrorMessage string `json:"error_message"`
}

func NewErrorIntegrationOutput(err error) IntegrationOutput {
	return IntegrationOutput{
		ItemsByOutputIndex: NewNodeItemsMap(0, "", []Item{ErrorItem{
			ErrorMessage: err.Error(),
		}}),
	}
}

type NodeExecutionEntry struct {
	NodeID             string
	ItemsByInputIndex  NodeItemsMap
	ItemsByOutputIndex NodeItemsMap
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
