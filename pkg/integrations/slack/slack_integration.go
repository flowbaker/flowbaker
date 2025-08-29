package slackintegration

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
)

const (
	SlackIntegrationActionType_SendMessage domain.IntegrationActionType = "send_message"
	SlackIntegrationActionType_GetMessage  domain.IntegrationActionType = "get_message"

	SlackIntegrationPeekable_Channels domain.IntegrationPeekableType = "channels"
)

type SlackIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewSlackIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &SlackIntegrationCreator{
		binder:           deps.ParameterBinder,
		CredentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *SlackIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewSlackIntegration(ctx, SlackIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.CredentialGetter,
	})
}

type SlackIntegration struct {
	slackClient *slack.Client

	binder domain.IntegrationParameterBinder

	actionFuncs map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error)
	peekFuncs   map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type SlackIntegrationDependencies struct {
	CredentialID     string
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewSlackIntegration(ctx context.Context, deps SlackIntegrationDependencies) (*SlackIntegration, error) {
	integration := &SlackIntegration{
		binder: deps.ParameterBinder,
	}

	actionFuncs := map[domain.IntegrationActionType]func(ctx context.Context, p domain.IntegrationInput) (domain.IntegrationOutput, error){
		SlackIntegrationActionType_SendMessage: integration.SendMessage,
		SlackIntegrationActionType_GetMessage:  integration.GetMessage,
	}

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		SlackIntegrationPeekable_Channels: integration.PeekChannels,
	}

	integration.peekFuncs = peekFuncs
	integration.actionFuncs = actionFuncs

	if integration.slackClient == nil {
		oauthAccount, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
		if err != nil {
			return nil, err
		}

		integration.slackClient = slack.New(oauthAccount.AccessToken)
	}

	return integration, nil
}

func (i *SlackIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	actionFunc, ok := i.actionFuncs[params.ActionType]
	if !ok {
		return domain.IntegrationOutput{}, fmt.Errorf("action function not found")
	}

	return actionFunc(ctx, params)
}

type SendMessageParams struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
}

type SendMessageOutputItem struct {
	ChannelID string `json:"channel_id"`
	Timestamp string `json:"timestamp"`
}

func (i *SlackIntegration) SendMessage(ctx context.Context, input domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := input.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := []domain.Item{}

	for _, items := range itemsByInputID {
		allItems = append(allItems, items...)
	}

	outputs := []domain.Item{}

	for _, item := range allItems {
		p := SendMessageParams{}

		err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		channel, ts, err := i.slackClient.PostMessageContext(ctx, p.ChannelID, slack.MsgOptionText(p.Message, false))
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to send message to channel %s: %w", p.ChannelID, err)
		}

		outputs = append(outputs, SendMessageOutputItem{
			ChannelID: channel,
			Timestamp: ts,
		})
	}

	resultJSON, err := json.Marshal(outputs)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

type GetMessageParams struct {
	ChannelID string `json:"channel_id"`
	MessageID string `json:"message_id"`
}

func (i *SlackIntegration) GetMessage(ctx context.Context, input domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := input.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := []domain.Item{}

	for _, items := range itemsByInputID {
		allItems = append(allItems, items...)
	}

	log.Info().Interface("all_items", allItems).Msg("All items")

	outputs := []domain.Item{}

	for _, item := range allItems {
		log.Info().Interface("item", item).Msg("Item")

		p := GetMessageParams{}

		err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		message, _, _, err := i.slackClient.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
			ChannelID: p.ChannelID,
			Timestamp: p.MessageID,
		})
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to get message from channel %s: %w", p.ChannelID, err)
		}

		outputs = append(outputs, message)
	}

	resultJSON, err := json.Marshal(outputs)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (i *SlackIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

func (i *SlackIntegration) PeekChannels(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	// FIXME: Need peekable pagination here.
	channels, _, err := i.slackClient.GetConversationsContext(ctx, &slack.GetConversationsParameters{
		Types: []string{"public_channel"},
		Limit: 200,
	})
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
