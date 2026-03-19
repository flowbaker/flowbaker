package domain

import (
	"errors"
	"time"
)

type WorkflowType string

const (
	WorkflowTypeDefault WorkflowType = "default"
	WorkflowTypeTesting WorkflowType = "testing"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
)

type WorkflowActivationStatus string

const (
	WorkflowActivationStatusActive   WorkflowActivationStatus = "active"
	WorkflowActivationStatusInactive WorkflowActivationStatus = "inactive"
)

type WorkflowEdge struct {
	SourceNodeID string `json:"source_node_id"`
	SourceIndex  int    `json:"source_index"`
	TargetNodeID string `json:"target_node_id"`
	TargetIndex  int    `json:"target_index"`
}

type Workflow struct {
	ID               string
	Name             string
	Description      string
	Slug             string
	WorkspaceID      string
	AuthorUserID     string
	Nodes            []WorkflowNode
	Edges            []WorkflowEdge
	Settings         WorkflowSettings
	LastUpdatedAt    time.Time
	ActivationStatus WorkflowActivationStatus
	DeletedAt        *time.Time
}

type WorkflowSettings struct {
	NodeExecutionLimit int
}

func (w Workflow) IsActive() bool {
	return w.ActivationStatus == WorkflowActivationStatusActive
}

func (w Workflow) GetNodeByID(nodeID string) (WorkflowNode, bool) {
	for _, node := range w.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return WorkflowNode{}, false
}

func (w Workflow) GetTriggerNodes() []WorkflowNode {
	triggerNodes := make([]WorkflowNode, 0)

	for _, node := range w.Nodes {
		if node.Type == NodeTypeTrigger {
			triggerNodes = append(triggerNodes, node)
		}
	}

	return triggerNodes
}

func (w Workflow) GetSubNodes(nodeID string) []WorkflowNode {
	subNodes := make([]WorkflowNode, 0)

	for _, node := range w.Nodes {
		if node.ParentID == nodeID {
			subNodes = append(subNodes, node)
		}
	}

	return subNodes
}

func (w Workflow) GetActionNodes() []WorkflowNode {
	actionNodes := make([]WorkflowNode, 0)

	for _, node := range w.Nodes {
		if node.Type == NodeTypeAction {
			actionNodes = append(actionNodes, node)
		}
	}

	return actionNodes
}

func (w Workflow) GetOutgoingEdges(nodeID string) []WorkflowEdge {
	edges := make([]WorkflowEdge, 0)
	for _, edge := range w.Edges {
		if edge.SourceNodeID == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func (w Workflow) GetIncomingEdges(nodeID string) []WorkflowEdge {
	edges := make([]WorkflowEdge, 0)
	for _, edge := range w.Edges {
		if edge.TargetNodeID == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func (w Workflow) GetConnectedInputIndices(nodeID string) []int {
	seen := map[int]struct{}{}
	indices := make([]int, 0)
	for _, edge := range w.Edges {
		if edge.TargetNodeID == nodeID {
			if _, ok := seen[edge.TargetIndex]; !ok {
				seen[edge.TargetIndex] = struct{}{}
				indices = append(indices, edge.TargetIndex)
			}
		}
	}
	return indices
}

func (w Workflow) FindEdge(targetNodeID, sourceNodeID string, sourceIndex int) (WorkflowEdge, bool) {
	for _, edge := range w.Edges {
		if edge.TargetNodeID == targetNodeID && edge.SourceNodeID == sourceNodeID && edge.SourceIndex == sourceIndex {
			return edge, true
		}
	}
	return WorkflowEdge{}, false
}

func (w Workflow) GetSourceNodesForInput(targetNodeID string, targetIndex int) []string {
	nodeIDs := make([]string, 0)
	for _, edge := range w.Edges {
		if edge.TargetNodeID == targetNodeID && edge.TargetIndex == targetIndex {
			nodeIDs = append(nodeIDs, edge.SourceNodeID)
		}
	}
	return nodeIDs
}

type SourceOutput struct {
	NodeID      string
	OutputIndex int
}

type EdgeIndex struct {
	targetNodesBySourceOutput map[SourceOutput][]WorkflowNode
}

func NewEdgeIndex(w Workflow) EdgeIndex {
	m := make(map[SourceOutput][]WorkflowNode)

	for _, edge := range w.Edges {
		key := SourceOutput{NodeID: edge.SourceNodeID, OutputIndex: edge.SourceIndex}

		if node, ok := w.GetNodeByID(edge.TargetNodeID); ok {
			m[key] = append(m[key], node)
		}
	}

	return EdgeIndex{
		targetNodesBySourceOutput: m,
	}
}

func (idx EdgeIndex) GetTargetNodes(nodeID string, outputIndex int) []WorkflowNode {
	sourceOutput := SourceOutput{
		NodeID:      nodeID,
		OutputIndex: outputIndex,
	}

	return idx.targetNodesBySourceOutput[sourceOutput]
}

type NodeType string

const (
	NodeTypeTrigger NodeType = "trigger"
	NodeTypeAction  NodeType = "action"
)

type WorkflowNode struct {
	ID                           string
	WorkflowID                   string
	Name                         string
	Type                         NodeType
	IntegrationType              IntegrationType
	Positions                    NodePositions
	IntegrationSettings          map[string]any
	Settings                     NodeSettings
	ExpressionSelectedProperties []string
	ProvidedByAgent              []string
	UsageContext                 string
	ParentID                     string

	TriggerNodeOpts TriggerNodeOpts `json:"trigger_node_opts,omitempty"`
	ActionNodeOpts  ActionNodeOpts  `json:"action_node_opts,omitempty"`
}

type TriggerNodeOpts struct {
	EventType IntegrationTriggerEventType `json:"event_type,omitempty"`
}

type ActionNodeOpts struct {
	ActionType IntegrationActionType `json:"action_type,omitempty"`
}

type NodeSettings struct {
	ReturnErrorAsItem       bool
	OverwriteExecutionLimit bool
	ExecutionLimit          int
}

type NodePositions struct {
	XPosition float64
	YPosition float64
}
