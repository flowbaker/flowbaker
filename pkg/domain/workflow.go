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

type Workflow struct {
	ID               string
	Name             string
	Description      string
	Slug             string
	WorkspaceID      string
	AuthorUserID     string
	Nodes            []WorkflowNode
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
	SubscribedOutputs            []Handle
	Positions                    NodePositions
	IntegrationSettings          map[string]any
	Settings                     NodeSettings
	ExpressionSelectedProperties []string
	ProvidedByAgent              []string
	Inputs                       []NodeInput
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

func (n *WorkflowNode) GetInputByIndex(index int) (NodeInput, bool) {
	for _, input := range n.Inputs {
		targetIndex := -1
		if input.Input.Index != -1 {
			targetIndex = input.Input.Index
		}

		if targetIndex != -1 && targetIndex == index {
			return input, true
		}
	}

	return NodeInput{}, false
}

type Handle struct {
	NodeID string `json:"node_id"`
	Index  int    `json:"index"`
}

type NodeInput struct {
	Input             Handle
	SubscribedOutputs []Handle
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
