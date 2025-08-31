package domain

import (
	"context"
)

type ExecutorScheduleManager interface {
	GetSchedule(ctx context.Context, workspaceID string, scheduleID string, workflowID string) (Schedule, error)
	CreateSchedule(ctx context.Context, params CreateScheduleParams) (Schedule, error)
}

type CreateScheduleParams struct {
	WorkflowID                   string
	TriggerID                    string
	IntegrationType              IntegrationType
	WorkflowType                 WorkflowType
	UserID                       string
	LastModifiedData             string
	PollingScheduleGap_AsSeconds int
	WorkspaceID                  string
}
