package telegram

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramIntegrationCreator struct {
	credentialGetter       domain.CredentialGetter[TelegramCredential]
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewTelegramIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &TelegramIntegrationCreator{
		credentialGetter:       managers.NewExecutorCredentialGetter[TelegramCredential](deps.ExecutorCredentialManager),
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *TelegramIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewTelegramIntegration(ctx, TelegramIntegrationDependencies{
		CredentialGetter:       c.credentialGetter,
		ParameterBinder:        c.binder,
		CredentialID:           p.CredentialID,
		WorkspaceID:            p.WorkspaceID,
		ExecutorStorageManager: c.executorStorageManager,
	})
}

type TelegramIntegration struct {
	bot *tgbotapi.BotAPI

	binder                 domain.IntegrationParameterBinder
	credentialGetter       domain.CredentialGetter[TelegramCredential]
	actionManager          *domain.IntegrationActionManager
	executorStorageManager domain.ExecutorStorageManager
	workspaceID            string
}

type TelegramCredential struct {
	BotToken string `json:"bot_token"`
}

type TelegramIntegrationDependencies struct {
	ParameterBinder        domain.IntegrationParameterBinder
	CredentialID           string
	CredentialGetter       domain.CredentialGetter[TelegramCredential]
	WorkspaceID            string
	ExecutorStorageManager domain.ExecutorStorageManager
}

func NewTelegramIntegration(ctx context.Context, deps TelegramIntegrationDependencies) (*TelegramIntegration, error) {
	integration := &TelegramIntegration{
		credentialGetter:       deps.CredentialGetter,
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
		workspaceID:            deps.WorkspaceID,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(TelegramActionType_SendTextMessage, integration.SendTextMessage).
		AddPerItem(TelegramActionType_GetChat, integration.GetChat).
		AddPerItemMulti(TelegramActionType_GetChatAdministrators, integration.GetChatAdministrators).
		AddPerItem(TelegramActionType_GetChatMember, integration.GetChatMember).
		AddPerItem(TelegramActionType_LeaveChat, integration.LeaveChat).
		AddPerItem(TelegramActionType_SetChatDescription, integration.SetChatDescription).
		AddPerItem(TelegramActionType_SetChatTitle, integration.SetChatTitle).
		AddPerItem(TelegramActionType_DeleteMessage, integration.DeleteMessage).
		AddPerItem(TelegramActionType_EditTextMessage, integration.EditTextMessage).
		AddPerItem(TelegramActionType_PinChatMessage, integration.PinChatMessage).
		AddPerItem(TelegramActionType_UnpinChatMessage, integration.UnpinChatMessage).
		AddPerItem(TelegramActionType_SendPhoto, integration.SendPhoto).
		AddPerItem(TelegramActionType_SendVideo, integration.SendVideo).
		AddPerItem(TelegramActionType_SendAudio, integration.SendAudio).
		AddPerItem(TelegramActionType_SendDocument, integration.SendDocument).
		AddPerItem(TelegramActionType_SendAnimation, integration.SendAnimation).
		AddPerItem(TelegramActionType_SendSticker, integration.SendSticker).
		AddPerItem(TelegramActionType_SendLocation, integration.SendLocation).
		AddPerItemMulti(TelegramActionType_SendMediaGroup, integration.SendMediaGroup)

	integration.actionManager = actionManager

	if deps.CredentialID == "" {
		return nil, fmt.Errorf("credential ID is required for Telegram integration")
	}

	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get decrypted Telegram credential: %w", err)
	}

	bot, err := tgbotapi.NewBotAPI(credential.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot client: %w", err)
	}

	integration.bot = bot

	return integration, nil
}

func (i *TelegramIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *TelegramIntegration) parseChatID(chatIDStr string) (int64, error) {
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid chat_id: %w", err)
	}
	return chatID, nil
}

func (i *TelegramIntegration) parseMessageID(messageIDStr string) (int, error) {
	messageID, err := strconv.Atoi(messageIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid message_id: %w", err)
	}
	return messageID, nil
}

func (i *TelegramIntegration) getFileReader(ctx context.Context, fileItem domain.FileItem) (io.Reader, string, error) {
	executionFile, err := i.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: i.workspaceID,
		UploadID:    fileItem.FileID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file: %w", err)
	}
	return executionFile.Reader, fileItem.Name, nil
}

type SendTextMessageParams struct {
	ChatID  string `json:"chat_id"`
	Message string `json:"message"`
}

