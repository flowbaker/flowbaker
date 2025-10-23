package startupswatchintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/rs/zerolog/log"
)

type StartupsWatchPollingHandler struct {
	credentialGetter        domain.CredentialGetter[StartupsWatchCredential]
	executorScheduleManager domain.ExecutorScheduleManager
	taskPublisher           domain.ExecutorTaskPublisher
	binder                  domain.IntegrationParameterBinder
}

func NewStartupsWatchPollingHandler(deps domain.IntegrationDeps) domain.IntegrationPoller {
	return &StartupsWatchPollingHandler{
		credentialGetter:        managers.NewExecutorCredentialGetter[StartupsWatchCredential](deps.ExecutorCredentialManager),
		executorScheduleManager: deps.ExecutorScheduleManager,
		taskPublisher:           deps.ExecutorTaskPublisher,
		binder:                  deps.ParameterBinder,
	}
}

func (h *StartupsWatchPollingHandler) HandlePollingEvent(ctx context.Context, p domain.PollingEvent) (domain.PollResult, error) {
	credentialID, ok := p.Trigger.IntegrationSettings["credential_id"]
	if !ok {
		return domain.PollResult{}, fmt.Errorf("credential_id not found in integration settings")
	}

	credentialIDStr, ok := credentialID.(string)
	if !ok {
		return domain.PollResult{}, fmt.Errorf("credential_id is not a string")
	}

	credential, err := h.credentialGetter.GetDecryptedCredential(ctx, credentialIDStr)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to get credential: %w", err)
	}

	switch p.Trigger.EventType {
	case IntegrationTriggerType_OnNewStartups:
		return h.PollNewStartups(ctx, p, credential)
	}

	return domain.PollResult{}, fmt.Errorf("poll function not found for event type: %s", p.Trigger.EventType)
}

type PollStartupsResponse struct {
	Startups []Startup `json:"startups"`
}

type Startup struct {
	ID string `json:"id"`
}

func (h *StartupsWatchPollingHandler) PollNewStartups(ctx context.Context, p domain.PollingEvent, credential StartupsWatchCredential) (domain.PollResult, error) {
	if p.Trigger.IntegrationSettings == nil {
		return domain.PollResult{}, fmt.Errorf("integration settings are nil")
	}

	schedule, err := h.executorScheduleManager.GetSchedule(ctx, p.WorkspaceID, p.Trigger.ID, p.Workflow.ID)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to get schedule: %w", err)
	}

	integration, err := NewStartupsWatchIntegration(ctx, StartupsWatchIntegrationDependencies{
		CredentialID:     "",
		ParameterBinder:  h.binder,
		CredentialGetter: h.credentialGetter,
	})
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to create integration: %w", err)
	}
	integration.token = credential.Token

	page := 1
	limit := "100"

	lastModifiedData := schedule.LastModifiedData
	if lastModifiedData == "" {
		limit = "1"
	}

	queryParams := map[string]string{
		"page":  strconv.Itoa(page),
		"limit": limit,
	}

	response, err := integration.makeRequest(ctx, "/startups", queryParams)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to fetch startups: %w", err)
	}

	// Unmarshal for ID extraction (for comparison)
	var pollingStartupsResponse PollStartupsResponse
	if err := json.Unmarshal(response, &pollingStartupsResponse); err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to unmarshal polling response: %w", err)
	}

	if len(pollingStartupsResponse.Startups) == 0 {
		log.Info().Msg("No startups found")
		return domain.PollResult{
			LastModifiedData: lastModifiedData,
		}, nil
	}

	var startupsResp ListStartupsResponse
	for _, startup := range pollingStartupsResponse.Startups {
		startupResponse, err := integration.makeRequest(ctx, "/startups/"+startup.ID, map[string]string{})
		if err != nil {
			return domain.PollResult{}, fmt.Errorf("failed to fetch startup: %w", err)
		}
		var startupResp any
		if err := json.Unmarshal(startupResponse, &startupResp); err != nil {
			return domain.PollResult{}, fmt.Errorf("failed to unmarshal startup response: %w", err)
		}

		startupsResp.Startups = append(startupsResp.Startups, startupResp)

	}

	// Handle first-time polling (empty lastModifiedData)
	if lastModifiedData == "" {
		startupID := pollingStartupsResponse.Startups[0].ID
		startupIDInt, err := strconv.Atoi(startupID)
		if err != nil {
			return domain.PollResult{}, fmt.Errorf("failed to convert startup ID to int: %w", err)
		}
		fullStartup := startupsResp.Startups[0]

		payloadBytes, err := json.Marshal(fullStartup)
		if err != nil {
			return domain.PollResult{}, fmt.Errorf("failed to marshal startup payload: %w", err)
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
			return domain.PollResult{}, fmt.Errorf("failed to enqueue task: %w", err)
		}

		return domain.PollResult{
			LastModifiedData: strconv.Itoa(startupIDInt),
		}, nil
	}

	// The API provides id as a string representing an integer, so we need to convert it to an int for accurate comparison.
	lastModifiedDataInt, err := strconv.Atoi(lastModifiedData)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("unable to convert lastModifiedData to integer: %w", err)
	}

	newLastModifiedData := lastModifiedDataInt
	{
		for i, pollStartup := range pollingStartupsResponse.Startups {
			startupID := pollStartup.ID
			startupIDInt, err := strconv.Atoi(startupID)
			if err != nil {
				log.Warn().Err(err).Str("startupID", startupID).Msg("Failed to convert startup ID to int, skipping")
				continue
			}

			if startupIDInt > lastModifiedDataInt {
				fullStartup := startupsResp.Startups[i]

				payloadBytes, err := json.Marshal(fullStartup)
				if err != nil {
					log.Error().Err(err).Str("startupID", startupID).Msg("Failed to marshal startup payload")
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
					log.Error().Err(err).Str("startupID", startupID).Msg("Failed to enqueue task")
					continue
				}

				if startupIDInt > newLastModifiedData {
					newLastModifiedData = startupIDInt
				}
				log.Info().Str("startupID", startupID).Msg("New startup processed")
			}
		}
	}

	return domain.PollResult{
		LastModifiedData: strconv.Itoa(newLastModifiedData),
	}, nil
}
