package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

type DiscordPollingHandler struct {
	credentialGetter     domain.CredentialGetter[DiscordCredential]
	taskPublisher        domain.ExecutorTaskPublisher
	taskSchedulerService domain.ExecutorScheduleManager
}

func NewDiscordPollingHandler(deps domain.IntegrationDeps) domain.IntegrationPoller {
	credentialGetter := managers.NewExecutorCredentialGetter[DiscordCredential](deps.ExecutorCredentialManager)

	return &DiscordPollingHandler{
		credentialGetter:     credentialGetter,
		taskPublisher:        deps.ExecutorTaskPublisher,
		taskSchedulerService: deps.ExecutorScheduleManager,
	}
}

func (i *DiscordPollingHandler) HandlePollingEvent(ctx context.Context, p domain.PollingEvent) (domain.PollResult, error) {
	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("workflowID", p.Workflow.ID).
		Str("triggerID", p.Trigger.ID).
		Str("eventType", string(p.Trigger.EventType)).
		Str("userID", p.UserID).
		Interface("integrationSettings", p.Trigger.IntegrationSettings).
		Msg("DiscordPollingHandler: Starting to handle polling event")

	credentialID, ok := p.Trigger.IntegrationSettings["credential_id"]
	if !ok {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Interface("integrationSettings", p.Trigger.IntegrationSettings).
			Msg("DiscordPollingHandler: credential_id not found in integration settings")
		return domain.PollResult{}, fmt.Errorf("credential_id not found in integration settings")
	}

	credentialIDStr, ok := credentialID.(string)
	if !ok {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Interface("credentialID", credentialID).
			Msg("DiscordPollingHandler: credential_id is not a string")
		return domain.PollResult{}, fmt.Errorf("credential_id is not a string")
	}

	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("credentialID", credentialIDStr).
		Msg("DiscordPollingHandler: Getting decrypted credential")

	credential, err := i.credentialGetter.GetDecryptedCredential(ctx, credentialIDStr)
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("credentialID", credentialIDStr).
			Msg("DiscordPollingHandler: Failed to get credential")
		return domain.PollResult{}, fmt.Errorf("failed to get credential: %w", err)
	}

	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("credentialID", credentialIDStr).
		Bool("hasToken", credential.Token != "").
		Msg("DiscordPollingHandler: Creating Discord session")

	discordSession, err := discordgo.New("Bot " + credential.Token)
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("credentialID", credentialIDStr).
			Msg("DiscordPollingHandler: Failed to create Discord session")
		return domain.PollResult{}, fmt.Errorf("failed to create Discord session: %w", err)
	}

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("eventType", string(p.Trigger.EventType)).
		Msg("DiscordPollingHandler: Processing event type")

	switch p.Trigger.EventType {
	case IntegrationTriggerType_MessageReceived:
		log.Info().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Msg("DiscordPollingHandler: Calling PollChannelMessages")
		return i.PollChannelMessages(ctx, p, discordSession)
	}

	log.Error().
		Str("workspaceID", p.WorkspaceID).
		Str("eventType", string(p.Trigger.EventType)).
		Msg("DiscordPollingHandler: Poll function not found for event type")
	return domain.PollResult{}, fmt.Errorf("poll function not found for event type: %s", p.Trigger.EventType)
}