type SendTextMessageOutput struct {
	MessageID int    `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	Date      int    `json:"date"`
}

func (i *TelegramIntegration) SendTextMessage(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendTextMessageParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.Message) == "" {
		return nil, fmt.Errorf("message is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	msg := tgbotapi.NewMessage(chatID, p.Message)
	sentMessage, err := i.bot.Send(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	output := SendTextMessageOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Text:      sentMessage.Text,
		Date:      sentMessage.Date,
	}

	return output, nil
}

type EditTextMessageParams struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

func (i *TelegramIntegration) EditTextMessage(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := EditTextMessageParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.MessageID) == "" {
		return nil, fmt.Errorf("message_id is required")
	}

	if strings.TrimSpace(p.Text) == "" {
		return nil, fmt.Errorf("text is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	messageID, err := i.parseMessageID(p.MessageID)
	if err != nil {
		return nil, err
	}

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, p.Text)
	editedMessage, err := i.bot.Send(editMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to edit message: %w", err)
	}

	return editedMessage, nil
}

type DeleteMessageParams struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
}

type DeleteMessageOutput struct {
	Success   bool  `json:"success"`
	ChatID    int64 `json:"chat_id"`
	MessageID int   `json:"message_id"`
}

func (i *TelegramIntegration) DeleteMessage(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteMessageParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.MessageID) == "" {
		return nil, fmt.Errorf("message_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	messageID, err := i.parseMessageID(p.MessageID)
	if err != nil {
		return nil, err
	}

	deleteConfig := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err = i.bot.Request(deleteConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to delete message: %w", err)
	}

	output := DeleteMessageOutput{
		Success:   true,
		ChatID:    chatID,
		MessageID: messageID,
	}

	return output, nil
}

type PinChatMessageParams struct {
	ChatID              string `json:"chat_id"`
	MessageID           string `json:"message_id"`
	DisableNotification bool   `json:"disable_notification"`
}

type PinChatMessageOutput struct {
	Success   bool  `json:"success"`
	ChatID    int64 `json:"chat_id"`
	MessageID int   `json:"message_id"`
}

func (i *TelegramIntegration) PinChatMessage(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := PinChatMessageParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.MessageID) == "" {
		return nil, fmt.Errorf("message_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	messageID, err := i.parseMessageID(p.MessageID)
	if err != nil {
		return nil, err
	}

	pinConfig := tgbotapi.PinChatMessageConfig{
		ChatID:              chatID,
		MessageID:           messageID,
		DisableNotification: p.DisableNotification,
	}

	_, err = i.bot.Request(pinConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to pin message: %w", err)
	}

	output := PinChatMessageOutput{
		Success:   true,
		ChatID:    chatID,
		MessageID: messageID,
	}

	return output, nil
}

type UnpinChatMessageParams struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
}

type UnpinChatMessageOutput struct {
	Success bool  `json:"success"`
	ChatID  int64 `json:"chat_id"`
}

func (i *TelegramIntegration) UnpinChatMessage(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UnpinChatMessageParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.MessageID) == "" {
		unpinAllConfig := tgbotapi.UnpinAllChatMessagesConfig{
			ChatID: chatID,
		}
		_, err = i.bot.Request(unpinAllConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to unpin all messages: %w", err)
		}
	} else {
		messageID, err := i.parseMessageID(p.MessageID)
		if err != nil {
			return nil, err
		}

		unpinConfig := tgbotapi.UnpinChatMessageConfig{
			ChatID:    chatID,
			MessageID: messageID,
		}
		_, err = i.bot.Request(unpinConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to unpin message: %w", err)
		}
	}

	output := UnpinChatMessageOutput{
		Success: true,
		ChatID:  chatID,
	}

	return output, nil
}

type SendPhotoParams struct {
	ChatID  string          `json:"chat_id"`
	Photo   domain.FileItem `json:"photo"`
	Caption string          `json:"caption"`
}

type SendMediaOutput struct {
	MessageID int    `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	Date      int    `json:"date"`
	Caption   string `json:"caption,omitempty"`
}

func (i *TelegramIntegration) SendPhoto(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendPhotoParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	reader, fileName, err := i.getFileReader(ctx, p.Photo)
	if err != nil {
		return nil, err
	}

	photoConfig := tgbotapi.NewPhoto(chatID, tgbotapi.FileReader{
		Name:   fileName,
		Reader: reader,
	})
	photoConfig.Caption = p.Caption

	sentMessage, err := i.bot.Send(photoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send photo: %w", err)
	}

	output := SendMediaOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Date:      sentMessage.Date,
		Caption:   sentMessage.Caption,
	}

	return output, nil
}

