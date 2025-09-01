package domain

import (
	"context"
	"time"
)

type Schedule struct {
	ID                           string
	WorkflowID                   string
	ScheduleCreatedAt            time.Time
	TriggerID                    string
	UserID                       string
	WorkflowType                 WorkflowType
	IntegrationType              IntegrationType
	LastCheckedAt                time.Time
	NextScheduledCheckAt         time.Time
	IsActive                     bool
	LastModifiedData             string
	PollingScheduleGap_AsSeconds int
}

type TaskSchedulerService interface {
	Schedule(ctx context.Context, params CreateScheduleParams) (Schedule, error)
	DeleteSchedule(ctx context.Context, scheduleID string, workflowType WorkflowType) error
	GetSchedule(ctx context.Context, scheduleID string, workflowID string) (Schedule, error)
	DeleteWorkflowSchedules(ctx context.Context, workflowID string, workflowType WorkflowType) error
	StartPolling(ctx context.Context) error
	PollSchedules(ctx context.Context, limit int) error
	ProcessSchedule(ctx context.Context, schedule Schedule) error
}