func (i *DiscordPollingHandler) PollChannelMessages(ctx context.Context, p domain.PollingEvent, discordSession *discordgo.Session) (domain.PollResult, error) {
	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("triggerID", p.Trigger.ID).
		Str("workflowID", p.Workflow.ID).
		Msg("DiscordPollingHandler: Starting PollChannelMessages")

	if p.Trigger.IntegrationSettings == nil {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Msg("DiscordPollingHandler: Integration settings are nil")
		return domain.PollResult{}, fmt.Errorf("integration settings are nil")
	}

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Interface("channelID", p.Trigger.IntegrationSettings["channel_id"]).
		Msg("DiscordPollingHandler: Polling channel messages")

	// Get and validate channel ID
	channelIDVal, ok := p.Trigger.IntegrationSettings["channel_id"]
	if !ok {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Interface("integrationSettings", p.Trigger.IntegrationSettings).
			Msg("DiscordPollingHandler: channel_id not found in integration settings")
		return domain.PollResult{}, fmt.Errorf("channel_id not found in integration settings")
	}

	channelID, ok := channelIDVal.(string)
	if !ok {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Interface("channelIDVal", channelIDVal).
			Msg("DiscordPollingHandler: channel_id is not a string")
		return domain.PollResult{}, fmt.Errorf("channel_id is not a string")
	}

	if channelID == "" {
		log.Error().
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Msg("DiscordPollingHandler: channel_id is empty")
		return domain.PollResult{}, fmt.Errorf("channel_id is empty")
	}

	log.Debug().
		Str("workspaceID", p.WorkspaceID).
		Str("channelID", channelID).
		Msg("DiscordPollingHandler: Getting schedule")

	schedule, err := i.taskSchedulerService.GetSchedule(ctx, p.WorkspaceID, p.Trigger.ID, p.Workflow.ID)
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("triggerID", p.Trigger.ID).
			Str("workflowID", p.Workflow.ID).
			Msg("DiscordPollingHandler: Failed to get schedule")
		return domain.PollResult{}, fmt.Errorf("failed to get schedule: %w", err)
	}

	lastModifiedData := schedule.LastModifiedData

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("channelID", channelID).
		Str("lastModifiedData", lastModifiedData).
		Time("scheduleCreatedAt", schedule.ScheduleCreatedAt).
		Time("lastCheckedAt", schedule.LastCheckedAt).
		Msg("DiscordPollingHandler: Fetching messages from Discord API")

	messages, err := discordSession.ChannelMessages(channelID, 100, "", lastModifiedData, "")
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", p.WorkspaceID).
			Str("channelID", channelID).
			Str("afterID", lastModifiedData).
			Msg("DiscordPollingHandler: Failed to fetch messages from Discord API")
		return domain.PollResult{}, err
	}

	log.Info().
		Str("workspaceID", p.WorkspaceID).
		Str("channelID", channelID).
		Int("messageCount", len(messages)).
		Msg("DiscordPollingHandler: Successfully fetched messages from Discord API")

	if lastModifiedData == "" {
		var lastSnowflake string

		for _, message := range messages {
			messageTimestamp, err := snowflakeToTime(message.ID)
			if err != nil {
				log.Error().Err(err).Str("messageID", message.ID).Msg("Failed to convert message ID to time")
				continue
			}

			if messageTimestamp.After(schedule.ScheduleCreatedAt) {
				log.Info().Str("messageID", message.ID).Str("messageContent", message.Content).Msg("Enqueuing task")
				lastSnowflake = message.ID
			} else {
				break
			}

			payloadBytes, err := json.Marshal(message)
			if err != nil {
				log.Error().Err(err).Str("messageID", message.ID).Msg("Failed to marshal message payload")
				continue
			}

			err = i.taskPublisher.EnqueueTask(ctx, p.WorkspaceID, domain.ExecuteWorkflowTask{
				WorkspaceID:  p.WorkspaceID,
				WorkflowID:   p.Workflow.ID,
				UserID:       p.UserID,
				WorkflowType: p.WorkflowType,
				FromNodeID:   p.Trigger.ID,
				Payload:      string(payloadBytes),
			})
			if err != nil {
				log.Error().Err(err).Str("messageID", message.ID).Msg("Failed to enqueue task")
				continue
			}

			if message.ID > lastSnowflake {
				lastSnowflake = message.ID
			}
		}

		log.Info().Str("lastSnowflake", lastSnowflake).Msg("Last snowflake")

		return domain.PollResult{
			LastModifiedData: lastSnowflake,
		}, nil

	}

	log.Info().Int("message_count", len(messages)).Msg("Fetched messages count")

	newLastMessageID := lastModifiedData
	for _, message := range messages {
		if message.ID > lastModifiedData {
			log.Info().Str("messageID", message.ID).Msg("Enqueuing task")

			payloadBytes, err := json.Marshal(message)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal message payload")
				continue
			}

			err = i.taskPublisher.EnqueueTask(ctx, p.WorkspaceID, domain.ExecuteWorkflowTask{
				WorkflowID:   p.Workflow.ID,
				UserID:       p.UserID,
				WorkflowType: p.WorkflowType,
				FromNodeID:   p.Trigger.ID,
				Payload:      string(payloadBytes),
			})

			if err != nil {
				log.Error().Err(err).Str("messageID", message.ID).Msg("Failed to enqueue task")
				continue
			}

			newLastMessageID = message.ID
			log.Info().Str("newLastMessageID", newLastMessageID).Msg("New last message ID")
		}
	}

	return domain.PollResult{
		LastModifiedData: newLastMessageID,
	}, nil
}

func snowflakeToTime(snowflake string) (time.Time, error) {
	const discordEpoch = 1420070400000
	id, err := strconv.ParseInt(snowflake, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	timestamp := (id >> 22) + discordEpoch
	return time.Unix(timestamp/1000, (timestamp%1000)*int64(time.Millisecond)), nil
}
