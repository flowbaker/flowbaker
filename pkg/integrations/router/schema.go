package router

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_Route domain.IntegrationActionType = "route"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Router,
		Name:                 "Router",
		Description:          "Route data to different paths based on AI classification.",
		CanTestConnection:    false,
		IsCredentialOptional: true,
		CredentialProperties: []domain.NodeProperty{},
		Triggers:             []domain.IntegrationTrigger{},
		Actions: []domain.IntegrationAction{
			{
				ID:          "route",
				Name:        "Route",
				ActionType:  IntegrationActionType_Route,
				Description: "Route data to different paths based on AI classification.",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "content",
						Name:        "Content",
						Description: "The content to route through the router.",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:              "classifications",
						Name:             "Classifications",
						Description:      "The classifications of the data.",
						Required:         true,
						Type:             domain.NodePropertyType_Array,
						GeneratesHandles: true,
						HandleGenerationOpts: &domain.HandleGenerationOptions{
							HandleType:        "output",
							NameFromProperty:  "Name",
							DefaultHandleType: domain.NodeHandleTypeDefault,
							Position:          domain.NodeHandlePositionBottom,
						},
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "Name",
									Name:        "Name",
									Description: "The name of the classification.",
									Type:        domain.NodePropertyType_String,
								},
								{
									Key:         "Description",
									Name:        "Description",
									Description: "The description of the classification.",
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
		},
	}
)
