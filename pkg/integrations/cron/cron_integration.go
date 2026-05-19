package cronintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

type CronPollingHandler struct {
	TaskPublisher   domain.ExecutorTaskPublisher
	ParameterBinder domain.IntegrationParameterBinder
}

func NewCronPollingHandler(deps domain.IntegrationDeps) domain.IntegrationPoller {
	return &CronPollingHandler{
		TaskPublisher:   deps.ExecutorTaskPublisher,
		ParameterBinder: deps.ParameterBinder,
	}
}

func (h *CronPollingHandler) HandlePollingEvent(ctx context.Context, params domain.PollingEvent) (domain.PollResult, error) {
	log.Info().
		Str("trigger_id", params.Trigger.ID).
		Str("workflow_id", params.Workflow.ID).
		Str("event_type", string(params.Trigger.TriggerNodeOpts.EventType)).
		Msg("Handling polling event")

	switch params.Trigger.TriggerNodeOpts.EventType {
	case IntegrationTriggerType_Cron:
		return h.HandleCronTrigger(ctx, params)
	case IntegrationTriggerType_Simple:
		return h.HandleSimpleTrigger(ctx, params)
	default:
		return domain.PollResult{}, fmt.Errorf("unsupported trigger event type: %s", params.Trigger.TriggerNodeOpts.EventType)
	}
}

type HandleCronTriggerParams struct {
	CronString string `json:"cron"`
}

type HandleSimpleTriggerParams struct {
	Interval string `json:"interval"`
	Second   int    `json:"second"`
	Minute   int    `json:"minute"`
	Hour     int    `json:"hour"`
	Day      int    `json:"day"`
}

func (h *CronPollingHandler) HandleCronTrigger(ctx context.Context, event domain.PollingEvent) (domain.PollResult, error) {
	var params HandleCronTriggerParams

	jsonIntegrationSettings, err := json.Marshal(event.Trigger.IntegrationSettings)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to marshal integration settings: %w", err)
	}

	err = json.Unmarshal(jsonIntegrationSettings, &params)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to unmarshal integration settings: %w", err)
	}

	if params.CronString == "" {
		return domain.PollResult{}, fmt.Errorf("cron string is empty")
	}

	return h.enqueueCronRun(ctx, event)
}

func (h *CronPollingHandler) HandleSimpleTrigger(ctx context.Context, event domain.PollingEvent) (domain.PollResult, error) {
	var params HandleSimpleTriggerParams

	jsonIntegrationSettings, err := json.Marshal(event.Trigger.IntegrationSettings)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to marshal integration settings: %w", err)
	}

	err = json.Unmarshal(jsonIntegrationSettings, &params)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to unmarshal integration settings: %w", err)
	}

	if params.Interval == "" {
		return domain.PollResult{}, fmt.Errorf("interval is empty")
	}

	if params.Day == 0 && params.Hour == 0 && params.Minute == 0 && params.Second == 0 {
		return domain.PollResult{}, fmt.Errorf("one of day, hour, minute, or second must be set")
	}

	return h.enqueueCronRun(ctx, event)
}

func (h *CronPollingHandler) enqueueCronRun(ctx context.Context, event domain.PollingEvent) (domain.PollResult, error) {
	log.Info().
		Str("workflow_id", event.Workflow.ID).
		Str("workflow_type", string(event.WorkflowType)).
		Str("trigger_id", event.Trigger.ID).
		Msg("Enqueuing cron run")

	payload := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to marshal cron event: %w", err)
	}

	if err := h.TaskPublisher.EnqueueTask(ctx, event.WorkspaceID, domain.ExecuteWorkflowTask{
		WorkspaceID:  event.WorkspaceID,
		WorkflowID:   event.Workflow.ID,
		UserID:       event.UserID,
		WorkflowType: event.WorkflowType,
		FromNodeID:   event.Trigger.ID,
		Payload:      string(payloadBytes),
	}); err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to enqueue cron task: %w", err)
	}

	return domain.PollResult{}, nil
}
