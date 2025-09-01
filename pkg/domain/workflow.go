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
	Triggers         []WorkflowTrigger
	Actions          []WorkflowNode
	LastUpdatedAt    time.Time
	ActivationStatus WorkflowActivationStatus
	DeletedAt        *time.Time
}

func (w Workflow) IsActive() bool {
	return w.ActivationStatus == WorkflowActivationStatusActive
}

func (w Workflow) GetActionNodeByID(nodeID string) (WorkflowNode, bool) {
	for _, action := range w.Actions {
		if action.ID == nodeID {
			return action, true
		}
	}

	return WorkflowNode{}, false
}

func (w Workflow) GetTriggerByID(triggerID string) (WorkflowTrigger, bool) {
	for _, trigger := range w.Triggers {
		if trigger.ID == triggerID {
			return trigger, true
		}
	}

	return WorkflowTrigger{}, false
}

type WorkflowNode struct {
	ID                           string
	WorkflowID                   string
	Name                         string
	NodeType                     IntegrationType
	ActionType                   IntegrationActionType
	SubscribedEvents             []string
	Positions                    NodePositions
	IntegrationSettings          map[string]any
	ExpressionSelectedProperties []string
	ProvidedByAgent              []string
	Inputs                       []NodeInput
	UsageContext                 string
}

type NodeInput struct {
	InputID          string
	SubscribedEvents []string
}

type NodePositions struct {
	XPosition float64
	YPosition float64
}

type WorkflowTrigger struct {
	ID                  string
	WorkflowID          string
	Name                string
	Description         string
	Type                IntegrationType
	EventType           IntegrationTriggerEventType
	IntegrationSettings map[string]any
	Positions           NodePositions
}
