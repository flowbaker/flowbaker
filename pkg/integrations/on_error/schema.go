package onerror

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationTriggerType_OnError domain.IntegrationTriggerEventType = "on_error"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_OnError,
		Name:                 "On Error",
		Description:          "Triggered when an error occurs in the workflow",
		CredentialProperties: []domain.NodeProperty{},
		IsCredentialOptional: true,
		CanTestConnection:    false,
		Actions:              []domain.IntegrationAction{},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "on_error",
				Name:        "On Error",
				EventType:   IntegrationTriggerType_OnError,
				Description: "Triggered when an error occurs in the workflow",
				Properties:  []domain.NodeProperty{},
				OutputHandles: []domain.NodeHandle{
					{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionBottom, Text: "Error"},
				},
			},
		},
	}
)
