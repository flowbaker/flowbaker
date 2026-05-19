package domain

import "time"

type PollingEvent struct {
	IntegrationType  IntegrationType
	Trigger          WorkflowNode
	Workflow         Workflow
	UserID           string
	WorkflowType     WorkflowType
	WorkspaceID      string
	LastModifiedData string
	BootstrapTime    time.Time
}

type PollResult struct {
	LastModifiedData string
}
