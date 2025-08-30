package sendresponse

import (
	"github.com/flowbaker/flowbaker/internal/domain"
)

const (
	IntegrationActionType_SendResponse = "send_response"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_SendResponse,
		Name:                 "Send Response",
		Description:          "Use Send Response integration to send a response to a webhook.",
		CredentialProperties: []domain.NodeProperty{},
		Actions: []domain.IntegrationAction{
			{
				ID:          "send_response",
				Name:        "Send Response",
				ActionType:  IntegrationActionType_SendResponse,
				Description: "Send a response to a webhook",
				Properties: []domain.NodeProperty{
					{
						Key:         "response_type",
						Name:        "Response",
						Description: "The response to send",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{
								Label:       "All Input Items",
								Description: "Send all input items as a JSON response",
								Value:       "all_input_items",
							},
							{
								Label:       "Text",
								Description: "Send a text response",
								Value:       "text",
							},
							{
								Label:       "JSON",
								Description: "Send a JSON response",
								Value:       "json",
							},
							{
								Label:       "HTML",
								Description: "Send an HTML response",
								Value:       "html",
							},
							{
								Label:       "Empty",
								Description: "Send an empty response",
								Value:       "empty",
							},
						},
					},
					{
						Key:         "json",
						Name:        "JSON",
						Description: "The JSON to send",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						DependsOn: &domain.DependsOn{
							PropertyKey: "response_type",
							Value:       "json",
						},
					},
					{
						Key:         "text",
						Name:        "Text",
						Description: "The text to send",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						DependsOn: &domain.DependsOn{
							PropertyKey: "response_type",
							Value:       "text",
						},
					},
					{
						Key:         "html",
						Name:        "HTML",
						Description: "The HTML to send",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						DependsOn: &domain.DependsOn{
							PropertyKey: "response_type",
							Value:       "html",
						},
					},
				},
			},
		},
	}
)
