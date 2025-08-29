package resendintegration

import (
	"flowbaker/internal/domain"
)

var (
	ResendSchema = domain.Integration{
		ID:          domain.IntegrationType_Resend,
		Name:        "Resend",
		Description: "Send emails and manage audiences with Resend.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "api_key",
				Name:        "API Key",
				Description: "The Resend API key for authentication",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "create_contact",
				Name:        "Create Contact",
				Description: "Create a new contact in a Resend audience",
				ActionType:  ResendIntegrationActionType_CreateContact,
				Properties: []domain.NodeProperty{
					{
						Key:          "audience_id",
						Name:         "Audience",
						Description:  "The audience to add the contact to",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: ResendIntegrationPeekable_Audiences,
					},
					{
						Key:         "email",
						Name:        "Email",
						Description: "The email address of the contact",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "first_name",
						Name:        "First Name",
						Description: "The first name of the contact",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "last_name",
						Name:        "Last Name",
						Description: "The last name of the contact",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "unsubscribed",
						Name:        "Unsubscribed",
						Description: "Whether the contact is unsubscribed",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
			{
				ID:          "send_email",
				Name:        "Send Email",
				Description: "Send an email using Resend",
				ActionType:  ResendIntegrationActionType_SendEmail,
				Properties: []domain.NodeProperty{
					{
						Key:         "from",
						Name:        "From",
						Description: "The sender email address",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "to",
						Name:        "To",
						Description: "The recipient email addresses",
						Required:    true,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "subject",
						Name:        "Subject",
						Description: "The email subject",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "html",
						Name:        "HTML Content",
						Description: "The HTML content of the email",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "text",
						Name:        "Text Content",
						Description: "The plain text content of the email",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "reply_to",
						Name:        "Reply To",
						Description: "The reply-to email address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "cc",
						Name:        "CC",
						Description: "CC email addresses",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "bcc",
						Name:        "BCC",
						Description: "BCC email addresses",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "tags",
						Name:        "Tags",
						Description: "Email tags",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{},
	}
)
