package telegram

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

// Trigger Event Types
const (
	TelegramEventType_Message           domain.IntegrationTriggerEventType = "telegram_message"
	TelegramEventType_EditedMessage     domain.IntegrationTriggerEventType = "telegram_edited_message"
	TelegramEventType_ChannelPost       domain.IntegrationTriggerEventType = "telegram_channel_post"
	TelegramEventType_EditedChannelPost domain.IntegrationTriggerEventType = "telegram_edited_channel_post"
)

// Action Types
const (
	TelegramActionType_SendTextMessage       domain.IntegrationActionType = "send_text_message"
	TelegramActionType_GetChat               domain.IntegrationActionType = "get_chat"
	TelegramActionType_GetChatAdministrators domain.IntegrationActionType = "get_chat_administrators"
	TelegramActionType_GetChatMember         domain.IntegrationActionType = "get_chat_member"
	TelegramActionType_LeaveChat             domain.IntegrationActionType = "leave_chat"
	TelegramActionType_SetChatDescription    domain.IntegrationActionType = "set_chat_description"
	TelegramActionType_SetChatTitle          domain.IntegrationActionType = "set_chat_title"
	TelegramActionType_DeleteMessage         domain.IntegrationActionType = "delete_message"
	TelegramActionType_EditTextMessage       domain.IntegrationActionType = "edit_text_message"
	TelegramActionType_PinChatMessage        domain.IntegrationActionType = "pin_chat_message"
	TelegramActionType_UnpinChatMessage      domain.IntegrationActionType = "unpin_chat_message"
	TelegramActionType_SendPhoto             domain.IntegrationActionType = "send_photo"
	TelegramActionType_SendVideo             domain.IntegrationActionType = "send_video"
	TelegramActionType_SendAudio             domain.IntegrationActionType = "send_audio"
	TelegramActionType_SendDocument          domain.IntegrationActionType = "send_document"
	TelegramActionType_SendAnimation         domain.IntegrationActionType = "send_animation"
	TelegramActionType_SendSticker           domain.IntegrationActionType = "send_sticker"
	TelegramActionType_SendLocation          domain.IntegrationActionType = "send_location"
	TelegramActionType_SendMediaGroup        domain.IntegrationActionType = "send_media_group"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Telegram,
		Name:        "Telegram",
		Description: "Send messages and manage chats via Telegram Bot API.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "bot_token",
				Name:        "Bot Token",
				Description: "Telegram Bot API token from @BotFather",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "send_text_message",
				Name:        "Send Text Message",
				Description: "Send a text message to a Telegram chat",
				ActionType:  TelegramActionType_SendTextMessage,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the message to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message",
						Name:        "Message",
						Description: "The text message to send",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "edit_text_message",
				Name:        "Edit Text Message",
				Description: "Edit a text message in a chat",
				ActionType:  TelegramActionType_EditTextMessage,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID where the message is",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The ID of the message to edit",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "text",
						Name:        "New Text",
						Description: "The new text for the message",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "delete_message",
				Name:        "Delete Message",
				Description: "Delete a message from a chat",
				ActionType:  TelegramActionType_DeleteMessage,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID where the message is",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The ID of the message to delete",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "pin_chat_message",
				Name:        "Pin Chat Message",
				Description: "Pin a message in a chat",
				ActionType:  TelegramActionType_PinChatMessage,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID where the message is",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The ID of the message to pin",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "disable_notification",
						Name:        "Disable Notification",
						Description: "Disable notification for pinning",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
			{
				ID:          "unpin_chat_message",
				Name:        "Unpin Chat Message",
				Description: "Unpin a message in a chat",
				ActionType:  TelegramActionType_UnpinChatMessage,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID where the message is",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The ID of the message to unpin (leave empty to unpin all)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "send_photo",
				Name:        "Send Photo",
				Description: "Send a photo to a chat",
				ActionType:  TelegramActionType_SendPhoto,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the photo to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "photo",
						Name:        "Photo",
						Description: "The photo file to send",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
					{
						Key:         "caption",
						Name:        "Caption",
						Description: "Photo caption",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "send_video",
				Name:        "Send Video",
				Description: "Send a video to a chat",
				ActionType:  TelegramActionType_SendVideo,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the video to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "video",
						Name:        "Video",
						Description: "The video file to send",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
					{
						Key:         "caption",
						Name:        "Caption",
						Description: "Video caption",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "send_audio",
				Name:        "Send Audio",
				Description: "Send an audio file to a chat",
				ActionType:  TelegramActionType_SendAudio,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the audio to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "audio",
						Name:        "Audio",
						Description: "The audio file to send",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
					{
						Key:         "caption",
						Name:        "Caption",
						Description: "Audio caption",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "send_document",
				Name:        "Send Document",
				Description: "Send a document to a chat",
				ActionType:  TelegramActionType_SendDocument,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the document to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "document",
						Name:        "Document",
						Description: "The document file to send",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
					{
						Key:         "caption",
						Name:        "Caption",
						Description: "Document caption",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "send_animation",
				Name:        "Send Animation",
				Description: "Send an animated file (GIF) to a chat",
				ActionType:  TelegramActionType_SendAnimation,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the animation to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "animation",
						Name:        "Animation",
						Description: "The animation file to send (GIF or MP4)",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
					{
						Key:         "caption",
						Name:        "Caption",
						Description: "Animation caption",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "send_sticker",
				Name:        "Send Sticker",
				Description: "Send a sticker to a chat",
				ActionType:  TelegramActionType_SendSticker,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the sticker to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "sticker",
						Name:        "Sticker",
						Description: "The sticker file to send (WEBP, TGS, or WEBM)",
						Required:    true,
						Type:        domain.NodePropertyType_File,
					},
				},
			},
			{
				ID:          "send_location",
				Name:        "Send Location",
				Description: "Send a location to a chat",
				ActionType:  TelegramActionType_SendLocation,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the location to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "latitude",
						Name:        "Latitude",
						Description: "Latitude of the location",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "longitude",
						Name:        "Longitude",
						Description: "Longitude of the location",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "send_media_group",
				Name:        "Send Media Group",
				Description: "Send a group of photos or videos as an album",
				ActionType:  TelegramActionType_SendMediaGroup,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to send the media group to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "media_urls",
						Name:        "Media URLs",
						Description: "Comma-separated list of photo/video URLs to send as album",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "get_chat",
				Name:        "Get Chat",
				Description: "Get information about a chat",
				ActionType:  TelegramActionType_GetChat,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to get information about",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_chat_administrators",
				Name:        "Get Chat Administrators",
				Description: "Get a list of administrators in a chat",
				ActionType:  TelegramActionType_GetChatAdministrators,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to get administrators from",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_chat_member",
				Name:        "Get Chat Member",
				Description: "Get information about a member of a chat",
				ActionType:  TelegramActionType_GetChatMember,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to get the member from",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "user_id",
						Name:        "User ID",
						Description: "The user ID of the member to get information about",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "leave_chat",
				Name:        "Leave Chat",
				Description: "Leave a group, supergroup, or channel",
				ActionType:  TelegramActionType_LeaveChat,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to leave",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "set_chat_description",
				Name:        "Set Chat Description",
				Description: "Set the description of a group, supergroup, or channel",
				ActionType:  TelegramActionType_SetChatDescription,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to set the description for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "New chat description (0-255 characters)",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "set_chat_title",
				Name:        "Set Chat Title",
				Description: "Set the title of a group, supergroup, or channel",
				ActionType:  TelegramActionType_SetChatTitle,
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID",
						Description: "The chat ID to set the title for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "title",
						Name:        "Title",
						Description: "New chat title (1-128 characters)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "telegram_message",
				Name:        "On Message",
				EventType:   TelegramEventType_Message,
				Description: "Triggered when a new message is received by the bot",
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID (Optional)",
						Description: "Filter messages from a specific chat. Leave empty to receive from all chats.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "telegram_edited_message",
				Name:        "On Edited Message",
				EventType:   TelegramEventType_EditedMessage,
				Description: "Triggered when a message is edited",
				Properties: []domain.NodeProperty{
					{
						Key:         "chat_id",
						Name:        "Chat ID (Optional)",
						Description: "Filter messages from a specific chat. Leave empty to receive from all chats.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "telegram_channel_post",
				Name:        "On Channel Post",
				EventType:   TelegramEventType_ChannelPost,
				Description: "Triggered when a new post is published in a channel where the bot is a member",
				Properties: []domain.NodeProperty{
					{
						Key:         "channel_id",
						Name:        "Channel ID (Optional)",
						Description: "Filter posts from a specific channel. Leave empty to receive from all channels.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "telegram_edited_channel_post",
				Name:        "On Edited Channel Post",
				EventType:   TelegramEventType_EditedChannelPost,
				Description: "Triggered when a channel post is edited",
				Properties: []domain.NodeProperty{
					{
						Key:         "channel_id",
						Name:        "Channel ID (Optional)",
						Description: "Filter posts from a specific channel. Leave empty to receive from all channels.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
	}
)
