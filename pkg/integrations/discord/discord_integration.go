package discord

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

const (
	IntegrationActionType_SendMessage    domain.IntegrationActionType = "send_message"
	IntegrationActionType_SendEmbed      domain.IntegrationActionType = "send_embed"
	IntegrationActionType_SendFile       domain.IntegrationActionType = "send_file"
	IntegrationActionType_SendReply      domain.IntegrationActionType = "send_reply"
	IntegrationActionType_SendReaction   domain.IntegrationActionType = "send_reaction"
	IntegrationActionType_GetChannels    domain.IntegrationActionType = "get_channels"
	IntegrationActionType_GetMessages    domain.IntegrationActionType = "get_messages"
	IntegrationActionType_GetMessageByID domain.IntegrationActionType = "get_message_by_id"
	IntegrationActionType_DeleteMessage  domain.IntegrationActionType = "delete_message"

	IntegrationTriggerType_MessageReceived domain.IntegrationTriggerEventType = "message_received"
)

const (
	DiscordIntegrationPeekable_Guilds   domain.IntegrationPeekableType = "guilds"
	DiscordIntegrationPeekable_Channels domain.IntegrationPeekableType = "channels"
)

type DiscordIntegrationCreator struct {
	binder                   domain.IntegrationParameterBinder
	executorCredentialGetter domain.CredentialGetter[DiscordCredential]
}

type DiscordIntegrationCreatorDeps struct {
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[DiscordCredential]
}

func NewDiscordIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &DiscordIntegrationCreator{
		binder:                   deps.ParameterBinder,
		executorCredentialGetter: managers.NewExecutorCredentialGetter[DiscordCredential](deps.ExecutorCredentialManager),
	}
}

func (c *DiscordIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewDiscordIntegration(ctx, DiscordIntegrationDependencies{
		CredentialGetter: c.executorCredentialGetter,
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
	})
}

type DiscordIntegration struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[DiscordCredential]

	discordSession *discordgo.Session

	actionManager *domain.IntegrationActionManager

	peekFuncs map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type DiscordCredential struct {
	Token string `json:"token"`
}

type DiscordIntegrationDependencies struct {
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialID     string
	CredentialGetter domain.CredentialGetter[DiscordCredential]
}

func NewDiscordIntegration(ctx context.Context, deps DiscordIntegrationDependencies) (*DiscordIntegration, error) {
	integration := &DiscordIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
		actionManager:    domain.NewIntegrationActionManager(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_SendMessage, integration.SendMessages).
		AddPerItem(IntegrationActionType_GetMessages, integration.GetMessages).
		AddPerItem(IntegrationActionType_DeleteMessage, integration.DeleteMessage).
		AddPerItem(IntegrationActionType_GetMessageByID, integration.GetMessageByID)

	integration.actionManager = actionManager

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		DiscordIntegrationPeekable_Guilds:   integration.PeekGuilds,
		DiscordIntegrationPeekable_Channels: integration.PeekChannels,
	}

	integration.peekFuncs = peekFuncs

	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	session, err := discordgo.New(fmt.Sprintf("Bot %s", credential.Token))
	if err != nil {
		return nil, err
	}

	log.Info().Msg("Discord session created")

	integration.discordSession = session

	return integration, nil
}

func (i *DiscordIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type SendMessageParams struct {
	CredentialID string `json:"credential_id"`
	ChannelID    string `json:"channel_id"`
	Content      string `json:"content"`
}

type GetMessagesParams struct {
	CredentialID string `json:"credential_id"`
	ChannelID    string `json:"channel_id"`
	MaxAmount    string `json:"amount"`
	BeforeID     string `json:"before_id"`
	AfterID      string `json:"after_id"`
}

type GetMessageByIDParams struct {
	CredentialID string `json:"credential_id"`
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
}

type DeleteMessageParams struct {
	CredentialID string `json:"credential_id"`
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
}

func (i *DiscordIntegration) SendMessages(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendMessageParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	sentMessage, err := i.discordSession.ChannelMessageSend(p.ChannelID, p.Content)
	if err != nil {
		return nil, err
	}

	return sentMessage, nil
}

func (i *DiscordIntegration) GetMessages(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetMessagesParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	maxAmount, err := strconv.Atoi(p.MaxAmount)
	if err != nil {
		return nil, fmt.Errorf("invalid max amount: %v", err)
	}

	if maxAmount > 100 {
		maxAmount = 100
	}

	messages, err := i.discordSession.ChannelMessages(p.ChannelID, maxAmount, p.BeforeID, p.AfterID, "")
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (i *DiscordIntegration) GetMessageByID(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetMessageByIDParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	message, err := i.discordSession.ChannelMessage(p.ChannelID, p.MessageID)
	if err != nil {
		return nil, err
	}

	return message, nil
}

func (i *DiscordIntegration) DeleteMessage(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteMessageParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	err = i.discordSession.ChannelMessageDelete(p.ChannelID, p.MessageID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (i *DiscordIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

func (i *DiscordIntegration) PeekGuilds(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	guilds, err := i.discordSession.UserGuilds(100, "", "", false)
	if err != nil {
		return domain.PeekResult{}, err
	}

	var results []domain.PeekResultItem

	for _, guild := range guilds {
		results = append(results, domain.PeekResultItem{
			Key:     guild.ID,
			Value:   guild.ID,
			Content: guild.Name,
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekChannelsParams struct {
	GuildID string `json:"guild_id"`
}

func (i *DiscordIntegration) PeekChannels(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	params := PeekChannelsParams{}

	err := json.Unmarshal(p.PayloadJSON, &params)
	if err != nil {
		return domain.PeekResult{}, err
	}

	channels, err := i.discordSession.GuildChannels(params.GuildID)
	if err != nil {
		return domain.PeekResult{}, err
	}

	var results []domain.PeekResultItem

	for _, channel := range channels {
		results = append(results, domain.PeekResultItem{
			Key:     channel.ID,
			Value:   channel.ID,
			Content: channel.Name,
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

const DiscordEpoch int64 = 1420070400000
