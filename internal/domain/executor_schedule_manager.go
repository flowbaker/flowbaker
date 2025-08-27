package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"flowbaker/pkg/flowbaker"
)

type ExecutorScheduleManager interface {
	GetSchedule(ctx context.Context, workspaceID string, scheduleID string, workflowID string) (Schedule, error)
	CreateSchedule(ctx context.Context, params CreateScheduleParams) (Schedule, error)
}

type executorScheduleManager struct {
	client flowbaker.ClientInterface
}

type ExecutorScheduleManagerDependencies struct {
	Client flowbaker.ClientInterface
}

func NewExecutorScheduleManager(deps ExecutorScheduleManagerDependencies) ExecutorScheduleManager {
	return &executorScheduleManager{
		client: deps.Client,
	}
}

func (m *executorScheduleManager) GetSchedule(ctx context.Context, workspaceID string, scheduleID string, workflowID string) (Schedule, error) {
	if scheduleID == "" {
		return Schedule{}, fmt.Errorf("schedule ID cannot be empty")
	}

	if workflowID == "" {
		return Schedule{}, fmt.Errorf("workflow ID cannot be empty")
	}

	responseJSON, err := m.client.GetSchedule(ctx, workspaceID, scheduleID, workflowID)
	if err != nil {
		return Schedule{}, fmt.Errorf("failed to get schedule: %w", err)
	}

	var flowbakerSchedule flowbaker.Schedule
	if err := json.Unmarshal(responseJSON, &flowbakerSchedule); err != nil {
		return Schedule{}, fmt.Errorf("failed to unmarshal schedule response: %w", err)
	}

	domainSchedule := Schedule{
		ID:                           flowbakerSchedule.ID,
		WorkflowID:                   flowbakerSchedule.WorkflowID,
		ScheduleCreatedAt:            flowbakerSchedule.ScheduleCreatedAt,
		TriggerID:                    flowbakerSchedule.TriggerID,
		UserID:                       flowbakerSchedule.UserID,
		WorkflowType:                 WorkflowType(flowbakerSchedule.WorkflowType),
		IntegrationType:              IntegrationType(flowbakerSchedule.IntegrationType),
		LastCheckedAt:                flowbakerSchedule.LastCheckedAt,
		NextScheduledCheckAt:         flowbakerSchedule.NextScheduledCheckAt,
		IsActive:                     flowbakerSchedule.IsActive,
		LastModifiedData:             flowbakerSchedule.LastModifiedData,
		PollingScheduleGap_AsSeconds: flowbakerSchedule.PollingScheduleGapSeconds,
	}

	return domainSchedule, nil
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

func (m *executorScheduleManager) CreateSchedule(ctx context.Context, params CreateScheduleParams) (Schedule, error) {
	if params.WorkflowID == "" {
		return Schedule{}, fmt.Errorf("workflow ID cannot be empty")
	}

	if params.TriggerID == "" {
		return Schedule{}, fmt.Errorf("trigger ID cannot be empty")
	}

	createRequest := flowbaker.CreateScheduleRequest{
		WorkflowID:                params.WorkflowID,
		TriggerID:                 params.TriggerID,
		IntegrationType:           string(params.IntegrationType),
		WorkflowType:              string(params.WorkflowType),
		UserID:                    params.UserID,
		LastModifiedData:          params.LastModifiedData,
		PollingScheduleGapSeconds: params.PollingScheduleGap_AsSeconds,
	}

	responseJSON, err := m.client.CreateSchedule(ctx, params.WorkspaceID, &createRequest)
	if err != nil {
		return Schedule{}, fmt.Errorf("failed to create schedule: %w", err)
	}

	var flowbakerResponse flowbaker.CreateScheduleResponse
	if err := json.Unmarshal(responseJSON, &flowbakerResponse); err != nil {
		return Schedule{}, fmt.Errorf("failed to unmarshal create schedule response: %w", err)
	}

	domainSchedule := Schedule{
		ID:                           flowbakerResponse.Schedule.ID,
		WorkflowID:                   flowbakerResponse.Schedule.WorkflowID,
		ScheduleCreatedAt:            flowbakerResponse.Schedule.ScheduleCreatedAt,
		TriggerID:                    flowbakerResponse.Schedule.TriggerID,
		UserID:                       flowbakerResponse.Schedule.UserID,
		WorkflowType:                 WorkflowType(flowbakerResponse.Schedule.WorkflowType),
		IntegrationType:              IntegrationType(flowbakerResponse.Schedule.IntegrationType),
		LastCheckedAt:                flowbakerResponse.Schedule.LastCheckedAt,
		NextScheduledCheckAt:         flowbakerResponse.Schedule.NextScheduledCheckAt,
		IsActive:                     flowbakerResponse.Schedule.IsActive,
		LastModifiedData:             flowbakerResponse.Schedule.LastModifiedData,
		PollingScheduleGap_AsSeconds: flowbakerResponse.Schedule.PollingScheduleGapSeconds,
	}

	return domainSchedule, nil
}