type SendVideoParams struct {
	ChatID  string          `json:"chat_id"`
	Video   domain.FileItem `json:"video"`
	Caption string          `json:"caption"`
}

func (i *TelegramIntegration) SendVideo(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendVideoParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	reader, fileName, err := i.getFileReader(ctx, p.Video)
	if err != nil {
		return nil, err
	}

	videoConfig := tgbotapi.NewVideo(chatID, tgbotapi.FileReader{
		Name:   fileName,
		Reader: reader,
	})
	videoConfig.Caption = p.Caption

	sentMessage, err := i.bot.Send(videoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send video: %w", err)
	}

	output := SendMediaOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Date:      sentMessage.Date,
		Caption:   sentMessage.Caption,
	}

	return output, nil
}

type SendAudioParams struct {
	ChatID  string          `json:"chat_id"`
	Audio   domain.FileItem `json:"audio"`
	Caption string          `json:"caption"`
}

func (i *TelegramIntegration) SendAudio(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendAudioParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	reader, fileName, err := i.getFileReader(ctx, p.Audio)
	if err != nil {
		return nil, err
	}

	audioConfig := tgbotapi.NewAudio(chatID, tgbotapi.FileReader{
		Name:   fileName,
		Reader: reader,
	})
	audioConfig.Caption = p.Caption

	sentMessage, err := i.bot.Send(audioConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send audio: %w", err)
	}

	output := SendMediaOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Date:      sentMessage.Date,
		Caption:   sentMessage.Caption,
	}

	return output, nil
}

type SendDocumentParams struct {
	ChatID   string          `json:"chat_id"`
	Document domain.FileItem `json:"document"`
	Caption  string          `json:"caption"`
}

func (i *TelegramIntegration) SendDocument(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendDocumentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	reader, fileName, err := i.getFileReader(ctx, p.Document)
	if err != nil {
		return nil, err
	}

	docConfig := tgbotapi.NewDocument(chatID, tgbotapi.FileReader{
		Name:   fileName,
		Reader: reader,
	})
	docConfig.Caption = p.Caption

	sentMessage, err := i.bot.Send(docConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send document: %w", err)
	}

	output := SendMediaOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Date:      sentMessage.Date,
		Caption:   sentMessage.Caption,
	}

	return output, nil
}

type SendAnimationParams struct {
	ChatID    string          `json:"chat_id"`
	Animation domain.FileItem `json:"animation"`
	Caption   string          `json:"caption"`
}

func (i *TelegramIntegration) SendAnimation(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendAnimationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	reader, fileName, err := i.getFileReader(ctx, p.Animation)
	if err != nil {
		return nil, err
	}

	animConfig := tgbotapi.NewAnimation(chatID, tgbotapi.FileReader{
		Name:   fileName,
		Reader: reader,
	})
	animConfig.Caption = p.Caption

	sentMessage, err := i.bot.Send(animConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send animation: %w", err)
	}

	output := SendMediaOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Date:      sentMessage.Date,
		Caption:   sentMessage.Caption,
	}

	return output, nil
}

type SendStickerParams struct {
	ChatID  string          `json:"chat_id"`
	Sticker domain.FileItem `json:"sticker"`
}

func (i *TelegramIntegration) SendSticker(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendStickerParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	reader, fileName, err := i.getFileReader(ctx, p.Sticker)
	if err != nil {
		return nil, err
	}

	stickerConfig := tgbotapi.NewSticker(chatID, tgbotapi.FileReader{
		Name:   fileName,
		Reader: reader,
	})

	sentMessage, err := i.bot.Send(stickerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send sticker: %w", err)
	}

	output := SendMediaOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Date:      sentMessage.Date,
	}

	return output, nil
}

type SendLocationParams struct {
	ChatID    string `json:"chat_id"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

type SendLocationOutput struct {
	MessageID int     `json:"message_id"`
	ChatID    int64   `json:"chat_id"`
	Date      int     `json:"date"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (i *TelegramIntegration) SendLocation(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendLocationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.Latitude) == "" {
		return nil, fmt.Errorf("latitude is required")
	}

	if strings.TrimSpace(p.Longitude) == "" {
		return nil, fmt.Errorf("longitude is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	latitude, err := strconv.ParseFloat(p.Latitude, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid latitude: %w", err)
	}

	longitude, err := strconv.ParseFloat(p.Longitude, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid longitude: %w", err)
	}

	locationConfig := tgbotapi.NewLocation(chatID, latitude, longitude)

	sentMessage, err := i.bot.Send(locationConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send location: %w", err)
	}

	output := SendLocationOutput{
		MessageID: sentMessage.MessageID,
		ChatID:    sentMessage.Chat.ID,
		Date:      sentMessage.Date,
		Latitude:  latitude,
		Longitude: longitude,
	}

	return output, nil
}

