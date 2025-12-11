package domain

type PollingEvent struct {
	IntegrationType IntegrationType
	Trigger         WorkflowNode
	Workflow        Workflow
	UserID          string
	WorkflowType    WorkflowType
	WorkspaceID     string
}

type PollResult struct {
	LastModifiedData string
}
