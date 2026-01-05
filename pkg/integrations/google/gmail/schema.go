package gmail

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Gmail,
		Name:        "Gmail",
		Description: "Use Gmail's APIs to interact with your email",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:                "oauth_provider",
				Name:               "OAuth Provider",
				Description:        "The oauth provider for the Gmail integration",
				Required:           false,
				Type:               domain.NodePropertyType_OAuth,
				OAuthType:          domain.OAuthTypeGoogle,
				IsApplicableToHTTP: true,
				IsCustomOAuthable:  true,
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:                            "on_message_received",
				Name:                          "On Message Received",
				EventType:                     IntegrationTriggerType_OnMessageReceived,
				Description:                   "Triggered when a new message is received",
				IsNonAvailableForDefaultOAuth: true,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "send_mail",
				Name:        "Send Mail",
				ActionType:  IntegrationActionType_SendMail,
				Description: "Send an email using your Gmail account",
				Properties: []domain.NodeProperty{
					{
						Key:         "to",
						Name:        "To",
						Description: "The email address of the recipient",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
						},
						RegexKey: "email",
					},
					{
						Key:         "cc",
						Name:        "Cc",
						Description: "The email address of the recipient to CC",
						Required:    false,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
						},
						RegexKey: "email",
					},
					{
						Key:         "bcc",
						Name:        "Bcc",
						Description: "The email address of the recipient to BCC",
						Required:    false,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
						},
						RegexKey: "email",
					},
					{
						Key:         "subject",
						Name:        "Subject",
						Description: "The subject of the email",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "The body of the email",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:                            "find_mail",
				Name:                          "Find Mail",
				ActionType:                    IntegrationActionType_FindMail,
				Description:                   "Find an email in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "query",
						Name:        "Query",
						Description: "The search query to use to find the email",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     1,
							Max:     500,
							Default: 10,
							Step:    1,
						},
					},
				},
			},
			{
				ID:                            "reply_mail",
				Name:                          "Reply Mail",
				ActionType:                    IntegrationActionType_ReplyMail,
				Description:                   "Reply to an email using your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to reply to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "The body of the email",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:                            "get_mail",
				Name:                          "Get Mail",
				ActionType:                    IntegrationActionType_GetMail,
				Description:                   "Get an email from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to get",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "get_mails",
				Name:                          "Get Mails",
				ActionType:                    IntegrationActionType_GetMails,
				Description:                   "Get emails from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     1,
							Max:     500,
							Default: 10,
							Step:    1,
						},
					},
				},
			},
			{
				ID:                            "send_mail_to_trash",
				Name:                          "Send Mail to Trash",
				ActionType:                    IntegrationActionType_SendMailToTrash,
				Description:                   "Send an email to trash from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to send to trash",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "send_mails_to_trash",
				Name:                          "Send Mails to Trash",
				ActionType:                    IntegrationActionType_SendMailsToTrash,
				Description:                   "Send emails to trash from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     1,
							Max:     500,
							Default: 10,
							Step:    1,
						},
					},
				},
			},
			{
				ID:                            "delete_mail",
				Name:                          "Delete Mail",
				ActionType:                    IntegrationActionType_DeleteMail,
				Description:                   "Delete an email from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to delete",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "delete_mails",
				Name:                          "Delete Mails",
				ActionType:                    IntegrationActionType_DeleteMails,
				Description:                   "Delete emails from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:                            "get_thread",
				Name:                          "Get Thread",
				ActionType:                    IntegrationActionType_GetThread,
				Description:                   "Get a thread from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the thread to get",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "get_threads",
				Name:                          "Get Threads",
				ActionType:                    IntegrationActionType_GetThreads,
				Description:                   "Get threads from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:                            "mark_mail_as_read",
				Name:                          "Mark Mail as Read",
				ActionType:                    IntegrationActionType_MarkMailAsRead,
				Description:                   "Mark an email as read in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to mark as read",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "mark_mail_as_unread",
				Name:                          "Mark Mail as Unread",
				ActionType:                    IntegrationActionType_MarkMailAsUnread,
				Description:                   "Mark an email as unread in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to mark as unread",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "mark_mails_as_read",
				Name:                          "Mark Mails as Read",
				ActionType:                    IntegrationActionType_MarkMailsAsRead,
				Description:                   "Mark emails as read in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:                            "mark_mails_as_unread",
				Name:                          "Mark Mails as Unread",
				ActionType:                    IntegrationActionType_MarkMailsAsUnread,
				Description:                   "Mark emails as unread in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:                            "create_draft",
				Name:                          "Create Draft",
				ActionType:                    IntegrationActionType_CreateDraft,
				Description:                   "Create a draft email in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "to",
						Name:        "To",
						Description: "The email address of the recipient",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
						},
					},
					{
						Key:         "cc",
						Name:        "Cc",
						Description: "The email address of the recipient to CC",
						Required:    false,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
						},
					},
					{
						Key:         "bcc",
						Name:        "Bcc",
						Description: "The email address of the recipient to BCC",
						Required:    false,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
						},
					},
					{
						Key:         "subject",
						Name:        "Subject",
						Description: "The subject of the email",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "The body of the email",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:                            "send_draft",
				Name:                          "Send Draft",
				ActionType:                    IntegrationActionType_SendDraft,
				Description:                   "Send a draft email from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "draft_id",
						Name:        "Draft ID",
						Description: "The draft ID of the email to send",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "get_draft",
				Name:                          "Get Draft",
				ActionType:                    IntegrationActionType_GetDraft,
				Description:                   "Get a draft email from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to get",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "get_drafts",
				Name:                          "Get Drafts",
				ActionType:                    IntegrationActionType_GetDrafts,
				Description:                   "Get drafts from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:                            "delete_draft",
				Name:                          "Delete Draft",
				ActionType:                    IntegrationActionType_DeleteDraft,
				Description:                   "Delete a draft email from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "draft_id",
						Name:        "Draft ID",
						Description: "The draft ID of the email to delete",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "delete_drafts",
				Name:                          "Delete Drafts",
				ActionType:                    IntegrationActionType_DeleteDrafts,
				Description:                   "Delete drafts from your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "get_labels",
				Name:        "Get Labels",
				ActionType:  IntegrationActionType_GetLabels,
				Description: "Get labels from your Gmail account",
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "add_label",
				Name:        "Add Label",
				ActionType:  IntegrationActionType_AddLabel,
				Description: "Add a label to your Gmail account",
				Properties: []domain.NodeProperty{
					{
						Key:         "label_name",
						Name:        "Label Name",
						Description: "The name of the label to add",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "remove_label",
				Name:        "Remove Label",
				ActionType:  IntegrationActionType_RemoveLabel,
				Description: "Remove a label from your Gmail account",
				Properties: []domain.NodeProperty{
					{
						Key:         "label_id",
						Name:        "Label ID",
						Description: "The ID of the label to remove",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "add_label_to_email",
				Name:                          "Add Label to Email",
				ActionType:                    IntegrationActionType_AddLabelToEmail,
				Description:                   "Add a label to an email in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to add the label to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "label_id",
						Name:        "Label ID",
						Description: "The ID of the label to add to the email",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:                            "remove_label_from_email",
				Name:                          "Remove Label from Email",
				ActionType:                    IntegrationActionType_RemoveLabelFromEmail,
				Description:                   "Remove a label from an email in your Gmail account",
				IsNonAvailableForDefaultOAuth: true,
				Properties: []domain.NodeProperty{
					{
						Key:         "message_id",
						Name:        "Message ID",
						Description: "The message ID of the email to remove the label from",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "label_id",
						Name:        "Label ID",
						Description: "The ID of the label to remove from the email",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
	}
)
