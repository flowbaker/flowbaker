package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailPollingHandler struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	taskPublisher    domain.ExecutorTaskPublisher
	binder           domain.IntegrationParameterBinder
}

func NewGmailPollingHandler(deps domain.IntegrationDeps) domain.IntegrationPoller {
	return &GmailPollingHandler{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		taskPublisher:    deps.ExecutorTaskPublisher,
		binder:           deps.ParameterBinder,
	}
}

func (h *GmailPollingHandler) HandlePollingEvent(ctx context.Context, p domain.PollingEvent) (domain.PollResult, error) {
	integration := &GmailIntegration{
		credentialGetter: h.credentialGetter,
		binder:           h.binder,
		service:          nil,
	}

	credentialID := p.Trigger.IntegrationSettings["credential_id"]
	if credentialID == nil {
		return domain.PollResult{}, fmt.Errorf("credential_id not found in integration settings")
	}

	credentialIDStr, ok := credentialID.(string)

	if !ok {
		return domain.PollResult{}, fmt.Errorf("credential_id is not a string")
	}

	oauthAccount, err := h.credentialGetter.GetDecryptedCredential(ctx, credentialIDStr)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to get oauth account: %w", err)
	}

	client, err := integration.getClient(ctx, oauthAccount)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to get client: %w", err)
	}

	service, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to create gmail service: %w", err)
	}

	integration.service = service

	switch p.Trigger.TriggerNodeOpts.EventType {
	case IntegrationTriggerType_OnMessageReceived:
		return h.OnMessageReceived(ctx, p, integration)
	}

	return domain.PollResult{}, nil
}

func (h *GmailPollingHandler) OnMessageReceived(ctx context.Context, p domain.PollingEvent, integration *GmailIntegration) (domain.PollResult, error) {
	lastCheckedAt := p.BootstrapTime.Unix()
	if p.LastModifiedData != "" {
		if parsed, err := strconv.ParseInt(p.LastModifiedData, 10, 64); err == nil {
			lastCheckedAt = parsed
		}
	}

	newMessages, err := integration.service.Users.Messages.List("me").Q(fmt.Sprintf("in:inbox after:%d", lastCheckedAt)).Do()
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to get messages: %w", err)
	}

	for _, message := range newMessages.Messages {
		payloadBytes, err := json.Marshal(message)
		if err != nil {
			log.Error().Err(err).Str("messageID", message.Id).Msg("Failed to marshal message payload")
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
			log.Error().Err(err).Str("messageID", message.Id).Msg("Failed to enqueue task")
			continue
		}
	}

	return domain.PollResult{
		LastModifiedData: strconv.FormatInt(time.Now().Unix(), 10),
	}, nil
}
