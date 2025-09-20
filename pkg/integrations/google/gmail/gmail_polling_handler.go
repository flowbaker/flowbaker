package gmail

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailPollingHandler struct {
	credentialGetter        domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	executorScheduleManager domain.ExecutorScheduleManager
	taskPublisher           domain.ExecutorTaskPublisher
	binder                  domain.IntegrationParameterBinder
}

func NewGmailPollingHandler(deps domain.IntegrationDeps) domain.IntegrationPoller {
	return &GmailPollingHandler{
		credentialGetter:        managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		executorScheduleManager: deps.ExecutorScheduleManager,
		taskPublisher:           deps.ExecutorTaskPublisher,
		binder:                  deps.ParameterBinder,
	}
}

func (h *GmailPollingHandler) HandlePollingEvent(ctx context.Context, p domain.PollingEvent) (domain.PollResult, error) {
	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("workflowID", p.Workflow.ID).
		Str("triggerID", p.Trigger.ID).
		Str("eventType", string(p.Trigger.EventType)).
		Str("userID", p.UserID).
		Interface("integrationSettings", p.Trigger.IntegrationSettings).
		Msg("GmailPollingHandler: Starting to handle polling event")

	integration := &GmailIntegration{
		credentialGetter: h.credentialGetter,
		binder:           h.binder,
		service:          nil,
	}

	credentialID := p.Trigger.IntegrationSettings["credential_id"]
	if credentialID == nil {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Interface("integrationSettings", p.Trigger.IntegrationSettings).
			Msg("GmailPollingHandler: credential_id not found in integration settings")
		return domain.PollResult{}, fmt.Errorf("credential_id not found in integration settings")
	}

	credentialIDStr, ok := credentialID.(string)

	if !ok {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Interface("credentialID", credentialID).
			Msg("GmailPollingHandler: credential_id is not a string")
		return domain.PollResult{}, fmt.Errorf("credential_id is not a string")
	}

	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("credentialID", credentialIDStr).
		Msg("GmailPollingHandler: Getting decrypted credential")

	oauthAccount, err := h.credentialGetter.GetDecryptedCredential(ctx, credentialIDStr)
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("credentialID", credentialIDStr).
			Msg("GmailPollingHandler: Failed to get OAuth account")
		return domain.PollResult{}, fmt.Errorf("failed to get oauth account: %w", err)
	}

	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("credentialID", credentialIDStr).
		Msg("GmailPollingHandler: Getting Gmail client")

	client, err := integration.getClient(ctx, oauthAccount)
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("credentialID", credentialIDStr).
			Msg("GmailPollingHandler: Failed to get Gmail client")
		return domain.PollResult{}, fmt.Errorf("failed to get client: %w", err)
	}

	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("credentialID", credentialIDStr).
		Msg("GmailPollingHandler: Creating Gmail service")

	service, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("credentialID", credentialIDStr).
			Msg("GmailPollingHandler: Failed to create Gmail service")
		return domain.PollResult{}, fmt.Errorf("failed to create gmail service: %w", err)
	}

	integration.service = service

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("eventType", string(p.Trigger.EventType)).
		Msg("GmailPollingHandler: Processing event type")

	switch p.Trigger.EventType {
	case IntegrationTriggerType_OnMessageReceived:
		log.Info().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Msg("GmailPollingHandler: Calling OnMessageReceived")
		return h.OnMessageReceived(ctx, p, integration)
	}

	log.Warn().
		Str("workspaceID", p.WorkspaceID).
		Str("eventType", string(p.Trigger.EventType)).
		Msg("GmailPollingHandler: No handler for event type, returning empty result")

	return domain.PollResult{}, nil
}

func (h *GmailPollingHandler) OnMessageReceived(ctx context.Context, p domain.PollingEvent, integration *GmailIntegration) (domain.PollResult, error) {
	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("triggerID", p.Trigger.ID).
		Str("workflowID", p.Workflow.ID).
		Msg("GmailPollingHandler: Starting OnMessageReceived")

	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("triggerID", p.Trigger.ID).
		Msg("GmailPollingHandler: Getting schedule")

	schedule, err := h.executorScheduleManager.GetSchedule(ctx, p.WorkspaceID, p.Trigger.ID, p.Workflow.ID)
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Str("workflowID", p.Workflow.ID).
			Msg("GmailPollingHandler: Failed to get schedule")
		return domain.PollResult{}, fmt.Errorf("failed to get schedule: %w", err)
	}

	scheduleLastCheckedAt := schedule.LastCheckedAt.Unix()

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Int64("lastCheckedAt", scheduleLastCheckedAt).
		Time("lastCheckedAtTime", schedule.LastCheckedAt).
		Msg("GmailPollingHandler: Querying Gmail for new messages")

	query := fmt.Sprintf("after:%d", scheduleLastCheckedAt)
	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("gmailQuery", query).
		Msg("GmailPollingHandler: Gmail API query")

	newMessages, err := integration.service.Users.Messages.List("me").Q(query).Do()
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("query", query).
			Msg("GmailPollingHandler: Failed to get messages from Gmail API")
		return domain.PollResult{}, fmt.Errorf("failed to get messages: %w", err)
	}

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Int("messageCount", len(newMessages.Messages)).
		Msg("GmailPollingHandler: Retrieved messages from Gmail API")

	successCount := 0
	failureCount := 0

	for _, message := range newMessages.Messages {
		log.Debug().
			Str("workspaceID", p.WorkspaceID).
			Str("messageID", message.Id).
			Msg("GmailPollingHandler: Processing message")

		payloadBytes, err := json.Marshal(message)
		if err != nil {
			log.Error().
				Err(err).
				Str("workspaceID", p.WorkspaceID).
				Str("messageID", message.Id).
				Msg("GmailPollingHandler: Failed to marshal message payload")
			failureCount++
			continue
		}

		err = h.taskPublisher.EnqueueTask(ctx, p.WorkspaceID, domain.ExecuteWorkflowTask{
			WorkspaceID:  p.WorkspaceID,
			WorkflowID:   p.Workflow.ID,
			UserID:       p.UserID,
			WorkflowType: p.WorkflowType,
			FromNodeID:   p.Trigger.ID,
			Payload:      string(payloadBytes),
		})

		if err != nil {
			log.Error().
				Err(err).
				Str("workspaceID", p.WorkspaceID).
				Str("messageID", message.Id).
				Msg("GmailPollingHandler: Failed to enqueue task")
			failureCount++
			continue
		}

		log.Info().
			Str("workspaceID", p.WorkspaceID).
			Str("messageID", message.Id).
			Str("workflowID", p.Workflow.ID).
			Msg("GmailPollingHandler: Successfully enqueued task for message")
		successCount++
	}

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Int("totalMessages", len(newMessages.Messages)).
		Int("successCount", successCount).
		Int("failureCount", failureCount).
		Msg("GmailPollingHandler: Completed message processing")

	return domain.PollResult{
		LastModifiedData: "",
	}, nil
}