type SendMediaGroupParams struct {
	ChatID    string `json:"chat_id"`
	MediaURLs string `json:"media_urls"`
}

func (i *TelegramIntegration) SendMediaGroup(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := SendMediaGroupParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.MediaURLs) == "" {
		return nil, fmt.Errorf("media_urls is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	urls := strings.Split(p.MediaURLs, ",")
	var media []interface{}
	for _, url := range urls {
		trimmedURL := strings.TrimSpace(url)
		if trimmedURL != "" {
			media = append(media, tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL(trimmedURL)))
		}
	}

	if len(media) < 2 {
		return nil, fmt.Errorf("at least 2 media items are required for a media group")
	}

	mediaGroupConfig := tgbotapi.NewMediaGroup(chatID, media)

	messages, err := i.bot.SendMediaGroup(mediaGroupConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to send media group: %w", err)
	}

	var items []domain.Item
	for _, msg := range messages {
		items = append(items, msg)
	}

	return items, nil
}

type GetChatParams struct {
	ChatID string `json:"chat_id"`
}

func (i *TelegramIntegration) GetChat(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetChatParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	chatConfig := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}

	chat, err := i.bot.GetChat(chatConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	return chat, nil
}

type GetChatAdministratorsParams struct {
	ChatID string `json:"chat_id"`
}

func (i *TelegramIntegration) GetChatAdministrators(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetChatAdministratorsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	chatConfig := tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}

	admins, err := i.bot.GetChatAdministrators(chatConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat administrators: %w", err)
	}

	var items []domain.Item
	for _, admin := range admins {
		items = append(items, admin)
	}

	return items, nil
}

type GetChatMemberParams struct {
	ChatID string `json:"chat_id"`
	UserID string `json:"user_id"`
}

func (i *TelegramIntegration) GetChatMember(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetChatMemberParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.UserID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	userID, err := strconv.ParseInt(p.UserID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	memberConfig := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	}

	member, err := i.bot.GetChatMember(memberConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat member: %w", err)
	}

	return member, nil
}

type LeaveChatParams struct {
	ChatID string `json:"chat_id"`
}

type LeaveChatOutput struct {
	Success bool  `json:"success"`
	ChatID  int64 `json:"chat_id"`
}

func (i *TelegramIntegration) LeaveChat(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := LeaveChatParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	leaveConfig := tgbotapi.LeaveChatConfig{
		ChatID: chatID,
	}

	_, err = i.bot.Request(leaveConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to leave chat: %w", err)
	}

	output := LeaveChatOutput{
		Success: true,
		ChatID:  chatID,
	}

	return output, nil
}

type SetChatDescriptionParams struct {
	ChatID      string `json:"chat_id"`
	Description string `json:"description"`
}

type SetChatDescriptionOutput struct {
	Success     bool   `json:"success"`
	ChatID      int64  `json:"chat_id"`
	Description string `json:"description"`
}

func (i *TelegramIntegration) SetChatDescription(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SetChatDescriptionParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	descConfig := tgbotapi.SetChatDescriptionConfig{
		ChatID:      chatID,
		Description: p.Description,
	}

	_, err = i.bot.Request(descConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to set chat description: %w", err)
	}

	output := SetChatDescriptionOutput{
		Success:     true,
		ChatID:      chatID,
		Description: p.Description,
	}

	return output, nil
}

type SetChatTitleParams struct {
	ChatID string `json:"chat_id"`
	Title  string `json:"title"`
}

type SetChatTitleOutput struct {
	Success bool   `json:"success"`
	ChatID  int64  `json:"chat_id"`
	Title   string `json:"title"`
}

func (i *TelegramIntegration) SetChatTitle(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SetChatTitleParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChatID) == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(p.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}

	chatID, err := i.parseChatID(p.ChatID)
	if err != nil {
		return nil, err
	}

	titleConfig := tgbotapi.SetChatTitleConfig{
		ChatID: chatID,
		Title:  p.Title,
	}

	_, err = i.bot.Request(titleConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to set chat title: %w", err)
	}

	output := SetChatTitleOutput{
		Success: true,
		ChatID:  chatID,
		Title:   p.Title,
	}

	return output, nil
}
