package domain

type PollingEvent struct {
	IntegrationType IntegrationType
	Trigger         WorkflowTrigger
	Workflow        Workflow
	UserID          string
	WorkflowType    WorkflowType
	WorkspaceID     string
}

type PollResult struct {
	LastModifiedData string
}
