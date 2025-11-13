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
	credentialID, ok := p.Trigger.IntegrationSettings["credential_id"]
	if !ok {
		return domain.PollResult{}, fmt.Errorf("credential_id not found in integration settings")
	}

	credentialIDStr, ok := credentialID.(string)
	if !ok {
		return domain.PollResult{}, fmt.Errorf("credential_id is not a string")
	}

	credential, err := i.credentialGetter.GetDecryptedCredential(ctx, credentialIDStr)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to get credential: %w", err)
	}

	discordSession, err := discordgo.New("Bot " + credential.Token)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to create Discord session: %w", err)
	}

	log.Info().Str("eventType", string(p.Trigger.EventType)).Msg("Processing event type")

	switch p.Trigger.EventType {
	case IntegrationTriggerType_MessageReceived:
		return i.PollChannelMessages(ctx, p, discordSession)
	}

	return domain.PollResult{}, fmt.Errorf("poll function not found for event type: %s", p.Trigger.EventType)
}

func (i *DiscordPollingHandler) PollChannelMessages(ctx context.Context, p domain.PollingEvent, discordSession *discordgo.Session) (domain.PollResult, error) {
	if p.Trigger.IntegrationSettings == nil {
		return domain.PollResult{}, fmt.Errorf("integration settings are nil")
	}

	channelIDVal, ok := p.Trigger.IntegrationSettings["channel_id"]
	if !ok {
		return domain.PollResult{}, fmt.Errorf("channel_id not found in integration settings")
	}

	channelID, ok := channelIDVal.(string)
	if !ok {
		return domain.PollResult{}, fmt.Errorf("channel_id is not a string")
	}

	if channelID == "" {
		return domain.PollResult{}, fmt.Errorf("channel_id is empty")
	}

	schedule, err := i.taskSchedulerService.GetSchedule(ctx, p.WorkspaceID, p.Trigger.ID, p.Workflow.ID)
	if err != nil {
		return domain.PollResult{}, fmt.Errorf("failed to get schedule: %w", err)
	}

	lastModifiedData := schedule.LastModifiedData

	messages, err := discordSession.ChannelMessages(channelID, 100, "", lastModifiedData, "")
	if err != nil {
		log.Error().Err(err).Str("channelID", channelID).Str("afterID", lastModifiedData).Msg("Failed to fetch messages")
		return domain.PollResult{}, err
	}

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

		return domain.PollResult{
			LastModifiedData: lastSnowflake,
		}, nil

	}

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

			if message.ID > newLastMessageID {
				newLastMessageID = message.ID
				log.Info().Str("newLastMessageID", newLastMessageID).Msg("New last message ID")
			}
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
